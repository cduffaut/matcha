// internal/validation/validation.go
package validation

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode"
)

// Règles de validation
const (
	MinPasswordLength  = 8
	MaxPasswordLength  = 128
	MinUsernameLength  = 3
	MaxUsernameLength  = 30
	MaxBiographyLength = 1000
	MaxTagLength       = 50
	MaxLocationLength  = 100
	MinNameLength      = 2
	MaxNameLength      = 50
)

// ValidationError représente une erreur de validation
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors représente une liste d'erreurs de validation
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "aucune erreur de validation"
	}

	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

// ValidateEmail valide un email
func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)

	if email == "" {
		return ValidationError{Field: "email", Message: "l'email est obligatoire"}
	}

	if len(email) > 254 {
		return ValidationError{Field: "email", Message: "l'email est trop long (max 254 caractères)"}
	}

	// ✅ NOUVEAU : Protection contre les injections SQL
	if containsSQLInjectionPatterns(email) {
		return ValidationError{Field: "email", Message: "l'email contient des caractères non autorisés"}
	}

	// ✅ NOUVEAU : Protection contre HTML
	if containsHTMLTags(email) {
		return ValidationError{Field: "email", Message: "l'email ne peut pas contenir de balises HTML"}
	}

	_, err := mail.ParseAddress(email)
	if err != nil {
		return ValidationError{Field: "email", Message: "format d'email invalide"}
	}

	return nil
}

// ValidateUsername valide un nom d'utilisateur
func ValidateUsername(username string) error {
	username = strings.TrimSpace(username)

	if username == "" {
		return ValidationError{Field: "username", Message: "le nom d'utilisateur est obligatoire"}
	}

	if len(username) < MinUsernameLength {
		return ValidationError{Field: "username", Message: fmt.Sprintf("le nom d'utilisateur doit contenir au moins %d caractères", MinUsernameLength)}
	}

	if len(username) > MaxUsernameLength {
		return ValidationError{Field: "username", Message: fmt.Sprintf("le nom d'utilisateur doit contenir au maximum %d caractères", MaxUsernameLength)}
	}

	// Seuls les caractères alphanumériques et _ sont autorisés
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_]+$`, username)
	if !matched {
		return ValidationError{Field: "username", Message: "le nom d'utilisateur ne peut contenir que des lettres, chiffres et _"}
	}

	return nil
}

// ValidatePassword valide un mot de passe
func ValidatePassword(password string) error {
	if password == "" {
		return ValidationError{Field: "password", Message: "le mot de passe est obligatoire"}
	}

	if len(password) < MinPasswordLength {
		return ValidationError{Field: "password", Message: fmt.Sprintf("le mot de passe doit contenir au moins %d caractères", MinPasswordLength)}
	}

	if len(password) > MaxPasswordLength {
		return ValidationError{Field: "password", Message: fmt.Sprintf("le mot de passe doit contenir au maximum %d caractères", MaxPasswordLength)}
	}

	// Vérifier qu'il contient au moins une lettre minuscule, une majuscule, un chiffre
	hasLower := false
	hasUpper := false
	hasDigit := false

	for _, char := range password {
		if unicode.IsLower(char) {
			hasLower = true
		}
		if unicode.IsUpper(char) {
			hasUpper = true
		}
		if unicode.IsDigit(char) {
			hasDigit = true
		}
	}

	if !hasLower {
		return ValidationError{Field: "password", Message: "le mot de passe doit contenir au moins une lettre minuscule"}
	}

	if !hasUpper {
		return ValidationError{Field: "password", Message: "le mot de passe doit contenir au moins une lettre majuscule"}
	}

	if !hasDigit {
		return ValidationError{Field: "password", Message: "le mot de passe doit contenir au moins un chiffre"}
	}

	return nil
}

// ValidateName valide un prénom ou nom
func ValidateName(name, fieldName string) error {
	name = strings.TrimSpace(name)

	if name == "" {
		return ValidationError{Field: fieldName, Message: fmt.Sprintf("le %s est obligatoire", fieldName)}
	}

	if len(name) < MinNameLength { // ✅ Utilise la constante
		return ValidationError{Field: fieldName, Message: fmt.Sprintf("le %s doit contenir au moins %d caractères", fieldName, MinNameLength)}
	}

	if len(name) > MaxNameLength { // ✅ Utilise la constante
		return ValidationError{Field: fieldName, Message: fmt.Sprintf("le %s doit contenir au maximum %d caractères", fieldName, MaxNameLength)}
	}

	// ✅ NOUVEAU : Protection contre les injections SQL
	if containsSQLInjectionPatterns(name) {
		return ValidationError{Field: fieldName, Message: fmt.Sprintf("le %s contient des caractères non autorisés", fieldName)}
	}

	// ✅ NOUVEAU : Protection contre HTML
	if containsHTMLTags(name) {
		return ValidationError{Field: fieldName, Message: fmt.Sprintf("le %s ne peut pas contenir de balises HTML", fieldName)}
	}

	// Seules les lettres, espaces, tirets et apostrophes sont autorisés
	matched, _ := regexp.MatchString(`^[a-zA-ZÀ-ÿ\s\-']+$`, name)
	if !matched {
		return ValidationError{Field: fieldName, Message: fmt.Sprintf("le %s contient des caractères non autorisés", fieldName)}
	}

	return nil
}

// ValidateBiography valide une biographie
func ValidateBiography(biography string) error {
	if len(biography) > MaxBiographyLength {
		return ValidationError{Field: "biography", Message: fmt.Sprintf("la biographie doit contenir au maximum %d caractères", MaxBiographyLength)}
	}

	// Nettoyer les caractères dangereux
	if containsHTMLTags(biography) {
		return ValidationError{Field: "biography", Message: "la biographie ne peut pas contenir de balises HTML"}
	}

	return nil
}

// ValidateTag valide un tag
func ValidateTag(tag string) error {
	tag = strings.TrimSpace(tag)

	if tag == "" {
		return ValidationError{Field: "tag", Message: "le tag ne peut pas être vide"}
	}

	if len(tag) > MaxTagLength {
		return ValidationError{Field: "tag", Message: fmt.Sprintf("le tag est trop long (maximum %d caractères)", MaxTagLength)}
	}

	// CORRECTION: Messages très spécifiques pour chaque problème
	if !strings.HasPrefix(tag, "#") {
		return ValidationError{Field: "tag", Message: "le tag doit commencer par # (exemple: #sport)"}
	}

	// Vérifier si le tag ne contient que le #
	if len(tag) == 1 {
		return ValidationError{Field: "tag", Message: "ajoutez du contenu après le # (exemple: #sport)"}
	}

	// Vérifier les caractères autorisés après le #
	tagContent := tag[1:]

	// CORRECTION: Détection spécifique des caractères non autorisés
	invalidChars := []string{}
	for _, char := range tagContent {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
			// Identifier le caractère problématique
			switch char {
			case ' ':
				invalidChars = append(invalidChars, "espace")
			case '.':
				invalidChars = append(invalidChars, "point")
			case ',':
				invalidChars = append(invalidChars, "virgule")
			case '!':
				invalidChars = append(invalidChars, "point d'exclamation")
			case '?':
				invalidChars = append(invalidChars, "point d'interrogation")
			case '@':
				invalidChars = append(invalidChars, "arobase")
			case '&':
				invalidChars = append(invalidChars, "esperluette")
			case '-':
				invalidChars = append(invalidChars, "tiret")
			case '_':
				invalidChars = append(invalidChars, "underscore")
			default:
				invalidChars = append(invalidChars, fmt.Sprintf("'%c'", char))
			}
		}
	}

	if len(invalidChars) > 0 {
		if len(invalidChars) == 1 {
			return ValidationError{Field: "tag", Message: fmt.Sprintf("caractère non autorisé : %s. Utilisez seulement des lettres et chiffres", invalidChars[0])}
		} else {
			return ValidationError{Field: "tag", Message: fmt.Sprintf("caractères non autorisés : %s. Utilisez seulement des lettres et chiffres", strings.Join(invalidChars, ", "))}
		}
	}

	// Vérifier si le tag est trop court (seulement après le #)
	if len(tagContent) < 2 {
		return ValidationError{Field: "tag", Message: "le tag doit contenir au moins 2 caractères après le # (exemple: #sport)"}
	}

	return nil
}

// ValidateGender valide un genre
func ValidateGender(gender string) error {
	validGenders := []string{"", "male", "female"}

	for _, valid := range validGenders {
		if gender == valid {
			return nil
		}
	}

	return ValidationError{Field: "gender", Message: "genre invalide"}
}

// ValidateSexualPreference valide une préférence sexuelle
func ValidateSexualPreference(pref string) error {
	validPrefs := []string{"", "heterosexual", "homosexual", "bisexual"}

	for _, valid := range validPrefs {
		if pref == valid {
			return nil
		}
	}

	return ValidationError{Field: "sexual_preference", Message: "préférence sexuelle invalide"}
}

// ValidateCoordinates valide des coordonnées GPS
func ValidateCoordinates(lat, lon float64) error {
	if lat < -90 || lat > 90 {
		return ValidationError{Field: "latitude", Message: "latitude invalide (doit être entre -90 et 90)"}
	}

	if lon < -180 || lon > 180 {
		return ValidationError{Field: "longitude", Message: "longitude invalide (doit être entre -180 et 180)"}
	}

	return nil
}

// SanitizeInput nettoie une chaîne d'entrée
func SanitizeInput(input string) string {
	// Supprimer les espaces en début et fin
	input = strings.TrimSpace(input)

	// Remplacer les caractères de contrôle
	input = regexp.MustCompile(`[\x00-\x1f\x7f]`).ReplaceAllString(input, "")

	// Limiter les espaces multiples
	input = regexp.MustCompile(`\s+`).ReplaceAllString(input, " ")

	return input
}

// containsHTMLTags vérifie si une chaîne contient des balises HTML
func containsHTMLTags(input string) bool {
	htmlTagPattern := `<[^>]*>`
	matched, _ := regexp.MatchString(htmlTagPattern, input)
	return matched
}

// ValidateRegistration valide tous les champs d'inscription
func ValidateRegistration(username, email, firstName, lastName, password string) ValidationErrors {
	var errors ValidationErrors

	if err := ValidateUsername(username); err != nil {
		errors = append(errors, err.(ValidationError))
	}

	if err := ValidateEmail(email); err != nil {
		errors = append(errors, err.(ValidationError))
	}

	if err := ValidateName(firstName, "prénom"); err != nil {
		errors = append(errors, err.(ValidationError))
	}

	if err := ValidateName(lastName, "nom"); err != nil {
		errors = append(errors, err.(ValidationError))
	}

	if err := ValidatePassword(password); err != nil {
		errors = append(errors, err.(ValidationError))
	}

	return errors
}

// containsSQLInjectionPatterns détecte les tentatives d'injection SQL
func containsSQLInjectionPatterns(input string) bool {
	lowerInput := strings.ToLower(strings.TrimSpace(input))

	dangerousPatterns := []string{
		"'", "\"", ";", "--", "/*", "*/", "union", "select", "insert",
		"update", "delete", "drop", "create", "alter", "script",
		"javascript:", "vbscript:", "onload", "onerror", "onclick",
		"<script", "</script>", "eval(", "expression(",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerInput, pattern) {
			return true
		}
	}

	return false
}
