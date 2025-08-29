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

// ProfileService fournit des services liés aux profils utilisateurs
type ProfileService struct {
	profileRepo         ProfileRepository
	userRepo            Repository
	uploadsDir          string
	notificationService notifications.NotificationService
}

// NewProfileService crée un nouveau service de profil
func NewProfileService(profileRepo ProfileRepository, userRepo Repository, uploadsDir string, notificationService notifications.NotificationService) *ProfileService {
	return &ProfileService{
		profileRepo:         profileRepo,
		userRepo:            userRepo,
		uploadsDir:          uploadsDir,
		notificationService: notificationService,
	}
}

// GetProfile récupère le profil d'un utilisateur
func (s *ProfileService) GetProfile(userID int) (*Profile, error) {
	profile, err := s.profileRepo.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération du profil: %w", err)
	}
	return profile, nil
}

// UpdateProfile met à jour le profil d'un utilisateur
func (s *ProfileService) UpdateProfile(userID int, profile *Profile) error {
	// Récupérer le profil existant
	existingProfile, err := s.profileRepo.GetByUserID(userID)
	if err != nil {
		return fmt.Errorf("erreur lors de la récupération du profil: %w", err)
	}

	// Mettre à jour les champs modifiables
	profile.UserID = userID
	profile.FameRating = existingProfile.FameRating // Ne pas permettre la modification directe du fame rating
	profile.Latitude = 46.53231280665685
	profile.Longitude = 6.567412345678901

	// Mettre à jour le profil
	if err := s.profileRepo.Update(profile); err != nil {
		return fmt.Errorf("erreur lors de la mise à jour du profil: %w", err)
	}

	return nil
}

// AddTag ajoute un tag au profil d'un utilisateur
func (s *ProfileService) AddTag(userID int, tagName string) error {
	// Nettoyer le nom du tag
	tagName = strings.TrimSpace(tagName)
	tagName = strings.ToLower(tagName)

	// Vérifier que le tag commence par #
	if !strings.HasPrefix(tagName, "#") {
		tagName = "#" + tagName
	}

	// Ajouter le tag
	if err := s.profileRepo.AddTag(userID, tagName); err != nil {
		return fmt.Errorf("erreur lors de l'ajout du tag: %w", err)
	}

	return nil
}

// UpdateLastConnection met à jour la dernière connexion d'un utilisateur
func (s *ProfileService) UpdateLastConnection(userID int) error {
	if err := s.profileRepo.UpdateLastConnection(userID); err != nil {
		return fmt.Errorf("erreur lors de la mise à jour de la dernière connexion: %w", err)
	}
	return nil
}

// RemoveTag supprime un tag du profil d'un utilisateur
func (s *ProfileService) RemoveTag(userID int, tagID int) error {
	if err := s.profileRepo.RemoveTag(userID, tagID); err != nil {
		return fmt.Errorf("erreur lors de la suppression du tag: %w", err)
	}
	return nil
}

// GetTags récupère les tags d'un utilisateur
func (s *ProfileService) GetTags(userID int) ([]Tag, error) {
	tags, err := s.profileRepo.GetTagsByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des tags: %w", err)
	}
	return tags, nil
}

// GetAllTags récupère tous les tags disponibles
func (s *ProfileService) GetAllTags() ([]Tag, error) {
	tags, err := s.profileRepo.GetAllTags()
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des tags: %w", err)
	}
	return tags, nil
}

// BlockUser bloque un utilisateur
func (s *ProfileService) BlockUser(blockerID, blockedID int) error {
	// Vérifier qu'on ne se bloque pas soi-même
	if blockerID == blockedID {
		return fmt.Errorf("vous ne pouvez pas vous bloquer vous-même")
	}

	// Bloquer l'utilisateur
	if err := s.profileRepo.BlockUser(blockerID, blockedID); err != nil {
		return fmt.Errorf("erreur lors du blocage de l'utilisateur: %w", err)
	}

	// Supprimer les likes mutuels s'ils existent
	_ = s.profileRepo.UnlikeUser(blockerID, blockedID)
	_ = s.profileRepo.UnlikeUser(blockedID, blockerID)

	return nil
}

// UnblockUser débloque un utilisateur
func (s *ProfileService) UnblockUser(blockerID, blockedID int) error {
	if err := s.profileRepo.UnblockUser(blockerID, blockedID); err != nil {
		return fmt.Errorf("erreur lors du déblocage de l'utilisateur: %w", err)
	}
	return nil
}

// GetBlockedUsers récupère la liste des utilisateurs bloqués
func (s *ProfileService) GetBlockedUsers(userID int) ([]BlockedUser, error) {
	blockedUsers, err := s.profileRepo.GetBlockedUsers(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des utilisateurs bloqués: %w", err)
	}
	return blockedUsers, nil
}

// IsUserBlocked vérifie si un utilisateur est bloqué
func (s *ProfileService) IsUserBlocked(userID, otherUserID int) (bool, error) {
	return s.profileRepo.IsBlocked(userID, otherUserID)
}

// UploadPhoto télécharge une photo et l'associe à un utilisateur
func (s *ProfileService) UploadPhoto(userID int, fileData []byte, filename string, isProfile bool) (*Photo, error) {
	// Vérifier le nombre de photos existantes
	photos, err := s.profileRepo.GetPhotosByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des photos: %w", err)
	}

	if len(photos) >= 5 {
		return nil, errors.New("limite de 5 photos atteinte")
	}

	// Créer le dossier de l'utilisateur s'il n'existe pas
	userDir := filepath.Join(s.uploadsDir, fmt.Sprintf("user_%d", userID))
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return nil, fmt.Errorf("erreur lors de la création du dossier: %w", err)
	}

	// Générer un nom de fichier unique
	ext := filepath.Ext(filename)
	newFilename := fmt.Sprintf("%d_%d%s", userID, len(photos)+1, ext)
	filePath := filepath.Join(userDir, newFilename)

	// Enregistrer le fichier
	if err := os.WriteFile(filePath, fileData, 0644); err != nil {
		return nil, fmt.Errorf("erreur lors de l'enregistrement du fichier: %w", err)
	}

	// Construire le chemin relatif pour l'URL
	urlPath := fmt.Sprintf("/uploads/user_%d/%s", userID, newFilename)
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}

	// Si c'est la première photo, ou si isProfile est true, la définir comme photo de profil
	photo := &Photo{
		UserID:    userID,
		FilePath:  urlPath,
		IsProfile: isProfile || len(photos) == 0,
	}

	// Ajouter la photo dans la base de données
	if err := s.profileRepo.AddPhoto(photo); err != nil {
		// Supprimer le fichier en cas d'erreur
		os.Remove(filePath)
		return nil, fmt.Errorf("erreur lors de l'ajout de la photo dans la base de données: %w", err)
	}

	// Si c'est une photo de profil et qu'il y en a déjà une, mettre à jour
	if isProfile && len(photos) > 0 {
		if err := s.profileRepo.SetProfilePhoto(photo.ID); err != nil {
			return nil, fmt.Errorf("erreur lors de la définition de la photo de profil: %w", err)
		}
	}

	return photo, nil
}

// DeletePhoto supprime une photo
func (s *ProfileService) DeletePhoto(userID int, photoID int) error {
	// Récupérer les informations sur la photo
	photos, err := s.profileRepo.GetPhotosByUserID(userID)
	if err != nil {
		return fmt.Errorf("erreur lors de la récupération des photos: %w", err)
	}

	var photoToDelete *Photo
	for _, p := range photos {
		if p.ID == photoID {
			photoToDelete = &p
			break
		}
	}

	if photoToDelete == nil {
		return errors.New("photo non trouvée ou n'appartenant pas à l'utilisateur")
	}

	// Supprimer la photo de la base de données
	if err := s.profileRepo.RemovePhoto(photoID); err != nil {
		return fmt.Errorf("erreur lors de la suppression de la photo: %w", err)
	}

	// Supprimer le fichier physique (silencieusement en cas d'erreur)
	filePath := filepath.Join(s.uploadsDir, strings.TrimPrefix(photoToDelete.FilePath, "/uploads/"))
	os.Remove(filePath) // Ignorer l'erreur car la suppression DB est déjà faite

	return nil
}

// SetProfilePhoto définit une photo comme photo de profil
func (s *ProfileService) SetProfilePhoto(userID int, photoID int) error {
	// Vérifier que la photo appartient à l'utilisateur
	photos, err := s.profileRepo.GetPhotosByUserID(userID)
	if err != nil {
		return fmt.Errorf("erreur lors de la récupération des photos: %w", err)
	}

	var found bool
	for _, p := range photos {
		if p.ID == photoID {
			found = true
			break
		}
	}

	if !found {
		return errors.New("photo non trouvée ou n'appartenant pas à l'utilisateur")
	}

	// Définir la photo comme photo de profil
	if err := s.profileRepo.SetProfilePhoto(photoID); err != nil {
		return fmt.Errorf("erreur lors de la définition de la photo de profil: %w", err)
	}

	return nil
}

// GetPhotos récupère les photos d'un utilisateur
func (s *ProfileService) GetPhotos(userID int) ([]Photo, error) {
	photos, err := s.profileRepo.GetPhotosByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des photos: %w", err)
	}
	return photos, nil
}

// ViewProfile enregistre une visite de profil
func (s *ProfileService) ViewProfile(visitorID, visitedID int) error {
	if err := s.profileRepo.RecordVisit(visitorID, visitedID); err != nil {
		return fmt.Errorf("erreur lors de l'enregistrement de la visite: %w", err)
	}

	return nil
}

// GetVisitors récupère les visiteurs du profil d'un utilisateur
func (s *ProfileService) GetVisitors(userID int) ([]ProfileVisit, error) {
	visits, err := s.profileRepo.GetVisitorsForUser(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des visiteurs: %w", err)
	}
	return visits, nil
}

// LikeUser enregistre un "like" d'un utilisateur pour un autre
func (s *ProfileService) LikeUser(likerID, likedID int) (bool, error) {
	// 1. Vérifier que l'utilisateur qui like a un profil complet
	likerProfile, err := s.profileRepo.GetByUserID(likerID)
	if err != nil {
		return false, fmt.Errorf("impossible de récupérer votre profil")
	}

	if !s.IsProfileComplete(likerProfile) {
		// Messages d'erreur spécifiques selon ce qui manque
		if likerProfile.Gender == "" {
			return false, fmt.Errorf("complétez votre profil : renseignez votre genre")
		}
		if likerProfile.SexualPreference == "" {
			return false, fmt.Errorf("complétez votre profil : renseignez vos préférences sexuelles")
		}
		if strings.TrimSpace(likerProfile.Biography) == "" {
			return false, fmt.Errorf("complétez votre profil : rédigez une biographie")
		}
		if likerProfile.BirthDate == nil {
			return false, fmt.Errorf("complétez votre profil : renseignez votre date de naissance")
		}
		if len(likerProfile.Tags) == 0 {
			return false, fmt.Errorf("complétez votre profil : ajoutez au moins un intérêt")
		}

		// Vérifier les photos
		hasProfilePhoto := false
		for _, photo := range likerProfile.Photos {
			if photo.IsProfile {
				hasProfilePhoto = true
				break
			}
		}
		if !hasProfilePhoto {
			if len(likerProfile.Photos) == 0 {
				return false, fmt.Errorf("complétez votre profil : ajoutez au moins une photo")
			} else {
				return false, fmt.Errorf("complétez votre profil : définissez une photo de profil")
			}
		}
	}

	// 2. Vérifier que l'utilisateur ciblé a un profil complet
	likedProfile, err := s.profileRepo.GetByUserID(likedID)
	if err != nil {
		return false, fmt.Errorf("impossible de récupérer le profil de cet utilisateur")
	}

	if !s.IsProfileComplete(likedProfile) {
		return false, fmt.Errorf("ce profil n'est pas encore complet, vous ne pouvez pas le liker")
	}

	// 3. Enregistrer le like
	if err := s.profileRepo.LikeUser(likerID, likedID); err != nil {
		return false, fmt.Errorf("erreur technique lors du like")
	}

	// 4. Créer une notification de like (ignorer les erreurs)
	s.notificationService.NotifyLike(likedID, likerID)

	// 5. Vérifier s'il y a un match
	matched, err := s.profileRepo.CheckIfMatched(likerID, likedID)
	if err != nil {
		return false, fmt.Errorf("like envoyé mais vérification du match échouée")
	}

	// 6. Si c'est un match, créer une notification de match (ignorer les erreurs)
	if matched {
		s.notificationService.NotifyMatch(likerID, likedID)
	}

	return matched, nil
}

// UnlikeUser supprime un "like" d'un utilisateur pour un autre
func (s *ProfileService) UnlikeUser(likerID, likedID int) error {
	if err := s.profileRepo.UnlikeUser(likerID, likedID); err != nil {
		return fmt.Errorf("erreur lors de la suppression du like: %w", err)
	}

	// Créer une notification d'unlike (ignorer les erreurs)
	s.notificationService.NotifyUnlike(likedID, likerID)

	return nil
}

// GetLikes récupère les "likes" reçus par un utilisateur
func (s *ProfileService) GetLikes(userID int) ([]UserLike, error) {
	likes, err := s.profileRepo.GetLikesForUser(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des likes: %w", err)
	}
	return likes, nil
}

// CheckIfLiked vérifie si un utilisateur a liké un autre
func (s *ProfileService) CheckIfLiked(likerID, likedID int) (bool, error) {
	return s.profileRepo.CheckIfLiked(likerID, likedID)
}

// CheckIfMatched vérifie si deux utilisateurs ont un match
func (s *ProfileService) CheckIfMatched(user1ID, user2ID int) (bool, error) {
	return s.profileRepo.CheckIfMatched(user1ID, user2ID)
}

// GetFameRating récupère le fame rating d'un utilisateur
func (s *ProfileService) GetFameRating(userID int) (int, error) {
	profile, err := s.profileRepo.GetByUserID(userID)
	if err != nil {
		return 0, fmt.Errorf("erreur lors de la récupération du profil: %w", err)
	}
	return profile.FameRating, nil
}

// UploadPhotoSecure télécharge une photo sécurisée
func (s *ProfileService) UploadPhotoSecure(userID int, fileData []byte, filename string, isProfile bool) (*Photo, error) {
	// Vérifier le nombre de photos existantes
	photos, err := s.profileRepo.GetPhotosByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des photos: %w", err)
	}

	if len(photos) >= 5 {
		return nil, errors.New("limite de 5 photos atteinte")
	}

	// Créer le dossier de l'utilisateur s'il n'existe pas
	userDir := filepath.Join(s.uploadsDir, fmt.Sprintf("user_%d", userID))
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return nil, fmt.Errorf("erreur lors de la création du dossier: %w", err)
	}

	// Générer un nom de fichier unique avec timestamp
	timestamp := time.Now().Unix()
	ext := filepath.Ext(filename)
	newFilename := fmt.Sprintf("%d_%d_%d%s", userID, timestamp, len(photos)+1, ext)
	filePath := filepath.Join(userDir, newFilename)

	// Enregistrer le fichier
	if err := os.WriteFile(filePath, fileData, 0644); err != nil {
		return nil, fmt.Errorf("erreur lors de l'enregistrement du fichier: %w", err)
	}

	// Construire le chemin relatif pour l'URL
	urlPath := fmt.Sprintf("/uploads/user_%d/%s", userID, newFilename)
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}

	// Si c'est la première photo, ou si isProfile est true, la définir comme photo de profil
	photo := &Photo{
		UserID:    userID,
		FilePath:  urlPath,
		IsProfile: isProfile || len(photos) == 0,
	}

	// Ajouter la photo dans la base de données
	if err := s.profileRepo.AddPhoto(photo); err != nil {
		// Supprimer le fichier en cas d'erreur
		os.Remove(filePath)
		return nil, fmt.Errorf("erreur lors de l'ajout de la photo dans la base de données: %w", err)
	}

	// Si c'est une photo de profil et qu'il y en a déjà une, mettre à jour
	if isProfile && len(photos) > 0 {
		if err := s.profileRepo.SetProfilePhoto(photo.ID); err != nil {
			return nil, fmt.Errorf("erreur lors de la définition de la photo de profil: %w", err)
		}
	}

	// Vérification finale
	if photo.UserID != userID {
		return nil, fmt.Errorf("erreur d'association utilisateur: photo.UserID=%d != userID=%d", photo.UserID, userID)
	}

	return photo, nil
}

// GetUserByID récupère les informations d'un utilisateur par son ID
func (s *ProfileService) GetUserByID(userID int) (*models.User, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération de l'utilisateur: %w", err)
	}
	return user, nil
}

// UpdateUserOnlineStatus met à jour le statut en ligne d'un utilisateur
func (s *ProfileService) UpdateUserOnlineStatus(userID int, isOnline bool) error {
	if err := s.profileRepo.SetOnline(userID, isOnline); err != nil {
		return fmt.Errorf("erreur lors de la mise à jour du statut en ligne: %w", err)
	}
	return nil
}

// CleanupInactiveUsers nettoie les utilisateurs inactifs
func (s *ProfileService) CleanupInactiveUsers(timeoutMinutes int) error {
	err := s.profileRepo.CleanupInactiveUsers(timeoutMinutes)
	if err != nil {
		return fmt.Errorf("erreur lors du nettoyage des utilisateurs inactifs: %w", err)
	}
	return nil
}

// GetUserOnlineStatus récupère le statut en ligne d'un utilisateur
func (s *ProfileService) GetUserOnlineStatus(userID int) (bool, *time.Time, error) {
	return s.profileRepo.GetUserOnlineStatus(userID)
}

// ReportUser signale un utilisateur
func (s *ProfileService) ReportUser(reporterID, reportedID int, reason string) error {
	// Vérifier qu'on ne se signale pas soi-même
	if reporterID == reportedID {
		return fmt.Errorf("vous ne pouvez pas vous signaler vous-même")
	}

	// Vérifier que l'utilisateur signalé existe
	_, err := s.userRepo.GetByID(reportedID)
	if err != nil {
		return fmt.Errorf("utilisateur à signaler non trouvé")
	}

	// Enregistrer le signalement
	if err := s.profileRepo.ReportUser(reporterID, reportedID, reason); err != nil {
		return fmt.Errorf("erreur lors du signalement: %w", err)
	}

	return nil
}

// GetAllReports récupère tous les signalements (pour les admins)
func (s *ProfileService) GetAllReports() ([]ReportData, error) {
	return s.profileRepo.GetAllReports()
}

// ProcessReport traite un signalement
func (s *ProfileService) ProcessReport(reportID int, adminComment, action string) error {
	return s.profileRepo.ProcessReport(reportID, adminComment, action)
}

// IsProfileComplete vérifie si un profil remplit toutes les conditions obligatoires
func (s *ProfileService) IsProfileComplete(profile *Profile) bool {
	// 1. Genre obligatoire
	if profile.Gender == "" {
		return false
	}

	// 2. Préférence sexuelle obligatoire
	if profile.SexualPreference == "" {
		return false
	}

	// 3. Biographie obligatoire (non vide après trim)
	if strings.TrimSpace(profile.Biography) == "" {
		return false
	}

	// 4. Date de naissance obligatoire
	if profile.BirthDate == nil {
		return false
	}

	// 5. Au moins un tag/intérêt obligatoire
	if len(profile.Tags) == 0 {
		return false
	}

	// 6. Au moins une photo de profil obligatoire
	hasProfilePhoto := false
	for _, photo := range profile.Photos {
		if photo.IsProfile {
			hasProfilePhoto = true
			break
		}
	}
	if !hasProfilePhoto {
		return false
	}

	return true
}
