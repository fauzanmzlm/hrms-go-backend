// Package config loads application configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port              int
	LogLevel          string
	ShutdownTimeout   time.Duration
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration

	DBDSN             string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
	AutoMigrate       bool

	JWTSecret string
	JWTIssuer string
	JWTTTL    time.Duration

	BcryptCost int

	PromotionDisciplinaryBlockMonths int

	// BootstrapAdminUsername and BootstrapAdminPassword, when both set, create
	// an ADMIN account at startup if that username does not exist yet.
	BootstrapAdminUsername string
	BootstrapAdminPassword string
}

// Load reads configuration from environment variables, applying defaults
// for optional values. All problems are reported together.
func Load() (*Config, error) {
	var errs []error
	l := loader{errs: &errs}

	cfg := &Config{
		Port:              l.int("APP_PORT", 8080),
		LogLevel:          l.string("LOG_LEVEL", "info"),
		ShutdownTimeout:   l.duration("SHUTDOWN_TIMEOUT", 15*time.Second),
		ReadHeaderTimeout: l.duration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
		ReadTimeout:       l.duration("HTTP_READ_TIMEOUT", 15*time.Second),
		WriteTimeout:      l.duration("HTTP_WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:       l.duration("HTTP_IDLE_TIMEOUT", 60*time.Second),

		DBDSN:             l.required("DB_DSN"),
		DBMaxOpenConns:    l.int("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:    l.int("DB_MAX_IDLE_CONNS", 25),
		DBConnMaxLifetime: l.duration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		AutoMigrate:       l.bool("AUTO_MIGRATE", false),

		JWTSecret: l.required("JWT_SECRET"),
		JWTIssuer: l.string("JWT_ISSUER", "hrms-go-backend"),
		JWTTTL:    l.duration("JWT_TTL", 8*time.Hour),

		BcryptCost: l.int("BCRYPT_COST", 12),

		PromotionDisciplinaryBlockMonths: l.int("PROMOTION_DISCIPLINARY_BLOCK_MONTHS", 12),

		BootstrapAdminUsername: os.Getenv("BOOTSTRAP_ADMIN_USERNAME"),
		BootstrapAdminPassword: os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
	}

	if cfg.Port <= 0 || cfg.Port > 65535 {
		errs = append(errs, fmt.Errorf("APP_PORT must be between 1 and 65535"))
	}
	if cfg.PromotionDisciplinaryBlockMonths < 0 {
		errs = append(errs, fmt.Errorf("PROMOTION_DISCIPLINARY_BLOCK_MONTHS must not be negative"))
	}
	if (cfg.BootstrapAdminUsername == "") != (cfg.BootstrapAdminPassword == "") {
		errs = append(errs, fmt.Errorf("BOOTSTRAP_ADMIN_USERNAME and BOOTSTRAP_ADMIN_PASSWORD must be set together"))
	}

	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, nil
}

type loader struct {
	errs *[]error
}

func (l loader) fail(key, want, got string) {
	*l.errs = append(*l.errs, fmt.Errorf("%s must be %s, got %q", key, want, got))
}

func (l loader) string(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func (l loader) required(key string) string {
	v := os.Getenv(key)
	if v == "" {
		*l.errs = append(*l.errs, fmt.Errorf("%s is required", key))
	}
	return v
}

func (l loader) int(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		l.fail(key, "an integer", v)
		return def
	}
	return n
}

func (l loader) bool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		l.fail(key, "a boolean", v)
		return def
	}
	return b
}

func (l loader) duration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		l.fail(key, "a positive duration such as 30s or 8h", v)
		return def
	}
	return d
}
