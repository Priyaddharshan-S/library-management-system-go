package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all the settings the app reads from environment variables.
type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	JWTExpiry   time.Duration
	LoanDays    int

	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
}

// Load reads a .env file if one exists (useful for running locally without
// Docker), then reads environment variables into a Config struct.
// In Docker, .env won't exist inside the container — that's fine, the
// variables are already injected by docker-compose.yml.
func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}

	expiryHours, err := strconv.Atoi(getEnv("JWT_EXPIRY_HOURS", "24"))
	if err != nil {
		expiryHours = 24
	}

	loanDays, err := strconv.Atoi(getEnv("LOAN_DAYS", "14"))
	if err != nil {
		loanDays = 14
	}

	cfg := Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", ""),
		JWTSecret:   getEnv("JWT_SECRET", ""),
		JWTExpiry:   time.Duration(expiryHours) * time.Hour,
		LoanDays:    loanDays,

		// SMTP fields are optional. If SMTPHost is empty, main.go falls
		// back to printing emails to the console instead of sending them.
		SMTPHost:     getEnv("SMTP_HOST", ""),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPUsername: getEnv("SMTP_USERNAME", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", "library-system@example.com"),
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required but was not set")
	}
	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required but was not set")
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
