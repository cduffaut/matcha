package auth

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cduffaut/matcha/internal/session"
)

// Handlers gère les requêtes HTTP pour l'authentification
type Handlers struct {
	service        *Service
	sessionManager *session.Manager
}

// NewHandlers crée des nouveaux gestionnaires pour l'authentification
func NewHandlers(service *Service, sessionManager *session.Manager) *Handlers {
	return &Handlers{
		service:        service,
		sessionManager: sessionManager,
	}
}

// RegisterHandler gère l'inscription
func (h *Handlers) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que la méthode est POST
	if r.Method != http.MethodPost {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	// Décoder le corps de la requête
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Format de requête invalide", http.StatusBadRequest)
		return
	}

	// Valider les données
	if req.Username == "" || req.Email == "" || req.FirstName == "" || req.LastName == "" || req.Password == "" {
		http.Error(w, "Tous les champs sont obligatoires", http.StatusBadRequest)
		return
	}

	// Enregistrer l'utilisateur
	user, err := h.service.Register(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Répondre avec succès
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Inscription réussie, veuillez vérifier votre adresse email",
		"user_id": user.ID,
	})
}

// VerifyEmailHandler gère la vérification d'email
func (h *Handlers) VerifyEmailHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer le token
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Token manquant", http.StatusBadRequest)
		return
	}

	// Vérifier l'email
	err := h.service.VerifyEmail(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Rediriger vers la page de connexion
	http.Redirect(w, r, "/login?verified=true", http.StatusFound)
}

// LoginHandler gère la connexion
func (h *Handlers) LoginHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que la méthode est POST
	if r.Method != http.MethodPost {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	fmt.Println("⚠️ Début LoginHandler")

	// Décoder le corps de la requête
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Printf("⚠️ Erreur décodage JSON: %v\n", err)
		http.Error(w, "Format de requête invalide", http.StatusBadRequest)
		return
	}

	fmt.Printf("⚠️ Requête de connexion reçue: %+v\n", req)
	fmt.Printf("⚠️ Tentative de connexion pour: %s\n", req.Username)

	// Connecter l'utilisateur
	user, err := h.service.Login(req)
	if err != nil {
		fmt.Printf("⚠️ Échec connexion: %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": err.Error(),
		})
		return
	}

	fmt.Printf("⚠️ Connexion réussie pour: %s (ID: %d)\n", user.Username, user.ID)

	// Créer une session
	sessionToken, err := h.sessionManager.CreateSession(w, user)
	if err != nil {
		fmt.Printf("⚠️ Erreur création session: %v\n", err)
		http.Error(w, "Erreur lors de la création de la session", http.StatusInternalServerError)
		return
	}

	fmt.Printf("⚠️ Session créée avec token: %s\n", sessionToken)

	// Répondre avec succès
	w.Header().Set("Content-Type", "application/json")
	respData := map[string]interface{}{
		"message": "Connexion réussie",
		"user": map[string]interface{}{
			"id":        user.ID,
			"username":  user.Username,
			"email":     user.Email,
			"firstName": user.FirstName,
			"lastName":  user.LastName,
		},
	}

	respJSON, err := json.Marshal(respData)
	if err != nil {
		fmt.Printf("⚠️ Erreur marshaling JSON: %v\n", err)
		http.Error(w, "Erreur interne", http.StatusInternalServerError)
		return
	}

	fmt.Printf("⚠️ Réponse envoyée: %s\n", string(respJSON))

	json.NewEncoder(w).Encode(respData)
}

// LogoutHandler gère la déconnexion
func (h *Handlers) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Détruire la session
	err := h.sessionManager.DestroySession(w, r)
	if err != nil {
		http.Error(w, "Erreur lors de la déconnexion", http.StatusInternalServerError)
		return
	}

	// Rediriger vers la page d'accueil
	http.Redirect(w, r, "/", http.StatusFound)
}

// ForgotPasswordHandler gère la demande de réinitialisation de mot de passe
func (h *Handlers) ForgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que la méthode est POST
	if r.Method != http.MethodPost {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	// Décoder le corps de la requête
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Format de requête invalide", http.StatusBadRequest)
		return
	}

	// Valider les données
	if req.Email == "" {
		http.Error(w, "L'adresse email est obligatoire", http.StatusBadRequest)
		return
	}

	// Envoyer l'email de réinitialisation
	err := h.service.ForgotPassword(req)
	if err != nil {
		// Ne pas révéler si l'email existe ou non
		http.Error(w, "Une erreur s'est produite", http.StatusInternalServerError)
		return
	}

	// Répondre avec succès
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Si l'adresse email existe, un lien de réinitialisation a été envoyé",
	})
}

// ResetPasswordHandler gère la réinitialisation de mot de passe
func (h *Handlers) ResetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que la méthode est POST
	if r.Method != http.MethodPost {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	// Décoder le corps de la requête
	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Format de requête invalide", http.StatusBadRequest)
		return
	}

	// Valider les données
	if req.Token == "" || req.Password == "" {
		http.Error(w, "Tous les champs sont obligatoires", http.StatusBadRequest)
		return
	}

	// Réinitialiser le mot de passe
	err := h.service.ResetPassword(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Répondre avec succès
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Mot de passe réinitialisé avec succès",
	})
}

// RegisterPageHandler affiche la page d'inscription
func (h *Handlers) RegisterPageHandler(w http.ResponseWriter, r *http.Request) {
	// À implémenter avec des templates HTML
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	fmt.Fprintf(w, `
        <!DOCTYPE html>
        <html>
        <head>
            <title>Inscription - Matcha</title>
            <meta name="viewport" content="width=device-width, initial-scale=1">
            <link rel="stylesheet" href="/static/css/auth.css">
        </head>
        <body>
            <div class="container">
                <h1>Inscription</h1>
                <form id="register-form">
                    <div class="form-group">
                        <label for="username">Nom d'utilisateur</label>
                        <input type="text" id="username" name="username" required>
                    </div>
                    <div class="form-group">
                        <label for="email">Email</label>
                        <input type="email" id="email" name="email" required>
                    </div>
                    <div class="form-group">
                        <label for="first_name">Prénom</label>
                        <input type="text" id="first_name" name="first_name" required>
                    </div>
                    <div class="form-group">
                        <label for="last_name">Nom</label>
                        <input type="text" id="last_name" name="last_name" required>
                    </div>
                    <div class="form-group">
                        <label for="password">Mot de passe</label>
                        <input type="password" id="password" name="password" required>
                    </div>
                    <button type="submit">S'inscrire</button>
                </form>
                <p>Déjà inscrit ? <a href="/login">Connexion</a></p>
            </div>
            <script src="/static/js/auth.js"></script>
        </body>
        </html>
    `)
}

// LoginPageHandler affiche la page de connexion
func (h *Handlers) LoginPageHandler(w http.ResponseWriter, r *http.Request) {
	// À implémenter avec des templates HTML
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	fmt.Fprintf(w, `
        <!DOCTYPE html>
        <html>
        <head>
            <title>Connexion - Matcha</title>
            <meta name="viewport" content="width=device-width, initial-scale=1">
            <link rel="stylesheet" href="/static/css/auth.css">
        </head>
        <body>
            <div class="container">
                <h1>Connexion</h1>
                <form id="login-form">
                    <div class="form-group">
                        <label for="username">Nom d'utilisateur</label>
                        <input type="text" id="username" name="username" required>
                    </div>
                    <div class="form-group">
                        <label for="password">Mot de passe</label>
                        <input type="password" id="password" name="password" required>
                    </div>
                    <button type="submit">Se connecter</button>
                </form>
                <p><a href="/forgot-password">Mot de passe oublié ?</a></p>
                <p>Pas encore inscrit ? <a href="/register">Inscription</a></p>
            </div>
            <script src="/static/js/auth.js"></script>
        </body>
        </html>
    `)
}
