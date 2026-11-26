package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime settings for the PlanetLogi Go API and worker binaries.
// Every instance stays 100% stateless so multiple VPS nodes can run behind a load balancer.
type Config struct {
	AppEnv          string
	APIPort         string
	WebOrigin       string
	AllowedOrigins  []string
	CookieDomain    string
	CookieSecure    bool
	DatabaseURL     string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	JWTAccessSecret string
	JWTRefreshSecret string
	JWTAccessTTL    time.Duration
	JWTRefreshTTL   time.Duration
	OTPDevEcho      bool
	BillingEnabled  bool
}

func Load() Config {
	webOrigin := getEnv("WEB_ORIGIN", "http://localhost:5173")
	originsRaw := getEnv("ALLOWED_ORIGINS", webOrigin)
	origins := make([]string, 0)
	for _, o := range strings.Split(originsRaw, ",") {
		trimmed := strings.TrimSpace(o)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	return Config{
		AppEnv:           getEnv("APP_ENV", "development"),
		APIPort:          getEnv("API_PORT", "8080"),
		WebOrigin:        webOrigin,
		AllowedOrigins:   origins,
		CookieDomain:     getEnv("COOKIE_DOMAIN", ""),
		CookieSecure:     getEnvBool("COOKIE_SECURE", false),
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://planetlogi:planetlogi_dev@localhost:5432/planetlogi?sslmode=disable"),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		RedisDB:          getEnvInt("REDIS_DB", 0),
		JWTAccessSecret:  getEnv("JWT_ACCESS_SECRET", "planetlogi-dev-access-secret-change-me"),
		JWTRefreshSecret: getEnv("JWT_REFRESH_SECRET", "planetlogi-dev-refresh-secret-change-me"),
		JWTAccessTTL:     time.Duration(getEnvInt("JWT_ACCESS_TTL_MIN", 15)) * time.Minute,
		JWTRefreshTTL:    time.Duration(getEnvInt("JWT_REFRESH_TTL_DAYS", 14)) * 24 * time.Hour,
		OTPDevEcho:       getEnvBool("OTP_DEV_ECHO", true),
		BillingEnabled:   getEnvBool("BILLING_ENABLED", false),
	}
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getEnvInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
