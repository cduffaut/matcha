package chat

import (
	"time"
)

// Message représente un message entre deux utilisateurs
type Message struct {
	ID          int       `json:"id" db:"id"`
	SenderID    int       `json:"sender_id" db:"sender_id"`
	RecipientID int       `json:"recipient_id" db:"recipient_id"`
	Content     string    `json:"content" db:"content"`
	IsRead      bool      `json:"is_read" db:"is_read"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`

	// Informations supplémentaires pour l'affichage
	SenderUsername string `json:"sender_username,omitempty" db:"-"`
	SenderName     string `json:"sender_name,omitempty" db:"-"`
}

// Conversation représente une conversation entre deux utilisateurs
type Conversation struct {
	UserID          int       `json:"user_id"`
	Username        string    `json:"username"`
	Name            string    `json:"name"`
	LastMessage     *Message  `json:"last_message,omitempty"`
	UnreadCount     int       `json:"unread_count"`
	LastMessageTime time.Time `json:"last_message_time"`
}

// MessageRepository interface pour la gestion des messages
type MessageRepository interface {
	// Créer un nouveau message
	CreateMessage(message *Message) error

	// Récupérer les messages d'une conversation
	GetMessages(userID1, userID2 int, limit, offset int) ([]*Message, error)

	// Récupérer la liste des conversations d'un utilisateur
	GetConversations(userID int) ([]*Conversation, error)

	// Marquer les messages d'une conversation comme lus
	MarkMessagesAsRead(senderID, recipientID int) error

	// Compter les messages non lus
	GetUnreadMessageCount(userID int) (int, error)

	// Compter les messages non lus d'une conversation spécifique
	GetUnreadCountForConversation(userID, otherUserID int) (int, error)

	// Vérifier si deux utilisateurs peuvent discuter (sont matchés)
	CanChat(userID1, userID2 int) (bool, error)
}

// MessageService interface pour la logique métier des messages
type MessageService interface {
	// Envoyer un message
	SendMessage(senderID, recipientID int, content string) (*Message, error)

	// Récupérer les messages d'une conversation
	GetConversationMessages(userID, otherUserID int, limit, offset int) ([]*Message, error)

	// Récupérer la liste des conversations
	GetUserConversations(userID int) ([]*Conversation, error)

	// Marquer les messages comme lus
	MarkAsRead(userID, otherUserID int) error

	// Obtenir le nombre de messages non lus
	GetUnreadCount(userID int) (int, error)
}

// SendMessageRequest représente une requête d'envoi de message
type SendMessageRequest struct {
	RecipientID int    `json:"recipient_id"`
	Content     string `json:"content"`
}

// WebSocket message types
const (
	MessageTypeChat         = "chat_message"
	MessageTypeNotification = "notification"
	MessageTypeError        = "error"
	MessageTypeAck          = "acknowledgment"
)

// WebSocketMessage représente un message WebSocket
type WebSocketMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// ChatMessage représente un message de chat via WebSocket
type ChatMessage struct {
	Message        *Message `json:"message"`
	ConversationID string   `json:"conversation_id"` // Format: "userID1-userID2"
}

// Client WebSocket
type Client struct {
	UserID int
	Conn   interface{} // WebSocket connection (à définir selon la lib utilisée)
	Send   chan []byte
}

// Hub gère les connexions WebSocket
type Hub struct {
	// Clients connectés indexés par UserID
	Clients map[int]*Client

	// Canal pour enregistrer les clients
	Register chan *Client

	// Canal pour désenregistrer les clients
	Unregister chan *Client

	// Canal pour diffuser les messages
	Broadcast chan []byte
}

// Run démarre le hub WebSocket
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Clients[client.UserID] = client

		case client := <-h.Unregister:
			if _, ok := h.Clients[client.UserID]; ok {
				delete(h.Clients, client.UserID)
				close(client.Send)
			}
		}
	}
}
