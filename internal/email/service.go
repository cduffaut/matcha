package email

import (
	"fmt"
)

// Service gère l'envoi d'emails
type Service struct {
	smtpHost     string
	smtpPort     string
	smtpUsername string
	smtpPassword string
	fromEmail    string
}

// NewService crée un nouveau service d'email
func NewService(smtpHost, smtpPort, smtpUsername, smtpPassword, fromEmail string) *Service {
	return &Service{
		smtpHost:     smtpHost,
		smtpPort:     smtpPort,
		smtpUsername: smtpUsername,
		smtpPassword: smtpPassword,
		fromEmail:    fromEmail,
	}
}

// SendVerificationEmail envoie un email de vérification
func (s *Service) SendVerificationEmail(to, username, verificationLink string) error {
	subject := "Vérification de votre compte Matcha"
	body := fmt.Sprintf(`
        <html>
        <body>
            <h1>Bienvenue sur Matcha, %s !</h1>
            <p>Merci de vous être inscrit. Pour vérifier votre compte, veuillez cliquer sur le lien ci-dessous :</p>
            <p><a href="%s">Vérifier mon compte</a></p>
            <p>Ce lien est valable pendant 24 heures.</p>
            <p>Si vous n'êtes pas à l'origine de cette inscription, veuillez ignorer cet email.</p>
        </body>
        </html>
    `, username, verificationLink)

	return s.sendEmail(to, subject, body)
}

// SendPasswordResetEmail envoie un email de réinitialisation de mot de passe
func (s *Service) SendPasswordResetEmail(to, username, resetLink string) error {
	subject := "Réinitialisation de votre mot de passe Matcha"
	body := fmt.Sprintf(`
        <html>
        <body>
            <h1>Réinitialisation de mot de passe</h1>
            <p>Bonjour %s,</p>
            <p>Vous avez demandé une réinitialisation de votre mot de passe. Veuillez cliquer sur le lien ci-dessous pour le réinitialiser :</p>
            <p><a href="%s">Réinitialiser mon mot de passe</a></p>
            <p>Ce lien est valable pendant 24 heures.</p>
            <p>Si vous n'êtes pas à l'origine de cette demande, veuillez ignorer cet email.</p>
        </body>
        </html>
    `, username, resetLink)

	return s.sendEmail(to, subject, body)
}

// sendEmail envoie un email
// func (s *Service) sendEmail(to, subject, body string) error {
// 	addr := fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort)
// 	auth := smtp.PlainAuth("", s.smtpUsername, s.smtpPassword, s.smtpHost)

// 	headers := make(map[string]string)
// 	headers["From"] = s.fromEmail
// 	headers["To"] = to
// 	headers["Subject"] = subject
// 	headers["MIME-Version"] = "1.0"
// 	headers["Content-Type"] = "text/html; charset=UTF-8"

// 	message := ""
// 	for k, v := range headers {
// 		message += fmt.Sprintf("%s: %s\r\n", k, v)
// 	}
// 	message += "\r\n" + body

//		return smtp.SendMail(addr, auth, s.fromEmail, []string{to}, []byte(message))
//	}
func (s *Service) sendEmail(to, subject, body string) error {
	// Version de développement : afficher l'email dans la console au lieu de l'envoyer
	fmt.Println("========== EMAIL ==========")
	fmt.Println("À:", to)
	fmt.Println("Sujet:", subject)
	fmt.Println("Corps:", body)
	fmt.Println("==========================")

	// Simulation de succès
	return nil
}
