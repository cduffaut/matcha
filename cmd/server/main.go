package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/cduffaut/matcha/internal/auth"
	"github.com/cduffaut/matcha/internal/config"
	"github.com/cduffaut/matcha/internal/database"
	"github.com/cduffaut/matcha/internal/email"
	"github.com/cduffaut/matcha/internal/middleware"
	"github.com/cduffaut/matcha/internal/session"
	"github.com/cduffaut/matcha/internal/user"
	"goji.io"
	"goji.io/pat"
)

func main() {
	// Charger la configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Erreur lors du chargement de la configuration: %v", err)
	}

	// Initialiser la base de données
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Erreur lors de la connexion à la base de données: %v", err)
	}
	defer db.Close()

	// Exécuter les migrations
	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("Erreur lors de l'exécution des migrations: %v", err)
	}

	// Initialiser les repositories
	userRepo := user.NewPostgresRepository(db)
	profileRepo := user.NewPostgresProfileRepository(db)

	// Initialiser les services
	emailService := email.NewService(
		os.Getenv("SMTP_HOST"),
		os.Getenv("SMTP_PORT"),
		os.Getenv("SMTP_USERNAME"),
		os.Getenv("SMTP_PASSWORD"),
		os.Getenv("FROM_EMAIL"),
	)

	baseURL := fmt.Sprintf("http://localhost:%s", cfg.Server.Port)
	authService := auth.NewService(userRepo, emailService, baseURL)
	sessionManager := session.NewManager("matcha_session")

	// Service de profil
	profileService := user.NewProfileService(profileRepo, userRepo, "web/static/uploads")

	// Initialiser les handlers
	authHandlers := auth.NewHandlers(authService, sessionManager)
	profileHandlers := user.NewProfileHandlers(profileService)
	browsingService := user.NewBrowsingService(userRepo, profileRepo)
	browsingHandlers := user.NewBrowsingHandlers(browsingService)

	// Initialiser les middlewares
	authMiddleware := middleware.NewAuthMiddleware(sessionManager)

	// Créer le multiplexeur Goji
	mux := goji.NewMux()

	// Route pour les fichiers statiques
	fileServer := http.FileServer(http.Dir("web/static"))
	mux.Handle(pat.Get("/static/*"), http.StripPrefix("/static/", fileServer))
	// Route pour les uploads (AJOUT IMPORTANT)
	uploadsServer := http.FileServer(http.Dir("web/static/uploads"))
	mux.Handle(pat.Get("/uploads/*"), http.StripPrefix("/uploads/", uploadsServer))

	// Pages d'authentification
	mux.HandleFunc(pat.Get("/register"), authHandlers.RegisterPageHandler)
	mux.HandleFunc(pat.Get("/login"), authHandlers.LoginPageHandler)

	// API d'authentification
	mux.HandleFunc(pat.Post("/api/register"), authHandlers.RegisterHandler)
	mux.HandleFunc(pat.Get("/verify-email"), authHandlers.VerifyEmailHandler)
	mux.HandleFunc(pat.Post("/api/login"), authHandlers.LoginHandler)
	mux.HandleFunc(pat.Get("/logout"), authHandlers.LogoutHandler)
	mux.HandleFunc(pat.Post("/api/forgot-password"), authHandlers.ForgotPasswordHandler)
	mux.HandleFunc(pat.Post("/api/reset-password"), authHandlers.ResetPasswordHandler)

	// Route pour la page d'accueil
	mux.HandleFunc(pat.Get("/"), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
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
	})

	// Routes protégées
	protectedMux := goji.SubMux()
	protectedMux.Use(authMiddleware.RequireAuth)

	// Routes pour le profil
	protectedMux.HandleFunc(pat.Get("/profile"), profileHandlers.ProfilePageHandler)
	protectedMux.HandleFunc(pat.Get("/api/profile"), profileHandlers.GetProfileHandler)
	protectedMux.HandleFunc(pat.Put("/api/profile"), profileHandlers.UpdateProfileHandler)

	// Routes pour les tags
	protectedMux.HandleFunc(pat.Get("/api/profile/tags"), profileHandlers.GetTagsHandler)
	protectedMux.HandleFunc(pat.Post("/api/profile/tags"), profileHandlers.AddTagHandler)
	protectedMux.HandleFunc(pat.Delete("/api/profile/tags/:tagID"), profileHandlers.RemoveTagHandler)
	protectedMux.HandleFunc(pat.Get("/api/tags"), profileHandlers.GetAllTagsHandler)

	// Routes pour les photos
	protectedMux.HandleFunc(pat.Get("/api/profile/photos"), profileHandlers.GetPhotosHandler)
	protectedMux.HandleFunc(pat.Post("/api/profile/photos"), profileHandlers.UploadPhotoHandler)
	protectedMux.HandleFunc(pat.Delete("/api/profile/photos/:photoID"), profileHandlers.DeletePhotoHandler)
	protectedMux.HandleFunc(pat.Put("/api/profile/photos/:photoID/set-profile"), profileHandlers.SetProfilePhotoHandler)

	// Routes pour visualiser d'autres profils
	protectedMux.HandleFunc(pat.Get("/profile/:userID"), profileHandlers.GetUserProfileHandler)
	protectedMux.HandleFunc(pat.Get("/profile/visitors"), profileHandlers.GetVisitorsHandler)
	protectedMux.HandleFunc(pat.Get("/profile/likes"), profileHandlers.GetLikesHandler)

	// Routes pour les likes
	protectedMux.HandleFunc(pat.Post("/api/profile/:userID/like"), profileHandlers.LikeUserHandler)
	protectedMux.HandleFunc(pat.Delete("/api/profile/:userID/like"), profileHandlers.UnlikeUserHandler)

	// Routes pour la navigation
	protectedMux.HandleFunc(pat.Get("/browse"), browsingHandlers.BrowsePageHandler)
	protectedMux.HandleFunc(pat.Get("/api/suggestions"), browsingHandlers.GetSuggestionsHandler)
	protectedMux.HandleFunc(pat.Get("/api/search"), browsingHandlers.SearchProfilesHandler)

	// Ajouter les routes protégées au mux principal
	mux.Handle(pat.New("/*"), protectedMux)

	// Démarrer le serveur
	serverAddr := fmt.Sprintf(":%s", cfg.Server.Port)
	log.Printf("Serveur démarré sur http://localhost%s\n", serverAddr)
	log.Fatal(http.ListenAndServe(serverAddr, mux))
}
