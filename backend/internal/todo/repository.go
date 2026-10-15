package todo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
)

type Repository struct {
	queries todoQueries
	db      transactionStarter
}

type transactionStarter interface {
	Begin(context.Context) (pgx.Tx, error)
}

type todoQueries interface {
	CreateTodo(context.Context, sqlc.CreateTodoParams) (sqlc.Todo, error)
	ListTodos(context.Context, sqlc.ListTodosParams) ([]sqlc.Todo, error)
	UpdateTodo(context.Context, sqlc.UpdateTodoParams) (sqlc.Todo, error)
	DeleteTodo(context.Context, sqlc.DeleteTodoParams) (int64, error)
}

func NewRepository(queries todoQueries, db transactionStarter) *Repository {
	return &Repository{
		queries: queries,
		db:      db,
	}
}

// Move places a TODO immediately before another TODO in the same status and priority group.
// A nil beforeID appends it to the end. The user row serializes moves per owner.
func (r *Repository) Move(ctx context.Context, userID [16]byte, id int64, beforeID *int64) error {
	if r.db == nil {
		return errors.New("todo transaction database is not configured")
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	owner := pgtype.UUID{Bytes: userID, Valid: true}
	var locked [16]byte
	if err := tx.QueryRow(ctx, "SELECT id FROM users WHERE id = $1 FOR UPDATE", owner).Scan(&locked); err != nil {
		return err
	}
	var status, level string
	if err := tx.QueryRow(ctx, "SELECT status, priority_level FROM todos WHERE id = $1 AND user_id = $2", id, owner).Scan(&status, &level); err != nil {
		return err
	}
	if status == StatusCompleted {
		return ErrInvalidPosition
	}
	rows, err := tx.Query(ctx, "SELECT id FROM todos WHERE user_id = $1 AND status = $2 AND priority_level = $3 ORDER BY priority, id", owner, status, level)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var itemID int64
		if err := rows.Scan(&itemID); err != nil {
			rows.Close()
			return err
		}
		if itemID != id {
			ids = append(ids, itemID)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	index := len(ids)
	if beforeID != nil {
		if *beforeID == id {
			return tx.Commit(ctx)
		}
		index = -1
		for i, itemID := range ids {
			if itemID == *beforeID {
				index = i
				break
			}
		}
		if index < 0 {
			return ErrInvalidPosition
		}
	}
	ids = append(ids, 0)
	copy(ids[index+1:], ids[index:])
	ids[index] = id
	_, err = tx.Exec(ctx, `UPDATE todos AS todo SET priority = ordered.position
		FROM unnest($1::bigint[]) WITH ORDINALITY AS ordered(id, position)
		WHERE todo.id = ordered.id AND todo.user_id = $2 AND todo.status = $3 AND todo.priority_level = $4`, ids, owner, status, level)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) Create(
	ctx context.Context,
	userID [16]byte,
	title, status, level string,
) (sqlc.Todo, error) {
	return r.queries.CreateTodo(ctx, sqlc.CreateTodoParams{Title: title, OwnerID: pgtype.UUID{Bytes: userID, Valid: true}, Status: status, PriorityLevel: level})
}

func (r *Repository) List(ctx context.Context, userID [16]byte, title string, limit, offset int32) ([]sqlc.Todo, error) {
	return r.queries.ListTodos(ctx, sqlc.ListTodosParams{
		OwnerID:    pgtype.UUID{Bytes: userID, Valid: true},
		Title:      title,
		PageLimit:  limit,
		PageOffset: offset,
	})
}

func (r *Repository) Update(ctx context.Context, userID [16]byte, id int64, title, status, level string) (sqlc.Todo, error) {
	return r.queries.UpdateTodo(ctx, sqlc.UpdateTodoParams{TodoID: id, OwnerID: pgtype.UUID{Bytes: userID, Valid: true}, Title: title, Status: status, PriorityLevel: level})
}

func (r *Repository) Delete(ctx context.Context, userID [16]byte, id int64) error {
	_, err := r.queries.DeleteTodo(ctx, sqlc.DeleteTodoParams{TodoID: id, OwnerID: pgtype.UUID{Bytes: userID, Valid: true}})
	return err
}
