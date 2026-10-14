package todo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
	"github.com/catechlounge/kyoya67-todo/backend/internal/openapi"
)

type fakeTodoService struct {
	createFn func(context.Context, string) error
	listFn   func(context.Context, string, int32, int32) ([]sqlc.Todo, error)
	updateFn func(context.Context, int64, string) error
	deleteFn func(context.Context, int64) error
}

func (f *fakeTodoService) Create(ctx context.Context, title string) error {
	return f.createFn(ctx, title)
}

func (f *fakeTodoService) List(ctx context.Context, title string, limit, offset int32) ([]sqlc.Todo, error) {
	return f.listFn(ctx, title, limit, offset)
}

func (f *fakeTodoService) Update(ctx context.Context, id int64, title string) error {
	return f.updateFn(ctx, id, title)
}

func (f *fakeTodoService) Delete(ctx context.Context, id int64) error {
	return f.deleteFn(ctx, id)
}

func newHandlerContext(method, target, body string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	response := httptest.NewRecorder()
	return e.NewContext(request, response), response
}

func TestTodoHandlerGetHealth(t *testing.T) {
	handler := NewTodoHandler(&fakeTodoService{})
	ctx, response := newHandlerContext(http.MethodGet, "/health", "")

	if err := handler.GetHealth(ctx); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || response.Body.String() != `{"status":"ok"}
` {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestTodoHandlerCreate(t *testing.T) {
	t.Run("returns bad request for invalid JSON", func(t *testing.T) {
		handler := NewTodoHandler(&fakeTodoService{createFn: func(context.Context, string) error { return nil }})
		ctx, response := newHandlerContext(http.MethodPost, "/todos", "{")
		if err := handler.CreateTodo(ctx); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", response.Code)
		}
	})

	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "invalid title", err: ErrInvalidTitle, want: http.StatusBadRequest},
		{name: "service error", err: errors.New("failed"), want: http.StatusServiceUnavailable},
		{name: "success", err: nil, want: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := NewTodoHandler(&fakeTodoService{createFn: func(_ context.Context, title string) error {
				if title != "todo" {
					t.Fatalf("title=%q", title)
				}
				return test.err
			}})
			ctx, response := newHandlerContext(http.MethodPost, "/todos", `{"title":"todo"}`)
			if err := handler.CreateTodo(ctx); err != nil {
				t.Fatal(err)
			}
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d", response.Code, test.want)
			}
		})
	}
}

func TestTodoHandlerList(t *testing.T) {
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	handler := NewTodoHandler(&fakeTodoService{listFn: func(_ context.Context, title string, limit, offset int32) ([]sqlc.Todo, error) {
		if title != "search" || limit != 10 || offset != 2 {
			t.Fatalf("title=%q limit=%d offset=%d", title, limit, offset)
		}
		return []sqlc.Todo{{ID: 1, Title: "todo", CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}, UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true}}}, nil
	}})
	title, limit, offset := "search", int32(10), int32(2)
	ctx, response := newHandlerContext(http.MethodGet, "/todos", "")
	if err := handler.ListTodos(ctx, openapi.ListTodosParams{Title: &title, Limit: &limit, Offset: &offset}); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d", response.Code)
	}
	var body openapi.TodoListResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Todos) != 1 || body.Todos[0].Id != 1 || body.Todos[0].Title != "todo" || !body.Todos[0].UpdatedAt.Equal(now) {
		t.Fatalf("body=%+v", body)
	}
}

func TestTodoHandlerUpdateAndDelete(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "invalid title", err: ErrInvalidTitle, want: http.StatusBadRequest},
		{name: "not found", err: ErrTodoNotFound, want: http.StatusNotFound},
		{name: "service error", err: errors.New("failed"), want: http.StatusServiceUnavailable},
		{name: "success", err: nil, want: http.StatusNoContent},
	} {
		t.Run("update "+test.name, func(t *testing.T) {
			handler := NewTodoHandler(&fakeTodoService{updateFn: func(_ context.Context, id int64, title string) error {
				if id != 1 || title != "updated" {
					t.Fatalf("id=%d title=%q", id, title)
				}
				return test.err
			}})
			ctx, response := newHandlerContext(http.MethodPut, "/todos/1", `{"title":"updated"}`)
			if err := handler.UpdateTodo(ctx, 1); err != nil {
				t.Fatal(err)
			}
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d", response.Code, test.want)
			}
		})

		t.Run("delete "+test.name, func(t *testing.T) {
			handler := NewTodoHandler(&fakeTodoService{deleteFn: func(_ context.Context, id int64) error {
				if id != 1 {
					t.Fatalf("id=%d", id)
				}
				return test.err
			}})
			ctx, response := newHandlerContext(http.MethodDelete, "/todos/1", "")
			if err := handler.DeleteTodo(ctx, 1); err != nil {
				t.Fatal(err)
			}
			if response.Code != test.want && !(test.name == "invalid title" && response.Code == http.StatusServiceUnavailable) {
				t.Fatalf("status=%d want=%d", response.Code, test.want)
			}
		})
	}
}
