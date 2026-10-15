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

type fakeCredentialQueries struct {
	row       sqlc.PasskeyCredential
	findErr   error
	updateErr error
	updated   sqlc.UpdatePasskeyCredentialParams
	rows      int64
}

func (f *fakeCredentialQueries) GetPasskeyCredential(context.Context, []byte) (sqlc.PasskeyCredential, error) {
	return f.row, f.findErr
}
func (f *fakeCredentialQueries) UpdatePasskeyCredential(_ context.Context, params sqlc.UpdatePasskeyCredentialParams) (int64, error) {
	f.updated = params
	return f.rows, f.updateErr
}

func TestCredentialRepositoryWithFakeQueries(t *testing.T) {
	id := [16]byte{1, 2, 3}
	credential := webauthn.Credential{ID: []byte{4, 5}, PublicKey: []byte{6}}
	data, err := json.Marshal(credential)
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeCredentialQueries{row: sqlc.PasskeyCredential{UserID: pgtype.UUID{Bytes: id, Valid: true}, CredentialData: data}, rows: 1}
	repo := NewRepository(failingStarter{}, fake)
	gotID, gotCredential, err := repo.FindCredential(context.Background(), credential.ID)
	if err != nil || gotID != id || string(gotCredential.ID) != string(credential.ID) {
		t.Fatalf("lookup: %v", err)
	}
	if err := repo.UpdateCredential(context.Background(), id, credential); err != nil {
		t.Fatal(err)
	}
	if fake.updated.UserID.Bytes != id || string(fake.updated.CredentialID) != string(credential.ID) {
		t.Fatal("wrong update arguments")
	}
	fake.rows = 0
	if err := repo.UpdateCredential(context.Background(), id, credential); err == nil {
		t.Fatal("missing row accepted")
	}
	fake.findErr = pgx.ErrNoRows
	if _, _, err := repo.FindCredential(context.Background(), credential.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing: %v", err)
	}
	fake.findErr = nil
	fake.row.CredentialData = []byte("{")
	if _, _, err := repo.FindCredential(context.Background(), credential.ID); err == nil {
		t.Fatal("corrupted credential accepted")
	}
	fake.row.CredentialData = data
	fake.row.UserID.Valid = false
	if _, _, err := repo.FindCredential(context.Background(), credential.ID); err == nil {
		t.Fatal("missing owner accepted")
	}
}

func (failingStarter) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("begin failed")
}

func TestCreateAccountBeginFailure(t *testing.T) {
	r := NewRepository(failingStarter{}, nil)
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
	r := NewRepository(pool, sqlc.New(pool))
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
	owner, loaded, err := r.FindCredential(ctx, credentialID)
	if err != nil || owner != firstID.Bytes || string(loaded.ID) != string(credentialID) {
		t.Fatalf("find credential: owner=%v credential=%+v err=%v", owner, loaded, err)
	}
	if _, _, err := r.FindCredential(ctx, []byte("missing")); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing credential: %v", err)
	}
	loaded.Authenticator.SignCount = 2
	if err := r.UpdateCredential(ctx, owner, loaded); err != nil {
		t.Fatalf("update credential: %v", err)
	}
	stored, err = queries.GetPasskeyCredential(ctx, credentialID)
	if err != nil || !stored.LastUsedAt.Valid {
		t.Fatalf("last_used_at not updated: %v", err)
	}
	if err := json.Unmarshal(stored.CredentialData, &decoded); err != nil || decoded.Authenticator.SignCount != 2 {
		t.Fatalf("counter not updated: %v", err)
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
