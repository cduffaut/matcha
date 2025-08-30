package chat

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/cduffaut/matcha/internal/notifications"
)

// Service implémentation du service de chat.
type Service struct {
	messageRepo         MessageRepository
	notificationService notifications.NotificationService
}

func NewService(messageRepo MessageRepository, notificationService notifications.NotificationService) MessageService {
	return &Service{messageRepo: messageRepo, notificationService: notificationService}
}

const (
	maxMessageLen  = 1000
	previewMaxRune = 50
)

/* ================================ Helpers ================================ */

func (s *Service) canChat(a, b int, errMsg string) error {
	ok, err := s.messageRepo.CanChat(a, b)
	if err != nil {
		return fmt.Errorf("erreur lors de la vérification du match: %w", err)
	}
	if !ok {
		return errors.New(errMsg)
	}
	return nil
}

func validateAndTrimContent(content string) (string, error) {
	c := strings.TrimSpace(content)
	if c == "" {
		return "", errors.New("le message ne peut pas être vide")
	}
	if len(c) > maxMessageLen {
		return "", fmt.Errorf("le message est trop long (maximum %d caractères)", maxMessageLen)
	}
	return c, nil
}

func preview(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

/* ================================== API ================================== */

func (s *Service) SendMessage(senderID, recipientID int, content string) (*Message, error) {
	if err := s.canChat(senderID, recipientID, "vous ne pouvez pas envoyer de message à cet utilisateur (pas de match)"); err != nil {
		return nil, err
	}
	c, err := validateAndTrimContent(content)
	if err != nil {
		return nil, err
	}

	msg := &Message{
		SenderID:    senderID,
		RecipientID: recipientID,
		Content:     c,
		IsRead:      false,
	}
	if err := s.messageRepo.CreateMessage(msg); err != nil {
		return nil, fmt.Errorf("erreur lors de la création du message: %w", err)
	}

	// Notification (best-effort)
	if nErr := s.notificationService.NotifyMessage(recipientID, senderID, preview(c, previewMaxRune)); nErr != nil {
		log.Printf("notification message échouée: %v", nErr)
	}

	return msg, nil
}

func (s *Service) GetConversationMessages(userID, otherUserID int, limit, offset int) ([]*Message, error) {
	if err := s.canChat(userID, otherUserID, "vous ne pouvez pas voir cette conversation (pas de match)"); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 50
	} else if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	msgs, err := s.messageRepo.GetMessages(userID, otherUserID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des messages: %w", err)
	}
	return msgs, nil
}

func (s *Service) GetUserConversations(userID int) ([]*Conversation, error) {
	convs, err := s.messageRepo.GetConversations(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des conversations: %w", err)
	}
	return convs, nil
}

func (s *Service) MarkAsRead(userID, otherUserID int) error {
	if err := s.canChat(userID, otherUserID, "vous ne pouvez pas accéder à cette conversation (pas de match)"); err != nil {
		return err
	}
	if err := s.messageRepo.MarkMessagesAsRead(otherUserID, userID); err != nil {
		return fmt.Errorf("erreur lors du marquage des messages comme lus: %w", err)
	}
	return nil
}

func (s *Service) GetUnreadCount(userID int) (int, error) {
	n, err := s.messageRepo.GetUnreadMessageCount(userID)
	if err != nil {
		return 0, fmt.Errorf("erreur lors du comptage des messages non lus: %w", err)
	}
	return n, nil
}
