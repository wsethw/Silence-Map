package identity

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const CookieName = "silence_session"

type contextKey struct{}

type Manager struct {
	secret []byte
}

func NewManager(secret string) *Manager {
	if strings.TrimSpace(secret) == "" {
		secret = randomSecret()
	}
	return &Manager{secret: []byte(secret)}
}

func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := m.read(r)
		if !ok {
			userID = "anon-" + uuid.NewString()
			m.write(w, r, userID)
		}

		ctx := context.WithValue(r.Context(), contextKey{}, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func FromContext(ctx context.Context) string {
	userID, _ := ctx.Value(contextKey{}).(string)
	return userID
}

func (m *Manager) read(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(CookieName)
	if err != nil || cookie.Value == "" {
		return "", false
	}

	decoded, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return "", false
	}

	parts := strings.SplitN(string(decoded), ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}

	if !m.valid(parts[0], parts[1]) {
		return "", false
	}

	return parts[0], true
}

func (m *Manager) write(w http.ResponseWriter, r *http.Request, userID string) {
	value := base64.RawURLEncoding.EncodeToString([]byte(userID + "." + m.sign(userID)))
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
}

func (m *Manager) sign(userID string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(userID))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (m *Manager) valid(userID, signature string) bool {
	expected := m.sign(userID)
	return hmac.Equal([]byte(signature), []byte(expected))
}

func randomSecret() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return uuid.NewString()
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
