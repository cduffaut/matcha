package auth

import (
	"context"

	"github.com/cduffaut/matcha/internal/session"
)

// key pour stock la session dans le contexte
type sessionKeyType struct{}

var sessionKey = sessionKeyType{}

// ajoute une session au contexte
func WithSession(ctx context.Context, session *session.Session) context.Context {
	return context.WithValue(ctx, sessionKey, session)
}

// recup la session du contexte
func SessionFromContext(ctx context.Context) (*session.Session, bool) {
	session, ok := ctx.Value(sessionKey).(*session.Session)
	return session, ok
}
