package auth

import "testing"

func TestNewWebAuthn(t *testing.T) {
	tests := []struct {
		name    string
		rpID    string
		origin  string
		wantErr bool
	}{
		{name: "local", rpID: "localhost", origin: "http://localhost:3000"},
		{name: "production", rpID: "ca-techlounge-todo.kyo8.dev", origin: "https://ca-techlounge-todo.kyo8.dev"},
		{name: "missing RP ID", origin: "http://localhost:3000", wantErr: true},
		{name: "invalid RP ID", rpID: "http://localhost:3000", origin: "http://localhost:3000", wantErr: true},
		{name: "missing origin", rpID: "localhost", wantErr: true},
		{name: "different host", rpID: "localhost", origin: "https://example.com", wantErr: true},
		{name: "insecure production", rpID: "example.com", origin: "http://example.com", wantErr: true},
		{name: "origin with path", rpID: "localhost", origin: "http://localhost:3000/login", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instance, err := NewWebAuthn(tt.rpID, tt.origin)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewWebAuthn() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && instance == nil {
				t.Fatal("NewWebAuthn() returned nil without error")
			}
		})
	}
}
