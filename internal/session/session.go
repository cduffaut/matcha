// internal/session/session.go
package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/cduffaut/matcha/internal/models"
)

// Session représente une session utilisateur
type Session struct {
	UserID    int
	Username  string
	ExpiresAt time.Time
}

// Manager gère les sessions utilisateur
type Manager struct {
	CookieName string
	Sessions   map[string]Session
}

// NewManager crée un nouveau gestionnaire de session
func NewManager(cookieName string) *Manager {
	return &Manager{
		CookieName: cookieName,
		Sessions:   make(map[string]Session),
	}
}

// CreateSession crée une nouvelle session pour un utilisateur
func (m *Manager) CreateSession(w http.ResponseWriter, user *models.User) (string, error) {
	fmt.Printf("⚠️ Création de session pour: %s (ID: %d)\n", user.Username, user.ID)

	// Générer un token de session
	sessionToken, err := generateRandomToken(32)
	if err != nil {
		fmt.Printf("⚠️ Erreur génération token: %v\n", err)
		return "", fmt.Errorf("erreur lors de la génération du token de session: %w", err)
	}

	fmt.Printf("⚠️ Token généré: %s\n", sessionToken)

	// Créer la session
	session := Session{
		UserID:    user.ID,
		Username:  user.Username,
		ExpiresAt: time.Now().Add(24 * time.Hour), // Session de 24 heures
	}

	// Stocker la session
	m.Sessions[sessionToken] = session

	fmt.Printf("⚠️ Session stockée: %+v\n", session)

	// Créer le cookie
	cookie := http.Cookie{
		Name:     m.CookieName,
		Value:    sessionToken,
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
		Secure:   false, // À mettre à false en développement
	}

	// Définir le cookie dans la réponse
	http.SetCookie(w, &cookie)

	fmt.Printf("⚠️ Cookie défini: %+v\n", cookie)

	return sessionToken, nil
}

// GetSession récupère une session à partir d'une requête
func (m *Manager) GetSession(r *http.Request) (*Session, error) {
	// Récupérer le cookie de session
	cookie, err := r.Cookie(m.CookieName)
	if err != nil {
		return nil, fmt.Errorf("pas de session trouvée")
	}

	// Récupérer la session
	session, exists := m.Sessions[cookie.Value]
	if !exists {
		return nil, fmt.Errorf("session invalide")
	}

	// Vérifier si la session a expiré
	if time.Now().After(session.ExpiresAt) {
		delete(m.Sessions, cookie.Value)
		return nil, fmt.Errorf("session expirée")
	}

	return &session, nil
}

// DestroySession détruit une session
func (m *Manager) DestroySession(w http.ResponseWriter, r *http.Request) error {
	// Récupérer le cookie de session
	cookie, err := r.Cookie(m.CookieName)
	if err != nil {
		return nil // Pas de session à détruire
	}

	// Supprimer la session
	delete(m.Sessions, cookie.Value)

	// Expirer le cookie
	expiredCookie := http.Cookie{
		Name:     m.CookieName,
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
		Secure:   true,
	}

	http.SetCookie(w, &expiredCookie)

	return nil
}

// Clé pour stocker la session dans le contexte
type sessionKeyType struct{}

var sessionKey = sessionKeyType{}

// WithSession ajoute une session au contexte
func WithSession(ctx context.Context, session *Session) context.Context {
	return context.WithValue(ctx, sessionKey, session)
}

// FromContext récupère la session du contexte
func FromContext(ctx context.Context) (*Session, bool) {
	session, ok := ctx.Value(sessionKey).(*Session)
	return session, ok
}

// generateRandomToken génère un token aléatoire de la taille spécifiée
func generateRandomToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
