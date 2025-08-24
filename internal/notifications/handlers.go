package notifications

import (
	"encoding/json"
	"html" // ✅ AJOUT de l'import html
	"net/http"
	"strconv"

	"github.com/cduffaut/matcha/internal/session"
	"goji.io/pat"
)

// Handlers gère les requêtes HTTP pour les notifications
type Handlers struct {
	service NotificationService
}

// NewHandlers crée de nouveaux handlers pour les notifications
func NewHandlers(service NotificationService) *Handlers {
	return &Handlers{
		service: service,
	}
}

// GetNotificationsHandler récupère les notifications de l'utilisateur connecté
func (h *Handlers) GetNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer le paramètre limit
	limitStr := r.URL.Query().Get("limit")
	limit := 20 // Valeur par défaut
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Récupérer les notifications
	notifications, err := h.service.GetNotifications(session.UserID, limit)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des notifications", http.StatusInternalServerError)
		return
	}

	// Répondre avec les notifications
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notifications)
}

// GetUnreadCountHandler récupère le nombre de notifications non lues
func (h *Handlers) GetUnreadCountHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer le nombre de notifications non lues
	count, err := h.service.GetUnreadCount(session.UserID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération du nombre de notifications", http.StatusInternalServerError)
		return
	}

	// Répondre avec le nombre
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"unread_count": count,
	})
}

// MarkAsReadHandler marque une notification comme lue
func (h *Handlers) MarkAsReadHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	_, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Récupérer l'ID de la notification
	notificationIDStr := pat.Param(r, "notificationID")
	notificationID, err := strconv.Atoi(notificationIDStr)
	if err != nil {
		http.Error(w, "ID de notification invalide", http.StatusBadRequest)
		return
	}

	// Marquer comme lue
	if err := h.service.MarkAsRead(notificationID); err != nil {
		http.Error(w, "Erreur lors du marquage de la notification", http.StatusInternalServerError)
		return
	}

	// Répondre avec succès
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Notification marquée comme lue",
	})
}

// MarkAllAsReadHandler marque toutes les notifications comme lues
func (h *Handlers) MarkAllAsReadHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	// Marquer toutes les notifications comme lues
	if err := h.service.MarkAllAsRead(session.UserID); err != nil {
		http.Error(w, "Erreur lors du marquage des notifications", http.StatusInternalServerError)
		return
	}

	// Répondre avec succès
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Toutes les notifications ont été marquées comme lues",
	})
}

// NotificationsPageHandler affiche la page des notifications
func (h *Handlers) NotificationsPageHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	session, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// Récupérer les notifications
	notifications, err := h.service.GetNotifications(session.UserID, 50)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des notifications", http.StatusInternalServerError)
		return
	}

	// Générer la page HTML
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.Write([]byte(generateNotificationsHTML(notifications)))
}

// generateNotificationsHTML génère le HTML pour la page des notifications
func generateNotificationsHTML(notifications []*Notification) string {
	htmlContent := `<!DOCTYPE html>
<html lang="fr">
<head>
    <title>Notifications - Matcha</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="stylesheet" href="/static/css/notifications.css">
	<link rel="stylesheet" href="/static/css/notifications_style_fix.css">
</head>
<body>
    <header>
        <h1>Matcha</h1>
		<nav>
			<a href="/profile">Mon profil</a>
			<a href="/browse">Explorer</a>
			<a href="/notifications">
				Notifications
				<span id="notification-count"></span>
			</a>
			<a href="/chat">
				Messages  
				<span id="message-count"></span>
			</a>
			<a href="/logout">Déconnexion</a>
		</nav>
    </header>
    
    <div class="container">
        <h2>Notifications</h2>
        <div class="notifications-actions">
            <button id="mark-all-read">Tout marquer comme lu</button>
        </div>
        <div class="notifications-list">`

	if len(notifications) == 0 {
		htmlContent += `<p class="no-notifications">Aucune notification pour le moment.</p>`
	} else {
		for _, notification := range notifications {
			readClass := ""
			if !notification.IsRead {
				readClass = "unread"
			}

			// ✅ SÉCURITÉ : Échapper les données utilisateur
			escapedUsername := html.EscapeString(notification.FromUser.Username)
			escapedMessage := html.EscapeString(notification.Message)

			htmlContent += `<div class="notification-item ` + readClass + `" data-id="` + strconv.Itoa(notification.ID) + `">
                <div class="notification-content">
                    <strong>` + escapedUsername + `</strong> ` + escapedMessage + `
                </div>
                <div class="notification-time">` + notification.CreatedAt.Format("02/01/2006 15:04") + `</div>
            </div>`
		}
	}

	htmlContent += `</div>
    </div>

	<script src="/static/js/global-error-handler.js"></script>
    <script src="/static/js/user_status.js"></script>
    <script src="/static/js/navigation_active.js"></script>
    <script src="/static/js/notifications_unified.js"></script>
    
    <!-- Script spécifique -->
    <script src="/static/js/profile.js"></script>
</body>
</html>`

	return htmlContent
}
