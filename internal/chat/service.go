package chat

import (
	"fmt"
	"strings"

	"github.com/cduffaut/matcha/internal/notifications"
)

// Service implémentation du service de chat
type Service struct {
	messageRepo         MessageRepository
	notificationService notifications.NotificationService
}

// NewService crée un nouveau service de chat
func NewService(messageRepo MessageRepository, notificationService notifications.NotificationService) MessageService {
	return &Service{
		messageRepo:         messageRepo,
		notificationService: notificationService,
	}
}

// SendMessage envoie un message
func (s *Service) SendMessage(senderID, recipientID int, content string) (*Message, error) {
	// Vérifier que les utilisateurs peuvent discuter
	canChat, err := s.messageRepo.CanChat(senderID, recipientID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la vérification du match: %w", err)
	}

	if !canChat {
		return nil, fmt.Errorf("vous ne pouvez pas envoyer de message à cet utilisateur (pas de match)")
	}

	// Valider le contenu
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("le message ne peut pas être vide")
	}

	if len(content) > 1000 {
		return nil, fmt.Errorf("le message est trop long (maximum 1000 caractères)")
	}

	// Créer le message
	message := &Message{
		SenderID:    senderID,
		RecipientID: recipientID,
		Content:     content,
		IsRead:      false,
	}

	// Sauvegarder le message
	if err := s.messageRepo.CreateMessage(message); err != nil {
		return nil, fmt.Errorf("erreur lors de la création du message: %w", err)
	}

	// Créer une notification pour le destinataire
	messagePreview := content
	if len(messagePreview) > 50 {
		messagePreview = messagePreview[:50] + "..."
	}

	if err := s.notificationService.NotifyMessage(recipientID, senderID, messagePreview); err != nil {
		// On continue même si la notification échoue
		fmt.Printf("Erreur lors de la création de la notification de message: %v\n", err)
	}

	return message, nil
}

// GetConversationMessages récupère les messages d'une conversation
func (s *Service) GetConversationMessages(userID, otherUserID int, limit, offset int) ([]*Message, error) {
	// Vérifier que les utilisateurs peuvent discuter
	canChat, err := s.messageRepo.CanChat(userID, otherUserID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la vérification du match: %w", err)
	}

	if !canChat {
		return nil, fmt.Errorf("vous ne pouvez pas voir cette conversation (pas de match)")
	}

	// Limites de pagination
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	// Récupérer les messages
	messages, err := s.messageRepo.GetMessages(userID, otherUserID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des messages: %w", err)
	}

	return messages, nil
}

// GetUserConversations récupère la liste des conversations d'un utilisateur
func (s *Service) GetUserConversations(userID int) ([]*Conversation, error) {
	conversations, err := s.messageRepo.GetConversations(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des conversations: %w", err)
	}

	return conversations, nil
}

// MarkAsRead marque les messages d'une conversation comme lus
func (s *Service) MarkAsRead(userID, otherUserID int) error {
	// Vérifier que les utilisateurs peuvent discuter
	canChat, err := s.messageRepo.CanChat(userID, otherUserID)
	if err != nil {
		return fmt.Errorf("erreur lors de la vérification du match: %w", err)
	}

	if !canChat {
		return fmt.Errorf("vous ne pouvez pas accéder à cette conversation (pas de match)")
	}

	// Marquer comme lus les messages envoyés par otherUserID à userID
	if err := s.messageRepo.MarkMessagesAsRead(otherUserID, userID); err != nil {
		return fmt.Errorf("erreur lors du marquage des messages comme lus: %w", err)
	}

	return nil
}

// GetUnreadCount obtient le nombre de messages non lus
func (s *Service) GetUnreadCount(userID int) (int, error) {
	count, err := s.messageRepo.GetUnreadMessageCount(userID)
	if err != nil {
		return 0, fmt.Errorf("erreur lors du comptage des messages non lus: %w", err)
	}

	return count, nil
}
