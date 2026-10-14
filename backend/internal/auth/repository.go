package auth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
)

type transactionStarter interface {
	Begin(context.Context) (pgx.Tx, error)
}

type Repository struct {
	db transactionStarter
}

func NewRepository(db transactionStarter) *Repository {
	return &Repository{db: db}
}

// CreateAccount persists a user and a previously verified credential atomically.
// The caller must verify the WebAuthn registration response before calling it.
func (r *Repository) CreateAccount(ctx context.Context, userID [16]byte, credential webauthn.Credential) error {
	databaseUserID := pgtype.UUID{Bytes: userID, Valid: true}
	credentialData, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("encode credential: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin account transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	queries := sqlc.New(tx)
	if _, err := queries.CreateUser(ctx, databaseUserID); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	if _, err := queries.CreatePasskeyCredential(ctx, sqlc.CreatePasskeyCredentialParams{
		CredentialID:   credential.ID,
		UserID:         databaseUserID,
		CredentialData: credentialData,
	}); err != nil {
		return fmt.Errorf("create passkey credential: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit account transaction: %w", err)
	}
	return nil
}
