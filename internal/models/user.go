package models

import "time"

// User représente un utilisateur du système
type User struct {
	ID                int        `json:"id"`
	Username          string     `json:"username"`
	Email             string     `json:"email"`
	FirstName         string     `json:"first_name"`
	LastName          string     `json:"last_name"`
	Password          string     `json:"-"` // Ne jamais exposer le mot de passe
	IsVerified        bool       `json:"is_verified"`
	VerificationToken *string    `json:"-"`
	ResetToken        *string    `json:"-"`
	ResetTokenExpiry  *time.Time `json:"-"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
