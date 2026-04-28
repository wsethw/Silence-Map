package identity

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddlewareCreatesSignedAnonymousIdentity(t *testing.T) {
	manager := NewManager("test-secret")
	var userID string
	handler := manager.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID = FromContext(r.Context())
	}))

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))

	if !strings.HasPrefix(userID, "anon-") {
		t.Fatalf("userID = %q, want anon-*", userID)
	}
	if res.Result().Cookies()[0].Name != CookieName {
		t.Fatalf("cookie name = %q, want %q", res.Result().Cookies()[0].Name, CookieName)
	}
}

func TestMiddlewareRejectsForgedCookie(t *testing.T) {
	manager := NewManager("test-secret")
	var userID string
	handler := manager.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID = FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "forged"})
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if !strings.HasPrefix(userID, "anon-") {
		t.Fatalf("userID = %q, want generated identity", userID)
	}
	if len(res.Result().Cookies()) == 0 {
		t.Fatal("expected replacement cookie")
	}
}
