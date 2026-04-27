package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/portfolio/silence-map/internal/domain"
)

type ReportRepository interface {
	CreateReport(ctx context.Context, report *domain.Report) error
	ConfirmReport(ctx context.Context, reportID, userID string) (*domain.Report, error)
	ListRecentReports(ctx context.Context, latitude, longitude, radiusMeters float64, limit int) ([]domain.Report, error)
	FindQuietPlaces(ctx context.Context, query QuietPlaceQuery) ([]domain.QuietPlace, error)
}

type EventPublisher interface {
	PublishReport(ctx context.Context, report domain.Report)
	PublishConfirmation(ctx context.Context, report domain.Report)
}

type ReportUseCase struct {
	repo      ReportRepository
	publisher EventPublisher
	timeZone  string
}

type CreateReportInput struct {
	UserID         string
	Latitude       float64
	Longitude      float64
	QuietnessLevel int
	PlaceName      string
}

type QuietPlaceQuery struct {
	Latitude     float64
	Longitude    float64
	RadiusMeters float64
	DayOfWeek    int
	Hour         int
	Limit        int
	TimeZone     string
}

func NewReportUseCase(repo ReportRepository, publisher EventPublisher, timeZone string) *ReportUseCase {
	if timeZone == "" {
		timeZone = "UTC"
	}
	return &ReportUseCase{
		repo:      repo,
		publisher: publisher,
		timeZone:  timeZone,
	}
}

func (uc *ReportUseCase) CreateReport(ctx context.Context, input CreateReportInput) (*domain.Report, error) {
	if err := validatePoint(input.Latitude, input.Longitude); err != nil {
		return nil, err
	}
	if input.QuietnessLevel < 1 || input.QuietnessLevel > 5 {
		return nil, domain.NewValidationError("quietness", "must be between 1 and 5")
	}

	placeName := strings.TrimSpace(input.PlaceName)
	if len(placeName) > 200 {
		return nil, domain.NewValidationError("place_name", "must be at most 200 characters")
	}

	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		userID = "anon-" + uuid.NewString()[:8]
	}
	if len(userID) > 64 {
		return nil, domain.NewValidationError("user_id", "must be at most 64 characters")
	}

	report := &domain.Report{
		ID:             uuid.NewString(),
		UserID:         userID,
		Location:       domain.Point{Latitude: input.Latitude, Longitude: input.Longitude},
		QuietnessLevel: input.QuietnessLevel,
		PlaceName:      placeName,
		CreatedAt:      time.Now().UTC(),
		Weight:         1,
	}

	if err := uc.repo.CreateReport(ctx, report); err != nil {
		return nil, err
	}

	if uc.publisher != nil {
		uc.publisher.PublishReport(ctx, *report)
	}

	return report, nil
}

func (uc *ReportUseCase) ConfirmReport(ctx context.Context, reportID, userID string) (*domain.Report, error) {
	reportID = strings.TrimSpace(reportID)
	userID = strings.TrimSpace(userID)

	if reportID == "" {
		return nil, domain.NewValidationError("report_id", "is required")
	}
	if userID == "" {
		return nil, domain.NewValidationError("user_id", "is required")
	}
	if len(userID) > 64 {
		return nil, domain.NewValidationError("user_id", "must be at most 64 characters")
	}

	report, err := uc.repo.ConfirmReport(ctx, reportID, userID)
	if err != nil {
		return nil, err
	}

	if uc.publisher != nil {
		uc.publisher.PublishConfirmation(ctx, *report)
	}

	return report, nil
}

func (uc *ReportUseCase) ListRecentReports(ctx context.Context, latitude, longitude, radiusMeters float64) ([]domain.Report, error) {
	if err := validateSearch(latitude, longitude, radiusMeters); err != nil {
		return nil, err
	}

	return uc.repo.ListRecentReports(ctx, latitude, longitude, radiusMeters, 2000)
}

func (uc *ReportUseCase) FindQuietPlaces(ctx context.Context, query QuietPlaceQuery) ([]domain.QuietPlace, error) {
	if err := validateSearch(query.Latitude, query.Longitude, query.RadiusMeters); err != nil {
		return nil, err
	}
	if query.DayOfWeek < 1 || query.DayOfWeek > 7 {
		return nil, domain.NewValidationError("day_of_week", "must be between 1 and 7 using ISO weekday")
	}
	if query.Hour < 0 || query.Hour > 23 {
		return nil, domain.NewValidationError("hour", "must be between 0 and 23")
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 50
	}
	if query.TimeZone == "" {
		query.TimeZone = uc.timeZone
	}

	return uc.repo.FindQuietPlaces(ctx, query)
}

func validateSearch(latitude, longitude, radiusMeters float64) error {
	if err := validatePoint(latitude, longitude); err != nil {
		return err
	}
	if radiusMeters <= 0 {
		return domain.NewValidationError("radius", "must be greater than zero")
	}
	if radiusMeters > 50000 {
		return domain.NewValidationError("radius", "must be at most 50000 meters")
	}
	return nil
}

func validatePoint(latitude, longitude float64) error {
	if latitude < -90 || latitude > 90 {
		return domain.NewValidationError("latitude", "must be between -90 and 90")
	}
	if longitude < -180 || longitude > 180 {
		return domain.NewValidationError("longitude", "must be between -180 and 180")
	}
	return nil
}
