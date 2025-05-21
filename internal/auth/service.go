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
	"golang.org/x/crypto/bcrypt"
)

// Service représente le service d'authentification
type Service struct {
	userRepo     user.Repository
	emailService *email.Service
	baseURL      string
}

// NewService crée un nouveau service d'authentification
func NewService(userRepo user.Repository, emailService *email.Service, baseURL string) *Service {
	return &Service{
		userRepo:     userRepo,
		emailService: emailService,
		baseURL:      baseURL,
	}
}

// RegisterRequest contient les données pour l'inscription
type RegisterRequest struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Password  string `json:"password"`
}

// LoginRequest contient les données pour la connexion
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ForgotPasswordRequest contient les données pour la récupération de mot de passe
type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// ResetPasswordRequest contient les données pour la réinitialisation de mot de passe
type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// Register inscrit un nouvel utilisateur
func (s *Service) Register(req RegisterRequest) (*models.User, error) {
	// Vérifier si l'utilisateur existe déjà
	existingUser, err := s.userRepo.GetByUsername(req.Username)
	if err == nil && existingUser != nil {
		return nil, fmt.Errorf("ce nom d'utilisateur existe déjà")
	}

	existingUser, err = s.userRepo.GetByEmail(req.Email)
	if err == nil && existingUser != nil {
		return nil, fmt.Errorf("cet email est déjà utilisé")
	}

	// Hash du mot de passe
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("erreur lors du hachage du mot de passe: %w", err)
	}

	// Générer un token de vérification
	verificationToken, err := generateRandomToken(32)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la génération du token: %w", err)
	}

	// Créer l'utilisateur
	newUser := &models.User{
		Username:          req.Username,
		Email:             req.Email,
		FirstName:         req.FirstName,
		LastName:          req.LastName,
		Password:          string(hashedPassword),
		VerificationToken: &verificationToken,
		IsVerified:        false,
	}

	// Sauvegarder l'utilisateur
	if err := s.userRepo.Create(newUser); err != nil {
		return nil, fmt.Errorf("erreur lors de la création de l'utilisateur: %w", err)
	}

	// TODO: Envoyer l'email de vérification
	verificationLink := fmt.Sprintf("%s/verify-email?token=%s", s.baseURL, verificationToken)
	if err := s.emailService.SendVerificationEmail(newUser.Email, newUser.Username, verificationLink); err != nil {
		log.Printf("Erreur lors de l'envoi de l'email de vérification: %v", err)
	}
	fmt.Printf("Lien de vérification: %s\n", verificationLink)

	return newUser, nil
}

// VerifyEmail vérifie l'email d'un utilisateur avec un token
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

// Login connecte un utilisateur
func (s *Service) Login(req LoginRequest) (*models.User, error) {
	user, err := s.userRepo.GetByUsername(req.Username)
	if err != nil {
		fmt.Println("err:", err)
		return nil, fmt.Errorf("nom d'utilisateur ou mot de passe incorrect")
	}
	fmt.Printf("⚠️ Utilisateur trouvé: %+v\n", user)
	fmt.Printf("⚠️ Comparaison de mot de passe: '%s' (haché) vs '%s' (saisi)\n", user.Password, req.Password)

	if !user.IsVerified {
		return nil, fmt.Errorf("veuillez vérifier votre adresse email avant de vous connecter")
	}

	// Vérifier le mot de passe
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		return nil, fmt.Errorf("nom d'utilisateur ou mot de passe incorrect")
	}

	return user, nil
}

// ForgotPassword envoie un email pour réinitialiser le mot de passe
func (s *Service) ForgotPassword(req ForgotPasswordRequest) error {
	user, err := s.userRepo.GetByEmail(req.Email)
	if err != nil {
		// Ne pas révéler si l'email existe ou non pour des raisons de sécurité
		return nil
	}

	// Générer un token de réinitialisation
	resetToken, err := generateRandomToken(32)
	if err != nil {
		return fmt.Errorf("erreur lors de la génération du token: %w", err)
	}

	// Définir une durée d'expiration (24 heures)
	expiry := time.Now().Add(24 * time.Hour)

	// Sauvegarder le token
	if err := s.userRepo.SaveResetToken(user.ID, resetToken, expiry); err != nil {
		return fmt.Errorf("erreur lors de l'enregistrement du token: %w", err)
	}

	// TODO: Envoyer l'email de réinitialisation
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", s.baseURL, resetToken)
	err = s.emailService.SendPasswordResetEmail(user.Email, user.Username, resetLink)
	if err != nil {
		log.Printf("Erreur lors de l'envoi de l'email de réinitialisation: %v", err)
		// On continue malgré l'erreur pour ne pas révéler si l'email existe
	}
	fmt.Printf("Lien de réinitialisation: %s\n", resetLink)

	return nil
}

// ResetPassword réinitialise le mot de passe d'un utilisateur
func (s *Service) ResetPassword(req ResetPasswordRequest) error {
	user, err := s.userRepo.GetByResetToken(req.Token)
	if err != nil {
		return fmt.Errorf("token de réinitialisation invalide ou expiré: %w", err)
	}

	// Hash du nouveau mot de passe
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("erreur lors du hachage du mot de passe: %w", err)
	}

	// Mettre à jour le mot de passe
	if err := s.userRepo.UpdatePassword(user.ID, string(hashedPassword)); err != nil {
		return fmt.Errorf("erreur lors de la mise à jour du mot de passe: %w", err)
	}

	return nil
}

// generateRandomToken génère un token aléatoire de la taille spécifiée
func generateRandomToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
