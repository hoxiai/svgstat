package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

type Config struct {
	HTTP      HTTPConfig
	Postgres  PostgresConfig
	Redis     RedisConfig
	Logging   LoggingConfig
	Cache     CacheConfig
	GeoIP     GeoIPConfig
	Analytics AnalyticsConfig
	Worker    WorkerConfig
	Admin     AdminConfig
}

type HTTPConfig struct {
	Addr           string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	CookieSecure     bool
	TrustedProxies   []string
	CSRFCheckEnabled bool
}

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SSLMode  string
	PoolMax  int
	PoolMin  int
}

type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
}

type LoggingConfig struct {
	Level string
}

type CacheConfig struct {
	TTL time.Duration
}

type GeoIPConfig struct {
	DBPath string
}

type AnalyticsConfig struct {
	// KeyTTL is how long day-scoped Redis analytics keys live after the last
	// write. Must comfortably exceed the worker flush interval.
	KeyTTL             time.Duration
	IPSalt             string
	MaxDailyVisitors   int
	MaxDimensionValues int
}

type WorkerConfig struct {
	// FlushInterval is how often Redis analytics aggregates are persisted
	// into PostgreSQL daily_statistics.
	FlushInterval time.Duration
	Enabled       bool
	LockID        int64
}

type AdminConfig struct {
	Emails []string
}

func Load() *Config {
	_ = godotenv.Load()

	config := &Config{
		HTTP: HTTPConfig{
			Addr:           getEnv("HTTP_ADDR", ":8080"),
			ReadTimeout:    getDurationEnv("HTTP_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:   getDurationEnv("HTTP_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:    getDurationEnv("HTTP_IDLE_TIMEOUT", 60*time.Second),
			CookieSecure:     getBoolEnv("HTTP_COOKIE_SECURE", false),
			TrustedProxies:   getCSVEnv("HTTP_TRUSTED_PROXIES"),
			CSRFCheckEnabled: getBoolEnv("CSRF_CHECK_ENABLED", false),
		},
		Postgres: PostgresConfig{
			Host:     getEnv("PG_HOST", "localhost"),
			Port:     getEnv("PG_PORT", "5432"),
			User:     getEnv("PG_USER", "svgstat"),
			Password: getEnv("PG_PASSWORD", "svgstat"),
			Database: getEnv("PG_DATABASE", "svgstat"),
			SSLMode:  getEnv("PG_SSL_MODE", "disable"),
			PoolMax:  getIntEnv("PG_POOL_MAX", 20),
			PoolMin:  getIntEnv("PG_POOL_MIN", 5),
		},
		Redis: RedisConfig{
			Addr:         getEnv("REDIS_ADDR", "localhost:6379"),
			Password:     getEnv("REDIS_PASSWORD", ""),
			DB:           getIntEnv("REDIS_DB", 0),
			PoolSize:     getIntEnv("REDIS_POOL_SIZE", 20),
			MinIdleConns: getIntEnv("REDIS_MIN_IDLE_CONNS", 5),
		},
		Logging: LoggingConfig{
			Level: getEnv("LOG_LEVEL", "info"),
		},
		Cache: CacheConfig{
			TTL: getDurationEnv("CACHE_TTL", 5*time.Minute),
		},
		GeoIP: GeoIPConfig{
			DBPath: getEnv("GEOIP_DB_PATH", "resource/GeoLite2-City.mmdb"),
		},
		Analytics: AnalyticsConfig{
			KeyTTL:             getDurationEnv("ANALYTICS_KEY_TTL", 72*time.Hour),
			IPSalt:             getEnv("ANALYTICS_IP_SALT", "svgstat-local-development"),
			MaxDailyVisitors:   getIntEnv("ANALYTICS_MAX_DAILY_VISITORS", 100000),
			MaxDimensionValues: getIntEnv("ANALYTICS_MAX_DIMENSION_VALUES", 1000),
		},
		Worker: WorkerConfig{
			FlushInterval: getDurationEnv("WORKER_FLUSH_INTERVAL", 15*time.Minute),
			Enabled:       getBoolEnv("WORKER_ENABLED", true),
			LockID:        getInt64Env("WORKER_LOCK_ID", 1937202501),
		},
		Admin: AdminConfig{
			Emails: getCSVEnv("ADMIN_EMAILS"),
		},
	}
	if config.Analytics.IPSalt == "svgstat-local-development" || config.Analytics.IPSalt == "change-me-in-production" {
		log.Warn().Msg("ANALYTICS_IP_SALT uses a development value; set a random secret before production")
	}
	if !config.HTTP.CookieSecure {
		log.Warn().Msg("HTTP_COOKIE_SECURE is disabled; enable it when serving over HTTPS")
	}
	return config
}

func getCSVEnv(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(strings.ToLower(part)); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		var result bool
		if _, err := fmt.Sscanf(value, "%t", &result); err == nil {
			return result
		}
		log.Warn().Str("key", key).Msg("Failed to parse bool env, using default")
	}
	return defaultValue
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		var result int
		_, err := time.ParseDuration(value)
		if err == nil {
			d, _ := time.ParseDuration(value)
			return int(d.Seconds())
		}
		_, err = fmt.Sscanf(value, "%d", &result)
		if err == nil {
			return result
		}
		log.Warn().Str("key", key).Msg("Failed to parse int env, using default")
	}
	return defaultValue
}

func getInt64Env(key string, defaultValue int64) int64 {
	if value, exists := os.LookupEnv(key); exists {
		var result int64
		if _, err := fmt.Sscanf(value, "%d", &result); err == nil {
			return result
		}
		log.Warn().Str("key", key).Msg("Failed to parse int64 env, using default")
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		d, err := time.ParseDuration(value)
		if err == nil {
			return d
		}
		log.Warn().Str("key", key).Msg("Failed to parse duration env, using default")
	}
	return defaultValue
}
