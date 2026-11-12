package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	Address               string
	DatabasePath          string
	BaseURL               string
	DefaultExpirationDays int
}

func Load() Config {
	return Config{
		Address:               getEnv("ADDRESS", ":8080"),
		DatabasePath:          getEnv("DATABASE_PATH", "./data/shrinkr.db"),
		BaseURL:               getEnv("BASE_URL", "http://localhost:8080"),
		DefaultExpirationDays: getEnvInt("DEFAULT_EXPIRATION_DAYS", 0),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		log.Printf("Invalid value for %s, using default %d", key, fallback)
		return fallback
	}

	return parsed
}
