package middleware

import (
	"net/http"
	"strings"
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

// Détermine si c'est une vraie action utilisateur ou une consultation passive
func (m *OnlineStatusMiddleware) isRealUserAction(r *http.Request) bool {
	path := r.URL.Path
	method := r.Method

	// Routes qui NE doivent PAS remettre en ligne (consultations passives)
	passiveRoutes := []string{
		"/api/profile/",                   // Consultation de profils
		"/api/notifications/unread-count", // Compteurs automatiques
		"/api/chat/unread-count",          // Compteurs automatiques
		"/static/",                        // Ressources statiques
		"/uploads/",                       // Images
	}

	for _, passive := range passiveRoutes {
		if strings.Contains(path, passive) {
			// Sauf si c'est une action POST/PUT/DELETE (vraie action)
			if method == "GET" {
				return false
			}
		}
	}

	// Routes qui indiquent une vraie activité utilisateur
	activeRoutes := []string{
		"/profile",            // Gestion du profil
		"/browse",             // Navigation/recherche
		"/chat",               // Page de chat
		"/notifications",      // Page notifications
		"/api/profile/tags",   // Gestion des tags
		"/api/profile/photos", // Gestion des photos
		"/api/profile/",       // Actions sur profils (like, block, etc.)
	}

	for _, active := range activeRoutes {
		if strings.Contains(path, active) {
			return true
		}
	}

	// Par défaut, considérer comme une activité
	return true
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
