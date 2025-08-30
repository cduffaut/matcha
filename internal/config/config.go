package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config globale de l'application.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
}

type ServerConfig struct {
	Port string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

// charge la configuration depuis l'environnement.
// PANIC + debug si une env requise est absente ou vide
func Load() (*Config, error) {
	_ = godotenv.Load() // facultatif

	cfg := &Config{
		Server: ServerConfig{
			Port: mustEnv("PORT"),
		},
		Database: DatabaseConfig{
			Host:     mustEnv("DB_HOST"),
			Port:     mustEnv("DB_PORT"),
			User:     mustEnv("DB_USER"),
			Password: mustEnv("DB_PASSWORD"),
			Name:     mustEnv("DB_NAME"),
		},
	}
	return cfg, nil
}

// retourne la valeur nettoyée ou panique avec un message clair.
func mustEnv(key string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		panic(fmt.Sprintf(
			"CONFIG ERROR: variable d'environnement manquante %q.\n"+
				"Exemple (.env): %s=<valeur>\n"+
				"Variables requises: PORT, DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME",
			key, key,
		))
	}
	return val
}
