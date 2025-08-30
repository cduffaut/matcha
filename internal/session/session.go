package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/cduffaut/matcha/internal/models"
)

type Session struct {
	UserID    int
	Username  string
	ExpiresAt time.Time
}

type Manager struct {
	CookieName string
	Duration   time.Duration
	SameSite   http.SameSite
	Secure     bool

	mu       sync.RWMutex
	sessions map[string]Session
}

func NewManager(cookieName string) *Manager {
	return &Manager{
		CookieName: cookieName,
		Duration:   24 * time.Hour,
		SameSite:   http.SameSiteLaxMode,
		Secure:     false, // set true in prod behind HTTPS
		sessions:   make(map[string]Session),
	}
}

func (m *Manager) CreateSession(w http.ResponseWriter, user *models.User) (string, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", err
	}
	s := Session{
		UserID:    user.ID,
		Username:  user.Username,
		ExpiresAt: time.Now().UTC().Add(m.Duration),
	}

	m.mu.Lock()
	m.sessions[token] = s
	m.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     m.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: m.SameSite,
		Secure:   m.Secure,
		Expires:  s.ExpiresAt,
		MaxAge:   int(m.Duration.Seconds()),
	})
	return token, nil
}

func (m *Manager) GetSession(r *http.Request) (*Session, error) {
	c, err := r.Cookie(m.CookieName)
	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	s, ok := m.sessions[c.Value]
	m.mu.RUnlock()
	if !ok {
		return nil, errors.New("session inconnue")
	}
	if time.Now().UTC().After(s.ExpiresAt) {
		m.mu.Lock()
		delete(m.sessions, c.Value)
		m.mu.Unlock()
		return nil, errors.New("session expirée")
	}
	return &s, nil
}

func (m *Manager) DestroySession(w http.ResponseWriter, r *http.Request) error {
	c, err := r.Cookie(m.CookieName)
	if err == nil {
		m.mu.Lock()
		delete(m.sessions, c.Value)
		m.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     m.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: m.SameSite,
		Secure:   m.Secure,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
	return nil
}

/* ---- context helpers ---- */

type sessionKeyType struct{}

var sessionKey = sessionKeyType{}

func WithSession(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, sessionKey, s)
}

func FromContext(ctx context.Context) (*Session, bool) {
	s, ok := ctx.Value(sessionKey).(*Session)
	return s, ok
}

/* ---- internals ---- */

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}