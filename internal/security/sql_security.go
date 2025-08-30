// internal/security/sql_security.go
package security

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

/* minimal, focused security helpers */

type SQLSecurity struct{ db *sql.DB }

var (
	htmlTagRe   = regexp.MustCompile(`(?is)<[^>]*>`)
	userRe      = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	emailRe     = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	tagRe       = regexp.MustCompile(`^#[a-zA-Z0-9]+$`)
	nameRe      = regexp.MustCompile(`^[a-zA-ZÀ-ÿ\s'\-]+$`)
	sqlDanger   = []string{
		"' or 1=1", "' or '1'='1'", "'; drop ", "'; delete ", "'; insert ", "'; update ",
		"union select", "union all select", "admin'--", "admin'/*", "waitfor delay",
		"pg_sleep(", "sleep(", "benchmark(", "load_file(", "into outfile",
		"information_schema", "@@version", "/**/", "0x3c736372697074",
	}
	sqlCtx = []string{
		"' and ", "' or ", "' union ", "' having ", "' group by ", "' order by ",
		"'; ", "'=", "'<", "'>", "' like ", "'||",
	}
	sqlCmd = []string{"; drop ", "; delete ", "; insert ", "; update ", "; create ", "; alter "}
)

func containsSQLInjection(input string) bool {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return false
	}
	for _, p := range sqlDanger {
		if strings.Contains(s, p) {
			return true
		}
	}
	if strings.Contains(s, "'") {
		for _, p := range sqlCtx {
			if strings.Contains(s, p) {
				return true
			}
		}
	}
	for _, p := range sqlCmd {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

func ValidateBiographyContent(bio string) error {
	s := strings.ToLower(strings.TrimSpace(bio))
	for _, p := range []string{"'; drop", "'; delete", "union select", "' or 1=1", "admin'--", "waitfor delay", "/**/", "@@version"} {
		if strings.Contains(s, p) {
			return fmt.Errorf("contenu suspect détecté")
		}
	}
	return nil
}

func ValidateUserInput(input, field string) error {
	switch field {
	case "username":
		return validateUsername(input)
	case "email":
		return validateEmail(input)
	case "biography":
		return validateBiography(input)
	case "tag":
		return validateTag(input)
	case "firstname":
		return validateName(input, "prénom")
	case "lastname":
		return validateName(input, "nom")
	default:
		return validateGeneral(input)
	}
}

func validateUsername(v string) error {
	if containsSQLInjection(v) || !userRe.MatchString(v) {
		return fmt.Errorf("nom d'utilisateur invalide")
	}
	return nil
}

func validateEmail(v string) error {
	if containsSQLInjection(v) || !emailRe.MatchString(v) {
		return fmt.Errorf("format d'email invalide")
	}
	return nil
}

func validateBiography(v string) error {
	if containsSQLInjection(v) || containsHTML(v) {
		return fmt.Errorf("biographie contient des caractères non autorisés")
	}
	return nil
}

func validateTag(v string) error {
	if containsSQLInjection(v) || !tagRe.MatchString(v) {
		return fmt.Errorf("format de tag invalide")
	}
	return nil
}

func validateName(v, label string) error {
	if containsSQLInjection(v) || containsHTML(v) || !nameRe.MatchString(v) {
		return fmt.Errorf("%s contient des caractères non autorisés", label)
	}
	return nil
}

func validateGeneral(v string) error {
	if containsSQLInjection(v) {
		return fmt.Errorf("entrée contient des caractères non autorisés")
	}
	return nil
}

func containsHTML(input string) bool { return htmlTagRe.MatchString(input) }

func LogSuspiciousActivity(userID int, input, endpoint string) {
	msg := input
	if len(msg) > 200 {
		msg = msg[:200]
	}
	msg = strings.ReplaceAll(msg, "\n", " ")
	msg = strings.ReplaceAll(msg, "\r", " ")
	fmt.Printf("SECURITY ALERT user=%d endpoint=%s input_preview=%q\n", userID, endpoint, msg)
}
