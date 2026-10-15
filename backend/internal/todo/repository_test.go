package todo

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
)

type fakeTodoQueries struct {
	createFn func(context.Context, sqlc.CreateTodoParams) (sqlc.Todo, error)
	listFn   func(context.Context, sqlc.ListTodosParams) ([]sqlc.Todo, error)
	updateFn func(context.Context, sqlc.UpdateTodoParams) (sqlc.Todo, error)
	deleteFn func(context.Context, sqlc.DeleteTodoParams) (int64, error)
}

func (f *fakeTodoQueries) CreateTodo(ctx context.Context, params sqlc.CreateTodoParams) (sqlc.Todo, error) {
	return f.createFn(ctx, params)
}

func (f *fakeTodoQueries) ListTodos(ctx context.Context, params sqlc.ListTodosParams) ([]sqlc.Todo, error) {
	return f.listFn(ctx, params)
}

func (f *fakeTodoQueries) UpdateTodo(ctx context.Context, params sqlc.UpdateTodoParams) (sqlc.Todo, error) {
	return f.updateFn(ctx, params)
}

func (f *fakeTodoQueries) DeleteTodo(ctx context.Context, params sqlc.DeleteTodoParams) (int64, error) {
	return f.deleteFn(ctx, params)
}

func TestRepository(t *testing.T) {
	wantErr := errors.New("database error")
	wantOwner := pgtype.UUID{Bytes: testUserID, Valid: true}
	queries := &fakeTodoQueries{
		createFn: func(_ context.Context, params sqlc.CreateTodoParams) (sqlc.Todo, error) {
			if params.OwnerID != wantOwner {
				t.Fatalf("create owner=%+v", params.OwnerID)
			}
			return sqlc.Todo{Title: params.Title}, nil
		},
		listFn: func(_ context.Context, params sqlc.ListTodosParams) ([]sqlc.Todo, error) {
			if params.OwnerID != wantOwner || params.PageLimit != 10 || params.PageOffset != 2 {
				t.Fatalf("list params=%+v", params)
			}
			return []sqlc.Todo{{Title: params.Title}}, nil
		},
		updateFn: func(_ context.Context, params sqlc.UpdateTodoParams) (sqlc.Todo, error) {
			if params.OwnerID != wantOwner {
				t.Fatalf("update owner=%+v", params.OwnerID)
			}
			return sqlc.Todo{ID: params.TodoID, Title: params.Title}, nil
		},
		deleteFn: func(_ context.Context, params sqlc.DeleteTodoParams) (int64, error) {
			if params.OwnerID != wantOwner {
				t.Fatalf("delete owner=%+v", params.OwnerID)
			}
			return params.TodoID, nil
		},
	}
	repository := NewRepository(queries)

	created, err := repository.Create(context.Background(), testUserID, "create")
	if err != nil || created.Title != "create" {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	listed, err := repository.List(context.Background(), testUserID, "list", 10, 2)
	if err != nil || len(listed) != 1 || listed[0].Title != "list" {
		t.Fatalf("listed=%+v err=%v", listed, err)
	}
	updated, err := repository.Update(context.Background(), testUserID, 2, "update")
	if err != nil || updated.ID != 2 || updated.Title != "update" {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	if err := repository.Delete(context.Background(), testUserID, 2); err != nil {
		t.Fatal(err)
	}

	queries.createFn = func(context.Context, sqlc.CreateTodoParams) (sqlc.Todo, error) { return sqlc.Todo{}, wantErr }
	if _, err := repository.Create(context.Background(), testUserID, "error"); !errors.Is(err, wantErr) {
		t.Fatalf("create err=%v", err)
	}
}
