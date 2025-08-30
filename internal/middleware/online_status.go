package middleware

import (
	"net/http"
	"time"

	"github.com/cduffaut/matcha/internal/session"
	"github.com/cduffaut/matcha/internal/user"
)

// OnlineStatusMiddleware met à jour le statut en ligne des utilisateurs
type OnlineStatusMiddleware struct {
	profileService *user.ProfileService
}

// NewOnlineStatusMiddleware crée un nouveau middleware de statut en ligne
func NewOnlineStatusMiddleware(profileService *user.ProfileService) *OnlineStatusMiddleware {
	return &OnlineStatusMiddleware{
		profileService: profileService,
	}
}

// UpdateOnlineStatus met à jour le statut en ligne de l'utilisateur
func (m *OnlineStatusMiddleware) UpdateOnlineStatus(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userSession, ok := session.FromContext(r.Context())
		if ok {
			// VÉRIFICATION : Lire le statut AVANT de le modifier
			m.profileService.GetUserOnlineStatus(userSession.UserID)

			go func() {
				// Marquer comme en ligne
				err := m.profileService.UpdateUserOnlineStatus(userSession.UserID, true)
				if err == nil {
					// VÉRIFICATION : Lire le statut APRÈS modification
					m.profileService.GetUserOnlineStatus(userSession.UserID)
				}
			}()
		}
		next.ServeHTTP(w, r)
	})
}

// StartCleanupRoutine démarre la routine de nettoyage automatique
func (m *OnlineStatusMiddleware) StartCleanupRoutine() {
	go func() {
		// CORRECTION : Réduire la fréquence pour éviter de trop nettoyer
		ticker := time.NewTicker(2 * time.Minute) // Toutes les 2 minutes au lieu de 30s
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// CORRECTION : Augmenter le timeout pour être moins agressif
				m.cleanupInactiveUsersDirectly(10) // 10 minutes au lieu de 5
			}
		}
	}()
}

// Nettoyage direct via repository
func (m *OnlineStatusMiddleware) cleanupInactiveUsersDirectly(timeoutMinutes int) error {
	// Accéder directement au repository via le service
	return m.profileService.CleanupInactiveUsers(timeoutMinutes)
}
