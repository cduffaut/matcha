package chat

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cduffaut/matcha/internal/session"
	"github.com/gorilla/websocket"
	"goji.io/pat"
)

var upgrader = websocket.Upgrader{
	// En prod: restreindre l'origine.
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Handlers struct {
	messageService MessageService
	hub            *Hub
}

func NewHandlers(messageService MessageService, hub *Hub) *Handlers {
	return &Handlers{messageService: messageService, hub: hub}
}

/* ----------------------------- HTTP HELPERS ------------------------------ */

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func mustSession(w http.ResponseWriter, r *http.Request) (*session.Session, bool) {
	sess, ok := session.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Utilisateur non connecté")
		return nil, false
	}
	return sess, true
}

func queryInt(r *http.Request, key string, def, min int) int {
	v := def
	if raw := r.URL.Query().Get(key); raw != "" {
		if i, err := strconv.Atoi(raw); err == nil && i >= min {
			v = i
		}
	}
	return v
}

/* -------------------------------- HANDLERS ------------------------------- */

func (h *Handlers) SendMessageHandler(w http.ResponseWriter, r *http.Request) {
	sess, ok := mustSession(w, r)
	if !ok {
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Format de requête invalide")
		return
	}

	msg, err := h.messageService.SendMessage(sess.UserID, req.RecipientID, req.Content)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.broadcastMessage(msg)
	writeJSON(w, http.StatusOK, msg)
}

func (h *Handlers) GetConversationHandler(w http.ResponseWriter, r *http.Request) {
	sess, ok := mustSession(w, r)
	if !ok {
		return
	}

	otherID, err := strconv.Atoi(pat.Param(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "ID utilisateur invalide")
		return
	}

	limit := queryInt(r, "limit", 50, 1)
	offset := queryInt(r, "offset", 0, 0)

	messages, err := h.messageService.GetConversationMessages(sess.UserID, otherID, limit, offset)
	if err != nil {
		// Toujours retourner un tableau JSON.
		if strings.Contains(err.Error(), "pas de match") {
			writeJSON(w, http.StatusOK, []*Message{})
			return
		}
		writeJSON(w, http.StatusInternalServerError, []*Message{})
		return
	}
	if messages == nil {
		messages = []*Message{}
	}

	if err := h.messageService.MarkAsRead(sess.UserID, otherID); err != nil {
		fmt.Printf("Erreur marquage lus: %v\n", err)
	}

	writeJSON(w, http.StatusOK, messages)
}

func (h *Handlers) GetConversationsHandler(w http.ResponseWriter, r *http.Request) {
	sess, ok := mustSession(w, r)
	if !ok {
		return
	}

	convs, err := h.messageService.GetUserConversations(sess.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, []*Conversation{})
		return
	}
	if convs == nil {
		convs = []*Conversation{}
	}

	writeJSON(w, http.StatusOK, convs)
}

func (h *Handlers) MarkAsReadHandler(w http.ResponseWriter, r *http.Request) {
	sess, ok := mustSession(w, r)
	if !ok {
		return
	}

	otherID, err := strconv.Atoi(pat.Param(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "ID utilisateur invalide")
		return
	}

	if err := h.messageService.MarkAsRead(sess.UserID, otherID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Messages marqués comme lus"})
}

func (h *Handlers) GetUnreadCountHandler(w http.ResponseWriter, r *http.Request) {
	sess, ok := mustSession(w, r)
	if !ok {
		return
	}

	count, err := h.messageService.GetUnreadCount(sess.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]int{"unread_count": 0})
		return
	}

	writeJSON(w, http.StatusOK, map[string]int{"unread_count": count})
}

func (h *Handlers) ChatPageHandler(w http.ResponseWriter, r *http.Request) {
	sess, ok := mustSession(w, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// Pré-chargement (conservé pour logique existante)
	if _, err := h.messageService.GetUserConversations(sess.UserID); err != nil {
		http.Error(w, "Erreur lors de la récupération des conversations", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	_, _ = w.Write([]byte(generateChatHTML(nil, sess.UserID)))
}

/* ----------------------------- WEBSOCKET PART ---------------------------- */

func (h *Handlers) broadcastMessage(message *Message) {
	if h.hub == nil {
		fmt.Println("Hub WebSocket non disponible")
		return
	}

	wsMessage := WebSocketMessage{
		Type: MessageTypeChat,
		Data: ChatMessage{
			Message:        message,
			ConversationID: fmt.Sprintf("%d-%d", message.SenderID, message.RecipientID),
		},
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(wsMessage)
	if err != nil {
		fmt.Printf("Erreur sérialisation WS: %v\n", err)
		return
	}

	for _, uid := range []int{message.SenderID, message.RecipientID} {
		if client, ok := h.hub.Clients[uid]; ok {
			select {
			case client.Send <- data:
				// ok
			default:
				close(client.Send)
				delete(h.hub.Clients, uid)
			}
		}
	}
}

func (h *Handlers) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	sess, ok := session.FromContext(r.Context())
	if !ok {
		http.Error(w, "Non autorisé", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Erreur WebSocket: %v", err)
		return
	}

	client := &Client{
		UserID: sess.UserID,
		Conn:   conn,
		Send:   make(chan []byte, 256),
	}

	h.hub.Register <- client

	// Reader
	go func() {
		defer func() {
			h.hub.Unregister <- client
			conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()

	// Writer
	go func() {
		defer conn.Close()
		for {
			select {
			case msg := <-client.Send:
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			}
		}
	}()
}

/* --------------------------------- HTML ---------------------------------- */

func generateChatHTML(_ []*Conversation, userID int) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="fr">
<head>
	<title>Messages - Matcha</title>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0, user-scalable=no">
	<link rel="stylesheet" href="/static/css/chat.css">
	<link rel="stylesheet" href="/static/css/notifications_style_fix.css">
</head>
<body>
	<header>
		<h1>Matcha</h1>
		<nav>
			<a href="/profile">Mon profil</a>
			<a href="/browse">Explorer</a>
			<a href="/notifications">Notifications <span id="notification-count"></span></a>
			<a href="/chat" class="active">Messages <span id="message-count"></span></a>
			<a href="/logout">Déconnexion</a>
		</nav>
	</header>

	<div class="chat-container">
		<div class="conversations-list">
			<h3>Conversations</h3>
			<div id="conversations"></div>
		</div>

		<div class="chat-area">
			<div class="chat-header" id="chat-header">
				<span>Sélectionnez une conversation</span>
			</div>

			<div class="messages-container" id="messages-container">
				<div class="chat-placeholder">
					<p>Sélectionnez une conversation pour commencer à chatter</p>
				</div>
			</div>

			<div class="message-input-container" id="message-input-container" style="display: none;">
				<form id="message-form">
					<input type="text" id="message-input" placeholder="Tapez votre message..." maxlength="1000" required>
					<button type="submit">Envoyer</button>
				</form>
			</div>
		</div>
	</div>

	<script>window.currentUserId = %d;</script>
	<style>
		input[type="text"], input[type="email"], input[type="password"], textarea { font-size: 16px !important; }
	</style>

	<script src="/static/js/global-error-handler.js"></script>
	<script src="/static/js/chat.js"></script>
	<script src="/static/js/notifications_unified.js"></script>
</body>
</html>`, userID)
}
