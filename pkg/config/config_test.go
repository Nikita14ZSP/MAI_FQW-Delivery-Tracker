package config_test

import (
	"os"
	"testing"

	"github.com/mozgovojnikita/delivery-tracker/pkg/config"
)

func TestLoad_MissingDatabaseURL(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("REDIS_URL")
	os.Unsetenv("KAFKA_BROKERS")
	os.Unsetenv("PORT")
	os.Unsetenv("ENV")
	os.Unsetenv("GRPC_PORT")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when DATABASE_URL is not set, got nil")
	}
	if !contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected error message to contain 'DATABASE_URL', got: %s", err.Error())
	}
}

func TestLoad_Success(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://test")
	os.Setenv("REDIS_URL", "redis://test")
	os.Setenv("KAFKA_BROKERS", "localhost:9092")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("KAFKA_BROKERS")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://test" {
		t.Errorf("expected DatabaseURL 'postgres://test', got '%s'", cfg.DatabaseURL)
	}
	if cfg.RedisURL != "redis://test" {
		t.Errorf("expected RedisURL 'redis://test', got '%s'", cfg.RedisURL)
	}
	if cfg.KafkaBrokers != "localhost:9092" {
		t.Errorf("expected KafkaBrokers 'localhost:9092', got '%s'", cfg.KafkaBrokers)
	}
	if cfg.Port != "8080" {
		t.Errorf("expected default Port '8080', got '%s'", cfg.Port)
	}
}

func TestLoad_CustomPort(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://test")
	os.Setenv("PORT", "3000")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("PORT")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "3000" {
		t.Errorf("expected Port '3000', got '%s'", cfg.Port)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
