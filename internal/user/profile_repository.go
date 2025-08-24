// internal/user/profile_repository.go
package user

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/cduffaut/matcha/internal/models"
)

// PostgresProfileRepository est l'implémentation PostgreSQL du ProfileRepository
type PostgresProfileRepository struct {
	db *sql.DB
}

// NewPostgresProfileRepository crée un nouveau repository pour les profils
func NewPostgresProfileRepository(db *sql.DB) ProfileRepository {
	return &PostgresProfileRepository{db: db}
}

// GetByUserID récupère le profil d'un utilisateur par son ID
func (r *PostgresProfileRepository) GetByUserID(userID int) (*Profile, error) {
	query := `
        SELECT user_id, gender, sexual_preferences, biography, birth_date, fame_rating, 
               latitude, longitude, location_name, created_at, updated_at
        FROM user_profiles
        WHERE user_id = $1
    `

	profile := &Profile{UserID: userID}
	var gender, sexPref sql.NullString
	var bio sql.NullString
	var birthDate sql.NullTime // Ajout de cette variable
	var lat, long sql.NullFloat64
	var locName sql.NullString

	err := r.db.QueryRow(query, userID).Scan(
		&profile.UserID,
		&gender,
		&sexPref,
		&bio,
		&birthDate, // Utiliser birthDate au lieu de &profile.BirthDate
		&profile.FameRating,
		&lat,
		&long,
		&locName,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// Le profil n'existe pas encore, on retourne un profil vide
			profile.Gender = ""
			profile.SexualPreference = ""
			profile.Biography = ""
			profile.BirthDate = nil // Initialiser à nil
			profile.Latitude = 0
			profile.Longitude = 0
			profile.LocationName = ""
			profile.CreatedAt = time.Now()
			profile.UpdatedAt = time.Now()
			return profile, nil
		}
		return nil, fmt.Errorf("erreur lors de la récupération du profil: %w", err)
	}

	// Assigner les valeurs nullables
	if gender.Valid {
		profile.Gender = Gender(gender.String)
	}
	if sexPref.Valid {
		profile.SexualPreference = SexualPreference(sexPref.String)
	}
	if bio.Valid {
		profile.Biography = bio.String
	}
	if birthDate.Valid { // Ajout de cette condition
		profile.BirthDate = &birthDate.Time
	}
	if lat.Valid {
		profile.Latitude = lat.Float64
	}
	if long.Valid {
		profile.Longitude = long.Float64
	}
	if locName.Valid {
		profile.LocationName = locName.String
	}

	// Récupérer les tags
	tags, err := r.GetTagsByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des tags: %w", err)
	}
	profile.Tags = tags

	// Récupérer les photos
	photos, err := r.GetPhotosByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des photos: %w", err)
	}
	profile.Photos = photos

	return profile, nil
}

// Create crée un nouveau profil utilisateur
func (r *PostgresProfileRepository) Create(profile *Profile) error {
	query := `
        INSERT INTO user_profiles (
            user_id, gender, sexual_preferences, biography, birth_date, fame_rating, 
            latitude, longitude, location_name
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        ON CONFLICT (user_id) DO NOTHING
        RETURNING created_at, updated_at
    `

	err := r.db.QueryRow(
		query,
		profile.UserID,
		profile.Gender,
		profile.SexualPreference,
		profile.Biography,
		profile.BirthDate,
		profile.FameRating,
		profile.Latitude,
		profile.Longitude,
		profile.LocationName,
	).Scan(&profile.CreatedAt, &profile.UpdatedAt)

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("erreur lors de la création du profil: %w", err)
	}

	return nil
}

// Update met à jour un profil utilisateur
func (r *PostgresProfileRepository) Update(profile *Profile) error {
	// CORRECTION: Requête SQL simplifiée et correcte
	query := `
        UPDATE user_profiles
        SET gender = $2, 
            sexual_preferences = $3, 
            biography = $4, 
            birth_date = $5, 
            latitude = $6, 
            longitude = $7, 
            location_name = $8,
            updated_at = CURRENT_TIMESTAMP
        WHERE user_id = $1
        RETURNING updated_at
    `

	fmt.Printf("SQL Update: Mise à jour du profil %d avec birth_date=%v\n", profile.UserID, profile.BirthDate)

	var updatedAt time.Time
	err := r.db.QueryRow(
		query,
		profile.UserID,
		profile.Gender,
		profile.SexualPreference,
		profile.Biography,
		profile.BirthDate,
		profile.Latitude,
		profile.Longitude,
		profile.LocationName,
	).Scan(&updatedAt)

	if err != nil {
		fmt.Printf("SQL Update Error: %v\n", err)
		if err == sql.ErrNoRows {
			// Si le profil n'existe pas encore, on le crée
			return r.Create(profile)
		}
		return fmt.Errorf("erreur lors de la mise à jour du profil: %w", err)
	}

	profile.UpdatedAt = updatedAt
	fmt.Printf("SQL Update Success: Profil %d mis à jour\n", profile.UserID)

	// CORRECTION: Mettre à jour le fame rating séparément
	if err := r.UpdateFameRating(profile.UserID); err != nil {
		fmt.Printf("Erreur mise à jour fame rating: %v\n", err)
		// Ne pas faire échouer toute la mise à jour pour ça
	}

	return nil
}

// AddTag ajoute un tag à un utilisateur
func (r *PostgresProfileRepository) AddTag(userID int, tagName string) error {
	// Vérifier si le tag existe déjà
	var tagID int
	err := r.db.QueryRow("SELECT id FROM tags WHERE name = $1", tagName).Scan(&tagID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Le tag n'existe pas, on le crée
			err = r.db.QueryRow("INSERT INTO tags (name) VALUES ($1) RETURNING id", tagName).Scan(&tagID)
			if err != nil {
				return fmt.Errorf("erreur lors de la création du tag: %w", err)
			}
		} else {
			return fmt.Errorf("erreur lors de la vérification du tag: %w", err)
		}
	}

	// Ajouter l'association utilisateur-tag
	_, err = r.db.Exec(
		"INSERT INTO user_tags (user_id, tag_id) VALUES ($1, $2) ON CONFLICT (user_id, tag_id) DO NOTHING",
		userID, tagID,
	)
	if err != nil {
		return fmt.Errorf("erreur lors de l'ajout du tag à l'utilisateur: %w", err)
	}

	return nil
}

// RemoveTag supprime un tag d'un utilisateur
func (r *PostgresProfileRepository) RemoveTag(userID int, tagID int) error {
	_, err := r.db.Exec("DELETE FROM user_tags WHERE user_id = $1 AND tag_id = $2", userID, tagID)
	if err != nil {
		return fmt.Errorf("erreur lors de la suppression du tag: %w", err)
	}
	return nil
}

// GetTagsByUserID récupère tous les tags d'un utilisateur
func (r *PostgresProfileRepository) GetTagsByUserID(userID int) ([]Tag, error) {
	query := `
        SELECT t.id, t.name, t.created_at
        FROM tags t
        JOIN user_tags ut ON t.id = ut.tag_id
        WHERE ut.user_id = $1
    `

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des tags: %w", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var tag Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.CreatedAt); err != nil {
			return nil, fmt.Errorf("erreur lors de la lecture d'un tag: %w", err)
		}
		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erreur lors du parcours des tags: %w", err)
	}

	return tags, nil
}

// GetAllTags récupère tous les tags disponibles
func (r *PostgresProfileRepository) GetAllTags() ([]Tag, error) {
	query := "SELECT id, name, created_at FROM tags ORDER BY name"

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des tags: %w", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var tag Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.CreatedAt); err != nil {
			return nil, fmt.Errorf("erreur lors de la lecture d'un tag: %w", err)
		}
		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erreur lors du parcours des tags: %w", err)
	}

	return tags, nil
}

// GetTagByID récupère un tag par son ID - MÉTHODE MANQUANTE AJOUTÉE
func (r *PostgresProfileRepository) GetTagByID(tagID int) (*Tag, error) {
	query := "SELECT id, name, created_at FROM tags WHERE id = $1"

	var tag Tag
	err := r.db.QueryRow(query, tagID).Scan(&tag.ID, &tag.Name, &tag.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tag avec ID %d non trouvé", tagID)
		}
		return nil, fmt.Errorf("erreur lors de la récupération du tag: %w", err)
	}

	return &tag, nil
}

// AddPhoto ajoute une photo à un utilisateur
func (r *PostgresProfileRepository) AddPhoto(photo *Photo) error {
	// Compter les photos actuelles
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM user_photos WHERE user_id = $1", photo.UserID).Scan(&count)
	if err != nil {
		return fmt.Errorf("erreur lors du comptage des photos: %w", err)
	}

	// Vérifier la limite de 5 photos
	if count >= 5 {
		return fmt.Errorf("limite de 5 photos atteinte")
	}

	// Si c'est la première photo, la définir comme photo de profil
	if count == 0 {
		photo.IsProfile = true
	}

	query := `
        INSERT INTO user_photos (user_id, file_path, is_profile)
        VALUES ($1, $2, $3)
        RETURNING id, created_at, updated_at
    `

	err = r.db.QueryRow(
		query,
		photo.UserID,
		photo.FilePath,
		photo.IsProfile,
	).Scan(&photo.ID, &photo.CreatedAt, &photo.UpdatedAt)

	if err != nil {
		return fmt.Errorf("erreur lors de l'ajout de la photo: %w", err)
	}

	return nil
}

// RemovePhoto supprime une photo
func (r *PostgresProfileRepository) RemovePhoto(photoID int) error {
	// Vérifier si c'est une photo de profil
	var isProfile bool
	var userID int
	err := r.db.QueryRow("SELECT is_profile, user_id FROM user_photos WHERE id = $1", photoID).Scan(&isProfile, &userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("photo non trouvée")
		}
		return fmt.Errorf("erreur lors de la vérification de la photo: %w", err)
	}

	// Supprimer la photo
	_, err = r.db.Exec("DELETE FROM user_photos WHERE id = $1", photoID)
	if err != nil {
		return fmt.Errorf("erreur lors de la suppression de la photo: %w", err)
	}

	// Si c'était une photo de profil, définir une autre photo comme photo de profil
	if isProfile {
		// CORRECTION : Utiliser une sous-requête au lieu d'ORDER BY direct
		_, err = r.db.Exec(`
            UPDATE user_photos
            SET is_profile = true
            WHERE id = (
                SELECT id FROM user_photos 
                WHERE user_id = $1 
                ORDER BY created_at 
                LIMIT 1
            )
        `, userID)
		// Ignorer l'erreur si pas d'autres photos
		if err != nil {
			// Log mais ne pas faire échouer - normal si plus de photos
			fmt.Printf("Info: Aucune autre photo à définir comme profil pour user %d\n", userID)
		}
	}

	return nil
}

// SetProfilePhoto définit une photo comme photo de profil
func (r *PostgresProfileRepository) SetProfilePhoto(photoID int) error {
	// Récupérer l'ID de l'utilisateur
	var userID int
	err := r.db.QueryRow("SELECT user_id FROM user_photos WHERE id = $1", photoID).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("photo non trouvée")
		}
		return fmt.Errorf("erreur lors de la récupération de la photo: %w", err)
	}

	// Désactiver toutes les photos de profil de l'utilisateur
	_, err = r.db.Exec("UPDATE user_photos SET is_profile = false WHERE user_id = $1", userID)
	if err != nil {
		return fmt.Errorf("erreur lors de la désactivation des photos de profil: %w", err)
	}

	// Définir la nouvelle photo de profil
	_, err = r.db.Exec("UPDATE user_photos SET is_profile = true WHERE id = $1", photoID)
	if err != nil {
		return fmt.Errorf("erreur lors de la définition de la photo de profil: %w", err)
	}

	return nil
}

// IsProfilePhoto vérifie si une photo est une photo de profil - MÉTHODE MANQUANTE AJOUTÉE
func (r *PostgresProfileRepository) IsProfilePhoto(userID int, photoID int) (bool, error) {
	var isProfile bool
	query := `
		SELECT is_profile 
		FROM user_photos 
		WHERE id = $1 AND user_id = $2
	`

	err := r.db.QueryRow(query, photoID, userID).Scan(&isProfile)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, fmt.Errorf("photo non trouvée pour cet utilisateur")
		}
		return false, fmt.Errorf("erreur lors de la vérification de la photo: %w", err)
	}

	return isProfile, nil
}

// GetPhotosByUserID récupère toutes les photos d'un utilisateur
func (r *PostgresProfileRepository) GetPhotosByUserID(userID int) ([]Photo, error) {
	query := `
        SELECT id, user_id, file_path, is_profile, created_at, updated_at
        FROM user_photos
        WHERE user_id = $1
        ORDER BY CASE WHEN is_profile THEN 0 ELSE 1 END, created_at
    `

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des photos: %w", err)
	}
	defer rows.Close()

	var photos []Photo
	for rows.Next() {
		var photo Photo
		if err := rows.Scan(
			&photo.ID,
			&photo.UserID,
			&photo.FilePath,
			&photo.IsProfile,
			&photo.CreatedAt,
			&photo.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("erreur lors de la lecture d'une photo: %w", err)
		}
		photos = append(photos, photo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erreur lors du parcours des photos: %w", err)
	}

	return photos, nil
}

// RecordVisit enregistre une visite de profil
func (r *PostgresProfileRepository) RecordVisit(visitorID, visitedID int) error {
	// Ne pas enregistrer si l'utilisateur visite son propre profil
	if visitorID == visitedID {
		return nil
	}

	query := `
        INSERT INTO profile_visits (visitor_id, visited_id)
        VALUES ($1, $2)
        ON CONFLICT (visitor_id, visited_id) DO UPDATE
        SET visited_at = CURRENT_TIMESTAMP
    `

	_, err := r.db.Exec(query, visitorID, visitedID)
	if err != nil {
		return fmt.Errorf("erreur lors de l'enregistrement de la visite: %w", err)
	}

	// Mettre à jour le fame rating
	if err := r.UpdateFameRating(visitedID); err != nil {
		return fmt.Errorf("erreur lors de la mise à jour du fame rating: %w", err)
	}

	return nil
}

// GetVisitorsForUser récupère les visiteurs d'un utilisateur
func (r *PostgresProfileRepository) GetVisitorsForUser(userID int) ([]ProfileVisit, error) {
	query := `
        SELECT pv.id, pv.visitor_id, pv.visited_id, pv.visited_at,
               u.username, u.first_name, u.last_name
        FROM profile_visits pv
        JOIN users u ON pv.visitor_id = u.id
        WHERE pv.visited_id = $1
        ORDER BY pv.visited_at DESC
    `

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des visiteurs: %w", err)
	}
	defer rows.Close()

	var visits []ProfileVisit
	for rows.Next() {
		var visit ProfileVisit
		var username, firstName, lastName string

		if err := rows.Scan(
			&visit.ID,
			&visit.VisitorID,
			&visit.VisitedID,
			&visit.VisitedAt,
			&username,
			&firstName,
			&lastName,
		); err != nil {
			return nil, fmt.Errorf("erreur lors de la lecture d'une visite: %w", err)
		}

		visit.Visitor = &models.User{
			ID:        visit.VisitorID,
			Username:  username,
			FirstName: firstName,
			LastName:  lastName,
		}

		visits = append(visits, visit)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erreur lors du parcours des visites: %w", err)
	}

	return visits, nil
}

// LikeUser enregistre un "like" entre deux utilisateurs
func (r *PostgresProfileRepository) LikeUser(likerID, likedID int) error {
	// Vérifier si l'utilisateur a une photo de profil
	var hasProfilePhoto bool
	err := r.db.QueryRow(`
        SELECT EXISTS(
            SELECT 1 FROM user_photos
            WHERE user_id = $1 AND is_profile = true
        )
    `, likerID).Scan(&hasProfilePhoto)

	if err != nil {
		return fmt.Errorf("erreur lors de la vérification de la photo de profil: %w", err)
	}

	if !hasProfilePhoto {
		return fmt.Errorf("vous devez avoir une photo de profil pour liker un utilisateur")
	}

	// Enregistrer le like
	query := `
        INSERT INTO user_likes (liker_id, liked_id)
        VALUES ($1, $2)
        ON CONFLICT (liker_id, liked_id) DO NOTHING
    `

	_, err = r.db.Exec(query, likerID, likedID)
	if err != nil {
		return fmt.Errorf("erreur lors de l'enregistrement du like: %w", err)
	}

	// Mettre à jour le fame rating
	if err := r.UpdateFameRating(likedID); err != nil {
		return fmt.Errorf("erreur lors de la mise à jour du fame rating: %w", err)
	}

	return nil
}

// UnlikeUser supprime un "like" entre deux utilisateurs
func (r *PostgresProfileRepository) UnlikeUser(likerID, likedID int) error {
	query := "DELETE FROM user_likes WHERE liker_id = $1 AND liked_id = $2"

	_, err := r.db.Exec(query, likerID, likedID)
	if err != nil {
		return fmt.Errorf("erreur lors de la suppression du like: %w", err)
	}

	// Mettre à jour le fame rating
	if err := r.UpdateFameRating(likedID); err != nil {
		return fmt.Errorf("erreur lors de la mise à jour du fame rating: %w", err)
	}

	return nil
}

// GetLikesForUser récupère les "likes" reçus par un utilisateur
func (r *PostgresProfileRepository) GetLikesForUser(userID int) ([]UserLike, error) {
	query := `
        SELECT ul.id, ul.liker_id, ul.liked_id, ul.created_at,
               u.username, u.first_name, u.last_name
        FROM user_likes ul
        JOIN users u ON ul.liker_id = u.id
        WHERE ul.liked_id = $1
        ORDER BY ul.created_at DESC
    `

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des likes: %w", err)
	}
	defer rows.Close()

	var likes []UserLike
	for rows.Next() {
		var like UserLike
		var username, firstName, lastName string

		if err := rows.Scan(
			&like.ID,
			&like.LikerID,
			&like.LikedID,
			&like.CreatedAt,
			&username,
			&firstName,
			&lastName,
		); err != nil {
			return nil, fmt.Errorf("erreur lors de la lecture d'un like: %w", err)
		}

		like.Liker = &models.User{
			ID:        like.LikerID,
			Username:  username,
			FirstName: firstName,
			LastName:  lastName,
		}

		likes = append(likes, like)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erreur lors du parcours des likes: %w", err)
	}

	return likes, nil
}

// CheckIfLiked vérifie si un utilisateur a liké un autre
func (r *PostgresProfileRepository) CheckIfLiked(likerID, likedID int) (bool, error) {
	var liked bool
	query := `
        SELECT EXISTS(
            SELECT 1 FROM user_likes
            WHERE liker_id = $1 AND liked_id = $2
        )
    `
	err := r.db.QueryRow(query, likerID, likedID).Scan(&liked)
	if err != nil {
		return false, fmt.Errorf("erreur lors de la vérification du like: %w", err)
	}
	return liked, nil
}

// GetAllProfiles récupère tous les profils
func (r *PostgresProfileRepository) GetAllProfiles() ([]*Profile, error) {
	query := `
        SELECT user_id, gender, sexual_preferences, biography, birth_date, fame_rating, 
               latitude, longitude, location_name, created_at, updated_at
        FROM user_profiles
    `

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des profils: %w", err)
	}
	defer rows.Close()

	var profiles []*Profile
	for rows.Next() {
		var profile Profile
		var gender, sexPref sql.NullString
		var bio sql.NullString
		var birthDate sql.NullTime // Ajout de cette variable
		var lat, long sql.NullFloat64
		var locName sql.NullString

		err := rows.Scan(
			&profile.UserID,
			&gender,
			&sexPref,
			&bio,
			&birthDate, // Utiliser birthDate au lieu de &profile.BirthDate
			&profile.FameRating,
			&lat,
			&long,
			&locName,
			&profile.CreatedAt,
			&profile.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("erreur lors de la lecture d'un profil: %w", err)
		}

		// Assigner les valeurs nullables
		if gender.Valid {
			profile.Gender = Gender(gender.String)
		}
		if sexPref.Valid {
			profile.SexualPreference = SexualPreference(sexPref.String)
		}
		if bio.Valid {
			profile.Biography = bio.String
		}
		if birthDate.Valid { // Ajout de cette condition
			profile.BirthDate = &birthDate.Time
		}
		if lat.Valid {
			profile.Latitude = lat.Float64
		}
		if long.Valid {
			profile.Longitude = long.Float64
		}
		if locName.Valid {
			profile.LocationName = locName.String
		}

		// Récupérer les tags
		tags, err := r.GetTagsByUserID(profile.UserID)
		if err == nil {
			profile.Tags = tags
		}

		// Récupérer les photos
		photos, err := r.GetPhotosByUserID(profile.UserID)
		if err == nil {
			profile.Photos = photos
		}

		profiles = append(profiles, &profile)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erreur lors du parcours des profils: %w", err)
	}

	return profiles, nil
}

// IsBlocked vérifie si un utilisateur est bloqué par un autre
func (r *PostgresProfileRepository) IsBlocked(userID1, userID2 int) (bool, error) {
	query := `
        SELECT EXISTS(
            SELECT 1 FROM user_blocks
            WHERE (blocker_id = $1 AND blocked_id = $2) OR (blocker_id = $2 AND blocked_id = $1)
        )
    `

	var blocked bool
	err := r.db.QueryRow(query, userID1, userID2).Scan(&blocked)
	if err != nil {
		return false, fmt.Errorf("erreur lors de la vérification du blocage: %w", err)
	}

	return blocked, nil
}

// CheckIfMatched vérifie si deux utilisateurs se sont mutuellement likés
func (r *PostgresProfileRepository) CheckIfMatched(user1ID, user2ID int) (bool, error) {
	query := `
        SELECT EXISTS(
            SELECT 1 FROM user_likes
            WHERE liker_id = $1 AND liked_id = $2
        ) AND EXISTS(
            SELECT 1 FROM user_likes
            WHERE liker_id = $2 AND liked_id = $1
        )
    `

	var matched bool
	err := r.db.QueryRow(query, user1ID, user2ID).Scan(&matched)
	if err != nil {
		return false, fmt.Errorf("erreur lors de la vérification du match: %w", err)
	}

	return matched, nil
}

// UpdateFameRating met à jour le fame rating d'un utilisateur
func (r *PostgresProfileRepository) UpdateFameRating(userID int) error {
	// Calcul du fame rating basé sur le nombre de visites, de likes et de matchs
	// Formule simple : visites = 1 point, likes = 2 points, matchs = 5 points
	query := `
        WITH user_stats AS (
            SELECT 
                COALESCE(visits.visit_count, 0) as visits,
                COALESCE(likes.like_count, 0) as likes,
                COALESCE(matches.match_count, 0) as matches
            FROM (SELECT 1) dummy
            LEFT JOIN (
                SELECT COUNT(*) as visit_count
                FROM profile_visits 
                WHERE visited_id = $1
            ) visits ON true
            LEFT JOIN (
                SELECT COUNT(*) as like_count
                FROM user_likes 
                WHERE liked_id = $1
            ) likes ON true
            LEFT JOIN (
                SELECT COUNT(*) as match_count
                FROM user_likes ul1
                WHERE ul1.liked_id = $1
                AND EXISTS (
                    SELECT 1 
                    FROM user_likes ul2 
                    WHERE ul2.liker_id = ul1.liked_id 
                    AND ul2.liked_id = ul1.liker_id
                )
            ) matches ON true
        )
        UPDATE user_profiles
        SET fame_rating = (
            SELECT LEAST(100, visits + (likes * 2) + (matches * 5))
            FROM user_stats
        )
        WHERE user_id = $1
    `

	_, err := r.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("erreur lors de la mise à jour du fame rating: %w", err)
	}

	return nil
}

// UnblockUser débloque un utilisateur
func (r *PostgresProfileRepository) UnblockUser(blockerID, blockedID int) error {
	query := `DELETE FROM user_blocks WHERE blocker_id = $1 AND blocked_id = $2`

	_, err := r.db.Exec(query, blockerID, blockedID)
	if err != nil {
		return fmt.Errorf("erreur lors du déblocage de l'utilisateur: %w", err)
	}

	return nil
}

// GetBlockedUsers récupère la liste des utilisateurs bloqués par un utilisateur
func (r *PostgresProfileRepository) GetBlockedUsers(userID int) ([]BlockedUser, error) {
	query := `
		SELECT ub.id, ub.blocker_id, ub.blocked_id, ub.created_at,
		       u.username, u.first_name, u.last_name
		FROM user_blocks ub
		JOIN users u ON ub.blocked_id = u.id
		WHERE ub.blocker_id = $1
		ORDER BY ub.created_at DESC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des utilisateurs bloqués: %w", err)
	}
	defer rows.Close()

	var blockedUsers []BlockedUser
	for rows.Next() {
		var blocked BlockedUser
		var username, firstName, lastName string

		err := rows.Scan(
			&blocked.ID,
			&blocked.BlockerID,
			&blocked.BlockedID,
			&blocked.CreatedAt,
			&username,
			&firstName,
			&lastName,
		)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de la lecture d'un utilisateur bloqué: %w", err)
		}

		// Créer l'objet utilisateur
		blocked.User = &models.User{
			ID:        blocked.BlockedID,
			Username:  username,
			FirstName: firstName,
			LastName:  lastName,
		}

		blockedUsers = append(blockedUsers, blocked)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erreur lors du parcours des utilisateurs bloqués: %w", err)
	}

	return blockedUsers, nil
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

// ReportUser enregistre un signalement d'utilisateur
func (r *PostgresProfileRepository) ReportUser(reporterID, reportedID int, reason string) error {
	query := `
		INSERT INTO user_reports (reporter_id, reported_id, reason)
		VALUES ($1, $2, $3)
		ON CONFLICT (reporter_id, reported_id) DO UPDATE
		SET reason = $3, created_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Exec(query, reporterID, reportedID, reason)
	if err != nil {
		return fmt.Errorf("erreur lors de l'enregistrement du signalement: %w", err)
	}

	return nil
}

// GetAllReports récupère tous les signalements avec informations des utilisateurs
func (r *PostgresProfileRepository) GetAllReports() ([]ReportData, error) {
	query := `
		SELECT ur.id, ur.reporter_id, ur.reported_id, ur.reason, ur.created_at,
		       ur.is_processed, ur.processed_at, ur.admin_comment,
		       u1.username as reporter_username, u1.first_name as reporter_first_name, u1.last_name as reporter_last_name,
		       u2.username as reported_username, u2.first_name as reported_first_name, u2.last_name as reported_last_name
		FROM user_reports ur
		JOIN users u1 ON ur.reporter_id = u1.id
		JOIN users u2 ON ur.reported_id = u2.id
		ORDER BY ur.created_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des signalements: %w", err)
	}
	defer rows.Close()

	var reports []ReportData
	for rows.Next() {
		var report ReportData
		var processedAt sql.NullTime
		var adminComment sql.NullString
		var reporterUsername, reporterFirstName, reporterLastName string
		var reportedUsername, reportedFirstName, reportedLastName string

		err := rows.Scan(
			&report.ID, &report.ReporterID, &report.ReportedID, &report.Reason, &report.CreatedAt,
			&report.IsProcessed, &processedAt, &adminComment,
			&reporterUsername, &reporterFirstName, &reporterLastName,
			&reportedUsername, &reportedFirstName, &reportedLastName,
		)
		if err != nil {
			return nil, fmt.Errorf("erreur lors de la lecture d'un signalement: %w", err)
		}

		if processedAt.Valid {
			report.ProcessedAt = &processedAt.Time
		}
		if adminComment.Valid {
			report.AdminComment = adminComment.String
		}

		// Créer les informations des utilisateurs
		report.ReporterInfo = &models.User{
			ID:        report.ReporterID,
			Username:  reporterUsername,
			FirstName: reporterFirstName,
			LastName:  reporterLastName,
		}

		report.ReportedInfo = &models.User{
			ID:        report.ReportedID,
			Username:  reportedUsername,
			FirstName: reportedFirstName,
			LastName:  reportedLastName,
		}

		reports = append(reports, report)
	}

	return reports, nil
}

// GetUserOnlineStatus récupère le statut en ligne d'un utilisateur
func (r *PostgresProfileRepository) GetUserOnlineStatus(userID int) (bool, *time.Time, error) {
	query := `
		SELECT is_online, last_connection
		FROM user_profiles
		WHERE user_id = $1
	`

	var isOnline bool
	var lastConnection sql.NullTime

	err := r.db.QueryRow(query, userID).Scan(&isOnline, &lastConnection)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil, nil
		}
		return false, nil, fmt.Errorf("erreur lors de la récupération du statut: %w", err)
	}

	var lastConn *time.Time
	if lastConnection.Valid {
		lastConn = &lastConnection.Time
	}

	return isOnline, lastConn, nil
}

func (r *PostgresProfileRepository) SetOnline(userID int, isOnline bool) error {
	// Vérifier les colonnes...
	var columnsExist bool
	err := r.db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.columns 
			WHERE table_name = 'user_profiles' 
			AND column_name IN ('is_online', 'last_connection')
			GROUP BY table_name
			HAVING COUNT(*) = 2
		)
	`).Scan(&columnsExist)
	if err != nil {
		return fmt.Errorf("erreur lors de la vérification des colonnes: %w", err)
	}

	if !columnsExist {
		_, err = r.db.Exec(`
			ALTER TABLE user_profiles 
			ADD COLUMN IF NOT EXISTS is_online BOOLEAN DEFAULT FALSE,
			ADD COLUMN IF NOT EXISTS last_connection TIMESTAMP
		`)
		if err != nil {
			return fmt.Errorf("erreur lors de l'ajout des colonnes: %w", err)
		}
	}

	// CORRECTION : Utiliser UTC explicitement
	nowUTC := time.Now().UTC()

	// Mettre à jour avec timestamp UTC explicite
	query := `
		UPDATE user_profiles
		SET is_online = $2, 
			last_connection = $3
		WHERE user_id = $1
	`
	_, err = r.db.Exec(query, userID, isOnline, nowUTC)
	if err != nil {
		return fmt.Errorf("erreur lors de la mise à jour du statut: %w", err)
	}

	return nil
}

// Nettoyer les utilisateurs inactifs
func (r *PostgresProfileRepository) CleanupInactiveUsers(timeoutMinutes int) error {
	// CORRECTION : Utiliser une comparaison avec timestamp Go au lieu de SQL
	// pour éviter les problèmes de fuseau horaire

	nowUTC := time.Now().UTC()
	cutoffTime := nowUTC.Add(-time.Duration(timeoutMinutes) * time.Minute)

	// Debug : Voir quels utilisateurs vont être nettoyés
	debugQuery := `
		SELECT user_id, is_online, last_connection
		FROM user_profiles
		WHERE is_online = TRUE 
	`

	rows, err := r.db.Query(debugQuery)
	if err == nil {
		defer rows.Close()
		var usersToCleanup []int

		for rows.Next() {
			var userID int
			var isOnline bool
			var lastConnection time.Time

			err := rows.Scan(&userID, &isOnline, &lastConnection)
			if err != nil {
				continue
			}

			// Vérifier si cet utilisateur doit être nettoyé
			if lastConnection.Before(cutoffTime) {
				usersToCleanup = append(usersToCleanup, userID)
			}
		}

		// Effectuer le nettoyage utilisateur par utilisateur
		var totalCleaned int
		for _, userID := range usersToCleanup {
			_, err := r.db.Exec(`UPDATE user_profiles SET is_online = FALSE WHERE user_id = $1`, userID)
			if err != nil {
				fmt.Printf("❌ CLEANUP ERROR: Erreur nettoyage user %d: %v\n", userID, err)
			} else {
				totalCleaned++
			}
		}
	}

	return nil
}

// ProcessReport marque un signalement comme traité
func (r *PostgresProfileRepository) ProcessReport(reportID int, adminComment, action string) error {
	query := `
		UPDATE user_reports 
		SET is_processed = TRUE, 
		    processed_at = CURRENT_TIMESTAMP,
		    admin_comment = $2
		WHERE id = $1
	`

	_, err := r.db.Exec(query, reportID, adminComment)
	if err != nil {
		return fmt.Errorf("erreur lors du traitement du signalement: %w", err)
	}

	// Ici on pourrait ajouter une logique pour appliquer l'action (suspension, etc.)
	// selon la valeur du paramètre 'action'

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
