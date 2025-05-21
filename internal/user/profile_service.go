package user

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProfileService fournit des services liés aux profils utilisateurs
type ProfileService struct {
	profileRepo ProfileRepository
	userRepo    Repository
	uploadDir   string
}

// NewProfileService crée un nouveau service de profil
func NewProfileService(profileRepo ProfileRepository, userRepo Repository, uploadDir string) *ProfileService {
	return &ProfileService{
		profileRepo: profileRepo,
		userRepo:    userRepo,
		uploadDir:   uploadDir,
	}
}

// GetProfile récupère le profil d'un utilisateur
func (s *ProfileService) GetProfile(userID int) (*Profile, error) {
	profile, err := s.profileRepo.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération du profil: %w", err)
	}
	// Dans la méthode ProfileService.GetProfile
	fmt.Printf("Profil récupéré avec date de naissance: %v\n", profile.BirthDate)
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

	fmt.Printf("Mise à jour du profil avec date de naissance: %v\n", profile.BirthDate)

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

// ReportUser enregistre un signalement d'un utilisateur par un autre
func (r *PostgresProfileRepository) ReportUser(reporterID, reportedID int, reason string) error {
	// Vérifier si la table de signalements existe, sinon la créer
	_, err := r.db.Exec(`
		CREATE TABLE IF NOT EXISTS user_reports (
			id SERIAL PRIMARY KEY,
			reporter_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			reported_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			reason TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			is_processed BOOLEAN DEFAULT FALSE,
			processed_at TIMESTAMP,
			admin_comment TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("erreur lors de la vérification de la table user_reports: %w", err)
	}

	// Vérifier si un rapport existe déjà
	var exists bool
	err = r.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM user_reports
			WHERE reporter_id = $1 AND reported_id = $2
		)
	`, reporterID, reportedID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("erreur lors de la vérification des rapports existants: %w", err)
	}

	// Si un rapport existe déjà, le mettre à jour
	if exists {
		_, err = r.db.Exec(`
			UPDATE user_reports
			SET reason = $3, created_at = CURRENT_TIMESTAMP, is_processed = FALSE, processed_at = NULL, admin_comment = NULL
			WHERE reporter_id = $1 AND reported_id = $2
		`, reporterID, reportedID, reason)
		if err != nil {
			return fmt.Errorf("erreur lors de la mise à jour du rapport: %w", err)
		}
	} else {
		// Sinon, créer un nouveau rapport
		_, err = r.db.Exec(`
			INSERT INTO user_reports (reporter_id, reported_id, reason)
			VALUES ($1, $2, $3)
		`, reporterID, reportedID, reason)
		if err != nil {
			return fmt.Errorf("erreur lors de l'enregistrement du rapport: %w", err)
		}
	}

	// Incrémenter le nombre de signalements sur le profil signalé
	// Cela pourrait affecter le fame rating ou d'autres métriques
	_, err = r.db.Exec(`
		UPDATE user_profiles
		SET updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1
	`, reportedID)
	if err != nil {
		return fmt.Errorf("erreur lors de la mise à jour du profil signalé: %w", err)
	}

	return nil
}

// SetOnline met à jour le statut en ligne d'un utilisateur
func (r *PostgresProfileRepository) SetOnline(userID int, isOnline bool) error {
	// Vérifier si la colonne is_online existe dans la table user_profiles
	var columnExists bool
	err := r.db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.columns 
			WHERE table_name = 'user_profiles' AND column_name = 'is_online'
		)
	`).Scan(&columnExists)
	if err != nil {
		return fmt.Errorf("erreur lors de la vérification de la colonne is_online: %w", err)
	}

	// Si la colonne n'existe pas, l'ajouter
	if !columnExists {
		_, err = r.db.Exec(`
			ALTER TABLE user_profiles 
			ADD COLUMN is_online BOOLEAN DEFAULT FALSE,
			ADD COLUMN last_connection TIMESTAMP
		`)
		if err != nil {
			return fmt.Errorf("erreur lors de l'ajout des colonnes de status en ligne: %w", err)
		}
	}

	// Mettre à jour le statut en ligne
	query := `
		UPDATE user_profiles
		SET is_online = $2, 
			last_connection = CASE WHEN $2 = false THEN CURRENT_TIMESTAMP ELSE last_connection END
		WHERE user_id = $1
	`
	_, err = r.db.Exec(query, userID, isOnline)
	if err != nil {
		return fmt.Errorf("erreur lors de la mise à jour du statut en ligne: %w", err)
	}

	return nil
}

// UpdateLastConnection met à jour l'horodatage de la dernière connexion d'un utilisateur
func (r *PostgresProfileRepository) UpdateLastConnection(userID int) error {
	// Vérifier si la colonne last_connection existe dans la table user_profiles
	var columnExists bool
	err := r.db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.columns 
			WHERE table_name = 'user_profiles' AND column_name = 'last_connection'
		)
	`).Scan(&columnExists)
	if err != nil {
		return fmt.Errorf("erreur lors de la vérification de la colonne last_connection: %w", err)
	}

	// Si la colonne n'existe pas, l'ajouter
	if !columnExists {
		_, err = r.db.Exec(`
			ALTER TABLE user_profiles 
			ADD COLUMN last_connection TIMESTAMP
		`)
		if err != nil {
			return fmt.Errorf("erreur lors de l'ajout de la colonne last_connection: %w", err)
		}
	}

	// Mettre à jour l'horodatage de la dernière connexion
	query := `
		UPDATE user_profiles
		SET last_connection = CURRENT_TIMESTAMP
		WHERE user_id = $1
	`
	_, err = r.db.Exec(query, userID)
	if err != nil {
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

// GetTagByID récupère un tag par son ID
func (r *PostgresProfileRepository) GetTagByID(tagID int) (*Tag, error) {
	query := "SELECT id, name, created_at FROM tags WHERE id = $1"

	var tag Tag
	err := r.db.QueryRow(query, tagID).Scan(&tag.ID, &tag.Name, &tag.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tag non trouvé: ID=%d", tagID)
		}
		return nil, fmt.Errorf("erreur lors de la récupération du tag: %w", err)
	}

	return &tag, nil
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
func (r *PostgresProfileRepository) BlockUser(blockerID, blockedID int) error {
	query := `
        INSERT INTO user_blocks (blocker_id, blocked_id)
        VALUES ($1, $2)
        ON CONFLICT (blocker_id, blocked_id) DO NOTHING
    `

	_, err := r.db.Exec(query, blockerID, blockedID)
	if err != nil {
		return fmt.Errorf("erreur lors du blocage de l'utilisateur: %w", err)
	}

	return nil
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
	userDir := filepath.Join(s.uploadDir, fmt.Sprintf("user_%d", userID))
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

	// Construire le chemin relatif pour l'URL (toujours commencer par /)
	urlPath := fmt.Sprintf("/uploads/user_%d/%s", userID, newFilename)

	// S'assurer que le chemin commence par un slash
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}

	// Si c'est la première photo, ou si isProfile est true, la définir comme photo de profil
	photo := &Photo{
		UserID:    userID,
		FilePath:  urlPath,
		IsProfile: isProfile || len(photos) == 0,
	}

	// Debug: vérifier le chemin de la photo
	fmt.Printf("Photo enregistrée: %s\n", urlPath)

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

	// Supprimer le fichier physique
	filePath := filepath.Join(s.uploadDir, strings.TrimPrefix(photoToDelete.FilePath, "/uploads/"))
	if err := os.Remove(filePath); err != nil {
		// On ne retourne pas d'erreur ici, car la suppression de la base de données est déjà effectuée
		fmt.Printf("Erreur lors de la suppression du fichier: %v\n", err)
	}

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

// IsProfilePhoto vérifie si une photo est la photo de profil d'un utilisateur
func (r *PostgresProfileRepository) IsProfilePhoto(userID int, photoID int) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM user_photos
			WHERE user_id = $1 AND id = $2 AND is_profile = true
		)
	`

	var isProfilePhoto bool
	err := r.db.QueryRow(query, userID, photoID).Scan(&isProfilePhoto)
	if err != nil {
		return false, fmt.Errorf("erreur lors de la vérification de la photo de profil: %w", err)
	}

	return isProfilePhoto, nil
}

// LikeUser enregistre un "like" d'un utilisateur pour un autre
func (s *ProfileService) LikeUser(likerID, likedID int) (bool, error) {
	// Vérifier que l'utilisateur a une photo de profil
	photos, err := s.profileRepo.GetPhotosByUserID(likerID)
	if err != nil {
		return false, fmt.Errorf("erreur lors de la vérification des photos: %w", err)
	}

	var hasProfilePhoto bool
	for _, p := range photos {
		if p.IsProfile {
			hasProfilePhoto = true
			break
		}
	}

	if !hasProfilePhoto {
		return false, errors.New("vous devez avoir une photo de profil pour liker un utilisateur")
	}

	// Enregistrer le like
	if err := s.profileRepo.LikeUser(likerID, likedID); err != nil {
		return false, fmt.Errorf("erreur lors de l'enregistrement du like: %w", err)
	}

	// Vérifier s'il y a un match
	matched, err := s.profileRepo.CheckIfMatched(likerID, likedID)
	if err != nil {
		return false, fmt.Errorf("erreur lors de la vérification du match: %w", err)
	}

	return matched, nil
}

// UnlikeUser supprime un "like" d'un utilisateur pour un autre
func (s *ProfileService) UnlikeUser(likerID, likedID int) error {
	if err := s.profileRepo.UnlikeUser(likerID, likedID); err != nil {
		return fmt.Errorf("erreur lors de la suppression du like: %w", err)
	}
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
