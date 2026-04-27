package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/portfolio/silence-map/internal/domain"
	"github.com/portfolio/silence-map/internal/usecase"
)

type PostgresReportRepository struct {
	db *sql.DB
}

func NewPostgresReportRepository(db *sql.DB) *PostgresReportRepository {
	return &PostgresReportRepository{db: db}
}

func (r *PostgresReportRepository) CreateReport(ctx context.Context, report *domain.Report) error {
	const query = `
		INSERT INTO reports (
			id,
			user_id,
			location,
			quietness_level,
			place_name,
			created_at
		)
		VALUES (
			$1,
			$2,
			ST_SetSRID(ST_MakePoint($3, $4), 4326)::geography,
			$5,
			NULLIF($6, ''),
			$7
		)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		report.ID,
		report.UserID,
		report.Location.Longitude,
		report.Location.Latitude,
		report.QuietnessLevel,
		report.PlaceName,
		report.CreatedAt,
	)
	return mapPostgresError(err)
}

func (r *PostgresReportRepository) ConfirmReport(ctx context.Context, reportID, userID string) (*domain.Report, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var ownerID string
	if err := tx.QueryRowContext(ctx, `SELECT user_id FROM reports WHERE id = $1`, reportID).Scan(&ownerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, mapPostgresError(err)
	}

	if ownerID == userID {
		return nil, domain.ErrSelfConfirmation
	}

	const insertConfirmation = `
		INSERT INTO confirmations (id, report_id, user_id)
		VALUES ($1, $2, $3)
	`
	if _, err := tx.ExecContext(ctx, insertConfirmation, uuid.NewString(), reportID, userID); err != nil {
		return nil, mapPostgresError(err)
	}

	report, err := scanReport(tx.QueryRowContext(ctx, selectReportByIDSQL, reportID))
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &report, nil
}

func (r *PostgresReportRepository) ListRecentReports(ctx context.Context, latitude, longitude, radiusMeters float64, limit int) ([]domain.Report, error) {
	const query = `
		SELECT
			r.id,
			r.user_id,
			ST_Y(r.location::geometry) AS latitude,
			ST_X(r.location::geometry) AS longitude,
			r.quietness_level,
			r.place_name,
			r.created_at,
			COUNT(c.id)::int AS confirmation_count,
			ST_AsGeoJSON(r.location::geometry) AS location_geojson,
			((1 + COUNT(c.id))::float * temporal_decay(r.created_at)) AS weight
		FROM reports r
		LEFT JOIN confirmations c ON c.report_id = r.id
		WHERE
			r.created_at >= NOW() - INTERVAL '2 hours'
			AND ST_DWithin(
				r.location,
				ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography,
				$3
			)
		GROUP BY r.id
		ORDER BY r.created_at DESC
		LIMIT $4
	`

	rows, err := r.db.QueryContext(ctx, query, latitude, longitude, radiusMeters, limit)
	if err != nil {
		return nil, mapPostgresError(err)
	}
	defer rows.Close()

	reports := make([]domain.Report, 0)
	for rows.Next() {
		report, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return reports, nil
}

func (r *PostgresReportRepository) FindQuietPlaces(ctx context.Context, query usecase.QuietPlaceQuery) ([]domain.QuietPlace, error) {
	const sqlQuery = `
		WITH report_scores AS (
			SELECT
				r.id,
				COALESCE(NULLIF(r.place_name, ''), 'Local sem nome') AS place_name,
				ST_SnapToGrid(r.location::geometry, 0.001) AS grid_point,
				r.created_at,
				r.quietness_level::float AS quietness,
				COUNT(c.id)::int AS confirmation_count,
				(1 + COUNT(c.id))::float AS collaboration_weight,
				temporal_decay(r.created_at) AS freshness_decay,
				(
					CASE
						WHEN EXTRACT(ISODOW FROM r.created_at AT TIME ZONE $6)::int = $4 THEN 1.0
						ELSE 0.65
					END
					*
					GREATEST(
						0.25,
						1.0 - (
							LEAST(
								ABS(EXTRACT(HOUR FROM r.created_at AT TIME ZONE $6)::int - $5),
								24 - ABS(EXTRACT(HOUR FROM r.created_at AT TIME ZONE $6)::int - $5)
							)::float / 12.0
						)
					)
				) AS time_affinity
			FROM reports r
			LEFT JOIN confirmations c ON c.report_id = r.id
			WHERE
				r.created_at >= NOW() - INTERVAL '24 hours'
				AND ST_DWithin(
					r.location,
					ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography,
					$3
				)
			GROUP BY r.id
		),
		place_scores AS (
			SELECT
				place_name,
				grid_point,
				SUM(quietness * collaboration_weight * freshness_decay * time_affinity) AS weighted_quietness_sum,
				SUM(collaboration_weight * freshness_decay * time_affinity) AS total_weight,
				COUNT(*)::int AS report_count,
				SUM(confirmation_count)::int AS confirmation_count,
				MAX(created_at) AS last_report_at
			FROM report_scores
			GROUP BY place_name, grid_point
			HAVING SUM(collaboration_weight * freshness_decay * time_affinity) > 0
		)
		SELECT
			place_name,
			ST_Y(ST_Centroid(grid_point)) AS latitude,
			ST_X(ST_Centroid(grid_point)) AS longitude,
			(weighted_quietness_sum / total_weight) AS average_quietness,
			(weighted_quietness_sum / total_weight) * LN(1 + total_weight) AS weighted_score,
			report_count,
			confirmation_count,
			last_report_at,
			LEAST(1.0, LN(1 + total_weight) / 3.0) AS confidence
		FROM place_scores
		ORDER BY weighted_score DESC, average_quietness DESC, confidence DESC
		LIMIT $7
	`

	rows, err := r.db.QueryContext(
		ctx,
		sqlQuery,
		query.Latitude,
		query.Longitude,
		query.RadiusMeters,
		query.DayOfWeek,
		query.Hour,
		query.TimeZone,
		query.Limit,
	)
	if err != nil {
		return nil, mapPostgresError(err)
	}
	defer rows.Close()

	places := make([]domain.QuietPlace, 0)
	for rows.Next() {
		var place domain.QuietPlace
		if err := rows.Scan(
			&place.PlaceName,
			&place.Location.Latitude,
			&place.Location.Longitude,
			&place.AverageQuietness,
			&place.WeightedScore,
			&place.ReportCount,
			&place.ConfirmationCount,
			&place.LastReportAt,
			&place.Confidence,
		); err != nil {
			return nil, err
		}
		places = append(places, place)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return places, nil
}

const selectReportByIDSQL = `
	SELECT
		r.id,
		r.user_id,
		ST_Y(r.location::geometry) AS latitude,
		ST_X(r.location::geometry) AS longitude,
		r.quietness_level,
		r.place_name,
		r.created_at,
		COUNT(c.id)::int AS confirmation_count,
		ST_AsGeoJSON(r.location::geometry) AS location_geojson,
		((1 + COUNT(c.id))::float * temporal_decay(r.created_at)) AS weight
	FROM reports r
	LEFT JOIN confirmations c ON c.report_id = r.id
	WHERE r.id = $1
	GROUP BY r.id
`

type scanner interface {
	Scan(dest ...any) error
}

func scanReport(row scanner) (domain.Report, error) {
	var report domain.Report
	var placeName sql.NullString
	var geoJSON string
	var weight sql.NullFloat64

	if err := row.Scan(
		&report.ID,
		&report.UserID,
		&report.Location.Latitude,
		&report.Location.Longitude,
		&report.QuietnessLevel,
		&placeName,
		&report.CreatedAt,
		&report.ConfirmationCount,
		&geoJSON,
		&weight,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Report{}, domain.ErrNotFound
		}
		return domain.Report{}, mapPostgresError(err)
	}

	if placeName.Valid {
		report.PlaceName = placeName.String
	}
	if weight.Valid {
		report.Weight = weight.Float64
	}

	return report, nil
}

func mapPostgresError(err error) error {
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return domain.ErrDuplicateConfirmation
		case "23503":
			return domain.ErrNotFound
		case "22P02":
			return domain.NewValidationError("id", "must be a valid UUID")
		}
	}

	return err
}
