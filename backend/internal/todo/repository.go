package todo

import (
	"context"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
)

type TodoRepository struct {
	queries todoQueries
}

type todoQueries interface {
	CreateTodo(context.Context, string) (sqlc.Todo, error)
	ListTodos(context.Context, sqlc.ListTodosParams) ([]sqlc.Todo, error)
	UpdateTodo(context.Context, sqlc.UpdateTodoParams) (sqlc.Todo, error)
	DeleteTodo(context.Context, int64) (int64, error)
}

func NewTodoRepository(queries todoQueries) *TodoRepository {
	return &TodoRepository{
		queries: queries,
	}
}

func (r *TodoRepository) Create(
	ctx context.Context,
	title string,
) (sqlc.Todo, error) {
	return r.queries.CreateTodo(ctx, title)
}

func (r *TodoRepository) List(ctx context.Context, title string, limit, offset int32) ([]sqlc.Todo, error) {
	return r.queries.ListTodos(ctx, sqlc.ListTodosParams{
		Title:      title,
		PageLimit:  limit,
		PageOffset: offset,
	})
}

func (r *TodoRepository) Update(ctx context.Context, id int64, title string) (sqlc.Todo, error) {
	return r.queries.UpdateTodo(ctx, sqlc.UpdateTodoParams{ID: id, Title: title})
}

func (r *TodoRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.queries.DeleteTodo(ctx, id)
	return err
}
