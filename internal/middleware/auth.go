package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/cduffaut/matcha/internal/session"
)

type AuthMiddleware struct{ sessionManager *session.Manager }

func NewAuthMiddleware(sm *session.Manager) *AuthMiddleware { return &AuthMiddleware{sessionManager: sm} }

func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userSession, err := m.sessionManager.GetSession(r)
		if err != nil {
			if isAPIRequest(r) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "Authentication required"})
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		ctx := session.WithSession(r.Context(), userSession)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isAPIRequest(r *http.Request) bool {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		return true
	}
	h := r.Header
	if strings.Contains(h.Get("Accept"), "application/json") ||
		strings.Contains(h.Get("Content-Type"), "application/json") ||
		strings.Contains(h.Get("Content-Type"), "multipart/form-data") ||
		strings.EqualFold(h.Get("X-Requested-With"), "XMLHttpRequest") {
		return true
	}
	return false
}
