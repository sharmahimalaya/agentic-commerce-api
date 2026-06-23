package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                 string
	ShutdownTimeout      time.Duration
	WebhookMaxRetries    int
	WebhookChannelBuffer int
	WebhookHTTPTimeout   time.Duration
	IdempotencyTTL       time.Duration
	AdminAPIKey          string
	MandatePublicKeyPEM  string
}

func Load() *Config {
	_ = godotenv.Load()

	defaultPublicKey := `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA3EHJuicZBmMBXcZuEGGq
ODBO/C52qAnFCftKWPVA3oTSG5i7sHSfzzn6SEWnWZQYxyJgX7UMdl54hv7J2SWO
IfwRtYipjSZwPlNJMFIqL5/qz6KMXqFNxaS4x45UffECOSdm65afV8JNJXKxMbvi
UCjLMNFV2xr8sJIdGEizNmW85s4Hw6VsI9Lql27hox9IUL54SkqKOcR0AjtfG27P
Ku/Vtr7C8zpVf88468csGx7l9wiJDZYbr/keL1bk9EQimljIGm7sD7WW1vjGf8pg
JjMY927D4sN29GkleD7onGfkrji4+NG3r/S5ZvRes0V5mCtKAsUO5rRnt/Ras98P
lwIDAQAB
-----END PUBLIC KEY-----`

	rawPublicKey := getEnv("MANDATE_PUBLIC_KEY_PEM", defaultPublicKey)
	// Replace escaped \n with actual newlines if read from env file single-lined
	formattedPublicKey := strings.ReplaceAll(rawPublicKey, "\\n", "\n")

	return &Config{
		Port:                 getEnv("PORT", "8080"),
		ShutdownTimeout:      getDurationEnv("SHUTDOWN_TIMEOUT", 5*time.Second),
		WebhookMaxRetries:    getIntEnv("WEBHOOK_MAX_RETRIES", 3),
		WebhookChannelBuffer: getIntEnv("WEBHOOK_CHANNEL_BUFFER", 100),
		WebhookHTTPTimeout:   getDurationEnv("WEBHOOK_HTTP_TIMEOUT", 5*time.Second),
		IdempotencyTTL:       getDurationEnv("IDEMPOTENCY_TTL", 24*time.Hour),
		AdminAPIKey:          getEnv("ADMIN_API_KEY", ""),
		MandatePublicKeyPEM:  formattedPublicKey,
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
