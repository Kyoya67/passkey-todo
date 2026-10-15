package todo

import (
	"bytes"
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
	"github.com/catechlounge/kyoya67-todo/backend/internal/middleware"
	"github.com/catechlounge/kyoya67-todo/backend/internal/openapi"
)

var testUserID = [16]byte{1}

type fakeService struct {
	createFn func(context.Context, [16]byte, string) error
	listFn   func(context.Context, [16]byte, string, int32, int32) ([]sqlc.Todo, error)
	updateFn func(context.Context, [16]byte, int64, string) error
	deleteFn func(context.Context, [16]byte, int64) error
}

func (f *fakeService) Create(ctx context.Context, userID [16]byte, title string) error {
	return f.createFn(ctx, userID, title)
}

func (f *fakeService) List(ctx context.Context, userID [16]byte, title string, limit, offset int32) ([]sqlc.Todo, error) {
	return f.listFn(ctx, userID, title, limit, offset)
}

func (f *fakeService) Update(ctx context.Context, userID [16]byte, id int64, title string) error {
	return f.updateFn(ctx, userID, id, title)
}

func (f *fakeService) Delete(ctx context.Context, userID [16]byte, id int64) error {
	return f.deleteFn(ctx, userID, id)
}

func newHandlerContext(method, target, body string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request = request.WithContext(middleware.WithUserID(request.Context(), testUserID))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	response := httptest.NewRecorder()
	return e.NewContext(request, response), response
}

func TestHandlerGetHealth(t *testing.T) {
	handler := NewHandler(&fakeService{})
	ctx, response := newHandlerContext(http.MethodGet, "/health", "")

	if err := handler.GetHealth(ctx); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || response.Body.String() != `{"status":"ok"}
` {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestHandlerRejectsMissingAuthenticatedUser(t *testing.T) {
	handler := NewHandler(&fakeService{})
	for _, tc := range []struct {
		method, target, body string
		call                 func(echo.Context) error
	}{
		{http.MethodPost, "/todos", `{"title":"todo"}`, handler.CreateTodo},
		{http.MethodGet, "/todos", "", func(c echo.Context) error { return handler.ListTodos(c, openapi.ListTodosParams{}) }},
		{http.MethodPut, "/todos/1", `{"title":"todo"}`, func(c echo.Context) error { return handler.UpdateTodo(c, 1) }},
		{http.MethodDelete, "/todos/1", "", func(c echo.Context) error { return handler.DeleteTodo(c, 1) }},
	} {
		ctx, response := newHandlerContext(tc.method, tc.target, tc.body)
		ctx.SetRequest(ctx.Request().WithContext(context.Background()))
		if err := tc.call(ctx); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status=%d", tc.method, response.Code)
		}
	}
}

func TestHandlerCreate(t *testing.T) {
	t.Run("returns bad request for invalid JSON", func(t *testing.T) {
		handler := NewHandler(&fakeService{createFn: func(context.Context, [16]byte, string) error { return nil }})
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
			handler := NewHandler(&fakeService{createFn: func(_ context.Context, userID [16]byte, title string) error {
				if userID != testUserID || title != "todo" {
					t.Fatalf("user=%v title=%q", userID, title)
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

func TestHandlerList(t *testing.T) {
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	handler := NewHandler(&fakeService{listFn: func(_ context.Context, userID [16]byte, title string, limit, offset int32) ([]sqlc.Todo, error) {
		if userID != testUserID || title != "search" || limit != 10 || offset != 2 {
			t.Fatalf("user=%v title=%q limit=%d offset=%d", userID, title, limit, offset)
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

func TestHandlerUpdateAndDelete(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "invalid title", err: ErrInvalidTitle, want: http.StatusBadRequest},
		{name: "not found", err: ErrNotFound, want: http.StatusNotFound},
		{name: "service error", err: errors.New("failed"), want: http.StatusServiceUnavailable},
		{name: "success", err: nil, want: http.StatusNoContent},
	} {
		t.Run("update "+test.name, func(t *testing.T) {
			handler := NewHandler(&fakeService{updateFn: func(_ context.Context, userID [16]byte, id int64, title string) error {
				if userID != testUserID || id != 1 || title != "updated" {
					t.Fatalf("user=%v id=%d title=%q", userID, id, title)
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
			handler := NewHandler(&fakeService{deleteFn: func(_ context.Context, userID [16]byte, id int64) error {
				if userID != testUserID || id != 1 {
					t.Fatalf("user=%v id=%d", userID, id)
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

func TestHandlerServiceFailureLogging(t *testing.T) {
	privateErr := errors.New("private SQL detail")
	service := &fakeService{
		createFn: func(context.Context, [16]byte, string) error { return privateErr },
		listFn: func(context.Context, [16]byte, string, int32, int32) ([]sqlc.Todo, error) {
			return nil, privateErr
		},
		updateFn: func(context.Context, [16]byte, int64, string) error { return privateErr },
		deleteFn: func(context.Context, [16]byte, int64) error { return privateErr },
	}
	handler := NewHandler(service)
	for _, tc := range []struct {
		operation, method, target, body string
		call                            func(echo.Context) error
	}{
		{"create", http.MethodPost, "/todos", `{"title":"private title"}`, handler.CreateTodo},
		{"list", http.MethodGet, "/todos", "", func(c echo.Context) error { return handler.ListTodos(c, openapi.ListTodosParams{}) }},
		{"update", http.MethodPut, "/todos/1", `{"title":"private title"}`, func(c echo.Context) error { return handler.UpdateTodo(c, 1) }},
		{"delete", http.MethodDelete, "/todos/1", "", func(c echo.Context) error { return handler.DeleteTodo(c, 1) }},
	} {
		t.Run(tc.operation, func(t *testing.T) {
			ctx, response := newHandlerContext(tc.method, tc.target, tc.body)
			var logs bytes.Buffer
			ctx.Echo().Logger.SetOutput(&logs)
			if err := tc.call(ctx); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusServiceUnavailable || response.Body.Len() != 0 {
				t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
			}
			if !strings.Contains(logs.String(), "operation="+tc.operation+" stage=service") {
				t.Fatalf("missing diagnostic label: %q", logs.String())
			}
			if strings.Contains(logs.String(), "private") {
				t.Fatalf("sensitive data leaked to logs: %q", logs.String())
			}
		})
	}
}
