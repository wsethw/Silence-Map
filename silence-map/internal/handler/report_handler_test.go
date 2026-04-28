package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/portfolio/silence-map/internal/domain"
	"github.com/portfolio/silence-map/internal/identity"
	"github.com/portfolio/silence-map/internal/usecase"
)

type handlerFakeRepo struct {
	createdReport  *domain.Report
	lastQuietQuery usecase.QuietPlaceQuery
}

func (f *handlerFakeRepo) CreateReport(_ context.Context, report *domain.Report) error {
	copied := *report
	f.createdReport = &copied
	return nil
}

func (f *handlerFakeRepo) ConfirmReport(context.Context, string, string) (*domain.Report, error) {
	return &domain.Report{
		ID:             "11111111-1111-4111-8111-111111111111",
		UserID:         "trusted-user",
		Location:       domain.Point{Latitude: -23.55, Longitude: -46.63},
		QuietnessLevel: 5,
		CreatedAt:      time.Now(),
	}, nil
}

func (f *handlerFakeRepo) ListRecentReports(context.Context, usecase.RecentReportsQuery) ([]domain.Report, error) {
	return nil, nil
}

func (f *handlerFakeRepo) FindQuietPlaces(_ context.Context, query usecase.QuietPlaceQuery) ([]domain.QuietPlace, error) {
	f.lastQuietQuery = query
	return []domain.QuietPlace{{
		PlaceName:        "Ibirapuera Park",
		Location:         domain.Point{Latitude: -23.58, Longitude: -46.65},
		AverageQuietness: 5,
		ReportCount:      1,
		LastReportAt:     time.Now(),
	}}, nil
}

func TestParseOptionalBounds(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/places/quiet?north=-23.4&south=-23.7&east=-46.5&west=-46.8", nil)
	bounds, err := parseOptionalBounds(req)
	if err != nil {
		t.Fatalf("parseOptionalBounds error = %v", err)
	}
	if bounds == nil || bounds.North != -23.4 || bounds.South != -23.7 || bounds.East != -46.5 || bounds.West != -46.8 {
		t.Fatalf("bounds = %+v", bounds)
	}
}

func TestParseOptionalBoundsRequiresAllFields(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/places/quiet?north=-23.4&south=-23.7", nil)
	if _, err := parseOptionalBounds(req); err == nil {
		t.Fatal("parseOptionalBounds error = nil, want error")
	}
}

func TestFindQuietPlacesParsesBoundsIntoUsecase(t *testing.T) {
	repo := &handlerFakeRepo{}
	router := testRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/places/quiet?lat=-23.55&lng=-46.63&radius=5000&day_of_week=1&hour=15&north=-23.4&south=-23.7&east=-46.5&west=-46.8", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if repo.lastQuietQuery.Bounds == nil {
		t.Fatal("bounds were not passed to usecase")
	}
	if repo.lastQuietQuery.Bounds.West != -46.8 {
		t.Fatalf("west = %v, want -46.8", repo.lastQuietQuery.Bounds.West)
	}
}

func TestCreateReportUsesTrustedSessionIdentity(t *testing.T) {
	repo := &handlerFakeRepo{}
	router := testRouter(repo)
	body := `{"user_id":"spoofed-user","latitude":-23.55,"longitude":-46.63,"quietness":5,"place_name":"Test"}`

	req := httptest.NewRequest(http.MethodPost, "/api/reports", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if repo.createdReport == nil {
		t.Fatal("report was not created")
	}
	if repo.createdReport.UserID == "spoofed-user" {
		t.Fatal("handler trusted client-supplied user_id")
	}
	if !strings.HasPrefix(repo.createdReport.UserID, "anon-") {
		t.Fatalf("user id = %q, want generated anonymous identity", repo.createdReport.UserID)
	}
}

func TestInvalidBoundsReturnsBadRequest(t *testing.T) {
	router := testRouter(&handlerFakeRepo{})
	req := httptest.NewRequest(http.MethodGet, "/api/places/quiet?lat=-23.55&lng=-46.63&radius=5000&day_of_week=1&hour=15&north=-23.8&south=-23.7&east=-46.5&west=-46.8", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
	var payload map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON error payload: %v", err)
	}
	if payload["error"] == "" {
		t.Fatal("missing error message")
	}
}

func testRouter(repo *handlerFakeRepo) http.Handler {
	uc := usecase.NewReportUseCase(repo, nil, "UTC")
	h := NewReportHandler(uc, "UTC")
	router := chi.NewRouter()
	router.Use(identity.NewManager("test-secret").Middleware)
	h.RegisterRoutes(router)
	return router
}
