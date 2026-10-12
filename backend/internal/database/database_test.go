package database

import (
	"strings"
	"testing"

	"github.com/catechlounge/kyoya67-todo/backend/internal/config"
)

func TestBuildDSN(t *testing.T) {
	cfg := &config.Config{
		DBUser:        "todo",
		DBPassword:    "secret",
		DBHost:        "db.example.com",
		DBPort:        "5432",
		DBName:        "todo",
		DBSSLMode:     "verify-full",
		DBSSLRootCert: "/etc/ssl/certs/rds-global-bundle.pem",
	}

	dsn := buildDSN(cfg)
	for _, want := range []string{
		"sslmode=verify-full",
		"sslrootcert=%2Fetc%2Fssl%2Fcerts%2Frds-global-bundle.pem",
	} {
		if !strings.Contains(dsn, want) {
			t.Fatalf("buildDSN() = %q, want substring %q", dsn, want)
		}
	}
}

func TestBuildDSNWithoutRootCertificate(t *testing.T) {
	cfg := &config.Config{
		DBUser:     "todo",
		DBPassword: "secret",
		DBHost:     "localhost",
		DBPort:     "5432",
		DBName:     "todo",
		DBSSLMode:  "disable",
	}

	dsn := buildDSN(cfg)
	if strings.Contains(dsn, "sslrootcert=") {
		t.Fatalf("buildDSN() = %q, must not include sslrootcert when it is empty", dsn)
	}
}
