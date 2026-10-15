package todo

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
)

type fakeRepository struct {
	createFn func(context.Context, string) (sqlc.Todo, error)
	listFn   func(context.Context, string, int32, int32) ([]sqlc.Todo, error)
	updateFn func(context.Context, int64, string) (sqlc.Todo, error)
	deleteFn func(context.Context, int64) error
}

func (f *fakeRepository) Create(ctx context.Context, title string) (sqlc.Todo, error) {
	return f.createFn(ctx, title)
}

func (f *fakeRepository) List(ctx context.Context, title string, limit, offset int32) ([]sqlc.Todo, error) {
	return f.listFn(ctx, title, limit, offset)
}

func (f *fakeRepository) Update(ctx context.Context, id int64, title string) (sqlc.Todo, error) {
	return f.updateFn(ctx, id, title)
}

func (f *fakeRepository) Delete(ctx context.Context, id int64) error {
	return f.deleteFn(ctx, id)
}

func TestServiceCreate(t *testing.T) {
	t.Run("rejects blank title", func(t *testing.T) {
		called := false
		repository := &fakeRepository{createFn: func(context.Context, string) (sqlc.Todo, error) {
			called = true
			return sqlc.Todo{}, nil
		}}

		err := NewService(repository).Create(context.Background(), "   ")
		if !errors.Is(err, ErrInvalidTitle) || called {
			t.Fatalf("err=%v called=%v", err, called)
		}
	})

	t.Run("passes title to repository", func(t *testing.T) {
		var got string
		repository := &fakeRepository{createFn: func(_ context.Context, title string) (sqlc.Todo, error) {
			got = title
			return sqlc.Todo{}, nil
		}}

		if err := NewService(repository).Create(context.Background(), "todo"); err != nil {
			t.Fatal(err)
		}
		if got != "todo" {
			t.Fatalf("title=%q", got)
		}
	})
}

func TestServiceList(t *testing.T) {
	t.Run("uses default limit", func(t *testing.T) {
		var gotLimit, gotOffset int32
		repository := &fakeRepository{listFn: func(_ context.Context, _ string, limit, offset int32) ([]sqlc.Todo, error) {
			gotLimit, gotOffset = limit, offset
			return nil, nil
		}}

		if _, err := NewService(repository).List(context.Background(), "", 0, 0); err != nil {
			t.Fatal(err)
		}
		if gotLimit != defaultTodoLimit || gotOffset != 0 {
			t.Fatalf("limit=%d offset=%d", gotLimit, gotOffset)
		}
	})

	for _, test := range []struct {
		name   string
		limit  int32
		offset int32
	}{
		{name: "negative limit", limit: -1, offset: 0},
		{name: "limit over maximum", limit: maxTodoLimit + 1, offset: 0},
		{name: "negative offset", limit: 1, offset: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &fakeRepository{listFn: func(context.Context, string, int32, int32) ([]sqlc.Todo, error) {
				t.Fatal("repository should not be called")
				return nil, nil
			}}
			_, err := NewService(repository).List(context.Background(), "", test.limit, test.offset)
			if !errors.Is(err, ErrInvalidPage) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestServiceUpdateAndDelete(t *testing.T) {
	t.Run("maps no rows to not found", func(t *testing.T) {
		repository := &fakeRepository{
			updateFn: func(context.Context, int64, string) (sqlc.Todo, error) { return sqlc.Todo{}, pgx.ErrNoRows },
			deleteFn: func(context.Context, int64) error { return pgx.ErrNoRows },
		}
		service := NewService(repository)
		if err := service.Update(context.Background(), 1, "todo"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("update err=%v", err)
		}
		if err := service.Delete(context.Background(), 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("delete err=%v", err)
		}
	})

	t.Run("rejects blank update title", func(t *testing.T) {
		called := false
		repository := &fakeRepository{updateFn: func(context.Context, int64, string) (sqlc.Todo, error) {
			called = true
			return sqlc.Todo{}, nil
		}}
		if err := NewService(repository).Update(context.Background(), 1, " "); !errors.Is(err, ErrInvalidTitle) || called {
			t.Fatalf("err=%v called=%v", err, called)
		}
	})
}
