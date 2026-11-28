package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultDevAccessSecret  = "planetlogi-dev-access-secret-change-me"
	DefaultDevRefreshSecret = "planetlogi-dev-refresh-secret-change-me"
)

// Config holds all runtime settings for the PlanetLogi Go API and worker binaries.
// Every instance stays 100% stateless so multiple VPS nodes can run behind a load balancer.
type Config struct {
	AppEnv           string
	APIPort          string
	WebOrigin        string
	AllowedOrigins   []string
	CookieDomain     string
	CookieSecure     bool
	CookieSameSite   string
	DatabaseURL      string
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	JWTAccessSecret  string
	JWTRefreshSecret string
	JWTAccessTTL     time.Duration
	JWTRefreshTTL    time.Duration
	OTPDevEcho       bool
	BillingEnabled   bool
	// SuperNovaEnforceOpen enforces the ranked Mon-Sat 06:00-22:00 window
	// server-side. Only demos/tests may disable it.
	SuperNovaEnforceOpen bool
}

func Load() Config {
	appEnv := getEnv("APP_ENV", "development")
	isProd := strings.EqualFold(appEnv, "production")
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
		AppEnv:               appEnv,
		APIPort:              getEnv("API_PORT", "8080"),
		WebOrigin:            webOrigin,
		AllowedOrigins:       origins,
		CookieDomain:         getEnv("COOKIE_DOMAIN", ""),
		CookieSecure:         getEnvBool("COOKIE_SECURE", isProd),
		CookieSameSite:       strings.ToLower(getEnv("COOKIE_SAME_SITE", "lax")),
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://planetlogi:planetlogi_dev@localhost:5432/planetlogi?sslmode=disable"),
		RedisAddr:            getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:        getEnv("REDIS_PASSWORD", ""),
		RedisDB:              getEnvInt("REDIS_DB", 0),
		JWTAccessSecret:      getEnv("JWT_ACCESS_SECRET", DefaultDevAccessSecret),
		JWTRefreshSecret:     getEnv("JWT_REFRESH_SECRET", DefaultDevRefreshSecret),
		JWTAccessTTL:         time.Duration(getEnvInt("JWT_ACCESS_TTL_MIN", 15)) * time.Minute,
		JWTRefreshTTL:        time.Duration(getEnvInt("JWT_REFRESH_TTL_DAYS", 14)) * 24 * time.Hour,
		OTPDevEcho:           getEnvBool("OTP_DEV_ECHO", false),
		BillingEnabled:       getEnvBool("BILLING_ENABLED", false),
		SuperNovaEnforceOpen: getEnvBool("SUPERNOVA_ENFORCE_OPEN", true),
	}
}

// ValidateForServer rejects insecure default or identical JWT secrets when running in production.
func (c Config) ValidateForServer() error {
	if strings.EqualFold(c.CookieSameSite, "none") && !c.CookieSecure {
		return errors.New("COOKIE_SAME_SITE=none requires COOKIE_SECURE=true")
	}
	if !strings.EqualFold(c.AppEnv, "production") {
		return nil
	}
	if c.JWTAccessSecret == "" || c.JWTAccessSecret == DefaultDevAccessSecret {
		return errors.New("production requires a non-default JWT_ACCESS_SECRET")
	}
	if c.JWTRefreshSecret == "" || c.JWTRefreshSecret == DefaultDevRefreshSecret {
		return errors.New("production requires a non-default JWT_REFRESH_SECRET")
	}
	if c.JWTAccessSecret == c.JWTRefreshSecret {
		return errors.New("production requires distinct JWT_ACCESS_SECRET and JWT_REFRESH_SECRET")
	}
	return nil
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
