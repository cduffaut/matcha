package middleware

import (
	"net/http"

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
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Stocker les informations de session dans le contexte
		ctx := r.Context()
		ctx = session.WithSession(ctx, userSession)

		// Passer à la prochaine fonction
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
