package internal

import (
	"fmt"
	"os"
)

type Config struct {
	Port       string
	DataDir    string
	Migrations string
	DB         PostgresConfig
}

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

func NewConfig() Config {
	return Config{
		Port:       getEnv("PORT", "8080"),
		DataDir:    getEnv("DATA_DIR", "data"),
		Migrations: getEnv("MIGRATIONS_DIR", "migrations"),
		DB: PostgresConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "topology"),
			Password: getEnv("DB_PASSWORD", "topology"),
			Name:     getEnv("DB_NAME", "topology"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
	}
}

func (c PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.User,
		c.Password,
		c.Host,
		c.Port,
		c.Name,
		c.SSLMode,
	)
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
