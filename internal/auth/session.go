package auth

import (
	"github.com/cduffaut/matcha/internal/session"
)

// cree un nouv gestionnaire de session
func NewSessionManager(cookieName string) *session.Manager {
	return session.NewManager(cookieName)
}
