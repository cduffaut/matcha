package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/cduffaut/matcha/internal/session"
	"github.com/cduffaut/matcha/internal/user"
)

// met à jour le statut en ligne et nettoie les inactifs.
type OnlineStatusMiddleware struct {
	profileService *user.ProfileService
	cleanupOnce    sync.Once
}

// retourne un middleware prêt à l'emploi.
func NewOnlineStatusMiddleware(profileService *user.ProfileService) *OnlineStatusMiddleware {
	return &OnlineStatusMiddleware{profileService: profileService}
}

// UpdateOnlineStatus marque l'utilisateur en ligne de manière asynchrone.
func (m *OnlineStatusMiddleware) UpdateOnlineStatus(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sess, ok := session.FromContext(r.Context()); ok {
			// Lecture avant
			_, _, _ = m.profileService.GetUserOnlineStatus(sess.UserID)

			go m.markOnline(sess.UserID)
		}
		next.ServeHTTP(w, r)
	})
}

func (m *OnlineStatusMiddleware) markOnline(userID int) {
	defer func() { _ = recover() }() // isolation goroutine
	if err := m.profileService.UpdateUserOnlineStatus(userID, true); err == nil {
		// Lecture après
		_, _, _ = m.profileService.GetUserOnlineStatus(userID)
	}
}

// lance une routine unique de nettoyage périodique.
func (m *OnlineStatusMiddleware) StartCleanupRoutine() {
	m.cleanupOnce.Do(func() {
		go m.runCleanup(2*time.Minute, 10) // interval=2m, timeout=10m
	})
}

func (m *OnlineStatusMiddleware) runCleanup(interval time.Duration, timeoutMinutes int) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for range t.C {
		_ = m.cleanupInactiveUsersDirectly(timeoutMinutes)
	}
}

// Nettoyage direct via service.
func (m *OnlineStatusMiddleware) cleanupInactiveUsersDirectly(timeoutMinutes int) error {
	return m.profileService.CleanupInactiveUsers(timeoutMinutes)
}
