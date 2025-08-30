package user

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/cduffaut/matcha/internal/models"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) Repository {
	return &PostgresRepository{db: db}
}

/* --- helpers --- */

const userSelect = `
	SELECT id, username, email, first_name, last_name, password, is_verified,
	       verification_token, reset_token, reset_token_expiry, created_at, updated_at
	FROM users
`

func scanUser(row *sql.Row) (*models.User, error) {
	var (
		u                 models.User
		verificationToken sql.NullString
		resetToken        sql.NullString
		resetTokenExpiry  sql.NullTime
	)
	if err := row.Scan(
		&u.ID,
		&u.Username,
		&u.Email,
		&u.FirstName,
		&u.LastName,
		&u.Password,
		&u.IsVerified,
		&verificationToken,
		&resetToken,
		&resetTokenExpiry,
		&u.CreatedAt,
		&u.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if verificationToken.Valid {
		u.VerificationToken = &verificationToken.String
	}
	if resetToken.Valid {
		u.ResetToken = &resetToken.String
	}
	if resetTokenExpiry.Valid {
		u.ResetTokenExpiry = &resetTokenExpiry.Time
	}
	return &u, nil
}

/* --- CRUD --- */

func (r *PostgresRepository) Create(user *models.User) error {
	const q = `
		INSERT INTO users (username, email, first_name, last_name, password, verification_token, is_verified)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`
	if err := r.db.QueryRow(
		q,
		user.Username,
		user.Email,
		user.FirstName,
		user.LastName,
		user.Password,
		user.VerificationToken,
		user.IsVerified,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetByID(id int) (*models.User, error) {
	row := r.db.QueryRow(userSelect+` WHERE id = $1`, id)
	u, err := scanUser(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("utilisateur avec ID %d non trouvé", id)
		}
		return nil, fmt.Errorf("get by id: %w", err)
	}
	return u, nil
}

func (r *PostgresRepository) GetByUsername(username string) (*models.User, error) {
	row := r.db.QueryRow(userSelect+` WHERE username = $1`, username)
	u, err := scanUser(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("utilisateur avec nom d'utilisateur %s non trouvé", username)
		}
		return nil, fmt.Errorf("get by username: %w", err)
	}
	return u, nil
}

func (r *PostgresRepository) GetByEmail(email string) (*models.User, error) {
	row := r.db.QueryRow(userSelect+` WHERE email = $1`, email)
	u, err := scanUser(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("utilisateur avec email %s non trouvé", email)
		}
		return nil, fmt.Errorf("get by email: %w", err)
	}
	return u, nil
}

/* --- tokens & verification --- */

func (r *PostgresRepository) UpdateVerificationStatus(id int, isVerified bool) error {
	const q = `
		UPDATE users
		SET is_verified = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`
	_, err := r.db.Exec(q, isVerified, id)
	return err
}

func (r *PostgresRepository) SaveVerificationToken(id int, token string) error {
	const q = `
		UPDATE users
		SET verification_token = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`
	_, err := r.db.Exec(q, token, id)
	return err
}

func (r *PostgresRepository) GetByVerificationToken(token string) (*models.User, error) {
	row := r.db.QueryRow(userSelect+` WHERE verification_token = $1`, token)
	u, err := scanUser(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("token de vérification %s non trouvé", token)
		}
		return nil, fmt.Errorf("get by verification token: %w", err)
	}
	return u, nil
}

func (r *PostgresRepository) SaveResetToken(id int, token string, expiry time.Time) error {
	const q = `
		UPDATE users
		SET reset_token = $1, reset_token_expiry = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
	`
	_, err := r.db.Exec(q, token, expiry, id)
	return err
}

func (r *PostgresRepository) GetByResetToken(token string) (*models.User, error) {
	row := r.db.QueryRow(userSelect+`
		WHERE reset_token = $1 AND reset_token_expiry > CURRENT_TIMESTAMP
	`, token)
	u, err := scanUser(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("token de réinitialisation %s non trouvé ou expiré", token)
		}
		return nil, fmt.Errorf("get by reset token: %w", err)
	}
	return u, nil
}

func (r *PostgresRepository) UpdatePassword(id int, password string) error {
	const q = `
		UPDATE users
		SET password = $1, reset_token = NULL, reset_token_expiry = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`
	_, err := r.db.Exec(q, password, id)
	return err
}

/* --- profile info --- */

func (r *PostgresRepository) UpdateUserInfo(id int, firstName, lastName, email string) error {
	const q = `
		UPDATE users
		SET first_name = $1, last_name = $2, email = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $4
	`
	if _, err := r.db.Exec(q, firstName, lastName, email, id); err != nil {
		return fmt.Errorf("update user info: %w", err)
	}
	return nil
}

func (r *PostgresRepository) CheckEmailExists(email string, excludeUserID int) (bool, error) {
	const q = `SELECT COUNT(*) FROM users WHERE email = $1 AND id != $2`
	var cnt int
	if err := r.db.QueryRow(q, email, excludeUserID).Scan(&cnt); err != nil {
		return false, fmt.Errorf("check email exists: %w", err)
	}
	return cnt > 0, nil
}
