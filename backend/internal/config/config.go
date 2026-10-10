package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
}

func Load() (*Config, error) {
	_ = godotenv.Load("../.env")

	config := &Config{
		DBHost:     os.Getenv("DB_HOST"),
		DBPort:     os.Getenv("DB_PORT"),
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     os.Getenv("DB_NAME"),
	}

	if config.DBHost == "" ||
		config.DBPort == "" ||
		config.DBUser == "" ||
		config.DBPassword == "" ||
		config.DBName == "" {
		return nil, fmt.Errorf("database environment variables are not configured")
	}

	return config, nil
}
