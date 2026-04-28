package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/portfolio/silence-map/internal/domain"
)

type fakeReportRepo struct {
	createErr       error
	confirmErr      error
	confirmedReport *domain.Report
	quietPlaces     []domain.QuietPlace
	lastQuietQuery  QuietPlaceQuery
	lastRecentQuery RecentReportsQuery
}

func (f *fakeReportRepo) CreateReport(context.Context, *domain.Report) error {
	return f.createErr
}

func (f *fakeReportRepo) ConfirmReport(context.Context, string, string) (*domain.Report, error) {
	if f.confirmErr != nil {
		return nil, f.confirmErr
	}
	if f.confirmedReport != nil {
		return f.confirmedReport, nil
	}
	return &domain.Report{ID: "report-1"}, nil
}

func (f *fakeReportRepo) ListRecentReports(_ context.Context, query RecentReportsQuery) ([]domain.Report, error) {
	f.lastRecentQuery = query
	return nil, nil
}

func (f *fakeReportRepo) FindQuietPlaces(_ context.Context, query QuietPlaceQuery) ([]domain.QuietPlace, error) {
	f.lastQuietQuery = query
	return f.quietPlaces, nil
}

func TestCreateReportValidation(t *testing.T) {
	uc := NewReportUseCase(&fakeReportRepo{}, nil, "UTC")

	tests := []CreateReportInput{
		{Latitude: -91, Longitude: -46.6, QuietnessLevel: 4},
		{Latitude: -23.5, Longitude: -181, QuietnessLevel: 4},
		{Latitude: -23.5, Longitude: -46.6, QuietnessLevel: 0},
		{Latitude: -23.5, Longitude: -46.6, QuietnessLevel: 6},
	}

	for _, input := range tests {
		if _, err := uc.CreateReport(context.Background(), input); !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("CreateReport(%+v) error = %v, want validation", input, err)
		}
	}
}

func TestListRecentReportsValidation(t *testing.T) {
	uc := NewReportUseCase(&fakeReportRepo{}, nil, "UTC")

	_, err := uc.ListRecentReports(context.Background(), RecentReportsQuery{
		Latitude:     -23.5,
		Longitude:    -46.6,
		RadiusMeters: 50001,
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("ListRecentReports radius error = %v, want validation", err)
	}
}

func TestFindQuietPlacesRejectsInvalidBounds(t *testing.T) {
	uc := NewReportUseCase(&fakeReportRepo{}, nil, "UTC")

	_, err := uc.FindQuietPlaces(context.Background(), QuietPlaceQuery{
		Latitude:     -23.5,
		Longitude:    -46.6,
		RadiusMeters: 5000,
		Bounds:       &domain.Bounds{North: -23.7, South: -23.4, East: -46.5, West: -46.8},
		DayOfWeek:    1,
		Hour:         15,
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("FindQuietPlaces invalid bounds error = %v, want validation", err)
	}
}

func TestFindQuietPlacesPassesBounds(t *testing.T) {
	repo := &fakeReportRepo{}
	uc := NewReportUseCase(repo, nil, "UTC")
	bounds := &domain.Bounds{North: -23.4, South: -23.7, East: -46.5, West: -46.8}

	if _, err := uc.FindQuietPlaces(context.Background(), QuietPlaceQuery{
		Latitude:     -23.55,
		Longitude:    -46.63,
		RadiusMeters: 5000,
		Bounds:       bounds,
		DayOfWeek:    1,
		Hour:         15,
	}); err != nil {
		t.Fatalf("FindQuietPlaces error = %v", err)
	}

	if repo.lastQuietQuery.Bounds == nil || *repo.lastQuietQuery.Bounds != *bounds {
		t.Fatalf("repo bounds = %+v, want %+v", repo.lastQuietQuery.Bounds, bounds)
	}
}

func TestConfirmReportPropagatesDuplicate(t *testing.T) {
	uc := NewReportUseCase(&fakeReportRepo{confirmErr: domain.ErrDuplicateConfirmation}, nil, "UTC")

	_, err := uc.ConfirmReport(context.Background(), "report-1", "user-1")
	if !errors.Is(err, domain.ErrDuplicateConfirmation) {
		t.Fatalf("ConfirmReport error = %v, want duplicate", err)
	}
}
