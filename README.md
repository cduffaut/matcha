# Matcha - Application de Rencontres

Matcha est une application web de rencontres développée en Go avec le framework Goji.

## Prérequis

- Go 1.19 ou supérieur
- PostgreSQL 12 ou supérieur
- Git

## Installation

1. Cloner le repository
```bash
git clone https://github.com/cduffaut/matcha.git
cd matcha
```

2. Installer les dépendances
```bash
go mod download
```

3. Configurer la base de données
```bash
# Créer la base de données
createdb matcha

# Copier le fichier .env exemple
cp .env.example .env

# Éditer le fichier .env avec vos paramètres
```

4. Créer les dossiers nécessaires
```bash
mkdir -p web/static/uploads
mkdir -p web/static/css
mkdir -p web/static/js
mkdir -p web/static/images
```

5. Démarrer l'application
```bash
go run cmd/server/main.go
```

L'application sera accessible sur http://localhost:8080

## Structure du projet

```
matcha/
├── cmd/
│   └── server/
│       └── main.go            # Point d'entrée de l'application
├── internal/
│   ├── auth/                  # Authentification
│   ├── config/                # Configuration
│   ├── database/              # Base de données et migrations
│   ├── email/                 # Service d'email
│   ├── middleware/            # Middlewares
│   ├── models/                # Modèles de données
│   ├── session/               # Gestion des sessions
│   └── user/                  # Profils et navigation
├── web/
│   └── static/
│       ├── css/               # Fichiers CSS
│       ├── js/                # Fichiers JavaScript
│       ├── images/            # Images statiques
│       └── uploads/           # Photos uploadées
├── .env.example               # Variables d'environnement exemple
├── go.mod                     # Dépendances Go
└── README.md                  # Ce fichier
```

## Fonctionnalités implémentées

- [x] Inscription et connexion
- [x] Vérification par email
- [x] Réinitialisation de mot de passe
- [x] Profil utilisateur
- [x] Upload de photos
- [x] Tags/intérêts
- [x] Géolocalisation
- [x] Navigation et suggestions
- [x] Recherche avancée
- [x] Système de likes
- [x] Historique des visites
- [x] Fame rating
- [ ] Chat en temps réel
- [ ] Notifications
- [ ] Blocage d'utilisateurs
- [ ] Signalement de faux comptes

## Test de l'application

1. Créer un compte utilisateur
2. Vérifier l'email (le lien s'affiche dans la console en mode développement)
3. Se connecter
4. Compléter le profil
5. Upload une photo de profil
6. Naviguer et liker d'autres profils

## Développement

Pour ajouter de nouvelles fonctionnalités :

1. Créer les migrations SQL nécessaires dans `internal/database/migrations/`
2. Ajouter les modèles dans le package approprié
3. Implémenter les repositories et services
4. Créer les handlers HTTP
5. Ajouter les routes dans `main.go`
6. Créer les pages HTML et fichiers JS/CSS nécessaires

## Sécurité

- Les mots de passe sont hashés avec bcrypt
- Protection contre les injections SQL
- Validation des formulaires
- Protection CSRF via sessions
- Validation des uploads de fichiers