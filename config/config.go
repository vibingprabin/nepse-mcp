package config

import (
	"os"
	"time"
)

// Config holds all configuration values for the NEPSE MCP server
type Config struct {
	NepseCacheTTL   time.Duration
	NepseTimeout    time.Duration
	LogLevel        string
}

// LoadConfig loads configuration from environment variables with defaults
func LoadConfig() *Config {
	return &Config{
		NepseCacheTTL:   getEnvDuration("NEPSE_CACHE_TTL", 60*time.Second),
		NepseTimeout:    getEnvDuration("NEPSE_API_TIMEOUT", 30*time.Second),
		LogLevel:        getEnv("NEPSE_LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return fallback
}
