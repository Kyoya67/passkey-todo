package todo

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
)

var (
	ErrInvalidTitle         = errors.New("title must not be empty")
	ErrInvalidStatus        = errors.New("invalid todo status")
	ErrInvalidPriorityLevel = errors.New("invalid todo priority level")
	ErrInvalidPage          = errors.New("invalid pagination")
	ErrInvalidPosition      = errors.New("invalid todo position")
	ErrNotFound             = errors.New("todo not found")
)

const (
	defaultTodoLimit int32 = 100
	maxTodoLimit     int32 = 1000
	StatusNotStarted       = "not_started"
	StatusInProgress       = "in_progress"
	StatusCompleted        = "completed"
	PriorityHigh           = "high"
	PriorityMedium         = "medium"
	PriorityLow            = "low"
)

type todoRepository interface {
	Create(context.Context, [16]byte, string, string, string) (sqlc.Todo, error)
	List(context.Context, [16]byte, string, int32, int32) ([]sqlc.Todo, error)
	Update(context.Context, [16]byte, int64, string, string, string) (sqlc.Todo, error)
	Delete(context.Context, [16]byte, int64) error
	Move(context.Context, [16]byte, int64, *int64) error
}

type Service struct {
	repository todoRepository
}

func NewService(repository todoRepository) *Service {
	return &Service{repository: repository}
}

func normalizeStatus(status string) (string, error) {
	if status == "" {
		return StatusNotStarted, nil
	}
	switch status {
	case StatusNotStarted, StatusInProgress, StatusCompleted:
		return status, nil
	default:
		return "", ErrInvalidStatus
	}
}

func normalizePriorityLevel(level string) (string, error) {
	if level == "" {
		return PriorityMedium, nil
	}
	switch level {
	case PriorityHigh, PriorityMedium, PriorityLow:
		return level, nil
	default:
		return "", ErrInvalidPriorityLevel
	}
}

func (s *Service) Create(ctx context.Context, userID [16]byte, title, status, level string) (sqlc.Todo, error) {
	if strings.TrimSpace(title) == "" {
		return sqlc.Todo{}, ErrInvalidTitle
	}
	status, err := normalizeStatus(status)
	if err != nil {
		return sqlc.Todo{}, err
	}
	level, err = normalizePriorityLevel(level)
	if err != nil {
		return sqlc.Todo{}, err
	}
	return s.repository.Create(ctx, userID, title, status, level)
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

func (s *Service) Update(ctx context.Context, userID [16]byte, id int64, title, status, level string) (sqlc.Todo, error) {
	if strings.TrimSpace(title) == "" {
		return sqlc.Todo{}, ErrInvalidTitle
	}
	if status != "" {
		if _, err := normalizeStatus(status); err != nil {
			return sqlc.Todo{}, err
		}
	}
	if level != "" {
		if _, err := normalizePriorityLevel(level); err != nil {
			return sqlc.Todo{}, err
		}
	}
	todo, err := s.repository.Update(ctx, userID, id, title, status, level)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Todo{}, ErrNotFound
	}
	return todo, err
}

func (s *Service) Delete(ctx context.Context, userID [16]byte, id int64) error {
	err := s.repository.Delete(ctx, userID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Service) Move(ctx context.Context, userID [16]byte, id int64, beforeID *int64) error {
	if id < 1 || (beforeID != nil && *beforeID < 1) {
		return ErrInvalidPosition
	}
	err := s.repository.Move(ctx, userID, id, beforeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
