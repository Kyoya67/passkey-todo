package repository

import (
	"context"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
)

type TodoRepository struct {
	queries *sqlc.Queries
}

func NewTodoRepository(queries *sqlc.Queries) *TodoRepository {
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
