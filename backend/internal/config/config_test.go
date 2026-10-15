package config

import "testing"

func TestLoad(t *testing.T) {
	for _, tc := range []struct {
		name, rpID, origin string
		wantErr            bool
	}{
		{name: "missing auth configuration", wantErr: true},
		{name: "local auth", rpID: "localhost", origin: "http://localhost:3000"},
		{name: "RP ID only", rpID: "localhost", wantErr: true},
		{name: "origin only", origin: "http://localhost:3000", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for key, value := range map[string]string{"APP_PORT": "8080", "DB_HOST": "localhost", "DB_PORT": "5432", "DB_USER": "todo", "DB_PASSWORD": "test", "DB_NAME": "todo", "DB_SSLMODE": "", "DB_SSLROOTCERT": "", "CORS_ALLOWED_ORIGINS": " http://localhost:3000, ,http://localhost:8081 ", "WEBAUTHN_RP_ID": tc.rpID, "WEBAUTHN_ORIGIN": tc.origin, "REDIS_ADDR": "localhost:6379", "REDIS_TLS": "false"} {
				t.Setenv(key, value)
			}
			cfg, err := Load()
			if (err != nil) != tc.wantErr {
				t.Fatalf("error %v, wantErr %v", err, tc.wantErr)
			}
			if err == nil && (cfg.WebAuthnRPID != tc.rpID || cfg.WebAuthnOrigin != tc.origin || cfg.DBSSLMode != "disable" || cfg.RedisAddress != "localhost:6379" || cfg.RedisTLS || len(cfg.CORSAllowedOrigins) != 2) {
				t.Fatal("configuration differs from environment")
			}
			if !tc.wantErr {
				if tc.rpID != "" {
					t.Setenv("REDIS_ADDR", "")
					if _, err := Load(); err == nil {
						t.Fatal("missing Redis address accepted")
					}
					t.Setenv("REDIS_ADDR", "localhost:6379")
				}
				t.Setenv("APP_PORT", "")
				if _, err := Load(); err == nil {
					t.Fatal("missing port accepted")
				}
			}
		})
	}
}

func TestConfigHelpers(t *testing.T) {
	t.Setenv("DB_SSLMODE", "verify-full")
	if getenvDefault("DB_SSLMODE", "disable") != "verify-full" {
		t.Fatal("explicit value ignored")
	}
	if len(splitCSV("  ")) != 0 {
		t.Fatal("blank origins should be empty")
	}
}
