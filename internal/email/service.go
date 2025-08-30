// internal/email/service.go
package email

import (
	"bytes"
	"fmt"
	"log"
	"mime"
	"net/mail"
	"net/smtp"
	"time"
)

type Service struct {
	smtpHost, smtpPort string
	smtpUsername       string
	smtpPassword       string
	fromEmail          string
}

// NewService retourne un service prêt, valeurs vides => mode dev (log uniquement).
func NewService(host, port, user, pass, from string) *Service {
	return &Service{
		smtpHost:     host,
		smtpPort:     port,
		smtpUsername: user,
		smtpPassword: pass,
		fromEmail:    from,
	}
}

func (s *Service) SendVerificationEmail(to, username, link string) error {
	subject := "Vérification de votre compte Matcha"
	body := fmt.Sprintf(`
<html><body>
<h1>Bienvenue sur Matcha, %s !</h1>
<p>Merci de vous être inscrit. Cliquez ci-dessous pour vérifier votre compte :</p>
<p><a href="%s">Vérifier mon compte</a></p>
<p>Valable 24 heures.</p>
</body></html>`, htmlEscape(username), link)
	return s.sendEmail(to, subject, body)
}

func (s *Service) SendPasswordResetEmail(to, username, link string) error {
	subject := "Réinitialisation de votre mot de passe Matcha"
	body := fmt.Sprintf(`
<html><body>
<h1>Réinitialisation de mot de passe</h1>
<p>Bonjour %s,</p>
<p>Pour réinitialiser votre mot de passe, cliquez :</p>
<p><a href="%s">Réinitialiser mon mot de passe</a></p>
<p>Valable 24 heures.</p>
</body></html>`, htmlEscape(username), link)
	return s.sendEmail(to, subject, body)
}

// --- internes ---

func (s *Service) sendEmail(to, subject, htmlBody string) error {
	// Mode dev si SMTP non configuré
	if s.smtpHost == "" || s.smtpPort == "" {
		log.Printf("[DEV EMAIL]\nTo: %s\nSubj: %s\nBody:\n%s\n", to, subject, htmlBody)
		return nil
	}

	// Validation adresses
	if _, err := mail.ParseAddress(to); err != nil {
		return fmt.Errorf("destinataire invalide: %w", err)
	}
	if _, err := mail.ParseAddress(s.fromEmail); err != nil {
		return fmt.Errorf("expéditeur invalide: %w", err)
	}

	raw := buildRFC822(
		s.fromEmail,
		to,
		subject,
		htmlBody,
	)

	addr := fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort)
	auth := smtp.PlainAuth("", s.smtpUsername, s.smtpPassword, s.smtpHost)

	if err := smtp.SendMail(addr, auth, s.fromEmail, []string{to}, raw); err != nil {
		return fmt.Errorf("échec envoi SMTP: %w", err)
	}
	return nil
}

func buildRFC822(from, to, subject, htmlBody string) []byte {
	var b bytes.Buffer

	// Encodage sujet UTF-8 (RFC 2047)
	encSubject := mime.QEncoding.Encode("UTF-8", subject)

	headers := map[string]string{
		"From":         from,
		"To":           to,
		"Subject":      encSubject,
		"Date":         time.Now().Format(time.RFC1123Z),
		"MIME-Version": "1.0",
		"Content-Type": "text/html; charset=UTF-8",
	}

	for k, v := range headers {
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(v)
		b.WriteString("\r\n")
	}
	b.WriteString("\r\n")
	b.WriteString(htmlBody)

	return b.Bytes()
}

// mini-escape suffisant pour noms affichés
func htmlEscape(s string) string {
	// évite l'import html juste pour 3 remplacements
	r := bytes.NewBuffer(make([]byte, 0, len(s)+8))
	for _, c := range s {
		switch c {
		case '&':
			r.WriteString("&amp;")
		case '<':
			r.WriteString("&lt;")
		case '>':
			r.WriteString("&gt;")
		case '"':
			r.WriteString("&quot;")
		case '\'':
			r.WriteString("&#39;")
		default:
			r.WriteRune(c)
		}
	}
	return r.String()
}
