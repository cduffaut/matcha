# Justifications Respect du Sujet Matcha

### 1. "Does not include an ORM, validators, or a User Account Manager."

Pas d'ORM > Requetes SQL manuelles dans database/database.go:
	db.Exec(string(content))
	RunMigrations(db *sql.DB) lit et execute les fichiers .sql

Validation maison > internal/validation/validation.go:
	func ValidateEmail(email string) error
	func ValidateUsername(username string) error
	func ValidatePassword(password string) error

Gestion utilisateurs maison > internal/session/
	func (m *Manager) CreateSession(w http.ResponseWriter, user *models.User)
	func (m *Manager) GetSession(r *http.Request)
	func (m *Manager) DestroySession(w http.ResponseWriter, r *http.Request)
	type Session struct {
		UserID    int
		Username  string
		ExpiresAt time.Time
	}

### 2. "You will also need to create your queries manually, like mature developers do."

Requetes manuelles: internal/user/repository.go
	func (r *PostgresRepository) Create(user *models.User) error
	func (r *PostgresRepository) GetByID(id int) (*models.User, error)
	func (r *PostgresRepository) GetByUsername(username string) (*models.User, error)

### 3. "built-in web server."
	internal/config/config.go
	cmd/server/main.go

	fileServer := http.FileServer(http.Dir("web/static"))

### 4. "All your forms should have proper validation"

	internal/validation/validation.go

	func ValidateEmail(email string) error
	func ValidateUsername(username string) error  
	func ValidatePassword(password string) error
	func ValidateName(name, fieldName string) error

### 5. "◦ Storing plain text passwords in your database. 
### 	◦ Allowing injection of HTML or user Javascript code in unprotected variables. ◦ Allowing upload of unwanted content.
### 	◦ Allowing alteration of SQL requests."

1.  Mots de passe haches:
	internal/auth/service.go
	bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	> Utilisation de bcrypt avec salt automatique

	Demonstration:
	psql -U csil -d matcha
	SELECT id, username, email, first_name, last_name, 
       password, is_verified, created_at 
	FROM users;


2. Protection ocontre injection HTML/Javascript:
	internal/security/sql_security.go
	- containsHTML(): détecte balises HTML

	Demonstration: dans un champs du site, inserer: <script>alert('XSS')</script> ou <iframe src=javascript:alert('XSS')> par ex.

	javascript:alert('XSS')

3. Upload securisé:
	internal/security/file_security.go
	ValidateImageFile(): validation complete
	scanForMaliciousContent(): scan de contenu malveillant
	validateFileSize(): limite de taille
	validateFileType(): types fichiers autorises

	Demonstration: tenter d'uploader un fichier .php 

4. Protection contre injections SQL:
	internal/security/sql_security.go
	containsSQLInjection(): detection patterns SQL

	admin' OR 1=1--

5. Locate user without their knowledge:
Si aucune localisation n'est associé au compte, le programme se charge d'appeler différentes API pour obtenir une geolocalisation sans en avertir l'utilisateur:
profile.js > getSilentLocation > getIPLocation (recupère la localisation grace à l'IP)

6. Calcul du pourcentage de compatibilité
Fichier : internal/user/browsing_service.go
Fonction : GetSuggestions
Calcul : 50% distance + 30% tags communs + 20% fame rating

6. != Ordre des profils
Fichier : internal/user/browsing_service.go
Fonction : GetSuggestions
Logique : Tri par zones géographiques (180km, 250km, 350km, 500km), puis par score de compatibilité dans chaque zone.