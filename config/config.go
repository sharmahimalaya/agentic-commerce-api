package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                 string
	ShutdownTimeout      time.Duration
	WebhookMaxRetries    int
	WebhookChannelBuffer int
	WebhookHTTPTimeout   time.Duration
	IdempotencyTTL       time.Duration
	AdminAPIKey          string
}

func Load() *Config {
	return &Config{
		Port:                 getEnv("PORT", "8080"),
		ShutdownTimeout:      getDurationEnv("SHUTDOWN_TIMEOUT", 5*time.Second),
		WebhookMaxRetries:    getIntEnv("WEBHOOK_MAX_RETRIES", 3),
		WebhookChannelBuffer: getIntEnv("WEBHOOK_CHANNEL_BUFFER", 100),
		WebhookHTTPTimeout:   getDurationEnv("WEBHOOK_HTTP_TIMEOUT", 5*time.Second),
		IdempotencyTTL:       getDurationEnv("IDEMPOTENCY_TTL", 24*time.Hour),
		AdminAPIKey:          getEnv("ADMIN_API_KEY", ""),
	}
}
func getEnv(key, fallback string) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	return fallback
}
func getIntEnv(key string, fallback int) int {
	if val, exists := os.LookupEnv(key); exists {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}
func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if val, exists := os.LookupEnv(key); exists {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return fallback
}
