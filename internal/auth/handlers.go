package auth

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strings"

    "github.com/cduffaut/matcha/internal/security"
    "github.com/cduffaut/matcha/internal/session"
    "github.com/cduffaut/matcha/internal/user"
    "github.com/cduffaut/matcha/internal/validation"
)

// gere requetes HTTP pour l'auth
type Handlers struct {
    service        *Service
    sessionManager *session.Manager
    profileService *user.ProfileService
}

// cree des news gestionnaires pour l'auth
func NewHandlers(service *Service, sessionManager *session.Manager, profileService *user.ProfileService) *Handlers {
    return &Handlers{
        service:        service,
        sessionManager: sessionManager,
        profileService: profileService,
    }
}

// gere l'inscription
func (h *Handlers) RegisterHandler(w http.ResponseWriter, r *http.Request) {
    // ✅ TOUJOURS définir le Content-Type en premier
    w.Header().Set("Content-Type", "application/json")

    if r.Method != http.MethodPost {
        w.WriteHeader(http.StatusMethodNotAllowed)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Méthode non autorisée",
        })
        return
    }

    var req RegisterRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Format de requête invalide",
        })
        return
    }

    // nettoyer les entrees
    req.Username = validation.SanitizeInput(req.Username)
    req.Email = validation.SanitizeInput(req.Email)
    req.FirstName = validation.SanitizeInput(req.FirstName)
    req.LastName = validation.SanitizeInput(req.LastName)

    // valider les champs avec messages directs
    validationErrors := validation.ValidateRegistration(
        req.Username, req.Email, req.FirstName, req.LastName, req.Password)

    if len(validationErrors) > 0 {
        w.WriteHeader(http.StatusBadRequest)

        firstError := validationErrors[0].Message
        json.NewEncoder(w).Encode(map[string]string{
            "error": firstError,
        })
        return
    }

    // verif les injections SQL
    for _, field := range []struct{ value, name string }{
        {req.Username, "username"},
        {req.Email, "email"},
        {req.FirstName, "firstname"},
        {req.LastName, "lastname"},
    } {
        if err := security.ValidateUserInput(field.value, field.name); err != nil {
            security.LogSuspiciousActivity(0, field.value, "/api/register")
            w.WriteHeader(http.StatusBadRequest)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Données invalides détectées",
            })
            return
        }
    }

    // enregistrer le user
    user, err := h.service.Register(req)
    if err != nil {
        w.WriteHeader(http.StatusBadRequest)

        errorMessage := err.Error()
        if strings.Contains(errorMessage, "nom d'utilisateur existe déjà") {
            errorMessage = "Ce nom d'utilisateur est déjà pris"
        } else if strings.Contains(errorMessage, "email est déjà utilisé") {
            errorMessage = "Cette adresse email est déjà utilisée"
        }

        json.NewEncoder(w).Encode(map[string]string{
            "error": errorMessage,
        })
        return
    }

    // si succes
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "message": "Inscription réussie ! Un email de vérification a été envoyé",
        "user_id": user.ID,
    })
}

// gere la verif d'email
func (h *Handlers) VerifyEmailHandler(w http.ResponseWriter, r *http.Request) {
    // recup le token
    token := r.URL.Query().Get("token")
    if token == "" {
        http.Error(w, "Token manquant", http.StatusBadRequest)
        return
    }

    // verif l'email
    err := h.service.VerifyEmail(token)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // redirige vers la page de connexion
    http.Redirect(w, r, "/login?verified=true", http.StatusFound)
}

// gere la connexion
func (h *Handlers) LoginHandler(w http.ResponseWriter, r *http.Request) {
    // verif que la methode est POST
    if r.Method != http.MethodPost {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusMethodNotAllowed)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Méthode non autorisée",
        })
        return
    }

    // decoder le corps de la request
    var req LoginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Format de requête invalide",
        })
        return
    }

    // nettoie les entrees
    req.Username = validation.SanitizeInput(req.Username)

    // valide les champs
    if err := validation.ValidateUsername(req.Username); err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Nom d'utilisateur invalide",
        })
        return
    }

    if len(req.Password) == 0 {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Mot de passe requis",
        })
        return
    }

    // verifier les injections SQL
    if err := security.ValidateUserInput(req.Username, "username"); err != nil {
        security.LogSuspiciousActivity(0, req.Username, "/api/login")
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Données invalides détectées",
        })
        return
    }

    // connecter le user
    user, err := h.service.Login(req)
    if err != nil {
        // retourner JSON positive
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusUnauthorized) // 401 au lieu de 500
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Nom d'utilisateur ou mot de passe incorrect",
        })
        return
    }

    // creer une session
    _, err = h.sessionManager.CreateSession(w, user)
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Erreur lors de la création de la session",
        })
        return
    }

    if h.profileService != nil {
        go func() {
            // M à J du statut en ligne et la dernière connexion
            _ = h.profileService.UpdateUserOnlineStatus(user.ID, true)
            _ = h.profileService.UpdateLastConnection(user.ID)
        }()
    }

    // réponse JSON valide
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "message": "Connexion réussie",
        "user": map[string]interface{}{
            "id":        user.ID,
            "username":  user.Username,
            "email":     user.Email,
            "firstName": user.FirstName,
            "lastName":  user.LastName,
        },
    })
}

// gere la déconnexion
func (h *Handlers) LogoutHandler(w http.ResponseWriter, r *http.Request) {

    // recup la session directement du sessionManager
    // au lieu d'utiliser le contexte qui n'est pas disponible
    userSession, err := h.sessionManager.GetSession(r)
    if err == nil && userSession != nil && h.profileService != nil {

        // marquer hors ligne en synchrone
        h.profileService.UpdateUserOnlineStatus(userSession.UserID, false)

        // lire le statut juste apres
        h.profileService.GetUserOnlineStatus(userSession.UserID)
    }

    // detruire la session
    err = h.sessionManager.DestroySession(w, r)
    if err != nil {
        http.Error(w, "Erreur lors de la déconnexion", http.StatusInternalServerError)
        return
    }

    // rediriger vers la homepage
    http.Redirect(w, r, "/", http.StatusFound)
}

// gere la demande de reinitialisation
func (h *Handlers) ForgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
    // def le ContentType JSON en first
    w.Header().Set("Content-Type", "application/json")

    if r.Method != http.MethodPost {
        w.WriteHeader(http.StatusMethodNotAllowed)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Méthode non autorisée",
        })
        return
    }

    var req ForgotPasswordRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Format de requête invalide",
        })
        return
    }

    // clean et valider email
    req.Email = validation.SanitizeInput(req.Email)

    if err := validation.ValidateEmail(req.Email); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Adresse email invalide",
        })
        return
    }

    if err := security.ValidateUserInput(req.Email, "email"); err != nil {
        security.LogSuspiciousActivity(0, req.Email, "/api/forgot-password")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Données invalides détectées",
        })
        return
    }

    // env l'email de reinitialisation
    err := h.service.ForgotPassword(req)
    if err != nil {
        // ne pas reveler si l'email existe ou non
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Une erreur s'est produite",
        })
        return
    }

    // repondre avec succes en JSON
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "success": "true",
        "message": "Si l'adresse email existe, un lien de réinitialisation a été envoyé",
    })
}

// gere la reinitialisation
func (h *Handlers) ResetPasswordHandler(w http.ResponseWriter, r *http.Request) {
    // def le Content Type JSON en first
    w.Header().Set("Content-Type", "application/json")

    if r.Method != http.MethodPost {
        w.WriteHeader(http.StatusMethodNotAllowed)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Méthode non autorisée",
        })
        return
    }

    var req ResetPasswordRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Format de requête invalide",
        })
        return
    }

    // valider le token et le mdp
    req.Token = validation.SanitizeInput(req.Token)

    if req.Token == "" || req.Password == "" {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Tous les champs sont obligatoires",
        })
        return
    }

    if err := validation.ValidatePassword(req.Password); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": err.Error(),
        })
        return
    }

    // reinitialiser le mdp
    err := h.service.ResetPassword(req)
    if err != nil {
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Token invalide ou expiré",
        })
        return
    }

    // rep avec succes en JSON
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "success": "true",
        "message": "Mot de passe réinitialisé avec succès",
    })
}

// affiche la page d'inscription
func (h *Handlers) RegisterPageHandler(w http.ResponseWriter, r *http.Request) {
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
                        <input type="text" id="username" name="username" required autocomplete="username">
                    </div>
                    <div class="form-group">
                        <label for="email">Email</label>
                        <input type="email" id="email" name="email" required autocomplete="email">
                    </div>
                    <div class="form-group">
                        <label for="first_name">Prénom</label>
                        <input type="text" id="first_name" name="first_name" required autocomplete="given-name">
                    </div>
                    <div class="form-group">
                        <label for="last_name">Nom</label>
                        <input type="text" id="last_name" name="last_name" required autocomplete="family-name">
                    </div>
                    <div class="form-group">
                        <label for="password">Mot de passe</label>
                        <input type="password" id="password" name="password" required autocomplete="new-password">
                    </div>
                    <div class="form-group">
                        <label for="confirm_password">Confirmer le mot de passe</label>
                        <input type="password" id="confirm_password" name="confirm_password" required autocomplete="new-password">
                    </div>
                    <button type="submit">S'inscrire</button>
                </form>
                <p>Déjà inscrit ? <a href="/login">Connexion</a></p>
            </div>
            <script src="/static/js/global-error-handler.js"></script>
            <script src="/static/js/auth.js"></script>
        </body>
        </html>
    `)
}

// affiche la page de connexion
func (h *Handlers) LoginPageHandler(w http.ResponseWriter, r *http.Request) {
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
                        <input type="text" id="username" name="username" required autocomplete="username">
                    </div>
                    <div class="form-group">
                        <label for="password">Mot de passe</label>
                        <input type="password" id="password" name="password" required autocomplete="current-password">
                    </div>
                    <button type="submit">Se connecter</button>
                </form>
                <p><a href="/forgot-password">Mot de passe oublié ?</a></p>
                <p>Pas encore inscrit ? <a href="/register">Inscription</a></p>
            </div>
            <script src="/static/js/global-error-handler.js"></script>
            <script src="/static/js/auth.js"></script>
        </body>
        </html>
    `)
}

// affiche la page de recuperation de mdp
func (h *Handlers) ForgotPasswordPageHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html; charset=UTF-8")
    fmt.Fprintf(w, `
        <!DOCTYPE html>
        <html>
        <head>
            <title>Mot de passe oublié - Matcha</title>
            <meta name="viewport" content="width=device-width, initial-scale=1">
            <link rel="stylesheet" href="/static/css/auth.css">
        </head>
        <body>
            <div class="container">
                <h1>Mot de passe oublié</h1>
                <p>Saisissez votre adresse email pour recevoir un lien de réinitialisation.</p>
                <form id="forgot-password-form">
                    <div class="form-group">
                        <label for="email">Email</label>
                        <input type="email" id="email" name="email" required autocomplete="email">
                    </div>
                    <button type="submit">Envoyer</button>
                </form>
                <p><a href="/login">← Retour à la connexion</a></p>
            </div>
            <script src="/static/js/global-error-handler.js"></script>
            <script src="/static/js/auth.js"></script>
        </body>
        </html>
    `)
}

// affiche la page de reinitialisation
func (h *Handlers) ResetPasswordPageHandler(w http.ResponseWriter, r *http.Request) {
    token := r.URL.Query().Get("token")
    if token == "" {
        http.Error(w, "Token manquant", http.StatusBadRequest)
        return
    }

    w.Header().Set("Content-Type", "text/html; charset=UTF-8")
    fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Réinitialiser mot de passe - Matcha</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="stylesheet" href="/static/css/auth.css">
</head>
<body>
    <div class="container">
        <h1>Nouveau mot de passe</h1>
        <form id="reset-password-form">
            <input type="hidden" id="token" value="%s">
            <div class="form-group">
                <label for="password">Nouveau mot de passe</label>
                <input type="password" id="password" name="password" required autocomplete="new-password">
                <small>Minimum 8 caractères, 1 majuscule, 1 minuscule, 1 chiffre</small>
            </div>
            <div class="form-group">
                <label for="confirm-password">Confirmer le mot de passe</label>
                <input type="password" id="confirm-password" name="confirm-password" required autocomplete="new-password">
            </div>
            <button type="submit">Réinitialiser</button>
        </form>
    </div>
    <script>
        document.getElementById('reset-password-form').addEventListener('submit', async function(e) {
            e.preventDefault();
            
            const password = document.getElementById('password').value;
            const confirmPassword = document.getElementById('confirm-password').value;
            const token = document.getElementById('token').value;
            
            if (password !== confirmPassword) {
                alert('Les mots de passe ne correspondent pas');
                return;
            }
            
            try {
                const response = await fetch('/api/reset-password', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({token: token, password: password}),
                });
                
                const data = await response.json();
                
                if (response.ok) {
                    alert('Mot de passe réinitialisé avec succès !');
                    window.location.href = '/login';
                } else {
                    alert(data.error || 'Erreur lors de la réinitialisation');
                }
            } catch (error) {
                alert('Erreur de connexion');
            }
        });
    </script>
</body>
</html>
    `, token)
}

// gere la m à j des infos user (nom, prenom, email)
func (h *Handlers) UpdateUserInfoHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPut {
        http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
        return
    }

    // recup la session utilisateur
    userSession, ok := session.FromContext(r.Context())
    if !ok {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Utilisateur non connecté",
        })
        return
    }

    // decoder la requete
    var req UpdateUserInfoRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "Format de requête invalide",
        })
        return
    }

    // nettoyer les entrees
    req.FirstName = validation.SanitizeInput(req.FirstName)
    req.LastName = validation.SanitizeInput(req.LastName)
    req.Email = validation.SanitizeInput(req.Email)

    // verifier les injections SQL
    for _, field := range []struct{ value, name string }{
        {req.FirstName, "firstname"},
        {req.LastName, "lastname"},
        {req.Email, "email"},
    } {
        if err := security.ValidateUserInput(field.value, field.name); err != nil {
            security.LogSuspiciousActivity(userSession.UserID, field.value, "/api/user/update")
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusBadRequest)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Données invalides détectées",
            })
            return
        }
    }

    // m à j les informations
    err := h.service.UpdateUserInfo(userSession.UserID, req)
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]string{
            "error": err.Error(),
        })
        return
    }

    // rep succes
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "message": "Informations mises à jour avec succès",
    })
}
