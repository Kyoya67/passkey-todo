package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/catechlounge/kyoya67-todo/backend/internal/auth"
)

type sessionLoader interface {
	Load(context.Context, string) ([16]byte, error)
}

type userIDContextKey struct{}

// UserID returns the user authenticated for this request.
func UserID(ctx context.Context) ([16]byte, bool) {
	id, ok := ctx.Value(userIDContextKey{}).([16]byte)
	return id, ok
}

// RequireTodoSession protects only the TODO routes. Auth and health remain public.
func RequireTodoSession(sessions sessionLoader) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			if c.Request().Method == http.MethodOptions || (path != "/todos" && !strings.HasPrefix(path, "/todos/")) {
				return next(c)
			}
			cookie, err := c.Cookie("todo_session")
			if err != nil || cookie.Value == "" {
				return c.NoContent(http.StatusUnauthorized)
			}
			id, err := sessions.Load(c.Request().Context(), cookie.Value)
			if errors.Is(err, auth.ErrSessionNotFound) {
				return c.NoContent(http.StatusUnauthorized)
			}
			if err != nil {
				c.Logger().Error("todo authentication failed: stage=session_store")
				return c.NoContent(http.StatusServiceUnavailable)
			}
			c.SetRequest(c.Request().WithContext(context.WithValue(c.Request().Context(), userIDContextKey{}, id)))
			return next(c)
		}
	}
}
