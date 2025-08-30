package notifications

import "strings"

type Service struct{ repo NotificationRepository }

func NewService(repo NotificationRepository) NotificationService { return &Service{repo: repo} }

func (s *Service) CreateNotification(userID, fromID int, t NotificationType, msg string) error {
	if userID == fromID { return nil }
	return s.repo.Create(&Notification{
		UserID:  userID,
		FromID:  fromID,
		Type:    t,
		Message: msg,
		IsRead:  false,
	})
}

func (s *Service) GetNotifications(userID, limit int) ([]*Notification, error) {
	if limit <= 0 { limit = 20 }
	return s.repo.GetByUserID(userID, limit)
}

func (s *Service) MarkAsRead(id int) error          { return s.repo.MarkAsRead(id) }
func (s *Service) MarkAllAsRead(userID int) error   { return s.repo.MarkAllAsRead(userID) }
func (s *Service) GetUnreadCount(userID int) (int, error) { return s.repo.GetUnreadCount(userID) }

func (s *Service) NotifyLike(likedUserID, likerID int) error {
	return s.CreateNotification(likedUserID, likerID, NotificationLike, "a liké votre profil")
}

func (s *Service) NotifyMessage(recipientID, senderID int, preview string) error {
	preview = strings.TrimSpace(preview)
	if len(preview) > 50 { preview = preview[:50] + "..." }
	return s.CreateNotification(recipientID, senderID, NotificationMessage, "vous a envoyé un message: "+preview)
}

func (s *Service) NotifyMatch(user1ID, user2ID int) error {
	const msg = "Vous avez un nouveau match ! Vous pouvez maintenant discuter"
	if err := s.CreateNotification(user1ID, user2ID, NotificationMatch, msg); err != nil { return err }
	return s.CreateNotification(user2ID, user1ID, NotificationMatch, msg)
}

func (s *Service) NotifyUnlike(unlikedUserID, unlikerID int) error {
	return s.CreateNotification(unlikedUserID, unlikerID, NotificationUnlike, "ne vous like plus")
}

func (s *Service) NotifyProfileView(viewedUserID, viewerID int) error {
	return s.CreateNotification(viewedUserID, viewerID, NotificationProfileView, "a consulté votre profil")
}
