package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/portfolio/silence-map/internal/domain"
	"github.com/portfolio/silence-map/internal/usecase"
)

type ReportHandler struct {
	useCase *usecase.ReportUseCase
	loc     *time.Location
}

func NewReportHandler(useCase *usecase.ReportUseCase, timeZone string) *ReportHandler {
	loc, err := time.LoadLocation(timeZone)
	if err != nil {
		loc = time.UTC
	}

	return &ReportHandler{
		useCase: useCase,
		loc:     loc,
	}
}

func (h *ReportHandler) RegisterRoutes(router chi.Router) {
	router.Post("/api/reports", h.createReport)
	router.Post("/api/reports/{id}/confirm", h.confirmReport)
	router.Get("/api/reports/recent", h.listRecentReports)
	router.Get("/api/places/quiet", h.findQuietPlaces)
}

type createReportRequest struct {
	UserID    string  `json:"user_id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Quietness int     `json:"quietness"`
	PlaceName string  `json:"place_name"`
}

type confirmReportRequest struct {
	UserID string `json:"user_id"`
}

func (h *ReportHandler) createReport(w http.ResponseWriter, r *http.Request) {
	var req createReportRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	userID := firstNonEmpty(req.UserID, r.Header.Get("X-User-ID"))
	report, err := h.useCase.CreateReport(r.Context(), usecase.CreateReportInput{
		UserID:         userID,
		Latitude:       req.Latitude,
		Longitude:      req.Longitude,
		QuietnessLevel: req.Quietness,
		PlaceName:      req.PlaceName,
	})
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, report)
}

func (h *ReportHandler) confirmReport(w http.ResponseWriter, r *http.Request) {
	var req confirmReportRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	reportID := chi.URLParam(r, "id")
	userID := firstNonEmpty(req.UserID, r.Header.Get("X-User-ID"))
	report, err := h.useCase.ConfirmReport(r.Context(), reportID, userID)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func (h *ReportHandler) listRecentReports(w http.ResponseWriter, r *http.Request) {
	latitude, err := parseFloatQuery(r, "lat")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	longitude, err := parseFloatQuery(r, "lng")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	radius, err := parseFloatQueryWithDefault(r, "radius", 5000)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	reports, err := h.useCase.ListRecentReports(r.Context(), latitude, longitude, radius)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, reports)
}

func (h *ReportHandler) findQuietPlaces(w http.ResponseWriter, r *http.Request) {
	now := time.Now().In(h.loc)
	defaultDay := int(now.Weekday())
	if defaultDay == 0 {
		defaultDay = 7
	}

	latitude, err := parseFloatQuery(r, "lat")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	longitude, err := parseFloatQuery(r, "lng")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	radius, err := parseFloatQueryWithDefault(r, "radius", 5000)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	dayOfWeek, err := parseIntQueryWithDefault(r, "day_of_week", defaultDay)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	hour, err := parseIntQueryWithDefault(r, "hour", now.Hour())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	limit, err := parseIntQueryWithDefault(r, "limit", 50)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	places, err := h.useCase.FindQuietPlaces(r.Context(), usecase.QuietPlaceQuery{
		Latitude:     latitude,
		Longitude:    longitude,
		RadiusMeters: radius,
		DayOfWeek:    dayOfWeek,
		Hour:         hour,
		Limit:        limit,
	})
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, places)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func parseFloatQuery(r *http.Request, name string) (float64, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return 0, domain.NewValidationError(name, "is required")
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, domain.NewValidationError(name, "must be a number")
	}
	return parsed, nil
}

func parseFloatQueryWithDefault(r *http.Request, name string, fallback float64) (float64, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, domain.NewValidationError(name, "must be a number")
	}
	return parsed, nil
}

func parseIntQueryWithDefault(r *http.Request, name string, fallback int) (int, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, domain.NewValidationError(name, "must be an integer")
	}
	return parsed, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func writeUseCaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "resource not found")
	case errors.Is(err, domain.ErrDuplicateConfirmation):
		writeError(w, http.StatusConflict, "user already confirmed this report")
	case errors.Is(err, domain.ErrSelfConfirmation):
		writeError(w, http.StatusConflict, "report author cannot confirm their own report")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
