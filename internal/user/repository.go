package user

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/cduffaut/matcha/internal/models"
)

// PostgresRepository est l'implémentation PostgreSQL du Repository
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository crée un nouveau repository utilisateur
func NewPostgresRepository(db *sql.DB) Repository {
	return &PostgresRepository{db: db}
}

// Create ajoute un nouvel utilisateur dans la base de données
func (r *PostgresRepository) Create(user *models.User) error {
	query := `
        INSERT INTO users (username, email, first_name, last_name, password, verification_token)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id, created_at, updated_at
    `

	return r.db.QueryRow(
		query,
		user.Username,
		user.Email,
		user.FirstName,
		user.LastName,
		user.Password,
		user.VerificationToken,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
}

// GetByID récupère un utilisateur par son ID
func (r *PostgresRepository) GetByID(id int) (*models.User, error) {
	query := `
        SELECT id, username, email, first_name, last_name, password, is_verified, 
               verification_token, reset_token, reset_token_expiry, created_at, updated_at
        FROM users
        WHERE id = $1
    `

	user := &models.User{}
	err := r.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.Password,
		&user.IsVerified,
		&user.VerificationToken,
		&user.ResetToken,
		&user.ResetTokenExpiry,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("utilisateur avec ID %d non trouvé", id)
		}
		return nil, err
	}

	return user, nil
}

// GetByUsername récupère un utilisateur par son nom d'utilisateur
func (r *PostgresRepository) GetByUsername(username string) (*models.User, error) {
	query := `
        SELECT id, username, email, first_name, last_name, password, is_verified, 
               verification_token, reset_token, reset_token_expiry, created_at, updated_at
        FROM users
        WHERE username = $1
    `

	user := &models.User{}
	err := r.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.Password,
		&user.IsVerified,
		&user.VerificationToken, // Maintenant un pointeur
		&user.ResetToken,        // Maintenant un pointeur
		&user.ResetTokenExpiry,  // Maintenant un pointeur
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("utilisateur avec nom d'utilisateur %s non trouvé", username)
		}
		return nil, err
	}

	return user, nil
}

// GetByEmail récupère un utilisateur par son email
func (r *PostgresRepository) GetByEmail(email string) (*models.User, error) {
	query := `
        SELECT id, username, email, first_name, last_name, password, is_verified, 
               verification_token, reset_token, reset_token_expiry, created_at, updated_at
        FROM users
        WHERE email = $1
    `

	user := &models.User{}
	err := r.db.QueryRow(query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.Password,
		&user.IsVerified,
		&user.VerificationToken,
		&user.ResetToken,
		&user.ResetTokenExpiry,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("utilisateur avec email %s non trouvé", email)
		}
		return nil, err
	}

	return user, nil
}

// UpdateVerificationStatus met à jour le statut de vérification d'un utilisateur
func (r *PostgresRepository) UpdateVerificationStatus(id int, isVerified bool) error {
	query := `
        UPDATE users
        SET is_verified = $1, updated_at = CURRENT_TIMESTAMP
        WHERE id = $2
    `

	_, err := r.db.Exec(query, isVerified, id)
	return err
}

// SaveVerificationToken enregistre un token de vérification pour un utilisateur
func (r *PostgresRepository) SaveVerificationToken(id int, token string) error {
	query := `
        UPDATE users
        SET verification_token = $1, updated_at = CURRENT_TIMESTAMP
        WHERE id = $2
    `

	_, err := r.db.Exec(query, token, id)
	return err
}

// GetByVerificationToken récupère un utilisateur par son token de vérification
func (r *PostgresRepository) GetByVerificationToken(token string) (*models.User, error) {
	query := `
        SELECT id, username, email, first_name, last_name, password, is_verified, 
               verification_token, 
               COALESCE(reset_token, ''), -- Utiliser une chaîne vide plutôt que NULL
               COALESCE(reset_token_expiry, '0001-01-01 00:00:00'::timestamp), -- Utiliser une date par défaut
               created_at, updated_at
        FROM users
        WHERE verification_token = $1
    `

	user := &models.User{}
	var resetToken sql.NullString     // Utiliser sql.NullString pour gérer les valeurs NULL
	var resetTokenExpiry sql.NullTime // Utiliser sql.NullTime pour gérer les valeurs NULL

	err := r.db.QueryRow(query, token).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.Password,
		&user.IsVerified,
		&user.VerificationToken,
		&resetToken,
		&resetTokenExpiry,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("token de vérification %s non trouvé", token)
		}
		return nil, err
	}

	// Assigner les valeurs nullable à l'utilisateur
	if resetToken.Valid {
		user.ResetToken = &resetToken.String
	} else {
		user.ResetToken = nil
	}

	if resetTokenExpiry.Valid {
		user.ResetTokenExpiry = &resetTokenExpiry.Time
	} else {
		user.ResetTokenExpiry = &time.Time{}
	}

	return user, nil
}

// SaveResetToken enregistre un token de réinitialisation de mot de passe
func (r *PostgresRepository) SaveResetToken(id int, token string, expiry time.Time) error {
	query := `
        UPDATE users
        SET reset_token = $1, reset_token_expiry = $2, updated_at = CURRENT_TIMESTAMP
        WHERE id = $3
    `

	_, err := r.db.Exec(query, token, expiry, id)
	return err
}

// GetByResetToken récupère un utilisateur par son token de réinitialisation
func (r *PostgresRepository) GetByResetToken(token string) (*models.User, error) {
	query := `
        SELECT id, username, email, first_name, last_name, password, is_verified, 
               verification_token, reset_token, reset_token_expiry, created_at, updated_at
        FROM users
        WHERE reset_token = $1 AND reset_token_expiry > CURRENT_TIMESTAMP
    `

	user := &models.User{}
	err := r.db.QueryRow(query, token).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.Password,
		&user.IsVerified,
		&user.VerificationToken,
		&user.ResetToken,
		&user.ResetTokenExpiry,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("token de réinitialisation %s non trouvé ou expiré", token)
		}
		return nil, err
	}

	return user, nil
}

// UpdatePassword met à jour le mot de passe d'un utilisateur
func (r *PostgresRepository) UpdatePassword(id int, password string) error {
	query := `
        UPDATE users
        SET password = $1, reset_token = NULL, reset_token_expiry = NULL, updated_at = CURRENT_TIMESTAMP
        WHERE id = $2
    `

	_, err := r.db.Exec(query, password, id)
	return err
}
