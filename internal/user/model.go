package user

import (
	"time"

	"github.com/cduffaut/matcha/internal/models"
)

// Repository interface pour accéder aux données utilisateur
type Repository interface {
	Create(user *models.User) error
	GetByID(id int) (*models.User, error)
	GetByUsername(username string) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	UpdateVerificationStatus(id int, isVerified bool) error
	SaveVerificationToken(id int, token string) error
	GetByVerificationToken(token string) (*models.User, error)
	SaveResetToken(id int, token string, expiry time.Time) error
	GetByResetToken(token string) (*models.User, error)
	UpdatePassword(id int, password string) error

	// ✅ NOUVELLES MÉTHODES pour modifier nom, prénom, email
	UpdateUserInfo(id int, firstName, lastName, email string) error
	CheckEmailExists(email string, excludeUserID int) (bool, error)
}
