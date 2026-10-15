package todo

import (
	"context"
	"errors"
	"testing"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
)

type fakeTodoQueries struct {
	createFn func(context.Context, string) (sqlc.Todo, error)
	listFn   func(context.Context, sqlc.ListTodosParams) ([]sqlc.Todo, error)
	updateFn func(context.Context, sqlc.UpdateTodoParams) (sqlc.Todo, error)
	deleteFn func(context.Context, int64) (int64, error)
}

func (f *fakeTodoQueries) CreateTodo(ctx context.Context, title string) (sqlc.Todo, error) {
	return f.createFn(ctx, title)
}

func (f *fakeTodoQueries) ListTodos(ctx context.Context, params sqlc.ListTodosParams) ([]sqlc.Todo, error) {
	return f.listFn(ctx, params)
}

func (f *fakeTodoQueries) UpdateTodo(ctx context.Context, params sqlc.UpdateTodoParams) (sqlc.Todo, error) {
	return f.updateFn(ctx, params)
}

func (f *fakeTodoQueries) DeleteTodo(ctx context.Context, id int64) (int64, error) {
	return f.deleteFn(ctx, id)
}

func TestRepository(t *testing.T) {
	wantErr := errors.New("database error")
	queries := &fakeTodoQueries{
		createFn: func(_ context.Context, title string) (sqlc.Todo, error) {
			return sqlc.Todo{Title: title}, nil
		},
		listFn: func(_ context.Context, params sqlc.ListTodosParams) ([]sqlc.Todo, error) {
			return []sqlc.Todo{{Title: params.Title}}, nil
		},
		updateFn: func(_ context.Context, params sqlc.UpdateTodoParams) (sqlc.Todo, error) {
			return sqlc.Todo{ID: params.ID, Title: params.Title}, nil
		},
		deleteFn: func(context.Context, int64) (int64, error) { return 1, nil },
	}
	repository := NewRepository(queries)

	created, err := repository.Create(context.Background(), "create")
	if err != nil || created.Title != "create" {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	listed, err := repository.List(context.Background(), "list", 10, 2)
	if err != nil || len(listed) != 1 || listed[0].Title != "list" {
		t.Fatalf("listed=%+v err=%v", listed, err)
	}
	updated, err := repository.Update(context.Background(), 2, "update")
	if err != nil || updated.ID != 2 || updated.Title != "update" {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	if err := repository.Delete(context.Background(), 2); err != nil {
		t.Fatal(err)
	}

	queries.createFn = func(context.Context, string) (sqlc.Todo, error) { return sqlc.Todo{}, wantErr }
	if _, err := repository.Create(context.Background(), "error"); !errors.Is(err, wantErr) {
		t.Fatalf("create err=%v", err)
	}
}
