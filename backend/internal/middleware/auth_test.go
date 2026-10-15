package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/catechlounge/kyoya67-todo/backend/internal/auth"
)

type fakeSessions struct {
	wantToken string
	userID    [16]byte
	err       error
	calls     int
}

func (s *fakeSessions) Load(_ context.Context, token string) ([16]byte, error) {
	s.calls++
	if token != s.wantToken {
		return [16]byte{}, auth.ErrSessionNotFound
	}
	return s.userID, s.err
}

func TestRequireTodoSession(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, cookie string
		storeErr                   error
		status                     int
		storeCalls, handlerCalls   int
	}{
		{"anonymous list", http.MethodGet, "/todos", "", nil, 401, 0, 0},
		{"anonymous create", http.MethodPost, "/todos", "", nil, 401, 0, 0},
		{"anonymous update", http.MethodPut, "/todos/1", "", nil, 401, 0, 0},
		{"anonymous delete", http.MethodDelete, "/todos/1", "", nil, 401, 0, 0},
		{"invalid session", http.MethodGet, "/todos", "wrong", nil, 401, 1, 0},
		{"expired session", http.MethodGet, "/todos", "valid", auth.ErrSessionNotFound, 401, 1, 0},
		{"redis failure", http.MethodGet, "/todos", "valid", errors.New("private redis error"), 503, 1, 0},
		{"authenticated list", http.MethodGet, "/todos", "valid", nil, 204, 1, 1},
		{"authenticated create", http.MethodPost, "/todos", "valid", nil, 204, 1, 1},
		{"authenticated update", http.MethodPut, "/todos/1", "valid", nil, 204, 1, 1},
		{"authenticated delete", http.MethodDelete, "/todos/1", "valid", nil, 204, 1, 1},
		{"health public", http.MethodGet, "/health", "", nil, 204, 0, 1},
		{"registration public", http.MethodPost, "/auth/registration/begin", "", nil, 204, 0, 1},
		{"preflight public", http.MethodOptions, "/todos", "", nil, 204, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeSessions{wantToken: "valid", userID: [16]byte{1}, err: tc.storeErr}
			e := echo.New()
			e.Use(RequireTodoSession(store))
			handlerCalls := 0
			e.Add(tc.method, tc.path, func(c echo.Context) error {
				handlerCalls++
				if tc.storeCalls > 0 && tc.handlerCalls > 0 {
					if id, ok := UserID(c.Request().Context()); !ok || id != store.userID {
						t.Fatal("authenticated user ID missing")
					}
				}
				return c.NoContent(http.StatusNoContent)
			})
			request := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.cookie != "" {
				request.AddCookie(&http.Cookie{Name: "todo_session", Value: tc.cookie})
			}
			response := httptest.NewRecorder()
			e.ServeHTTP(response, request)
			if response.Code != tc.status || store.calls != tc.storeCalls || handlerCalls != tc.handlerCalls {
				t.Fatalf("status=%d store=%d handler=%d, want %d/%d/%d", response.Code, store.calls, handlerCalls, tc.status, tc.storeCalls, tc.handlerCalls)
			}
		})
	}
}

func TestRequireTodoSessionWithCORS(t *testing.T) {
	e := echo.New()
	store := &fakeSessions{wantToken: "valid", userID: [16]byte{1}}
	e.Use(CORS([]string{"http://localhost:3000"}))
	e.Use(RequireTodoSession(store))
	e.GET("/todos", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	for _, tc := range []struct {
		method string
		want   int
	}{
		{http.MethodOptions, http.StatusNoContent},
		{http.MethodGet, http.StatusUnauthorized},
	} {
		request := httptest.NewRequest(tc.method, "/todos", nil)
		request.Header.Set(echo.HeaderOrigin, "http://localhost:3000")
		if tc.method == http.MethodOptions {
			request.Header.Set(echo.HeaderAccessControlRequestMethod, http.MethodGet)
		}
		response := httptest.NewRecorder()
		e.ServeHTTP(response, request)
		if response.Code != tc.want || response.Header().Get(echo.HeaderAccessControlAllowOrigin) != "http://localhost:3000" {
			t.Fatalf("%s: status=%d origin=%q", tc.method, response.Code, response.Header().Get(echo.HeaderAccessControlAllowOrigin))
		}
	}
	if store.calls != 0 {
		t.Fatalf("unexpected session lookup: %d", store.calls)
	}
}
