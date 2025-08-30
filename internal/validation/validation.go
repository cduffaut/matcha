// internal/validation/validation.go
package validation

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode"
)

/* ---- règles ---- */

const (
	MinPasswordLength  = 8
	MaxPasswordLength  = 128
	MinUsernameLength  = 3
	MaxUsernameLength  = 30
	MaxBiographyLength = 1000
	MaxTagLength       = 50
	MinNameLength      = 2
	MaxNameLength      = 50
)

/* ---- erreurs ---- */

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }

type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "aucune erreur de validation"
	}
	msgs := make([]string, 0, len(e))
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

/* ---- regex précompilées ---- */

var (
	reUsername   = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	reName       = regexp.MustCompile(`^[A-Za-zÀ-ÿ\s\-']+$`)
	reHTMLTag    = regexp.MustCompile(`<[^>]*>`)
	reCtrl       = regexp.MustCompile(`[\x00-\x1f\x7f]`)
	reMultiSpace = regexp.MustCompile(`\s+`)
)

/* ---- helpers ---- */

func containsHTMLTags(s string) bool { return reHTMLTag.MatchString(s) }

func containsSQLInjectionPatterns(s string) bool {
	// volontairement restrictif aux patterns XSS/HTML évidents
	l := strings.ToLower(s)
	danger := []string{
		"<script", "</script", "javascript:", "data:text/html", "onerror=", "onload=", "onclick=",
	}
	for _, d := range danger {
		if strings.Contains(l, d) {
			return true
		}
	}
	return false
}

func inSet[T comparable](v T, set map[T]struct{}) bool {
	_, ok := set[v]
	return ok
}

/* ---- validations champs ---- */

func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return ValidationError{"email", "l'email est obligatoire"}
	}
	if len(email) > 254 {
		return ValidationError{"email", "l'email est trop long (max 254 caractères)"}
	}
	if containsHTMLTags(email) || containsSQLInjectionPatterns(email) {
		return ValidationError{"email", "l'email contient des caractères non autorisés"}
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return ValidationError{"email", "format d'email invalide"}
	}
	return nil
}

func ValidateUsername(username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return ValidationError{"username", "le nom d'utilisateur est obligatoire"}
	}
	if l := len(username); l < MinUsernameLength || l > MaxUsernameLength {
		return ValidationError{"username", fmt.Sprintf("le nom d'utilisateur doit contenir entre %d et %d caractères", MinUsernameLength, MaxUsernameLength)}
	}
	if !reUsername.MatchString(username) {
		return ValidationError{"username", "seules lettres, chiffres et _ sont autorisés"}
	}
	return nil
}

func ValidatePassword(password string) error {
	if password == "" {
		return ValidationError{"password", "le mot de passe est obligatoire"}
	}
	if l := len(password); l < MinPasswordLength || l > MaxPasswordLength {
		return ValidationError{"password", fmt.Sprintf("le mot de passe doit contenir entre %d et %d caractères", MinPasswordLength, MaxPasswordLength)}
	}
	var lower, upper, digit bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		}
	}
	if !lower {
		return ValidationError{"password", "au moins une lettre minuscule requise"}
	}
	if !upper {
		return ValidationError{"password", "au moins une lettre majuscule requise"}
	}
	if !digit {
		return ValidationError{"password", "au moins un chiffre requis"}
	}
	return nil
}

func ValidateName(name, field string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ValidationError{field, fmt.Sprintf("le %s est obligatoire", field)}
	}
	if l := len(name); l < MinNameLength || l > MaxNameLength {
		return ValidationError{field, fmt.Sprintf("le %s doit contenir entre %d et %d caractères", field, MinNameLength, MaxNameLength)}
	}
	if containsHTMLTags(name) || containsSQLInjectionPatterns(name) {
		return ValidationError{field, fmt.Sprintf("le %s contient des caractères non autorisés", field)}
	}
	if !reName.MatchString(name) {
		return ValidationError{field, fmt.Sprintf("le %s contient des caractères non autorisés", field)}
	}
	return nil
}

func ValidateBiography(bio string) error {
	if len(bio) > MaxBiographyLength {
		return ValidationError{"biography", fmt.Sprintf("la biographie doit contenir au maximum %d caractères", MaxBiographyLength)}
	}
	if containsHTMLTags(bio) {
		return ValidationError{"biography", "la biographie ne peut pas contenir de balises HTML"}
	}
	return nil
}

func ValidateTag(tag string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ValidationError{"tag", "le tag ne peut pas être vide"}
	}
	if !strings.HasPrefix(tag, "#") {
		return ValidationError{"tag", "le tag doit commencer par # (ex. #sport)"}
	}
	if len(tag) > MaxTagLength {
		return ValidationError{"tag", fmt.Sprintf("le tag est trop long (max %d caractères)", MaxTagLength)}
	}
	content := tag[1:]
	if len(content) < 2 {
		return ValidationError{"tag", "au moins 2 caractères après #"}
	}
	for _, r := range content {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return ValidationError{"tag", "utilisez uniquement des lettres et chiffres (sans espaces ni symboles)"}
		}
	}
	return nil
}

func ValidateGender(g string) error {
	valid := map[string]struct{}{"": {}, "male": {}, "female": {}}
	if !inSet(g, valid) {
		return ValidationError{"gender", "genre invalide"}
	}
	return nil
}

func ValidateSexualPreference(p string) error {
	valid := map[string]struct{}{"": {}, "heterosexual": {}, "homosexual": {}, "bisexual": {}}
	if !inSet(p, valid) {
		return ValidationError{"sexual_preference", "préférence sexuelle invalide"}
	}
	return nil
}

func ValidateCoordinates(lat, lon float64) error {
	if lat < -90 || lat > 90 {
		return ValidationError{"latitude", "latitude invalide (entre -90 et 90)"}
	}
	if lon < -180 || lon > 180 {
		return ValidationError{"longitude", "longitude invalide (entre -180 et 180)"}
	}
	return nil
}

/* ---- sanitisation ---- */

func SanitizeInput(s string) string {
	s = strings.TrimSpace(s)
	s = reCtrl.ReplaceAllString(s, "")
	s = reMultiSpace.ReplaceAllString(s, " ")
	return s
}

/* ---- agrégat inscription ---- */

func ValidateRegistration(username, email, firstName, lastName, password string) ValidationErrors {
	var errs ValidationErrors
	if err := ValidateUsername(username); err != nil {
		errs = append(errs, err.(ValidationError))
	}
	if err := ValidateEmail(email); err != nil {
		errs = append(errs, err.(ValidationError))
	}
	if err := ValidateName(firstName, "prénom"); err != nil {
		errs = append(errs, err.(ValidationError))
	}
	if err := ValidateName(lastName, "nom"); err != nil {
		errs = append(errs, err.(ValidationError))
	}
	if err := ValidatePassword(password); err != nil {
		errs = append(errs, err.(ValidationError))
	}
	return errs
}
