package todo

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
)

type Repository struct {
	queries todoQueries
}

type todoQueries interface {
	CreateTodo(context.Context, sqlc.CreateTodoParams) (sqlc.Todo, error)
	ListTodos(context.Context, sqlc.ListTodosParams) ([]sqlc.Todo, error)
	UpdateTodo(context.Context, sqlc.UpdateTodoParams) (sqlc.Todo, error)
	DeleteTodo(context.Context, sqlc.DeleteTodoParams) (int64, error)
}

func NewRepository(queries todoQueries) *Repository {
	return &Repository{
		queries: queries,
	}
}

func (r *Repository) Create(
	ctx context.Context,
	userID [16]byte,
	title, status string,
) (sqlc.Todo, error) {
	return r.queries.CreateTodo(ctx, sqlc.CreateTodoParams{Title: title, OwnerID: pgtype.UUID{Bytes: userID, Valid: true}, Status: status})
}

func (r *Repository) List(ctx context.Context, userID [16]byte, title string, limit, offset int32) ([]sqlc.Todo, error) {
	return r.queries.ListTodos(ctx, sqlc.ListTodosParams{
		OwnerID:    pgtype.UUID{Bytes: userID, Valid: true},
		Title:      title,
		PageLimit:  limit,
		PageOffset: offset,
	})
}

func (r *Repository) Update(ctx context.Context, userID [16]byte, id int64, title, status string) (sqlc.Todo, error) {
	return r.queries.UpdateTodo(ctx, sqlc.UpdateTodoParams{TodoID: id, OwnerID: pgtype.UUID{Bytes: userID, Valid: true}, Title: title, Status: status})
}

func (r *Repository) Delete(ctx context.Context, userID [16]byte, id int64) error {
	_, err := r.queries.DeleteTodo(ctx, sqlc.DeleteTodoParams{TodoID: id, OwnerID: pgtype.UUID{Bytes: userID, Valid: true}})
	return err
}
