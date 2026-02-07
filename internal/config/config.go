package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Booking  BookingConfig
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
	}

	return config, nil
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
