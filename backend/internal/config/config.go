package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort           string
	DBHost             string
	DBPort             string
	DBUser             string
	DBPassword         string
	DBName             string
	CORSAllowedOrigins []string
}

func Load() (*Config, error) {
	_ = godotenv.Load("../.env")

	config := &Config{
		AppPort:           os.Getenv("APP_PORT"),
		DBHost:             os.Getenv("DB_HOST"),
		DBPort:             os.Getenv("DB_PORT"),
		DBUser:             os.Getenv("DB_USER"),
		DBPassword:         os.Getenv("DB_PASSWORD"),
		DBName:             os.Getenv("DB_NAME"),
		CORSAllowedOrigins: splitCSV(os.Getenv("CORS_ALLOWED_ORIGINS")),
	}

	if config.AppPort == "" ||
		config.DBHost == "" ||
		config.DBPort == "" ||
		config.DBUser == "" ||
		config.DBPassword == "" ||
		config.DBName == "" {
		return nil, fmt.Errorf("database environment variables are not configured")
	}

	return config, nil
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
