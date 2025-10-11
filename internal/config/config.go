package config

import (
	"os"
	"strconv"
)

type Config struct {
	Environment    string
	ServerPort     string
	DatabaseURL    string
	RedisURL       string
	JWTSecret      string
	BcryptCost     int
}

func LoadConfig() *Config {
	config := &Config{
		Environment: getEnv("ENVIRONMENT", "development"),
		ServerPort:  getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:123456@localhost:5432/delivery_management_system"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:   getEnv("JWT_SECRET", "default-secret-key-change-in-production"),
		BcryptCost:  getEnvAsInt("BCRYPT_COST", 12),
	}
	
	return config
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}