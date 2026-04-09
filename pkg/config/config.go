package config

import (
	"fmt"
	"os"
)

// Config holds all environment-based configuration for a microservice.
type Config struct {
	DatabaseURL  string
	RedisURL     string
	KafkaBrokers string
	Port         string
	Env          string
	GRPCPort     string
}

// Load reads configuration from environment variables.
// DATABASE_URL is required; all other fields have sensible defaults.
func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	return &Config{
		DatabaseURL:  dbURL,
		RedisURL:     os.Getenv("REDIS_URL"),
		KafkaBrokers: os.Getenv("KAFKA_BROKERS"),
		Port:         getEnvOrDefault("PORT", "8080"),
		Env:          getEnvOrDefault("ENV", "development"),
		GRPCPort:     getEnvOrDefault("GRPC_PORT", "9090"),
	}, nil
}

// getEnvOrDefault returns the value of the environment variable named by key,
// or def if the variable is empty or unset.
func getEnvOrDefault(key, def string) string {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	return val
}
