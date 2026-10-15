package todo

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
	ErrNotFound     = errors.New("todo not found")
)

const (
	defaultTodoLimit int32 = 100
	maxTodoLimit     int32 = 1000
)

type todoRepository interface {
	Create(context.Context, [16]byte, string) (sqlc.Todo, error)
	List(context.Context, [16]byte, string, int32, int32) ([]sqlc.Todo, error)
	Update(context.Context, [16]byte, int64, string) (sqlc.Todo, error)
	Delete(context.Context, [16]byte, int64) error
}

type Service struct {
	repository todoRepository
}

func NewService(repository todoRepository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Create(ctx context.Context, userID [16]byte, title string) error {
	if strings.TrimSpace(title) == "" {
		return ErrInvalidTitle
	}
	_, err := s.repository.Create(ctx, userID, title)
	return err
}

func (s *Service) List(ctx context.Context, userID [16]byte, title string, limit, offset int32) ([]sqlc.Todo, error) {
	if limit == 0 {
		limit = defaultTodoLimit
	}
	if limit < 1 || limit > maxTodoLimit || offset < 0 {
		return nil, ErrInvalidPage
	}

	return s.repository.List(ctx, userID, title, limit, offset)
}

func (s *Service) Update(ctx context.Context, userID [16]byte, id int64, title string) error {
	if strings.TrimSpace(title) == "" {
		return ErrInvalidTitle
	}
	_, err := s.repository.Update(ctx, userID, id, title)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Service) Delete(ctx context.Context, userID [16]byte, id int64) error {
	err := s.repository.Delete(ctx, userID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
