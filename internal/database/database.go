package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/cduffaut/matcha/internal/config"
	_ "github.com/lib/pq"
)

var migrationFiles = []string{
	"internal/database/migrations/create_users_table.sql",
	"internal/database/migrations/create_profile_tables.sql",
	"internal/database/migrations/add_birth_date_to_profiles.sql",
	"internal/database/migrations/add_online_status_to_profiles.sql",
	"internal/database/migrations/create_blocks_table.sql",
	"internal/database/migrations/create_notifications_table.sql",
	"internal/database/migrations/create_messages_table.sql",
	"internal/database/migrations/create_reports_table.sql",
	"internal/database/migrations/add_500_seed.sql",
}

// exécute chaque fichier dans une transaction séparée.
func RunMigrations(db *sql.DB) error {
	for _, path := range migrationFiles {
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("lecture migration %s: %w", path, err)
		}
		if err := execTx(db, string(sqlBytes)); err != nil {
			return fmt.Errorf("exécution migration %s: %w", path, err)
		}
	}
	return nil
}

func execTx(db *sql.DB, stmt string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if _, err := tx.Exec(stmt); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("exec: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// ouvre la connexion et applique des réglages sûrs.
func Connect(cfg config.DatabaseConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("ouverture DB: %w", err)
	}

	// Params de pool raisonnables par défaut.
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping DB: %w", err)
	}

	return db, nil
}
