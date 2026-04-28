package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/portfolio/silence-map/internal/domain"
	"github.com/portfolio/silence-map/internal/identity"
	"github.com/portfolio/silence-map/internal/ratelimit"
	"github.com/portfolio/silence-map/internal/usecase"
)

type ReportHandler struct {
	useCase        *usecase.ReportUseCase
	loc            *time.Location
	reportLimiter  *ratelimit.Limiter
	confirmLimiter *ratelimit.Limiter
}

func NewReportHandler(useCase *usecase.ReportUseCase, timeZone string) *ReportHandler {
	loc, err := time.LoadLocation(timeZone)
	if err != nil {
		loc = time.UTC
	}

	return &ReportHandler{
		useCase:        useCase,
		loc:            loc,
		reportLimiter:  ratelimit.New(12, time.Minute),
		confirmLimiter: ratelimit.New(60, time.Minute),
	}
}

func (h *ReportHandler) RegisterRoutes(router chi.Router) {
	router.Post("/api/reports", h.createReport)
	router.Post("/api/reports/{id}/confirm", h.confirmReport)
	router.Get("/api/reports/recent", h.listRecentReports)
	router.Get("/api/places/quiet", h.findQuietPlaces)
}

type createReportRequest struct {
	UserID    string  `json:"user_id"` // accepted for backward compatibility but ignored
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Quietness int     `json:"quietness"`
	PlaceName string  `json:"place_name"`
}

type confirmReportRequest struct {
	UserID string `json:"user_id"` // accepted for backward compatibility but ignored
}

func (h *ReportHandler) createReport(w http.ResponseWriter, r *http.Request) {
	var req createReportRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	userID := requestUserID(r)
	if !allowWrite(h.reportLimiter, r, "report") {
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}

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
	if err := decodeOptionalJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	reportID := chi.URLParam(r, "id")
	userID := requestUserID(r)
	if !allowWrite(h.confirmLimiter, r, "confirm") {
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}

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
	bounds, err := parseOptionalBounds(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	reports, err := h.useCase.ListRecentReports(r.Context(), usecase.RecentReportsQuery{
		Latitude:     latitude,
		Longitude:    longitude,
		RadiusMeters: radius,
		Bounds:       bounds,
	})
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
	bounds, err := parseOptionalBounds(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	places, err := h.useCase.FindQuietPlaces(r.Context(), usecase.QuietPlaceQuery{
		Latitude:     latitude,
		Longitude:    longitude,
		RadiusMeters: radius,
		Bounds:       bounds,
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
	if err := decoder.Decode(dst); err != nil {
		return normalizeJSONError(err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return domain.NewValidationError("body", "must contain a single JSON object")
	}
	return nil
}

func decodeOptionalJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}
	return decodeJSON(w, r, dst)
}

func normalizeJSONError(err error) error {
	if errors.Is(err, io.EOF) {
		return domain.NewValidationError("body", "is required")
	}
	var syntaxError *json.SyntaxError
	if errors.As(err, &syntaxError) {
		return domain.NewValidationError("body", "must be valid JSON")
	}
	var typeError *json.UnmarshalTypeError
	if errors.As(err, &typeError) {
		return domain.NewValidationError(typeError.Field, "has an invalid type")
	}
	if strings.HasPrefix(err.Error(), "json: unknown field ") {
		return domain.NewValidationError("body", err.Error())
	}
	return err
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

func parseOptionalBounds(r *http.Request) (*domain.Bounds, error) {
	q := r.URL.Query()
	names := []string{"north", "south", "east", "west"}
	present := 0
	for _, name := range names {
		if q.Get(name) != "" {
			present++
		}
	}
	if present == 0 {
		return nil, nil
	}
	if present != len(names) {
		return nil, domain.NewValidationError("bounds", "north, south, east, and west must be provided together")
	}

	north, err := strconv.ParseFloat(q.Get("north"), 64)
	if err != nil {
		return nil, domain.NewValidationError("north", "must be a number")
	}
	south, err := strconv.ParseFloat(q.Get("south"), 64)
	if err != nil {
		return nil, domain.NewValidationError("south", "must be a number")
	}
	east, err := strconv.ParseFloat(q.Get("east"), 64)
	if err != nil {
		return nil, domain.NewValidationError("east", "must be a number")
	}
	west, err := strconv.ParseFloat(q.Get("west"), 64)
	if err != nil {
		return nil, domain.NewValidationError("west", "must be a number")
	}

	bounds := domain.Bounds{North: north, South: south, East: east, West: west}
	if !bounds.Valid() {
		return nil, domain.NewValidationError("bounds", "must be a valid north/south/east/west viewport")
	}
	return &bounds, nil
}

func requestUserID(r *http.Request) string {
	userID := identity.FromContext(r.Context())
	if userID != "" {
		return userID
	}
	return "anon-ip-" + hashedClientIP(r)
}

func allowWrite(limiter *ratelimit.Limiter, r *http.Request, action string) bool {
	for _, key := range rateLimitKeys(r, action) {
		if !limiter.Allow(key) {
			return false
		}
	}
	return true
}

func rateLimitKeys(r *http.Request, action string) []string {
	return []string{
		action + ":identity:" + requestUserID(r),
		action + ":ip:" + hashedClientIP(r),
	}
}

func hashedClientIP(r *http.Request) string {
	sum := sha256.Sum256([]byte(normalizedClientIP(r)))
	return hex.EncodeToString(sum[:8])
}

func normalizedClientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		first := strings.TrimSpace(strings.Split(forwarded, ",")[0])
		if ip := net.ParseIP(first); ip != nil {
			return ip.String()
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		if ip := net.ParseIP(realIP); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return "unknown"
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
