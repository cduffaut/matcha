package user

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cduffaut/matcha/internal/models"
	"github.com/cduffaut/matcha/internal/notifications"
)

type ProfileService struct {
	profileRepo         ProfileRepository
	userRepo            Repository
	uploadsDir          string
	notificationService notifications.NotificationService
}

func NewProfileService(profileRepo ProfileRepository, userRepo Repository, uploadsDir string, notificationService notifications.NotificationService) *ProfileService {
	return &ProfileService{
		profileRepo:         profileRepo,
		userRepo:            userRepo,
		uploadsDir:          uploadsDir,
		notificationService: notificationService,
	}
}

func (s *ProfileService) GetProfile(userID int) (*Profile, error) {
	p, err := s.profileRepo.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}
	return p, nil
}

func (s *ProfileService) UpdateProfile(userID int, in *Profile) error {
	existing, err := s.profileRepo.GetByUserID(userID)
	if err != nil {
		return fmt.Errorf("get profile: %w", err)
	}

	// Only mutable fields; keep server-owned values
	in.UserID = userID
	in.FameRating = existing.FameRating

	// If location fields are zero-values, keep previous ones
	if in.Latitude == 0 && in.Longitude == 0 && in.LocationName == "" {
		in.Latitude, in.Longitude, in.LocationName = existing.Latitude, existing.Longitude, existing.LocationName
	}

	if err := s.profileRepo.Update(in); err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	return nil
}

func (s *ProfileService) AddTag(userID int, tagName string) error {
	tagName = normalizeTag(tagName)
	if tagName == "#" {
		return fmt.Errorf("tag invalide")
	}
	if err := s.profileRepo.AddTag(userID, tagName); err != nil {
		return fmt.Errorf("add tag: %w", err)
	}
	return nil
}

func (s *ProfileService) UpdateLastConnection(userID int) error {
	if err := s.profileRepo.UpdateLastConnection(userID); err != nil {
		return fmt.Errorf("update last connection: %w", err)
	}
	return nil
}

func (s *ProfileService) RemoveTag(userID, tagID int) error {
	if err := s.profileRepo.RemoveTag(userID, tagID); err != nil {
		return fmt.Errorf("remove tag: %w", err)
	}
	return nil
}

func (s *ProfileService) GetTags(userID int) ([]Tag, error) {
	t, err := s.profileRepo.GetTagsByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("get tags: %w", err)
	}
	return t, nil
}

func (s *ProfileService) GetAllTags() ([]Tag, error) {
	t, err := s.profileRepo.GetAllTags()
	if err != nil {
		return nil, fmt.Errorf("get all tags: %w", err)
	}
	return t, nil
}

func (s *ProfileService) BlockUser(blockerID, blockedID int) error {
	if blockerID == blockedID {
		return fmt.Errorf("action invalide")
	}
	if err := s.profileRepo.BlockUser(blockerID, blockedID); err != nil {
		return fmt.Errorf("block user: %w", err)
	}
	_ = s.profileRepo.UnlikeUser(blockerID, blockedID)
	_ = s.profileRepo.UnlikeUser(blockedID, blockerID)
	return nil
}

func (s *ProfileService) UnblockUser(blockerID, blockedID int) error {
	if err := s.profileRepo.UnblockUser(blockerID, blockedID); err != nil {
		return fmt.Errorf("unblock user: %w", err)
	}
	return nil
}

func (s *ProfileService) GetBlockedUsers(userID int) ([]BlockedUser, error) {
	u, err := s.profileRepo.GetBlockedUsers(userID)
	if err != nil {
		return nil, fmt.Errorf("get blocked users: %w", err)
	}
	return u, nil
}

func (s *ProfileService) IsUserBlocked(userID, otherUserID int) (bool, error) {
	return s.profileRepo.IsBlocked(userID, otherUserID)
}

func (s *ProfileService) DeletePhoto(userID, photoID int) error {
	photos, err := s.profileRepo.GetPhotosByUserID(userID)
	if err != nil {
		return fmt.Errorf("get photos: %w", err)
	}
	var target *Photo
	for _, p := range photos {
		if p.ID == photoID {
			target = &p
			break
		}
	}
	if target == nil {
		return errors.New("photo introuvable")
	}
	if err := s.profileRepo.RemovePhoto(photoID); err != nil {
		return fmt.Errorf("remove photo: %w", err)
	}
	// Best-effort file cleanup
	path := filepath.Clean(filepath.Join(s.uploadsDir, strings.TrimPrefix(target.FilePath, "/uploads/")))
	_ = os.Remove(path)
	return nil
}

func (s *ProfileService) SetProfilePhoto(userID, photoID int) error {
	photos, err := s.profileRepo.GetPhotosByUserID(userID)
	if err != nil {
		return fmt.Errorf("get photos: %w", err)
	}
	found := false
	for _, p := range photos {
		if p.ID == photoID {
			found = true
			break
		}
	}
	if !found {
		return errors.New("photo introuvable")
	}
	if err := s.profileRepo.SetProfilePhoto(photoID); err != nil {
		return fmt.Errorf("set profile photo: %w", err)
	}
	return nil
}

func (s *ProfileService) GetPhotos(userID int) ([]Photo, error) {
	p, err := s.profileRepo.GetPhotosByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("get photos: %w", err)
	}
	return p, nil
}

func (s *ProfileService) ViewProfile(visitorID, visitedID int) error {
	if err := s.profileRepo.RecordVisit(visitorID, visitedID); err != nil {
		return fmt.Errorf("record visit: %w", err)
	}
	return nil
}

func (s *ProfileService) GetVisitors(userID int) ([]ProfileVisit, error) {
	v, err := s.profileRepo.GetVisitorsForUser(userID)
	if err != nil {
		return nil, fmt.Errorf("get visitors: %w", err)
	}
	return v, nil
}

func (s *ProfileService) LikeUser(likerID, likedID int) (bool, error) {
	lp, err := s.profileRepo.GetByUserID(likerID)
	if err != nil {
		return false, fmt.Errorf("profil non disponible")
	}
	if !s.IsProfileComplete(lp) {
		return false, fmt.Errorf("complétez votre profil pour liker")
	}

	tp, err := s.profileRepo.GetByUserID(likedID)
	if err != nil {
		return false, fmt.Errorf("profil cible non disponible")
	}
	if !s.IsProfileComplete(tp) {
		return false, fmt.Errorf("ce profil n'est pas encore complet")
	}

	if err := s.profileRepo.LikeUser(likerID, likedID); err != nil {
		return false, fmt.Errorf("like indisponible")
	}

	if s.notificationService != nil {
		s.notificationService.NotifyLike(likedID, likerID)
	}

	match, err := s.profileRepo.CheckIfMatched(likerID, likedID)
	if err != nil {
		return false, fmt.Errorf("vérification du match échouée")
	}
	if match && s.notificationService != nil {
		s.notificationService.NotifyMatch(likerID, likedID)
	}
	return match, nil
}

func (s *ProfileService) UnlikeUser(likerID, likedID int) error {
	if err := s.profileRepo.UnlikeUser(likerID, likedID); err != nil {
		return fmt.Errorf("unlike: %w", err)
	}
	if s.notificationService != nil {
		s.notificationService.NotifyUnlike(likedID, likerID)
	}
	return nil
}

func (s *ProfileService) GetLikes(userID int) ([]UserLike, error) {
	ls, err := s.profileRepo.GetLikesForUser(userID)
	if err != nil {
		return nil, fmt.Errorf("get likes: %w", err)
	}
	return ls, nil
}

func (s *ProfileService) CheckIfLiked(likerID, likedID int) (bool, error) {
	return s.profileRepo.CheckIfLiked(likerID, likedID)
}

func (s *ProfileService) CheckIfMatched(user1ID, user2ID int) (bool, error) {
	return s.profileRepo.CheckIfMatched(user1ID, user2ID)
}

func (s *ProfileService) UploadPhotoSecure(userID int, fileData []byte, filename string, isProfile bool) (*Photo, error) {
	photos, err := s.profileRepo.GetPhotosByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("get photos: %w", err)
	}
	if len(photos) >= 5 {
		return nil, errors.New("limite de 5 photos atteinte")
	}

	userDir := filepath.Join(s.uploadsDir, fmt.Sprintf("user_%d", userID))
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	ext := filepath.Ext(filename)
	newName := fmt.Sprintf("%d_%d_%d%s", userID, time.Now().Unix(), len(photos)+1, ext)
	fsPath := filepath.Join(userDir, newName)

	if err := os.WriteFile(fsPath, fileData, 0o644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	urlPath := fmt.Sprintf("/uploads/user_%d/%s", userID, newName)
	photo := &Photo{
		UserID:    userID,
		FilePath:  "/" + strings.TrimPrefix(urlPath, "/"),
		IsProfile: isProfile || len(photos) == 0,
	}

	if err := s.profileRepo.AddPhoto(photo); err != nil {
		_ = os.Remove(fsPath)
		return nil, fmt.Errorf("db add photo: %w", err)
	}

	if isProfile && len(photos) > 0 {
		if err := s.profileRepo.SetProfilePhoto(photo.ID); err != nil {
			return nil, fmt.Errorf("set profile photo: %w", err)
		}
	}
	return photo, nil
}

func (s *ProfileService) GetUserByID(userID int) (*models.User, error) {
	u, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

func (s *ProfileService) UpdateUserOnlineStatus(userID int, isOnline bool) error {
	if err := s.profileRepo.SetOnline(userID, isOnline); err != nil {
		return fmt.Errorf("set online: %w", err)
	}
	return nil
}

func (s *ProfileService) CleanupInactiveUsers(timeoutMinutes int) error {
	if err := s.profileRepo.CleanupInactiveUsers(timeoutMinutes); err != nil {
		return fmt.Errorf("cleanup inactive: %w", err)
	}
	return nil
}

func (s *ProfileService) GetUserOnlineStatus(userID int) (bool, *time.Time, error) {
	return s.profileRepo.GetUserOnlineStatus(userID)
}

func (s *ProfileService) ReportUser(reporterID, reportedID int, reason string) error {
	if reporterID == reportedID {
		return fmt.Errorf("action invalide")
	}
	if _, err := s.userRepo.GetByID(reportedID); err != nil {
		return fmt.Errorf("utilisateur introuvable")
	}
	if err := s.profileRepo.ReportUser(reporterID, reportedID, reason); err != nil {
		return fmt.Errorf("report user: %w", err)
	}
	return nil
}

func (s *ProfileService) IsProfileComplete(p *Profile) bool {
	return p != nil &&
		p.Gender != "" &&
		p.SexualPreference != "" &&
		strings.TrimSpace(p.Biography) != "" &&
		p.BirthDate != nil &&
		len(p.Tags) > 0 &&
		hasProfilePhoto(p.Photos)
}

/* helpers */

func normalizeTag(t string) string {
	t = strings.TrimSpace(strings.ToLower(t))
	if t == "" {
		return t
	}
	if !strings.HasPrefix(t, "#") {
		t = "#" + t
	}
	return t
}

func hasProfilePhoto(ps []Photo) bool {
	for _, ph := range ps {
		if ph.IsProfile {
			return true
		}
	}
	return false
}
