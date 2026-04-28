package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRestrictedDemoCORSAllowsFileOrigin(t *testing.T) {
	handler := securityHeaders(restrictedDemoCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodOptions, "/api/reports", nil)
	req.Header.Set("Origin", "null")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", res.Code)
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "null" {
		t.Fatalf("allow origin = %q, want null", got)
	}
	if got := res.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("allow credentials = %q, want true", got)
	}
	if got := res.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type" {
		t.Fatalf("allow headers = %q, want Content-Type", got)
	}
}

func TestRestrictedDemoCORSRejectsUnknownOrigin(t *testing.T) {
	handler := restrictedDemoCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/reports/recent", nil)
	req.Header.Set("Origin", "https://example.com")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected allow origin = %q", got)
	}
}
