package notifications

import (
	"time"
)

// NotificationType représente le type de notification
type NotificationType string

const (
	NotificationLike        NotificationType = "like"    // Quelqu'un a liké votre profil
	NotificationMessage     NotificationType = "message" // Nouveau message reçu
	NotificationMatch       NotificationType = "match"   // Match mutuel
	NotificationUnlike      NotificationType = "unlike"  // Quelqu'un vous a unliké
	NotificationProfileView NotificationType = "profile_view"
)

// Notification représente une notification utilisateur
type Notification struct {
	ID        int              `json:"id" db:"id"`
	UserID    int              `json:"user_id" db:"user_id"` // Utilisateur qui reçoit la notification
	FromID    int              `json:"from_id" db:"from_id"` // Utilisateur qui a déclenché la notification
	Type      NotificationType `json:"type" db:"type"`
	Message   string           `json:"message" db:"message"` // Message de la notification
	IsRead    bool             `json:"is_read" db:"is_read"`
	CreatedAt time.Time        `json:"created_at" db:"created_at"`

	// Informations supplémentaires sur l'utilisateur source
	FromUser *UserInfo `json:"from_user,omitempty" db:"-"`
}

// UserInfo contient les informations de base d'un utilisateur pour les notifications
type UserInfo struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

// NotificationRepository interface pour la gestion des notifications
type NotificationRepository interface {
	Create(notification *Notification) error
	GetByUserID(userID int, limit int) ([]*Notification, error)
	MarkAsRead(notificationID int) error
	MarkAllAsRead(userID int) error
	GetUnreadCount(userID int) (int, error)
	Delete(notificationID int) error
}

// NotificationService interface pour la logique métier des notifications
type NotificationService interface {
	CreateNotification(userID, fromID int, notificationType NotificationType, message string) error
	GetNotifications(userID int, limit int) ([]*Notification, error)
	MarkAsRead(notificationID int) error
	MarkAllAsRead(userID int) error
	GetUnreadCount(userID int) (int, error)

	// Méthodes pour créer des notifications spécifiques
	NotifyLike(likedUserID, likerID int) error
	NotifyMessage(recipientID, senderID int, messagePreview string) error
	NotifyMatch(user1ID, user2ID int) error
	NotifyUnlike(unlikedUserID, unlikerID int) error
	NotifyProfileView(viewedUserID, viewerID int) error // ✅ AJOUTER
}
