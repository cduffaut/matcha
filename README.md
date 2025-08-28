Lancement de l'application :
```bash
# créer le dossier pour le volume docker de la db
mkdir -p internal/database/postgresql
# copier et remplir le fichier .env
cp .env.example .env
# lancer le conteneur en arrière-plan
docker compose up --build -d
```

Vérification si tout s'est bien passé : 
```bash
# vérifier les conteneurs en cours d'exec
docker ps
# vérifier les logs du conteneur
docker logs -f matcha-db
docker logs -f matcha-app
```

Stopper l'application :
```bash
docker compose down
# si destruction des volumes
docker volume prune -f
```

<br>

---

<br>

# Justifications Respect du Sujet Matcha

## 1. "Does not include an ORM, validators, or a User Account Manager."

- **Pas d'ORM**  
	Requêtes SQL manuelles dans `database/database.go` :  
	- `db.Exec(string(content))`
	- `RunMigrations(db *sql.DB)` lit et exécute les fichiers `.sql`

- **Validation maison**  
	`internal/validation/validation.go` :  
	- `ValidateEmail(email string) error`
	- `ValidateUsername(username string) error`
	- `ValidatePassword(password string) error`

- **Gestion utilisateurs maison**  
	`internal/session/` :  
	- `CreateSession(w http.ResponseWriter, user *models.User)`
	- `GetSession(r *http.Request)`
	- `DestroySession(w http.ResponseWriter, r *http.Request)`
	- `Session struct { UserID int; Username string; ExpiresAt time.Time }`

---

## 2. "You will also need to create your queries manually, like mature developers do."

- **Requêtes manuelles**  
	`internal/user/repository.go` :  
	- `Create(user *models.User) error`
	- `GetByID(id int) (*models.User, error)`
	- `GetByUsername(username string) (*models.User, error)`

---

## 3. "Built-in web server."

- `internal/config/config.go`
- `cmd/server/main.go`
- `fileServer := http.FileServer(http.Dir("web/static"))`

---

## 4. "All your forms should have proper validation"

- `internal/validation/validation.go` :  
	- `ValidateEmail(email string) error`
	- `ValidateUsername(username string) error`
	- `ValidatePassword(password string) error`
	- `ValidateName(name, fieldName string) error`

---

## 5. Sécurité

### a. "Storing plain text passwords in your database."

- **Mots de passe hachés**  
	`internal/auth/service.go`  
	- `bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)`  
	- Utilisation de bcrypt avec salt automatique  
	- **Démonstration** :  
		```sql
		psql -U csil -d matcha
		SELECT id, username, email, first_name, last_name, password, is_verified, created_at FROM users;
		```

### b. "Allowing injection of HTML or user Javascript code in unprotected variables."

- **Protection contre injection HTML/Javascript**  
	`internal/security/sql_security.go`  
	- `containsHTML()` : détecte balises HTML  
	- **Démonstration** :  
		Dans un champ du site, insérer :  
		`<script>alert('XSS')</script>` ou `<iframe src=javascript:alert('XSS')>`

### c. "Allowing upload of unwanted content."

- **Upload sécurisé**  
	`internal/security/file_security.go`  
	- `ValidateImageFile()` : validation complète
	- `scanForMaliciousContent()` : scan de contenu malveillant
	- `validateFileSize()` : limite de taille
	- `validateFileType()` : types de fichiers autorisés  
	- **Démonstration** : tenter d'uploader un fichier `.php`

### d. "Allowing alteration of SQL requests."

- **Protection contre injections SQL**  
	`internal/security/sql_security.go`  
	- `containsSQLInjection()` : détection patterns SQL  
	- Exemple : `admin' OR 1=1--`

---

## 6. Localisation de l'utilisateur sans son consentement

- Si aucune localisation n'est associée au compte, le programme appelle différentes API pour obtenir une géolocalisation sans en avertir l'utilisateur :  
	- `profile.js > getSilentLocation > getIPLocation` (récupère la localisation grâce à l'IP)

---

## 7. Calcul du pourcentage de compatibilité

- **Fichier** : `internal/user/browsing_service.go`
- **Fonction** : `GetSuggestions`
- **Calcul** : 50% distance + 30% tags communs + 20% fame rating

---

## 8. Ordre des profils

- **Fichier** : `internal/user/browsing_service.go`
- **Fonction** : `GetSuggestions`
- **Logique** :  
	- Tri par zones géographiques (180km, 250km, 350km, 500km)
	- Puis par score de compatibilité dans chaque zone
