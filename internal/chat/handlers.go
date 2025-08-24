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
	CheckOrigin: func(r *http.Request) bool {
		return true // En production, vérifier l'origine
	},
}

// Handlers gère les requêtes HTTP pour le chat
type Handlers struct {
	messageService MessageService
	hub            *Hub
}

// NewHandlers crée de nouveaux handlers pour le chat
func NewHandlers(messageService MessageService, hub *Hub) *Handlers {
	return &Handlers{
		messageService: messageService,
		hub:            hub,
	}
}

// SendMessageHandler envoie un message
func (h *Handlers) SendMessageHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	userSession, ok := session.FromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Utilisateur non connecté"})
		return
	}

	// Décoder la requête
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Format de requête invalide"})
		return
	}

	// Envoyer le message
	message, err := h.messageService.SendMessage(userSession.UserID, req.RecipientID, req.Content)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// ✅ DIFFUSER VIA WEBSOCKET AVANT DE RÉPONDRE
	h.broadcastMessage(message)

	// ✅ RÉPONDRE AVEC LE MESSAGE CRÉÉ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(message)
}

// GetConversationHandler récupère les messages d'une conversation
func (h *Handlers) GetConversationHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	userSession, ok := session.FromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Utilisateur non connecté"})
		return
	}

	// Récupérer l'ID de l'autre utilisateur
	otherUserIDStr := pat.Param(r, "userID")
	otherUserID, err := strconv.Atoi(otherUserIDStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "ID utilisateur invalide"})
		return
	}

	// Paramètres de pagination
	limit := 50
	offset := 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Récupérer les messages
	messages, err := h.messageService.GetConversationMessages(userSession.UserID, otherUserID, limit, offset)
	if err != nil {
		// ✅ TOUJOURS RETOURNER UN TABLEAU JSON MÊME EN CAS D'ERREUR
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(err.Error(), "pas de match") {
			// Pour un nouveau match, retourner un tableau vide plutôt qu'une erreur
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]*Message{})
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode([]*Message{})
		}
		return
	}

	// ✅ S'ASSURER QUE messages N'EST JAMAIS NIL
	if messages == nil {
		messages = []*Message{}
	}

	// Marquer les messages comme lus
	if err := h.messageService.MarkAsRead(userSession.UserID, otherUserID); err != nil {
		fmt.Printf("Erreur lors du marquage des messages comme lus: %v\n", err)
		// Ne pas retourner d'erreur, continuer avec la réponse
	}

	// ✅ RÉPONDRE TOUJOURS AVEC UN TABLEAU JSON VALIDE
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(messages)
}

// GetConversationsHandler récupère la liste des conversations
func (h *Handlers) GetConversationsHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	userSession, ok := session.FromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Utilisateur non connecté"})
		return
	}

	// Récupérer les conversations
	conversations, err := h.messageService.GetUserConversations(userSession.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode([]*Conversation{}) // Tableau vide en cas d'erreur
		return
	}

	// ✅ S'ASSURER QUE conversations N'EST JAMAIS NIL
	if conversations == nil {
		conversations = []*Conversation{}
	}

	// Répondre avec les conversations
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(conversations)
}

// MarkAsReadHandler marque les messages d'une conversation comme lus
func (h *Handlers) MarkAsReadHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	userSession, ok := session.FromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Utilisateur non connecté"})
		return
	}

	// Récupérer l'ID de l'autre utilisateur
	otherUserIDStr := pat.Param(r, "userID")
	otherUserID, err := strconv.Atoi(otherUserIDStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "ID utilisateur invalide"})
		return
	}

	// Marquer comme lus
	if err := h.messageService.MarkAsRead(userSession.UserID, otherUserID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Répondre avec succès
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Messages marqués comme lus",
	})
}

// GetUnreadCountHandler récupère le nombre de messages non lus
func (h *Handlers) GetUnreadCountHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	userSession, ok := session.FromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Utilisateur non connecté"})
		return
	}

	// Récupérer le nombre de messages non lus
	count, err := h.messageService.GetUnreadCount(userSession.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]int{"unread_count": 0}) // 0 en cas d'erreur
		return
	}

	// Répondre avec le nombre
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]int{
		"unread_count": count,
	})
}

// ChatPageHandler affiche la page de chat
func (h *Handlers) ChatPageHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer la session
	userSession, ok := session.FromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// Récupérer les conversations pour l'affichage initial
	conversations, err := h.messageService.GetUserConversations(userSession.UserID)
	if err != nil {
		http.Error(w, "Erreur lors de la récupération des conversations", http.StatusInternalServerError)
		return
	}

	// ✅ PASSER L'ID UTILISATEUR
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.Write([]byte(generateChatHTML(conversations, userSession.UserID)))
}

// generateChatHTML génère le HTML pour la page de chat
func generateChatHTML(conversations []*Conversation, userID int) string {
	html := `<!DOCTYPE html>
<html lang="fr">
<head>
    <title>Messages - Matcha</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
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
            <div id="conversations">
                <!-- Les conversations seront chargées ici -->
            </div>
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
    
    <script>
        window.currentUserId = ` + strconv.Itoa(userID) + `;
    </script>

	<style>
	input[type="text"], input[type="email"], input[type="password"], textarea {
		font-size: 16px !important;
	}
	</style>
    
	<script src="/static/js/global-error-handler.js"></script>
    <script src="/static/js/chat.js"></script>
	<script src="/static/js/notifications_unified.js"></script>
</body>
</html>`

	return html
}

// ✅ AMÉLIORER LA DIFFUSION WEBSOCKET POUR ÉVITER LES DOUBLONS
func (h *Handlers) broadcastMessage(message *Message) {
	if h.hub == nil {
		fmt.Println("Hub WebSocket non disponible")
		return
	}

	// Créer le message WebSocket avec un ID unique
	wsMessage := WebSocketMessage{
		Type: MessageTypeChat,
		Data: ChatMessage{
			Message:        message,
			ConversationID: fmt.Sprintf("%d-%d", message.SenderID, message.RecipientID),
		},
		Timestamp: time.Now(),
	}

	// Convertir en JSON
	data, err := json.Marshal(wsMessage)
	if err != nil {
		fmt.Printf("Erreur lors de la sérialisation du message WebSocket: %v\n", err)
		return
	}

	// ✅ DIFFUSER VERS LES DEUX PARTICIPANTS
	participants := []int{message.SenderID, message.RecipientID}
	for _, userID := range participants {
		if client, ok := h.hub.Clients[userID]; ok {
			select {
			case client.Send <- data:
				fmt.Printf("Message WebSocket envoyé à l'utilisateur %d\n", userID)
			default:
				// Canal plein, déconnecter le client
				fmt.Printf("Canal WebSocket plein pour l'utilisateur %d, déconnexion\n", userID)
				close(client.Send)
				delete(h.hub.Clients, userID)
			}
		} else {
			fmt.Printf("Client WebSocket non trouvé pour l'utilisateur %d\n", userID)
		}
	}
}

func (h *Handlers) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	userSession, ok := session.FromContext(r.Context())
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
		UserID: userSession.UserID,
		Conn:   conn,
		Send:   make(chan []byte, 256),
	}

	h.hub.Register <- client

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

	go func() {
		defer conn.Close()
		for {
			select {
			case message := <-client.Send:
				if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
					return
				}
			}
		}
	}()
}
