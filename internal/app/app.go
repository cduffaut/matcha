package app

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/cduffaut/matcha/internal/auth"
	"github.com/cduffaut/matcha/internal/config"
	"github.com/cduffaut/matcha/internal/email"
	"github.com/cduffaut/matcha/internal/middleware"
	"github.com/cduffaut/matcha/internal/session"
	"github.com/cduffaut/matcha/internal/user"
	"goji.io"
	"goji.io/pat"
)

// App représente l'application Matcha
type App struct {
	config         *config.Config
	db             *sql.DB
	sessionManager *session.Manager
	mux            *goji.Mux
}

// New crée une nouvelle instance de l'application
func New(config *config.Config) *App {
	return &App{
		config: config,
		mux:    goji.NewMux(),
	}
}

// Initialize initialise l'application
func (a *App) Initialize() error {
	// Initialiser la base de données
	db, err := connectDB(a.config.Database)
	if err != nil {
		return fmt.Errorf("erreur lors de la connexion à la base de données: %w", err)
	}
	a.db = db

	// Créer le dossier d'uploads s'il n'existe pas
	uploadsDir := "web/static/uploads"
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		return fmt.Errorf("erreur lors de la création du dossier d'uploads: %w", err)
	}

	// Initialiser les services
	baseURL := fmt.Sprintf("http://localhost:%s", a.config.Server.Port)
	sessionManager := auth.NewSessionManager("matcha_session")
	a.sessionManager = sessionManager

	// Configurer les routes
	if err := a.setupRoutes(baseURL, uploadsDir); err != nil {
		return fmt.Errorf("erreur lors de la configuration des routes: %w", err)
	}

	return nil
}

// Run démarre le serveur HTTP
func (a *App) Run() error {
	serverAddr := fmt.Sprintf(":%s", a.config.Server.Port)
	log.Printf("Serveur démarré sur http://localhost%s\n", serverAddr)
	return http.ListenAndServe(serverAddr, a.mux)
}

// Close ferme toutes les ressources ouvertes
func (a *App) Close() {
	if a.db != nil {
		a.db.Close()
	}
}

// connectDB se connecte à la base de données
func connectDB(dbConfig config.DatabaseConfig) (*sql.DB, error) {
	// Code de connexion à la base de données...
	// Cela devrait aussi inclure l'exécution des migrations
	// Récupérez ce code depuis database.Connect et database.RunMigrations
	return nil, nil // Remplacer par le vrai code
}

// setupRoutes configure toutes les routes de l'application
func (a *App) setupRoutes(baseURL, uploadsDir string) error {
	// Initialiser les repositories
	userRepo := user.NewPostgresRepository(a.db)
	// profileRepo := user.NewPostgresProfileRepository(a.db)

	// Initialiser les services
	emailService := email.NewService(
		os.Getenv("SMTP_HOST"),
		os.Getenv("SMTP_PORT"),
		os.Getenv("SMTP_USERNAME"),
		os.Getenv("SMTP_PASSWORD"),
		os.Getenv("FROM_EMAIL"),
	)

	authService := auth.NewService(userRepo, emailService, baseURL)
	// profileService := user.NewProfileService(profileRepo, userRepo, uploadsDir)

	// Initialiser les handlers
	authHandlers := auth.NewHandlers(authService, a.sessionManager)
	// profileHandlers := user.NewProfileHandlers(profileService)

	// Initialiser les middlewares
	authMiddleware := middleware.NewAuthMiddleware(a.sessionManager)

	// Configurer les routes publiques
	a.setupPublicRoutes(authHandlers)

	// Configurer les routes protégées
	a.setupProtectedRoutes(authMiddleware)

	// Route pour les fichiers statiques
	a.mux.Handle(pat.Get("/static/*"), http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	a.mux.Handle(pat.Get("/uploads/*"), http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadsDir))))

	return nil
}

// setupPublicRoutes configure les routes publiques
func (a *App) setupPublicRoutes(authHandlers *auth.Handlers) {
	// Pages d'authentification
	a.mux.HandleFunc(pat.Get("/register"), authHandlers.RegisterPageHandler)
	a.mux.HandleFunc(pat.Get("/login"), authHandlers.LoginPageHandler)

	// API d'authentification
	a.mux.HandleFunc(pat.Post("/api/register"), authHandlers.RegisterHandler)
	a.mux.HandleFunc(pat.Get("/verify-email"), authHandlers.VerifyEmailHandler)
	a.mux.HandleFunc(pat.Post("/api/login"), authHandlers.LoginHandler)
	a.mux.HandleFunc(pat.Get("/logout"), authHandlers.LogoutHandler)
	a.mux.HandleFunc(pat.Post("/api/forgot-password"), authHandlers.ForgotPasswordHandler)
	a.mux.HandleFunc(pat.Post("/api/reset-password"), authHandlers.ResetPasswordHandler)

	// Page d'accueil
	a.mux.HandleFunc(pat.Get("/"), a.homeHandler)
}

// setupProtectedRoutes configure les routes protégées
func (a *App) setupProtectedRoutes(authMiddleware *middleware.AuthMiddleware) {
	// Routes protégées (nécessitant une authentification)
	protectedMux := goji.SubMux()
	protectedMux.Use(authMiddleware.RequireAuth)

	// TODO: Ajouter les gestionnaires pour le profil, etc.

	// Enregistrer les routes protégées
	a.mux.Handle(pat.New("/profile/*"), protectedMux)
}

// homeHandler gère la page d'accueil
func (a *App) homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
        <!DOCTYPE html>
        <html>
        <head>
            <title>Matcha</title>
            <meta name="viewport" content="width=device-width, initial-scale=1">
            <style>
                body {
                    font-family: Arial, sans-serif;
                    margin: 0;
                    padding: 0;
                    background-color: #f5f5f5;
                }
                header {
                    background-color: #4CAF50;
                    color: white;
                    padding: 1rem;
                    text-align: center;
                }
                .container {
                    max-width: 1200px;
                    margin: 0 auto;
                    padding: 1rem;
                }
                .hero {
                    text-align: center;
                    padding: 2rem 0;
                }
                .hero h1 {
                    font-size: 2.5rem;
                    margin-bottom: 1rem;
                }
                .hero p {
                    font-size: 1.2rem;
                    margin-bottom: 2rem;
                }
                .cta-button {
                    display: inline-block;
                    background-color: #4CAF50;
                    color: white;
                    padding: 0.8rem 1.5rem;
                    text-decoration: none;
                    border-radius: 4px;
                    font-weight: bold;
                    margin: 0 0.5rem;
                }
                footer {
                    background-color: #333;
                    color: white;
                    text-align: center;
                    padding: 1rem;
                    position: absolute;
                    bottom: 0;
                    width: 100%;
                }
            </style>
        </head>
        <body>
            <header>
                <h1>Matcha</h1>
            </header>
            <div class="container">
                <div class="hero">
                    <h1>Trouvez l'amour à portée de clic</h1>
                    <p>Parce que l'amour peut aussi être industrialisé.</p>
                    <a href="/register" class="cta-button">S'inscrire</a>
                    <a href="/login" class="cta-button">Se connecter</a>
                </div>
            </div>
            <footer>
                <p>&copy; 2025 Matcha - Tous droits réservés</p>
            </footer>
        </body>
        </html>
    `)
}
