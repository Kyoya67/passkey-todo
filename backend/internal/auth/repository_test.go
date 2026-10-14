package auth

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
)

type failingStarter struct{}

func (failingStarter) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("begin failed")
}

func TestCreateAccountBeginFailure(t *testing.T) {
	r := NewRepository(failingStarter{})
	userID := newTestUUID(t)
	if err := r.CreateAccount(context.Background(), userID.Bytes, webauthn.Credential{ID: []byte{1}, PublicKey: []byte{2}}); err == nil {
		t.Fatal("expected begin error")
	}
}

func TestCreateAccountIntegration(t *testing.T) {
	databaseURL := os.Getenv("INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("INTEGRATION_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	firstID := newTestUUID(t)
	secondID := newTestUUID(t)
	defer func() {
		for _, id := range []pgtype.UUID{firstID, secondID} {
			if _, err := pool.Exec(ctx, "DELETE FROM users WHERE id = $1", id); err != nil {
				t.Errorf("clean up user: %v", err)
			}
		}
	}()

	credentialID := make([]byte, 32)
	if _, err := rand.Read(credentialID); err != nil {
		t.Fatalf("generate credential ID: %v", err)
	}
	credential := &webauthn.Credential{ID: credentialID, PublicKey: []byte{1, 2, 3}}
	r := NewRepository(pool)
	if err := r.CreateAccount(ctx, firstID.Bytes, *credential); err != nil {
		t.Fatalf("create account: %v", err)
	}
	queries := sqlc.New(pool)
	if _, err := queries.GetUserByID(ctx, firstID); err != nil {
		t.Fatalf("user not persisted: %v", err)
	}
	stored, err := queries.GetPasskeyCredential(ctx, credentialID)
	if err != nil {
		t.Fatalf("credential not persisted: %v", err)
	}
	if stored.UserID != firstID {
		t.Fatalf("credential owner = %v, want %v", stored.UserID, firstID)
	}
	var decoded webauthn.Credential
	if err := json.Unmarshal(stored.CredentialData, &decoded); err != nil {
		t.Fatalf("decode stored credential: %v", err)
	}
	if string(decoded.ID) != string(credential.ID) || string(decoded.PublicKey) != string(credential.PublicKey) {
		t.Fatal("stored credential differs from input")
	}

	if err := r.CreateAccount(ctx, secondID.Bytes, *credential); err == nil {
		t.Fatal("expected duplicate credential ID to fail")
	}
	if _, err := queries.GetUserByID(ctx, secondID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("second user should have rolled back, got %v", err)
	}
}

func newTestUUID(t *testing.T) pgtype.UUID {
	t.Helper()
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		t.Fatalf("generate user ID: %v", err)
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}
