package auth

import (
	"bytes"
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
	db      transactionStarter
	queries credentialQueries
}

type credentialQueries interface {
	GetPasskeyCredential(context.Context, []byte) (sqlc.PasskeyCredential, error)
	UpdatePasskeyCredential(context.Context, sqlc.UpdatePasskeyCredentialParams) (int64, error)
}

func NewRepository(db transactionStarter, queries credentialQueries) *Repository {
	return &Repository{db: db, queries: queries}
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

func (r *Repository) FindCredential(ctx context.Context, credentialID []byte) ([16]byte, webauthn.Credential, error) {
	row, err := r.queries.GetPasskeyCredential(ctx, credentialID)
	if err != nil {
		return [16]byte{}, webauthn.Credential{}, err
	}
	if !row.UserID.Valid {
		return [16]byte{}, webauthn.Credential{}, fmt.Errorf("credential has no user ID")
	}
	var credential webauthn.Credential
	if err := json.Unmarshal(row.CredentialData, &credential); err != nil {
		return [16]byte{}, webauthn.Credential{}, fmt.Errorf("decode credential: %w", err)
	}
	if !bytes.Equal(credential.ID, credentialID) {
		return [16]byte{}, webauthn.Credential{}, fmt.Errorf("credential ID mismatch")
	}
	return row.UserID.Bytes, credential, nil
}

func (r *Repository) UpdateCredential(ctx context.Context, userID [16]byte, credential webauthn.Credential) error {
	data, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("encode credential: %w", err)
	}
	rows, err := r.queries.UpdatePasskeyCredential(ctx, sqlc.UpdatePasskeyCredentialParams{
		CredentialData: data,
		CredentialID:   credential.ID,
		UserID:         pgtype.UUID{Bytes: userID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("update credential: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("credential no longer exists")
	}
	return nil
}
