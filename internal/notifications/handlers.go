package notifications

import (
	"encoding/json"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/cduffaut/matcha/internal/session"
	"goji.io/pat"
)

type Handlers struct {
	service NotificationService
}

func NewHandlers(service NotificationService) *Handlers { return &Handlers{service: service} }

/* helpers */

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func requireSession(r *http.Request) (*session.Session, bool) { return session.FromContext(r.Context()) }

func qInt(r *http.Request, key string, def int) int {
	if s := r.URL.Query().Get(key); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			return v
		}
	}
	return def
}

/* API */

func (h *Handlers) GetNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(r)
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}
	limit := qInt(r, "limit", 20)

	ns, err := h.service.GetNotifications(sess.UserID, limit)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des notifications", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, ns)
}

func (h *Handlers) GetUnreadCountHandler(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(r)
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}

	count, err := h.service.GetUnreadCount(sess.UserID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération du nombre de notifications", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"unread_count": count})
}

func (h *Handlers) MarkAsReadHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(r); !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}
	id, err := strconv.Atoi(pat.Param(r, "notificationID"))
	if err != nil {
		http.Error(w, "ID de notification invalide", http.StatusBadRequest)
		return
	}
	if err := h.service.MarkAsRead(id); err != nil {
		http.Error(w, "Erreur lors du marquage de la notification", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Notification marquée comme lue"})
}

func (h *Handlers) MarkAllAsReadHandler(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(r)
	if !ok {
		http.Error(w, "Utilisateur non connecté", http.StatusUnauthorized)
		return
	}
	if err := h.service.MarkAllAsRead(sess.UserID); err != nil {
		http.Error(w, "Erreur lors du marquage des notifications", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Toutes les notifications ont été marquées comme lues"})
}

func (h *Handlers) NotificationsPageHandler(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	ns, err := h.service.GetNotifications(sess.UserID, 50)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des notifications", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	_, _ = io.WriteString(w, generateNotificationsHTML(ns))
}

/* HTML */

func generateNotificationsHTML(ns []*Notification) string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="fr">
<head>
  <title>Notifications - Matcha</title>
  <meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" href="/static/css/notifications.css">
  <link rel="stylesheet" href="/static/css/notifications_style_fix.css">
</head>
<body>
  <header>
    <h1>Matcha</h1>
    <nav>
      <a href="/profile">Mon profil</a>
      <a href="/browse">Explorer</a>
      <a href="/notifications">Notifications <span id="notification-count"></span></a>
      <a href="/chat">Messages  <span id="message-count"></span></a>
      <a href="/logout">Déconnexion</a>
    </nav>
  </header>
  <div class="container">
    <h2>Notifications</h2>
    <div class="notifications-actions">
      <button id="mark-all-read">Tout marquer comme lu</button>
    </div>
    <div class="notifications-list">`)

	if len(ns) == 0 {
		b.WriteString(`<p class="no-notifications">Aucune notification pour le moment.</p>`)
	} else {
		for _, n := range ns {
			readClass := ""
			if !n.IsRead {
				readClass = "unread"
			}
			username := ""
			if n.FromUser != nil {
				username = n.FromUser.Username
			}
			b.WriteString(`<div class="notification-item ` + readClass + `" data-id="` + strconv.Itoa(n.ID) + `">`)
			b.WriteString(`<div class="notification-content"><strong>` + html.EscapeString(username) + `</strong> ` + html.EscapeString(n.Message) + `</div>`)
			b.WriteString(`<div class="notification-time">` + n.CreatedAt.Format("02/01/2006 15:04") + `</div>`)
			b.WriteString(`</div>`)
		}
	}

	b.WriteString(`</div>
  </div>
  <script src="/static/js/global-error-handler.js"></script>
  <script src="/static/js/user_status.js"></script>
  <script src="/static/js/navigation_active.js"></script>
  <script src="/static/js/notifications_unified.js"></script>
  <script src="/static/js/profile.js"></script>
</body>
</html>`)

	return b.String()
}
