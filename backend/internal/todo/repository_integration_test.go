package todo

import (
	"context"
	"os"
	"testing"
	"time"

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
	title := "integration-test-" + time.Now().UTC().Format("20060102150405.000000000")

	created, err := repository.Create(ctx, title)
	if err != nil {
		t.Fatalf("create todo: %v", err)
	}
	deleted := false
	defer func() {
		if !deleted {
			if err := repository.Delete(ctx, created.ID); err != nil {
				t.Errorf("cleanup todo: %v", err)
			}
		}
	}()

	if created.ID == 0 || created.Title != title || !created.CreatedAt.Valid || !created.UpdatedAt.Valid {
		t.Fatalf("unexpected created todo: %+v", created)
	}
	if !created.CreatedAt.Time.Equal(created.UpdatedAt.Time) {
		t.Fatalf("created and updated timestamps differ: created=%v updated=%v", created.CreatedAt.Time, created.UpdatedAt.Time)
	}

	list, err := repository.List(ctx, title, 10, 0)
	if err != nil {
		t.Fatalf("list todos: %v", err)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("unexpected list result: %+v", list)
	}

	time.Sleep(2 * time.Millisecond)
	updatedTitle := title + "-updated"
	updated, err := repository.Update(ctx, created.ID, updatedTitle)
	if err != nil {
		t.Fatalf("update todo: %v", err)
	}
	if updated.Title != updatedTitle || !updated.UpdatedAt.Valid || !updated.UpdatedAt.Time.After(created.UpdatedAt.Time) {
		t.Fatalf("unexpected updated todo: created=%+v updated=%+v", created, updated)
	}

	if err := repository.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete todo: %v", err)
	}
	deleted = true
	if _, err := repository.Update(ctx, created.ID, "deleted"); err == nil {
		t.Fatal("expected update after delete to fail")
	}
}
