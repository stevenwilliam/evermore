// Package config loads configuration from the environment. Secrets only ever
// arrive this way — nothing secret is in git (CLAUDE.md §4).
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the whole of this service's configuration.
type Config struct {
	AppEnv        string
	AppName       string
	Bind          string
	Port          int
	BaseURL       string
	Timezone      string
	DefaultLocale string
	Locales       []string
	LogLevel      string

	DatabaseURL     string
	TestDatabaseURL string

	JWTSigningKey     string
	TOTPEncryptionKey string
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration

	RedisURL string

	SMTPHost      string
	SMTPPort      int
	SMTPFromEmail string
	SMTPFromName  string

	MinioEndpoint       string
	MinioAccessKey      string
	MinioSecretKey      string
	MinioBucket         string
	MinioUseSSL         bool
	MinioPublicEndpoint string

	GoogleMapsBrowserKey string
	GoogleMapsServerKey  string

	WAHAURL     string
	WAHASession string
	WAHAAPIKey  string
	WAHAEnabled bool

	CORSAllowedOrigins []string

	// Location is Timezone resolved once at startup, so business-day logic
	// never has to parse it again or fall back to the server's zone.
	Location *time.Location
}

// Load reads configuration from the process environment, having first merged
// in any .env file at path (which does not override variables already set).
func Load(envFile string) (*Config, error) {
	if envFile != "" {
		if err := loadDotEnv(envFile); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	c := &Config{
		AppEnv:        str("APP_ENV", "development"),
		AppName:       str("APP_NAME", "Evermore"),
		Bind:          str("APP_BIND", "127.0.0.1"),
		BaseURL:       str("APP_BASE_URL", "http://127.0.0.1:8082"),
		Timezone:      str("APP_TIMEZONE", "Asia/Jakarta"),
		DefaultLocale: str("APP_DEFAULT_LOCALE", "id-ID"),
		LogLevel:      str("LOG_LEVEL", "info"),

		DatabaseURL:     str("DATABASE_URL", ""),
		TestDatabaseURL: str("TEST_DATABASE_URL", ""),

		JWTSigningKey:     str("JWT_SIGNING_KEY", ""),
		TOTPEncryptionKey: str("TOTP_ENCRYPTION_KEY", ""),

		RedisURL: str("REDIS_URL", ""),

		SMTPHost:      str("SMTP_HOST", "127.0.0.1"),
		SMTPFromEmail: str("SMTP_FROM_EMAIL", "no-reply@evermore.co.id"),
		SMTPFromName:  str("SMTP_FROM_NAME", "Evermore"),

		MinioEndpoint:       str("MINIO_ENDPOINT", ""),
		MinioAccessKey:      str("MINIO_ACCESS_KEY", ""),
		MinioSecretKey:      str("MINIO_SECRET_KEY", ""),
		MinioBucket:         str("MINIO_BUCKET", "evermore-app"),
		MinioPublicEndpoint: str("MINIO_PUBLIC_ENDPOINT", ""),

		GoogleMapsBrowserKey: str("GOOGLE_MAPS_BROWSER_KEY", ""),
		GoogleMapsServerKey:  str("GOOGLE_MAPS_SERVER_KEY", ""),

		WAHAURL:     str("WAHA_URL", ""),
		WAHASession: str("WAHA_SESSION", "evermore"),
		WAHAAPIKey:  str("WAHA_API_KEY", ""),
	}

	var err error
	if c.Port, err = intVal("APP_PORT", 8082); err != nil {
		return nil, err
	}
	if c.SMTPPort, err = intVal("SMTP_PORT", 1025); err != nil {
		return nil, err
	}
	if c.MinioUseSSL, err = boolVal("MINIO_USE_SSL", false); err != nil {
		return nil, err
	}
	if c.WAHAEnabled, err = boolVal("WAHA_ENABLED", false); err != nil {
		return nil, err
	}
	if c.AccessTokenTTL, err = durVal("ACCESS_TOKEN_TTL", 15*time.Minute); err != nil {
		return nil, err
	}
	if c.RefreshTokenTTL, err = durVal("REFRESH_TOKEN_TTL", 30*24*time.Hour); err != nil {
		return nil, err
	}

	c.Locales = csv(str("APP_LOCALES", "id-ID,en"))
	c.CORSAllowedOrigins = csv(str("CORS_ALLOWED_ORIGINS", ""))

	if c.Location, err = time.LoadLocation(c.Timezone); err != nil {
		return nil, fmt.Errorf("config: APP_TIMEZONE %q is not a known zone: %w", c.Timezone, err)
	}

	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// validate refuses to start on a configuration that would fail later, in a
// harder-to-diagnose place. A missing signing key must not become a service
// that issues unsigned tokens.
func (c *Config) validate() error {
	var missing []string
	require := func(name, v string) {
		if strings.TrimSpace(v) == "" {
			missing = append(missing, name)
		}
	}
	require("DATABASE_URL", c.DatabaseURL)
	require("JWT_SIGNING_KEY", c.JWTSigningKey)
	require("TOTP_ENCRYPTION_KEY", c.TOTPEncryptionKey)
	if len(missing) > 0 {
		return fmt.Errorf("config: required settings are missing: %s", strings.Join(missing, ", "))
	}

	if len(c.JWTSigningKey) < 32 {
		return fmt.Errorf("config: JWT_SIGNING_KEY is %d characters; at least 32 are required", len(c.JWTSigningKey))
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("config: APP_PORT %d is out of range", c.Port)
	}
	if c.AccessTokenTTL <= 0 || c.AccessTokenTTL > time.Hour {
		return fmt.Errorf("config: ACCESS_TOKEN_TTL %s is outside the sane range (0, 1h]", c.AccessTokenTTL)
	}
	known := map[string]bool{"development": true, "staging": true, "production": true, "test": true}
	if !known[c.AppEnv] {
		return fmt.Errorf("config: APP_ENV %q is not one of development, staging, production, test", c.AppEnv)
	}
	return nil
}

// IsProduction reports whether this is a production deployment.
func (c *Config) IsProduction() bool { return c.AppEnv == "production" }

// Addr is the host:port the HTTP server binds.
func (c *Config) Addr() string { return fmt.Sprintf("%s:%d", c.Bind, c.Port) }

// Redacted returns the configuration with every secret masked, for logging at
// startup. Secret-flagged values are masked in logs (CLAUDE.md §7).
func (c *Config) Redacted() map[string]string {
	mask := func(s string) string {
		if s == "" {
			return ""
		}
		return "****"
	}
	return map[string]string{
		"APP_ENV":             c.AppEnv,
		"APP_ADDR":            c.Addr(),
		"APP_BASE_URL":        c.BaseURL,
		"APP_TIMEZONE":        c.Timezone,
		"APP_LOCALES":         strings.Join(c.Locales, ","),
		"LOG_LEVEL":           c.LogLevel,
		"DATABASE_URL":        redactDSN(c.DatabaseURL),
		"JWT_SIGNING_KEY":     mask(c.JWTSigningKey),
		"TOTP_ENCRYPTION_KEY": mask(c.TOTPEncryptionKey),
		"REDIS_URL":           redactDSN(c.RedisURL),
		"SMTP":                fmt.Sprintf("%s:%d", c.SMTPHost, c.SMTPPort),
		"MINIO_ENDPOINT":      c.MinioEndpoint,
		"MINIO_BUCKET":        c.MinioBucket,
		"MINIO_SECRET_KEY":    mask(c.MinioSecretKey),
		"GOOGLE_MAPS_KEYS":    mask(c.GoogleMapsServerKey),
		"WAHA_ENABLED":        strconv.FormatBool(c.WAHAEnabled),
	}
}

// redactDSN strips the password from a URL-shaped DSN so it can be logged.
func redactDSN(dsn string) string {
	if dsn == "" {
		return ""
	}
	at := strings.LastIndex(dsn, "@")
	scheme := strings.Index(dsn, "://")
	if at < 0 || scheme < 0 || at < scheme {
		return dsn
	}
	creds := dsn[scheme+3 : at]
	if i := strings.Index(creds, ":"); i >= 0 {
		creds = creds[:i] + ":****"
	}
	return dsn[:scheme+3] + creds + dsn[at:]
}

func str(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func intVal(key string, def int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s=%q is not an integer", key, v)
	}
	return n, nil
}

func boolVal(key string, def bool) (bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("config: %s=%q is not a boolean", key, v)
	}
	return b, nil
}

func durVal(key string, def time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s=%q is not a duration (try 15m, 720h)", key, v)
	}
	return d, nil
}

func csv(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// loadDotEnv merges a .env file into the environment without overriding
// anything already set, so a systemd EnvironmentFile always wins.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, val); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}
