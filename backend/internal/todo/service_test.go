package todo

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
)

type fakeTodoRepository struct {
	createFn func(context.Context, string) (sqlc.Todo, error)
	listFn   func(context.Context, string, int32, int32) ([]sqlc.Todo, error)
	updateFn func(context.Context, int64, string) (sqlc.Todo, error)
	deleteFn func(context.Context, int64) error
}

func (f *fakeTodoRepository) Create(ctx context.Context, title string) (sqlc.Todo, error) {
	return f.createFn(ctx, title)
}

func (f *fakeTodoRepository) List(ctx context.Context, title string, limit, offset int32) ([]sqlc.Todo, error) {
	return f.listFn(ctx, title, limit, offset)
}

func (f *fakeTodoRepository) Update(ctx context.Context, id int64, title string) (sqlc.Todo, error) {
	return f.updateFn(ctx, id, title)
}

func (f *fakeTodoRepository) Delete(ctx context.Context, id int64) error {
	return f.deleteFn(ctx, id)
}

func TestTodoServiceCreate(t *testing.T) {
	t.Run("rejects blank title", func(t *testing.T) {
		called := false
		repository := &fakeTodoRepository{createFn: func(context.Context, string) (sqlc.Todo, error) {
			called = true
			return sqlc.Todo{}, nil
		}}

		err := NewTodoService(repository).Create(context.Background(), "   ")
		if !errors.Is(err, ErrInvalidTitle) || called {
			t.Fatalf("err=%v called=%v", err, called)
		}
	})

	t.Run("passes title to repository", func(t *testing.T) {
		var got string
		repository := &fakeTodoRepository{createFn: func(_ context.Context, title string) (sqlc.Todo, error) {
			got = title
			return sqlc.Todo{}, nil
		}}

		if err := NewTodoService(repository).Create(context.Background(), "todo"); err != nil {
			t.Fatal(err)
		}
		if got != "todo" {
			t.Fatalf("title=%q", got)
		}
	})
}

func TestTodoServiceList(t *testing.T) {
	t.Run("uses default limit", func(t *testing.T) {
		var gotLimit, gotOffset int32
		repository := &fakeTodoRepository{listFn: func(_ context.Context, _ string, limit, offset int32) ([]sqlc.Todo, error) {
			gotLimit, gotOffset = limit, offset
			return nil, nil
		}}

		if _, err := NewTodoService(repository).List(context.Background(), "", 0, 0); err != nil {
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
			repository := &fakeTodoRepository{listFn: func(context.Context, string, int32, int32) ([]sqlc.Todo, error) {
				t.Fatal("repository should not be called")
				return nil, nil
			}}
			_, err := NewTodoService(repository).List(context.Background(), "", test.limit, test.offset)
			if !errors.Is(err, ErrInvalidPage) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestTodoServiceUpdateAndDelete(t *testing.T) {
	t.Run("maps no rows to not found", func(t *testing.T) {
		repository := &fakeTodoRepository{
			updateFn: func(context.Context, int64, string) (sqlc.Todo, error) { return sqlc.Todo{}, pgx.ErrNoRows },
			deleteFn: func(context.Context, int64) error { return pgx.ErrNoRows },
		}
		service := NewTodoService(repository)
		if err := service.Update(context.Background(), 1, "todo"); !errors.Is(err, ErrTodoNotFound) {
			t.Fatalf("update err=%v", err)
		}
		if err := service.Delete(context.Background(), 1); !errors.Is(err, ErrTodoNotFound) {
			t.Fatalf("delete err=%v", err)
		}
	})

	t.Run("rejects blank update title", func(t *testing.T) {
		called := false
		repository := &fakeTodoRepository{updateFn: func(context.Context, int64, string) (sqlc.Todo, error) {
			called = true
			return sqlc.Todo{}, nil
		}}
		if err := NewTodoService(repository).Update(context.Background(), 1, " "); !errors.Is(err, ErrInvalidTitle) || called {
			t.Fatalf("err=%v called=%v", err, called)
		}
	})
}
