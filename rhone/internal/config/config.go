package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application.
type Config struct {
	// Server
	Port        string
	BaseURL     string
	Environment string // development, staging, production

	// Database
	DatabaseURL string

	// GitHub OAuth (user authentication)
	GitHubClientID     string
	GitHubClientSecret string
	GitHubCallbackURL  string

	// GitHub App (repository access)
	GitHubAppID         int64
	GitHubAppPrivateKey string
	GitHubAppSlug       string

	// Session
	SessionSecret string
	SessionMaxAge time.Duration

	// Security
	CSRFSecret string
}

// Load reads configuration from environment variables.
// In development, it will also load from a .env file if present.
func Load() (*Config, error) {
	// Load .env file in development (ignore errors if file doesn't exist)
	_ = godotenv.Load()

	cfg := &Config{
		Port:        getEnv("PORT", "8080"),
		BaseURL:     getEnv("BASE_URL", "http://localhost:8080"),
		Environment: getEnv("ENVIRONMENT", "development"),

		DatabaseURL: mustGetEnv("DATABASE_URL"),

		GitHubClientID:     mustGetEnv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: mustGetEnv("GITHUB_CLIENT_SECRET"),

		GitHubAppPrivateKey: mustGetEnv("GITHUB_APP_PRIVATE_KEY"),
		GitHubAppSlug:       mustGetEnv("GITHUB_APP_SLUG"),

		SessionSecret: mustGetEnv("SESSION_SECRET"),
		SessionMaxAge: 7 * 24 * time.Hour, // 1 week

		CSRFSecret: mustGetEnv("CSRF_SECRET"),
	}

	cfg.GitHubCallbackURL = cfg.BaseURL + "/auth/callback"

	// Parse GitHub App ID
	appIDStr := mustGetEnv("GITHUB_APP_ID")
	appID, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("GITHUB_APP_ID must be a valid integer: %w", err)
	}
	cfg.GitHubAppID = appID

	// Validate session secret length (need 64 bytes for hash key + block key)
	if len(cfg.SessionSecret) < 64 {
		return nil, fmt.Errorf("SESSION_SECRET must be at least 64 characters, got %d", len(cfg.SessionSecret))
	}

	return cfg, nil
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction returns true if running in production mode.
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// getEnv returns the value of an environment variable or a fallback default.
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// mustGetEnv returns the value of an environment variable or panics if not set.
func mustGetEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return value
}
