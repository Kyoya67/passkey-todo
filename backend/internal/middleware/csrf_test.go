package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestRequireTodoOrigin(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, origin string
		want                      int
	}{
		{"allowed create", http.MethodPost, "/todos", "https://ca-techlounge-todo.kyo8.dev", 204},
		{"allowed update", http.MethodPut, "/todos/1", "http://localhost:3000", 204},
		{"allowed delete", http.MethodDelete, "/todos/1", "https://ca-techlounge-todo.kyo8.dev", 204},
		{"allowed position", http.MethodPut, "/todos/1/position", "https://ca-techlounge-todo.kyo8.dev", 204},
		{"other origin", http.MethodPost, "/todos", "https://evil.kyo8.dev", 403},
		{"missing origin", http.MethodDelete, "/todos/1", "", 403},
		{"null origin", http.MethodPut, "/todos/1", "null", 403},
		{"similar origin", http.MethodPost, "/todos", "https://ca-techlounge-todo.kyo8.dev.evil.com", 403},
		{"read without origin", http.MethodGet, "/todos", "", 204},
		{"preflight", http.MethodOptions, "/todos", "https://evil.kyo8.dev", 204},
		{"auth untouched", http.MethodPost, "/auth/login/begin", "", 204},
		{"health untouched", http.MethodGet, "/health", "", 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			e.Use(RequireTodoOrigin([]string{"https://ca-techlounge-todo.kyo8.dev", "http://localhost:3000"}))
			e.Add(tc.method, tc.path, func(c echo.Context) error {
				return c.NoContent(http.StatusNoContent)
			})
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.origin != "" {
				req.Header.Set(echo.HeaderOrigin, tc.origin)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status=%d, want %d", rec.Code, tc.want)
			}
		})
	}
}
