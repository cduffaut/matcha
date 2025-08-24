package middleware

import (
	"net/http"
	"strings"

	"github.com/cduffaut/matcha/internal/session"
)

// AuthMiddleware est un middleware pour vérifier l'authentification des utilisateurs
type AuthMiddleware struct {
	sessionManager *session.Manager
}

// NewAuthMiddleware crée un nouveau middleware d'authentification
func NewAuthMiddleware(sessionManager *session.Manager) *AuthMiddleware {
	return &AuthMiddleware{
		sessionManager: sessionManager,
	}
}

// RequireAuth est un middleware qui vérifie si l'utilisateur est authentifié
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Vérifier la session
		userSession, err := m.sessionManager.GetSession(r)
		if err != nil {

			// CORRECTION : Différencier les requêtes API des pages HTML
			if isAPIRequest(r) {
				// Pour les APIs, retourner JSON
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": "Authentication required"}`))
				return
			} else {
				// Pour les pages, rediriger vers login
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
		}

		// CORRECTION : Stocker les informations de session dans le contexte
		ctx := r.Context()
		ctx = session.WithSession(ctx, userSession)

		// Passer à la prochaine fonction
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isAPIRequest détermine si c'est une requête API
func isAPIRequest(r *http.Request) bool {
	// Vérifier le chemin
	if strings.HasPrefix(r.URL.Path, "/api/") {
		return true
	}

	// Vérifier l'en-tête Accept
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return true
	}

	// Vérifier l'en-tête Content-Type
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "multipart/form-data") {
		return true
	}

	return false
}
