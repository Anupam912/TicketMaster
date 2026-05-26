package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	JWT       JWTConfig
	Booking   BookingConfig
	WebSocket WebSocketConfig
	Stripe    StripeConfig
}

type ServerConfig struct {
	Port    string
	GinMode string
}

type DatabaseConfig struct {
	Host      string
	Port      string
	User      string
	Password  string
	Name      string
	SSLMode   string
	ReadHost  string
	ReadPort  string
	ReadUser  string
	ReadPassword string
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode)
}

func (d DatabaseConfig) ReadDSN() string {
	if d.ReadHost == "" {
		return ""
	}
	
	readHost := d.ReadHost
	readPort := d.ReadPort
	if readPort == "" {
		readPort = d.Port
	}
	readUser := d.ReadUser
	if readUser == "" {
		readUser = d.User
	}
	readPassword := d.ReadPassword
	if readPassword == "" {
		readPassword = d.Password
	}
	
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		readHost, readPort, readUser, readPassword, d.Name, d.SSLMode)
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%s", r.Host, r.Port)
}

type JWTConfig struct {
	Secret      string
	ExpiryHours int
}

func (j JWTConfig) ExpiryDuration() time.Duration {
	return time.Duration(j.ExpiryHours) * time.Hour
}

type BookingConfig struct {
	ReservationTimeoutMinutes int
}

func (b BookingConfig) ReservationTimeout() time.Duration {
	return time.Duration(b.ReservationTimeoutMinutes) * time.Minute
}

// WebSocketConfig holds WebSocket configuration.
type WebSocketConfig struct {
	AllowedOrigins []string
}

// IsOriginAllowed checks if an origin is allowed for WebSocket connections.
func (w WebSocketConfig) IsOriginAllowed(origin string) bool {
	if len(w.AllowedOrigins) == 0 {
		return true
	}
	for _, allowed := range w.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

// StripeConfig holds Stripe payment gateway configuration.
type StripeConfig struct {
	SecretKey      string
	PublishableKey string
	WebhookSecret  string
	WebhookURL     string
	Currency       string
}

// IsConfigured returns true if Stripe is properly configured.
func (s StripeConfig) IsConfigured() bool {
	return s.SecretKey != ""
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	config := &Config{
		Server: ServerConfig{
			Port:    getEnv("PORT", "8080"),
			GinMode: getEnv("GIN_MODE", "release"),
		},
		Database: DatabaseConfig{
			Host:        getEnv("DB_HOST", "localhost"),
			Port:        getEnv("DB_PORT", "5432"),
			User:        getEnv("DB_USER", "postgres"),
			Password:    getEnv("DB_PASSWORD", "postgres"),
			Name:        getEnv("DB_NAME", "event_ticketing"),
			SSLMode:     getEnv("DB_SSLMODE", "disable"),
			ReadHost:    getEnv("DB_READ_HOST", ""),
			ReadPort:    getEnv("DB_READ_PORT", ""),
			ReadUser:    getEnv("DB_READ_USER", ""),
			ReadPassword: getEnv("DB_READ_PASSWORD", ""),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Secret:      getEnv("JWT_SECRET", "your-super-secret-jwt-key-change-this-in-production"),
			ExpiryHours: getEnvAsInt("JWT_EXPIRY_HOURS", 24),
		},
		Booking: BookingConfig{
			ReservationTimeoutMinutes: getEnvAsInt("RESERVATION_TIMEOUT_MINUTES", 10),
		},
		WebSocket: WebSocketConfig{
			AllowedOrigins: getEnvAsStringSlice("WEBSOCKET_ALLOWED_ORIGINS", []string{}),
		},
		Stripe: StripeConfig{
			SecretKey:      getEnv("STRIPE_SECRET_KEY", ""),
			PublishableKey: getEnv("STRIPE_PUBLISHABLE_KEY", ""),
			WebhookSecret:  getEnv("STRIPE_WEBHOOK_SECRET", ""),
			WebhookURL:     getEnv("STRIPE_WEBHOOK_URL", "http://localhost:8080/api/webhooks/stripe"),
			Currency:       getEnv("STRIPE_CURRENCY", "usd"),
		},
	}

	return config, nil
}

func getEnvAsStringSlice(key string, defaultValue []string) []string {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	var result []string
	for _, v := range strings.Split(valueStr, ",") {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
