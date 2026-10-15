package todo

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
)

func TestRepositoryIntegration(t *testing.T) {
	databaseURL := os.Getenv("INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("INTEGRATION_DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	repository := NewRepository(sqlc.New(pool))
	var ownerA, ownerB [16]byte
	if _, err := rand.Read(ownerA[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(ownerB[:]); err != nil {
		t.Fatal(err)
	}
	for _, owner := range [][16]byte{ownerA, ownerB} {
		if _, err := pool.Exec(ctx, "INSERT INTO users (id) VALUES ($1)", pgtype.UUID{Bytes: owner, Valid: true}); err != nil {
			t.Fatal(err)
		}
		defer func(owner [16]byte) {
			if _, err := pool.Exec(ctx, "DELETE FROM users WHERE id = $1", pgtype.UUID{Bytes: owner, Valid: true}); err != nil {
				t.Errorf("cleanup user: %v", err)
			}
		}(owner)
	}
	title := "integration-test-" + time.Now().UTC().Format("20060102150405.000000000")

	created, err := repository.Create(ctx, ownerA, title, StatusNotStarted)
	if err != nil {
		t.Fatalf("create todo: %v", err)
	}
	deleted := false
	defer func() {
		if !deleted {
			if err := repository.Delete(ctx, ownerA, created.ID); err != nil {
				t.Errorf("cleanup todo: %v", err)
			}
		}
	}()

	if created.ID == 0 || created.Title != title || !created.CreatedAt.Valid || !created.UpdatedAt.Valid {
		t.Fatalf("unexpected created todo: %+v", created)
	}
	if created.Status != StatusNotStarted {
		t.Fatalf("created status=%q", created.Status)
	}
	if !created.CreatedAt.Time.Equal(created.UpdatedAt.Time) {
		t.Fatalf("created and updated timestamps differ: created=%v updated=%v", created.CreatedAt.Time, created.UpdatedAt.Time)
	}

	list, err := repository.List(ctx, ownerA, title, 10, 0)
	if err != nil {
		t.Fatalf("list todos: %v", err)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("unexpected list result: %+v", list)
	}
	if list[0].Status != StatusNotStarted {
		t.Fatalf("listed status=%q", list[0].Status)
	}
	_, err = pool.Exec(ctx, "INSERT INTO todos (title, user_id, status) VALUES ($1, $2, $3)", title, pgtype.UUID{Bytes: ownerA, Valid: true}, "invalid")
	var statusErr *pgconn.PgError
	if !errors.As(err, &statusErr) || statusErr.Code != "23514" {
		t.Fatalf("invalid status must be rejected by DB CHECK: %v", err)
	}
	otherList, err := repository.List(ctx, ownerB, title, 10, 0)
	if err != nil || len(otherList) != 0 {
		t.Fatalf("other user list=%+v err=%v", otherList, err)
	}
	if _, err := repository.Update(ctx, ownerB, created.ID, "forbidden", ""); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("other user update: %v", err)
	}
	if err := repository.Delete(ctx, ownerB, created.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("other user delete: %v", err)
	}
	_, err = pool.Exec(ctx, "INSERT INTO todos (title) VALUES ($1)", title)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23502" {
		t.Fatalf("ownerless todo must be rejected with NOT NULL violation: %v", err)
	}

	time.Sleep(2 * time.Millisecond)
	updatedTitle := title + "-updated"
	updated, err := repository.Update(ctx, ownerA, created.ID, updatedTitle, StatusCompleted)
	if err != nil {
		t.Fatalf("update todo: %v", err)
	}
	if updated.Title != updatedTitle || !updated.UpdatedAt.Valid || !updated.UpdatedAt.Time.After(created.UpdatedAt.Time) {
		t.Fatalf("unexpected updated todo: created=%+v updated=%+v", created, updated)
	}
	if updated.Status != StatusCompleted {
		t.Fatalf("updated status=%q", updated.Status)
	}
	retained, err := repository.Update(ctx, ownerA, created.ID, updatedTitle, "")
	if err != nil || retained.Status != StatusCompleted {
		t.Fatalf("retained status=%q err=%v", retained.Status, err)
	}

	if err := repository.Delete(ctx, ownerA, created.ID); err != nil {
		t.Fatalf("delete todo: %v", err)
	}
	deleted = true
	if _, err := repository.Update(ctx, ownerA, created.ID, "deleted", ""); err == nil {
		t.Fatal("expected update after delete to fail")
	}
}
