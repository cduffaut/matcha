package notifications

import (
	"fmt"
)

// Service implémentation du service de notifications
type Service struct {
	repo NotificationRepository
}

// NewService crée un nouveau service de notifications
func NewService(repo NotificationRepository) NotificationService {
	return &Service{
		repo: repo,
	}
}

// CreateNotification crée une nouvelle notification
func (s *Service) CreateNotification(userID, fromID int, notificationType NotificationType, message string) error {
	// Éviter les notifications à soi-même
	if userID == fromID {
		return nil
	}

	notification := &Notification{
		UserID:  userID,
		FromID:  fromID,
		Type:    notificationType,
		Message: message,
		IsRead:  false,
	}

	return s.repo.Create(notification)
}

// GetNotifications récupère les notifications d'un utilisateur
func (s *Service) GetNotifications(userID int, limit int) ([]*Notification, error) {
	if limit <= 0 {
		limit = 20 // Limite par défaut
	}
	return s.repo.GetByUserID(userID, limit)
}

// MarkAsRead marque une notification comme lue
func (s *Service) MarkAsRead(notificationID int) error {
	return s.repo.MarkAsRead(notificationID)
}

// MarkAllAsRead marque toutes les notifications d'un utilisateur comme lues
func (s *Service) MarkAllAsRead(userID int) error {
	return s.repo.MarkAllAsRead(userID)
}

// GetUnreadCount récupère le nombre de notifications non lues
func (s *Service) GetUnreadCount(userID int) (int, error) {
	return s.repo.GetUnreadCount(userID)
}

// NotifyLike crée une notification de "like"
func (s *Service) NotifyLike(likedUserID, likerID int) error {
	message := "a liké votre profil"
	return s.CreateNotification(likedUserID, likerID, NotificationLike, message)
}

// NotifyMessage crée une notification de nouveau message
func (s *Service) NotifyMessage(recipientID, senderID int, messagePreview string) error {
	message := fmt.Sprintf("vous a envoyé un message: %s", messagePreview)
	if len(messagePreview) > 50 {
		message = fmt.Sprintf("vous a envoyé un message: %s...", messagePreview[:50])
	}
	return s.CreateNotification(recipientID, senderID, NotificationMessage, message)
}

// NotifyMatch crée une notification de match
func (s *Service) NotifyMatch(user1ID, user2ID int) error {
	message := "Vous avez un nouveau match ! Vous pouvez maintenant discuter"

	// Créer une notification pour chaque utilisateur
	err1 := s.CreateNotification(user1ID, user2ID, NotificationMatch, message)
	err2 := s.CreateNotification(user2ID, user1ID, NotificationMatch, message)

	if err1 != nil {
		return err1
	}
	return err2
}

// NotifyUnlike crée une notification d'"unlike"
func (s *Service) NotifyUnlike(unlikedUserID, unlikerID int) error {
	message := "ne vous like plus"
	return s.CreateNotification(unlikedUserID, unlikerID, NotificationUnlike, message)
}

// NotifyProfileView crée une notification de vue de profil
func (s *Service) NotifyProfileView(viewedUserID, viewerID int) error {
	message := "a consulté votre profil"
	return s.CreateNotification(viewedUserID, viewerID, NotificationProfileView, message)
}
