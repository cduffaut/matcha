package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"github.com/cduffaut/matcha/internal/email"
	"github.com/cduffaut/matcha/internal/models"
	"github.com/cduffaut/matcha/internal/user"
	"github.com/cduffaut/matcha/internal/validation"
	"golang.org/x/crypto/bcrypt"
)

// serv d'authentification
type Service struct {
	userRepo     user.Repository
	emailService *email.Service
	baseURL      string
}

// cree un nouveau service d'auth
func NewService(userRepo user.Repository, emailService *email.Service, baseURL string) *Service {
	return &Service{
		userRepo:     userRepo,
		emailService: emailService,
		baseURL:      baseURL,
	}
}

// data pour l'inscription
type RegisterRequest struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Password  string `json:"password"`
}

// data pour la connexion
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// data pour la recup de mdp
type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// data pour la reinitialisation de mdp
type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// data pour la m à j des infos users
type UpdateUserInfoRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

// inscrit un nouv user
func (s *Service) Register(req RegisterRequest) (*models.User, error) {
	// verif si le user existe deja
	existingUser, err := s.userRepo.GetByUsername(req.Username)
	if err == nil && existingUser != nil {
		return nil, fmt.Errorf("ce nom d'utilisateur existe déjà")
	}

	existingUser, err = s.userRepo.GetByEmail(req.Email)
	if err == nil && existingUser != nil {
		return nil, fmt.Errorf("cet email est déjà utilisé")
	}

	// hash du mdp
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("erreur lors du hachage du mot de passe: %w", err)
	}

	// gen un token de verif
	verificationToken, err := generateRandomToken(32)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la génération du token: %w", err)
	}

	// cree user non verif
	newUser := &models.User{
		Username:          req.Username,
		Email:             req.Email,
		FirstName:         req.FirstName,
		LastName:          req.LastName,
		Password:          string(hashedPassword),
		VerificationToken: &verificationToken,
		IsVerified:        false, // user non verif
	}

	// save user
	if err := s.userRepo.Create(newUser); err != nil {
		return nil, fmt.Errorf("erreur lors de la création de l'utilisateur: %w", err)
	}

	// send email de verif
	verificationLink := fmt.Sprintf("%s/verify-email?token=%s", s.baseURL, verificationToken)
	if err := s.emailService.SendVerificationEmail(newUser.Email, newUser.Username, verificationLink); err != nil {
		log.Printf("Erreur lors de l'envoi de l'email de vérification: %v", err)
	}

	fmt.Printf("📧 Email de vérification envoyé à: %s\n", newUser.Email)
	fmt.Printf("🔗 Lien de vérification: %s\n", verificationLink)

	return newUser, nil
}

// verif email user avec un token
func (s *Service) VerifyEmail(token string) error {
	user, err := s.userRepo.GetByVerificationToken(token)
	if err != nil {
		return fmt.Errorf("token de vérification invalide: %w", err)
	}

	if user.IsVerified {
		return fmt.Errorf("cet utilisateur est déjà vérifié")
	}

	if err := s.userRepo.UpdateVerificationStatus(user.ID, true); err != nil {
		return fmt.Errorf("erreur lors de la mise à jour du statut de vérification: %w", err)
	}

	return nil
}

// connecte un user
func (s *Service) Login(req LoginRequest) (*models.User, error) {

	user, err := s.userRepo.GetByUsername(req.Username)
	if err != nil {
		return nil, fmt.Errorf("nom d'utilisateur ou mot de passe incorrect")
	}

	if !user.IsVerified {
		return nil, fmt.Errorf("veuillez vérifier votre adresse email avant de vous connecter")
	}

	// verif le mdp
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		return nil, fmt.Errorf("nom d'utilisateur ou mot de passe incorrect")
	}

	return user, nil
}

// envoie un email pour reinit le mdp
func (s *Service) ForgotPassword(req ForgotPasswordRequest) error {
	user, err := s.userRepo.GetByEmail(req.Email)
	if err != nil {
		// ne pas reveal si mail existe pour des raisons de secu
		return nil
	}

	// gen un token de reinit
	resetToken, err := generateRandomToken(32)
	if err != nil {
		return fmt.Errorf("erreur lors de la génération du token: %w", err)
	}

	// def une duree d'exp (24h)
	expiry := time.Now().Add(24 * time.Hour)

	// save le token
	if err := s.userRepo.SaveResetToken(user.ID, resetToken, expiry); err != nil {
		return fmt.Errorf("erreur lors de l'enregistrement du token: %w", err)
	}

	// send mail de reinit
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", s.baseURL, resetToken)
	err = s.emailService.SendPasswordResetEmail(user.Email, user.Username, resetLink)
	if err != nil {
		log.Printf("Erreur lors de l'envoi de l'email de réinitialisation: %v", err)
	}
	fmt.Printf("Lien de réinitialisation: %s\n", resetLink)

	return nil
}

// reinit le mdp d'un user
func (s *Service) ResetPassword(req ResetPasswordRequest) error {
	user, err := s.userRepo.GetByResetToken(req.Token)
	if err != nil {
		return fmt.Errorf("token de réinitialisation invalide ou expiré: %w", err)
	}

	// hash du nouveau mdp
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("erreur lors du hachage du mot de passe: %w", err)
	}

	// m à j le mdp
	if err := s.userRepo.UpdatePassword(user.ID, string(hashedPassword)); err != nil {
		return fmt.Errorf("erreur lors de la mise à jour du mot de passe: %w", err)
	}

	return nil
}

// gen un token aleatoire de la taille donnee
func generateRandomToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// m à j les infos de base d'un user
func (s *Service) UpdateUserInfo(userID int, req UpdateUserInfoRequest) error {
	if err := validation.ValidateName(req.FirstName, "prénom"); err != nil {
		return err
	}

	if err := validation.ValidateName(req.LastName, "nom"); err != nil {
		return err
	}

	if err := validation.ValidateEmail(req.Email); err != nil {
		return err
	}

	// verif si le mail n'est pas deja utilise par un autre user
	emailExists, err := s.userRepo.CheckEmailExists(req.Email, userID)
	if err != nil {
		return fmt.Errorf("erreur lors de la vérification de l'email: %w", err)
	}

	if emailExists {
		return fmt.Errorf("cette adresse email est déjà utilisée")
	}

	// m à j les infos
	err = s.userRepo.UpdateUserInfo(userID, req.FirstName, req.LastName, req.Email)
	if err != nil {
		return fmt.Errorf("erreur lors de la mise à jour des informations: %w", err)
	}

	return nil
}
