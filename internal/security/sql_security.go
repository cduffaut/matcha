// internal/security/sql_security.go
package security

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// SQLSecurity fournit des fonctions pour sécuriser les requêtes SQL
type SQLSecurity struct {
	db *sql.DB
}

// NewSQLSecurity crée une nouvelle instance de SQLSecurity
func NewSQLSecurity(db *sql.DB) *SQLSecurity {
	return &SQLSecurity{db: db}
}

// ValidateAndSanitizeSQLInput valide et nettoie les entrées SQL
func ValidateAndSanitizeSQLInput(input string) (string, error) {
	// Supprimer les espaces en début et fin
	input = strings.TrimSpace(input)

	// Vérifier la longueur
	if len(input) > 1000 {
		return "", fmt.Errorf("entrée trop longue")
	}

	// Détecter les tentatives d'injection SQL
	if containsSQLInjection(input) {
		return "", fmt.Errorf("tentative d'injection SQL détectée")
	}

	return input, nil
}

// containsSQLInjection détecte les patterns d'injection SQL
func containsSQLInjection(input string) bool {
	lowerInput := strings.ToLower(strings.TrimSpace(input))

	// NOUVELLE LOGIQUE: Patterns vraiment dangereux avec contexte
	realDangerousPatterns := []string{
		"' or 1=1",           // Classic SQL injection
		"' or '1'='1'",       // String-based injection
		"'; drop table",      // Command injection
		"'; delete from",     // DELETE injection
		"'; insert into",     // INSERT injection
		"'; update ",         // UPDATE injection
		"union select",       // UNION attack
		"union all select",   // UNION ALL attack
		"admin'--",           // Comment-based bypass
		"admin'/*",           // Comment-based bypass
		"waitfor delay",      // Time-based attack
		"pg_sleep(",          // PostgreSQL sleep
		"sleep(",             // MySQL sleep
		"benchmark(",         // MySQL benchmark
		"load_file(",         // File reading
		"into outfile",       // File writing
		"information_schema", // Schema attack
		"@@version",          // System variables
		"/**/",               // Comment evasion
		"0x3c736372697074",   // Hex encoded script
	}

	// Vérifier les patterns vraiment dangereux
	for _, pattern := range realDangerousPatterns {
		if strings.Contains(lowerInput, pattern) {
			return true
		}
	}

	// Vérifier les apostrophes SEULEMENT si elles sont dans un contexte SQL dangereux
	if strings.Contains(lowerInput, "'") {
		sqlContextPatterns := []string{
			"' and ",
			"' or ",
			"' union ",
			"' having ",
			"' group by ",
			"' order by ",
			"'; ",
			"'=",
			"'<",
			"'>",
			"' like ",
			"'||", // Concatenation
		}

		for _, pattern := range sqlContextPatterns {
			if strings.Contains(lowerInput, pattern) {
				return true
			}
		}
	}

	// Vérifier les patterns de commande SQL sans apostrophes
	standaloneCommands := []string{
		"; drop ",
		"; delete ",
		"; insert ",
		"; update ",
		"; create ",
		"; alter ",
	}

	for _, cmd := range standaloneCommands {
		if strings.Contains(lowerInput, cmd) {
			return true
		}
	}

	return false
}

// Fonction spécialisée pour la biographie (encore plus permissive)
func ValidateBiographyContent(bio string) error {
	// Pour une biographie, on est encore plus permissif
	bio = strings.ToLower(strings.TrimSpace(bio))

	// Seulement les patterns VRAIMENT dangereux
	veryDangerousPatterns := []string{
		"'; drop",
		"'; delete",
		"union select",
		"' or 1=1",
		"admin'--",
		"waitfor delay",
		"/**/",
		"@@version",
	}

	for _, pattern := range veryDangerousPatterns {
		if strings.Contains(bio, pattern) {
			return fmt.Errorf("contenu suspect détecté")
		}
	}

	return nil
}

// SafeQuery exécute une requête de manière sécurisée avec validation
func (s *SQLSecurity) SafeQuery(query string, args ...interface{}) (*sql.Rows, error) {
	// Valider les arguments
	for i, arg := range args {
		if str, ok := arg.(string); ok {
			validated, err := ValidateAndSanitizeSQLInput(str)
			if err != nil {
				return nil, fmt.Errorf("argument %d invalide: %w", i, err)
			}
			args[i] = validated
		}
	}

	// Exécuter la requête
	return s.db.Query(query, args...)
}

// SafeExec exécute une requête d'écriture de manière sécurisée
func (s *SQLSecurity) SafeExec(query string, args ...interface{}) (sql.Result, error) {
	// Valider les arguments
	for i, arg := range args {
		if str, ok := arg.(string); ok {
			validated, err := ValidateAndSanitizeSQLInput(str)
			if err != nil {
				return nil, fmt.Errorf("argument %d invalide: %w", i, err)
			}
			args[i] = validated
		}
	}

	// Exécuter la requête
	return s.db.Exec(query, args...)
}

// ValidateUserInput valide spécifiquement les entrées utilisateur
func ValidateUserInput(input string, fieldName string) error {
	// Vérifications spécifiques selon le type de champ
	switch fieldName {
	case "username":
		return validateUsername(input)
	case "email":
		return validateEmail(input)
	case "biography":
		return validateBiography(input)
	case "tag":
		return validateTag(input)
	case "firstname": // ✅ NOUVEAU
		return validateFirstName(input)
	case "lastname": // ✅ NOUVEAU
		return validateLastName(input)
	default:
		// Validation générale
		return validateGeneralInput(input)
	}
}

// validateUsername valide un nom d'utilisateur contre les injections
func validateUsername(username string) error {
	if containsSQLInjection(username) {
		return fmt.Errorf("nom d'utilisateur contient des caractères non autorisés")
	}

	// Seuls les caractères alphanumériques et _ sont autorisés
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_]+$`, username)
	if !matched {
		return fmt.Errorf("nom d'utilisateur invalide")
	}

	return nil
}

// validateEmail valide un email contre les injections
func validateEmail(email string) error {
	if containsSQLInjection(email) {
		return fmt.Errorf("email contient des caractères non autorisés")
	}

	// Pattern email simple mais sécurisé
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, email)
	if !matched {
		return fmt.Errorf("format d'email invalide")
	}

	return nil
}

// validateBiography valide une biographie
func validateBiography(bio string) error {
	if containsSQLInjection(bio) {
		return fmt.Errorf("biographie contient des caractères non autorisés")
	}

	// Supprimer les balises HTML potentiellement dangereuses
	if containsHTML(bio) {
		return fmt.Errorf("biographie ne peut pas contenir de balises HTML")
	}

	return nil
}

// validateTag valide un tag
func validateTag(tag string) error {
	if containsSQLInjection(tag) {
		return fmt.Errorf("tag contient des caractères non autorisés")
	}

	// Les tags doivent être alphanumériques avec #
	matched, _ := regexp.MatchString(`^#[a-zA-Z0-9]+$`, tag)
	if !matched {
		return fmt.Errorf("format de tag invalide")
	}

	return nil
}

// validateGeneralInput validation générale
func validateGeneralInput(input string) error {
	if containsSQLInjection(input) {
		return fmt.Errorf("entrée contient des caractères non autorisés")
	}

	return nil
}

// containsHTML détecte les balises HTML
func containsHTML(input string) bool {
	htmlPattern := `<[^>]*>`
	matched, _ := regexp.MatchString(htmlPattern, input)
	return matched
}

// EscapeString échappe une chaîne pour l'usage SQL (en plus des prepared statements)
func EscapeString(input string) string {
	// Remplacer les caractères dangereux
	replacements := map[string]string{
		"'":    "''",   // Échapper les guillemets simples
		"\\":   "\\\\", // Échapper les backslashes
		"\x00": "\\0",  // NULL byte
		"\n":   "\\n",  // Newline
		"\r":   "\\r",  // Carriage return
		"\x1a": "\\Z",  // EOF
	}

	result := input
	for old, new := range replacements {
		result = strings.ReplaceAll(result, old, new)
	}

	return result
}

// LogSuspiciousActivity enregistre les tentatives d'injection
func LogSuspiciousActivity(userID int, input string, endpoint string) {
	// Dans un vrai projet, enregistrer dans un système de logs sécurisé
	fmt.Printf("SECURITY ALERT: Tentative d'injection détectée - User: %d, Endpoint: %s, Input: %s\n",
		userID, endpoint, input[:min(50, len(input))])
}

// min retourne le minimum entre deux entiers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// validateFirstName valide un prénom contre les injections
func validateFirstName(firstName string) error {
	if containsSQLInjection(firstName) {
		return fmt.Errorf("prénom contient des caractères non autorisés")
	}

	// Supprimer les balises HTML potentiellement dangereuses
	if containsHTML(firstName) {
		return fmt.Errorf("prénom ne peut pas contenir de balises HTML")
	}

	// Seuls les caractères alphabétiques, espaces, apostrophes et tirets sont autorisés
	matched, _ := regexp.MatchString(`^[a-zA-ZÀ-ÿ\s'\-]+$`, firstName)
	if !matched {
		return fmt.Errorf("prénom contient des caractères non autorisés")
	}

	return nil
}

// validateLastName valide un nom de famille contre les injections
func validateLastName(lastName string) error {
	if containsSQLInjection(lastName) {
		return fmt.Errorf("nom contient des caractères non autorisés")
	}

	// Supprimer les balises HTML potentiellement dangereuses
	if containsHTML(lastName) {
		return fmt.Errorf("nom ne peut pas contenir de balises HTML")
	}

	// Seuls les caractères alphabétiques, espaces, apostrophes et tirets sont autorisés
	matched, _ := regexp.MatchString(`^[a-zA-ZÀ-ÿ\s'\-]+$`, lastName)
	if !matched {
		return fmt.Errorf("nom contient des caractères non autorisés")
	}

	return nil
}

// EscapeHTMLForDisplay échappe les caractères HTML pour l'affichage sécurisé
func EscapeHTMLForDisplay(input string) string {
	replacements := map[string]string{
		"<":  "&lt;",
		">":  "&gt;",
		"&":  "&amp;",
		"\"": "&quot;",
		"'":  "&#39;",
	}

	result := input
	for old, new := range replacements {
		result = strings.ReplaceAll(result, old, new)
	}

	return result
}
