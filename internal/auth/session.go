package auth

import (
	"github.com/cduffaut/matcha/internal/session"
)

// NewSessionManager crée un nouveau gestionnaire de session
func NewSessionManager(cookieName string) *session.Manager {
	return session.NewManager(cookieName)
}
