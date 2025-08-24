package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/cduffaut/matcha/internal/auth"
	"github.com/cduffaut/matcha/internal/chat"
	"github.com/cduffaut/matcha/internal/config"
	"github.com/cduffaut/matcha/internal/database"
	"github.com/cduffaut/matcha/internal/email"
	"github.com/cduffaut/matcha/internal/middleware"
	"github.com/cduffaut/matcha/internal/notifications"
	"github.com/cduffaut/matcha/internal/session"
	"github.com/cduffaut/matcha/internal/user"
	"goji.io"
	"goji.io/pat"
)

func main() {
	// charger la config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Erreur lors du chargement de la configuration: %v", err)
	}

	// initialiser la DB
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Erreur lors de la connexion à la base de données: %v", err)
	}
	defer db.Close()

	// exec les migrations
	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("Erreur lors de l'exécution des migrations: %v", err)
	}

	// init les repos
	userRepo := user.NewPostgresRepository(db)
	profileRepo := user.NewPostgresProfileRepository(db)

	// init les services
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

	// init le sys de notifs
	notificationRepo := notifications.NewPostgresNotificationRepository(db)
	notificationService := notifications.NewService(notificationRepo)
	notificationHandlers := notifications.NewHandlers(notificationService)

	// service de profil
	profileService := user.NewProfileService(profileRepo, userRepo, "web/static/uploads", notificationService)
	onlineStatusMiddleware := middleware.NewOnlineStatusMiddleware(profileService)

	chatHub := &chat.Hub{
		Clients:    make(map[int]*chat.Client),
		Register:   make(chan *chat.Client),
		Unregister: make(chan *chat.Client),
		Broadcast:  make(chan []byte),
	}

	// init les handlers
	authHandlers := auth.NewHandlers(authService, sessionManager, profileService)
	profileHandlers := user.NewProfileHandlers(profileService, notificationService, chatHub)
	browsingService := user.NewBrowsingService(userRepo, profileRepo)
	browsingHandlers := user.NewBrowsingHandlers(browsingService)

	// Démarrer le nettoyage périodique
	onlineStatusMiddleware.StartCleanupRoutine()

	// init le sys de chat
	chatRepo := chat.NewPostgresMessageRepository(db)
	chatService := chat.NewService(chatRepo, notificationService)
	chatHandlers := chat.NewHandlers(chatService, chatHub)

	go chatHub.Run()
	// init les middlewares
	authMiddleware := middleware.NewAuthMiddleware(sessionManager)

	// creation multiplexeur goji
	mux := goji.NewMux()
	mux.Use(middleware.CSRFMiddleware)

	// route fichiers statiques
	fileServer := http.FileServer(http.Dir("web/static"))
	mux.Handle(pat.Get("/static/*"), http.StripPrefix("/static/", fileServer))
	// route pour uploads
	uploadsServer := http.FileServer(http.Dir("web/static/uploads"))
	mux.Handle(pat.Get("/uploads/*"), http.StripPrefix("/uploads/", uploadsServer))

	// page d'auth
	mux.HandleFunc(pat.Get("/register"), authHandlers.RegisterPageHandler)
	mux.HandleFunc(pat.Get("/login"), authHandlers.LoginPageHandler)

	// API d'auth
	mux.HandleFunc(pat.Post("/api/register"), authHandlers.RegisterHandler)
	mux.HandleFunc(pat.Get("/verify-email"), authHandlers.VerifyEmailHandler)
	mux.HandleFunc(pat.Post("/api/login"), authHandlers.LoginHandler)
	mux.HandleFunc(pat.Get("/logout"), authHandlers.LogoutHandler)
	mux.HandleFunc(pat.Post("/api/forgot-password"), authHandlers.ForgotPasswordHandler)
	mux.HandleFunc(pat.Post("/api/reset-password"), authHandlers.ResetPasswordHandler)

	// route pour la homepage
	mux.HandleFunc(pat.Get("/"), homeHandler)

	// routes publiques
	mux.HandleFunc(pat.Get("/forgot-password"), authHandlers.ForgotPasswordPageHandler)
	mux.HandleFunc(pat.Get("/reset-password"), authHandlers.ResetPasswordPageHandler)

	// routes protegees
	protectedMux := goji.SubMux()
	protectedMux.Use(authMiddleware.RequireAuth)
	protectedMux.Use(onlineStatusMiddleware.UpdateOnlineStatus)
	// modifier infos users
	protectedMux.HandleFunc(pat.Put("/api/user/update"), authHandlers.UpdateUserInfoHandler)

	// routes profil
	protectedMux.HandleFunc(pat.Get("/profile"), profileHandlers.ProfilePageHandler)
	protectedMux.HandleFunc(pat.Get("/profile/visitors"), profileHandlers.VisitorsPageHandler)
	protectedMux.HandleFunc(pat.Get("/profile/likes"), profileHandlers.LikesPageHandler)
	protectedMux.HandleFunc(pat.Get("/profile/blocked"), profileHandlers.BlockedUsersPageHandler)

	// voir profil autre user
	protectedMux.HandleFunc(pat.Get("/profile/:userID"), profileHandlers.ViewUserProfilePageHandler)
	protectedMux.HandleFunc(pat.Get("/api/profile"), profileHandlers.GetProfileHandler)
	protectedMux.HandleFunc(pat.Put("/api/profile"), profileHandlers.UpdateProfileHandler)
	protectedMux.HandleFunc(pat.Get("/api/profile/:userID"), profileHandlers.GetUserProfileHandler)

	protectedMux.HandleFunc(pat.Get("/api/profile/:userID/status"), profileHandlers.GetUserStatusHandler)
	protectedMux.HandleFunc(pat.Post("/api/profile/:userID/report"), profileHandlers.ReportUserHandler)

	// routes pour tags
	protectedMux.HandleFunc(pat.Get("/api/profile/tags"), profileHandlers.GetTagsHandler)
	protectedMux.HandleFunc(pat.Post("/api/profile/tags"), profileHandlers.AddTagHandler)
	protectedMux.HandleFunc(pat.Delete("/api/profile/tags/:tagID"), profileHandlers.RemoveTagHandler)
	protectedMux.HandleFunc(pat.Get("/api/tags"), profileHandlers.GetAllTagsHandler)

	// routes pour photos
	protectedMux.HandleFunc(pat.Get("/api/profile/photos"), profileHandlers.GetPhotosHandler)
	protectedMux.HandleFunc(pat.Post("/api/profile/photos"), profileHandlers.UploadPhotoHandler)
	protectedMux.HandleFunc(pat.Delete("/api/profile/photos/:photoID"), profileHandlers.DeletePhotoHandler)
	protectedMux.HandleFunc(pat.Put("/api/profile/photos/:photoID/set-profile"), profileHandlers.SetProfilePhotoHandler)

	// routes pour likes
	protectedMux.HandleFunc(pat.Post("/api/profile/:userID/like"), profileHandlers.LikeUserHandler)
	protectedMux.HandleFunc(pat.Delete("/api/profile/:userID/like"), profileHandlers.UnlikeUserHandler)

	// routes pour navigation
	protectedMux.HandleFunc(pat.Get("/browse"), browsingHandlers.BrowsePageHandler)
	protectedMux.HandleFunc(pat.Get("/api/suggestions"), browsingHandlers.GetSuggestionsHandler)
	protectedMux.HandleFunc(pat.Get("/api/search"), browsingHandlers.SearchProfilesHandler)

	// routes pour notifications
	protectedMux.HandleFunc(pat.Get("/api/notifications"), notificationHandlers.GetNotificationsHandler)
	protectedMux.HandleFunc(pat.Get("/api/notifications/unread-count"), notificationHandlers.GetUnreadCountHandler)
	protectedMux.HandleFunc(pat.Put("/api/notifications/:notificationID/read"), notificationHandlers.MarkAsReadHandler)
	protectedMux.HandleFunc(pat.Post("/api/notifications/mark-all-read"), notificationHandlers.MarkAllAsReadHandler)
	protectedMux.HandleFunc(pat.Get("/notifications"), notificationHandlers.NotificationsPageHandler)

	// routes pour le chat
	protectedMux.HandleFunc(pat.Get("/chat"), chatHandlers.ChatPageHandler)
	protectedMux.HandleFunc(pat.Get("/ws"), chatHandlers.WebSocketHandler)
	protectedMux.HandleFunc(pat.Get("/api/chat/conversations"), chatHandlers.GetConversationsHandler)
	protectedMux.HandleFunc(pat.Get("/api/chat/conversation/:userID"), chatHandlers.GetConversationHandler)
	protectedMux.HandleFunc(pat.Post("/api/chat/send"), chatHandlers.SendMessageHandler)
	protectedMux.HandleFunc(pat.Put("/api/chat/conversation/:userID/read"), chatHandlers.MarkAsReadHandler)
	protectedMux.HandleFunc(pat.Get("/api/chat/unread-count"), chatHandlers.GetUnreadCountHandler)

	// routes pour blocage users
	protectedMux.HandleFunc(pat.Post("/api/profile/:userID/block"), profileHandlers.BlockUserHandler)
	protectedMux.HandleFunc(pat.Delete("/api/profile/:userID/block"), profileHandlers.UnblockUserHandler)
	protectedMux.HandleFunc(pat.Get("/api/profile/blocked"), profileHandlers.GetBlockedUsersHandler)

	// routes pour online status et signalement
	protectedMux.HandleFunc(pat.Get("/api/profile/:userID/status"), profileHandlers.GetUserOnlineStatusHandler)
	protectedMux.HandleFunc(pat.Post("/api/profile/:userID/report"), profileHandlers.ReportUserHandler)
	protectedMux.HandleFunc(pat.Get("/api/geolocation"), profileHandlers.IPGeolocationHandler)

	// rep vide pour fav icon pour eviter erreur
	mux.HandleFunc(pat.Get("/favicon.ico"), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		w.WriteHeader(http.StatusNoContent) // 204 No Content
	})

	// add les routes protegees au mux principal
	mux.Handle(pat.New("/*"), protectedMux)

	// start le serv
	serverAddr := fmt.Sprintf(":%s", cfg.Server.Port)
	log.Printf("Serveur démarré sur http://localhost%s\n", serverAddr)
	log.Fatal(http.ListenAndServe(serverAddr, mux))
}

// homeHandler gère la homepage
func homeHandler(w http.ResponseWriter, r *http.Request) {
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
                    display: flex;
                    flex-direction: column;
                    min-height: 100vh;
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
                    flex: 1;
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
                    padding: 2rem 1rem;
                    margin-top: auto;
                }
                footer p {
                    margin: 0.5rem 0;
                    font-size: 14px;
                }
                footer a {
                    color: #4CAF50;
                    text-decoration: none;
                    margin: 0 1rem;
                }
                footer a:hover {
                    text-decoration: underline;
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
                <div class="footer-links">
                    <a href="/about">À propos</a>
                    <a href="/privacy">Confidentialité</a>
                    <a href="/terms">Conditions d'utilisation</a>
                    <a href="/contact">Contact</a>
                </div>
                <p>&copy; 2025 Matcha - Tous droits réservés</p>
                <p>Trouvez l'amour, partagez vos passions</p>
            </footer>
            <script src="/static/js/global-error-handler.js"></script>
        </body>
        </html>
    `)
}
