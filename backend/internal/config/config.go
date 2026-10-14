package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort            string
	DBHost             string
	DBPort             string
	DBUser             string
	DBPassword         string
	DBName             string
	DBSSLMode          string
	DBSSLRootCert      string
	CORSAllowedOrigins []string
	WebAuthnRPID       string
	WebAuthnOrigin     string
	RedisAddress       string
	RedisTLS           bool
}

func Load() (*Config, error) {
	_ = godotenv.Load("../.env")

	config := &Config{
		AppPort:            os.Getenv("APP_PORT"),
		DBHost:             os.Getenv("DB_HOST"),
		DBPort:             os.Getenv("DB_PORT"),
		DBUser:             os.Getenv("DB_USER"),
		DBPassword:         os.Getenv("DB_PASSWORD"),
		DBName:             os.Getenv("DB_NAME"),
		DBSSLMode:          getenvDefault("DB_SSLMODE", "disable"),
		DBSSLRootCert:      os.Getenv("DB_SSLROOTCERT"),
		CORSAllowedOrigins: splitCSV(os.Getenv("CORS_ALLOWED_ORIGINS")),
		WebAuthnRPID:       os.Getenv("WEBAUTHN_RP_ID"),
		WebAuthnOrigin:     os.Getenv("WEBAUTHN_ORIGIN"),
		RedisAddress:       os.Getenv("REDIS_ADDR"),
		RedisTLS:           os.Getenv("REDIS_TLS") == "true",
	}

	if config.AppPort == "" ||
		config.DBHost == "" ||
		config.DBPort == "" ||
		config.DBUser == "" ||
		config.DBPassword == "" ||
		config.DBName == "" {
		return nil, fmt.Errorf("database environment variables are not configured")
	}

	if (config.WebAuthnRPID == "") != (config.WebAuthnOrigin == "") {
		return nil, fmt.Errorf("WEBAUTHN_RP_ID and WEBAUTHN_ORIGIN must be configured together")
	}
	if config.WebAuthnRPID != "" && config.RedisAddress == "" {
		return nil, fmt.Errorf("REDIS_ADDR is required when WebAuthn is configured")
	}
	return config, nil
}

func getenvDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	values := strings.Split(value, ",")
	origins := make([]string, 0, len(values))
	for _, value := range values {
		if origin := strings.TrimSpace(value); origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}
