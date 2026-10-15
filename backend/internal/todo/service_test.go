package todo

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
)

type fakeRepository struct {
	createFn func(context.Context, [16]byte, string, string) (sqlc.Todo, error)
	listFn   func(context.Context, [16]byte, string, int32, int32) ([]sqlc.Todo, error)
	updateFn func(context.Context, [16]byte, int64, string, string) (sqlc.Todo, error)
	deleteFn func(context.Context, [16]byte, int64) error
	moveFn   func(context.Context, [16]byte, int64, *int64) error
}

func (f *fakeRepository) Create(ctx context.Context, userID [16]byte, title, status string) (sqlc.Todo, error) {
	return f.createFn(ctx, userID, title, status)
}

func (f *fakeRepository) List(ctx context.Context, userID [16]byte, title string, limit, offset int32) ([]sqlc.Todo, error) {
	return f.listFn(ctx, userID, title, limit, offset)
}

func (f *fakeRepository) Update(ctx context.Context, userID [16]byte, id int64, title, status string) (sqlc.Todo, error) {
	return f.updateFn(ctx, userID, id, title, status)
}

func (f *fakeRepository) Delete(ctx context.Context, userID [16]byte, id int64) error {
	return f.deleteFn(ctx, userID, id)
}

func (f *fakeRepository) Move(ctx context.Context, userID [16]byte, id int64, beforeID *int64) error {
	if f.moveFn == nil {
		return nil
	}
	return f.moveFn(ctx, userID, id, beforeID)
}

func TestServiceMove(t *testing.T) {
	service := NewService(&fakeRepository{moveFn: func(_ context.Context, userID [16]byte, id int64, beforeID *int64) error {
		if userID != testUserID || id != 3 || beforeID == nil || *beforeID != 2 {
			t.Fatalf("move args: %v %d %v", userID, id, beforeID)
		}
		return pgx.ErrNoRows
	}})
	anchor := int64(2)
	if err := service.Move(context.Background(), testUserID, 3, &anchor); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
	if err := service.Move(context.Background(), testUserID, 0, nil); !errors.Is(err, ErrInvalidPosition) {
		t.Fatalf("err=%v", err)
	}
}

func TestServiceCreate(t *testing.T) {
	t.Run("rejects invalid status", func(t *testing.T) {
		repository := &fakeRepository{createFn: func(context.Context, [16]byte, string, string) (sqlc.Todo, error) {
			t.Fatal("repository should not be called")
			return sqlc.Todo{}, nil
		}}
		if _, err := NewService(repository).Create(context.Background(), testUserID, "todo", "invalid"); !errors.Is(err, ErrInvalidStatus) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("rejects blank title", func(t *testing.T) {
		called := false
		repository := &fakeRepository{createFn: func(context.Context, [16]byte, string, string) (sqlc.Todo, error) {
			called = true
			return sqlc.Todo{}, nil
		}}

		_, err := NewService(repository).Create(context.Background(), testUserID, "   ", "")
		if !errors.Is(err, ErrInvalidTitle) || called {
			t.Fatalf("err=%v called=%v", err, called)
		}
	})

	t.Run("passes title to repository", func(t *testing.T) {
		var got string
		repository := &fakeRepository{createFn: func(_ context.Context, userID [16]byte, title, status string) (sqlc.Todo, error) {
			if status != StatusNotStarted {
				t.Fatalf("status=%q", status)
			}
			if userID != testUserID {
				t.Fatalf("user=%v", userID)
			}
			got = title
			return sqlc.Todo{}, nil
		}}

		if _, err := NewService(repository).Create(context.Background(), testUserID, "todo", ""); err != nil {
			t.Fatal(err)
		}
		if got != "todo" {
			t.Fatalf("title=%q", got)
		}
	})
}

func TestServiceList(t *testing.T) {
	t.Run("uses default limit", func(t *testing.T) {
		var gotLimit, gotOffset int32
		repository := &fakeRepository{listFn: func(_ context.Context, userID [16]byte, _ string, limit, offset int32) ([]sqlc.Todo, error) {
			if userID != testUserID {
				t.Fatalf("user=%v", userID)
			}
			gotLimit, gotOffset = limit, offset
			return nil, nil
		}}

		if _, err := NewService(repository).List(context.Background(), testUserID, "", 0, 0); err != nil {
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
			repository := &fakeRepository{listFn: func(context.Context, [16]byte, string, int32, int32) ([]sqlc.Todo, error) {
				t.Fatal("repository should not be called")
				return nil, nil
			}}
			_, err := NewService(repository).List(context.Background(), testUserID, "", test.limit, test.offset)
			if !errors.Is(err, ErrInvalidPage) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestServiceUpdateAndDelete(t *testing.T) {
	t.Run("passes status to repository", func(t *testing.T) {
		repository := &fakeRepository{updateFn: func(_ context.Context, _ [16]byte, _ int64, _, status string) (sqlc.Todo, error) {
			if status != StatusInProgress {
				t.Fatalf("status=%q", status)
			}
			return sqlc.Todo{}, nil
		}}
		if _, err := NewService(repository).Update(context.Background(), testUserID, 1, "todo", StatusInProgress); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("rejects invalid status", func(t *testing.T) {
		repository := &fakeRepository{updateFn: func(context.Context, [16]byte, int64, string, string) (sqlc.Todo, error) {
			t.Fatal("repository should not be called")
			return sqlc.Todo{}, nil
		}}
		if _, err := NewService(repository).Update(context.Background(), testUserID, 1, "todo", "invalid"); !errors.Is(err, ErrInvalidStatus) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("maps no rows to not found", func(t *testing.T) {
		repository := &fakeRepository{
			updateFn: func(context.Context, [16]byte, int64, string, string) (sqlc.Todo, error) {
				return sqlc.Todo{}, pgx.ErrNoRows
			},
			deleteFn: func(context.Context, [16]byte, int64) error { return pgx.ErrNoRows },
		}
		service := NewService(repository)
		if _, err := service.Update(context.Background(), testUserID, 1, "todo", ""); !errors.Is(err, ErrNotFound) {
			t.Fatalf("update err=%v", err)
		}
		if err := service.Delete(context.Background(), testUserID, 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("delete err=%v", err)
		}
	})

	t.Run("rejects blank update title", func(t *testing.T) {
		called := false
		repository := &fakeRepository{updateFn: func(context.Context, [16]byte, int64, string, string) (sqlc.Todo, error) {
			called = true
			return sqlc.Todo{}, nil
		}}
		if _, err := NewService(repository).Update(context.Background(), testUserID, 1, " ", ""); !errors.Is(err, ErrInvalidTitle) || called {
			t.Fatalf("err=%v called=%v", err, called)
		}
	})
}
