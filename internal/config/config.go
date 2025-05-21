package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config contient la configuration globale de l'application
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
}

// ServerConfig contient la configuration du serveur web
type ServerConfig struct {
	Port string
}

// DatabaseConfig contient la configuration de la base de données
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

// Load charge la configuration depuis les variables d'environnement
func Load() (*Config, error) {
	// Charger les variables d'environnement depuis .env si présent
	_ = godotenv.Load()

	// Configuration du serveur
	serverPort := os.Getenv("PORT")
	if serverPort == "" {
		serverPort = "8080"
	}

	// Configuration de la base de données
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}

	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}

	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}

	dbPassword := os.Getenv("DB_PASSWORD")

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "matcha"
	}

	config := &Config{
		Server: ServerConfig{
			Port: serverPort,
		},
		Database: DatabaseConfig{
			Host:     dbHost,
			Port:     dbPort,
			User:     dbUser,
			Password: dbPassword,
			Name:     dbName,
		},
	}

	return config, nil
}
