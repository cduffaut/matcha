package database

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/cduffaut/matcha/internal/config"
	_ "github.com/lib/pq" // Driver PostgreSQL
)

// RunMigrations exécute les scripts de migration pour créer/mettre à jour les tables
func RunMigrations(db *sql.DB) error {
	// Chemin vers le dossier des migrations
	migrationFiles := []string{
		"internal/database/migrations/create_users_table.sql",
		"internal/database/migrations/create_profile_tables.sql",
		"internal/database/migrations/add_birth_date_to_profiles.sql",
		"internal/database/migrations/add_online_status_to_profiles.sql", 
		"internal/database/migrations/create_blocks_table.sql",
		"internal/database/migrations/create_notifications_table.sql",
		"internal/database/migrations/create_messages_table.sql",
		"internal/database/migrations/create_reports_table.sql",
	}

	for _, file := range migrationFiles {
		// Lire le contenu du fichier
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("erreur lors de la lecture du fichier de migration %s: %w", file, err)
		}

		// Exécuter le script SQL
		_, err = db.Exec(string(content))
		if err != nil {
			return fmt.Errorf("erreur lors de l'exécution de la migration %s: %w", file, err)
		}
	}

	return nil
}

// Connect établit une connexion à la base de données
func Connect(cfg config.DatabaseConfig) (*sql.DB, error) {
	// Construire la chaîne de connexion
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name)

	// Ouvrir la connexion
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("erreur d'ouverture de connexion à la base de données: %w", err)
	}

	// Vérifier la connexion
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("erreur de ping à la base de données: %w", err)
	}

	return db, nil
}
