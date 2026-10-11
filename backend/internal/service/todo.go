package service

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
)

var (
	ErrInvalidTitle = errors.New("title must not be empty")
	ErrInvalidPage  = errors.New("invalid pagination")
	ErrTodoNotFound = errors.New("todo not found")
)

const (
	defaultTodoLimit int32 = 100
	maxTodoLimit     int32 = 1000
)

type TodoRepository interface {
	Create(context.Context, string) (sqlc.Todo, error)
	List(context.Context, string, int32, int32) ([]sqlc.Todo, error)
	Update(context.Context, int64, string) (sqlc.Todo, error)
	Delete(context.Context, int64) error
}

type TodoService struct {
	repository TodoRepository
}

func NewTodoService(repository TodoRepository) *TodoService {
	return &TodoService{repository: repository}
}

func (s *TodoService) Create(ctx context.Context, title string) error {
	if strings.TrimSpace(title) == "" {
		return ErrInvalidTitle
	}
	_, err := s.repository.Create(ctx, title)
	return err
}

func (s *TodoService) List(ctx context.Context, title string, limit, offset int32) ([]sqlc.Todo, error) {
	if limit == 0 {
		limit = defaultTodoLimit
	}
	if limit < 1 || limit > maxTodoLimit || offset < 0 {
		return nil, ErrInvalidPage
	}

	return s.repository.List(ctx, title, limit, offset)
}

func (s *TodoService) Update(ctx context.Context, id int64, title string) error {
	if strings.TrimSpace(title) == "" {
		return ErrInvalidTitle
	}
	_, err := s.repository.Update(ctx, id, title)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTodoNotFound
	}
	return err
}

func (s *TodoService) Delete(ctx context.Context, id int64) error {
	err := s.repository.Delete(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTodoNotFound
	}
	return err
}
