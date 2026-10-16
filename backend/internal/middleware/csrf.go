package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// RequireTodoOrigin rejects cross-origin state-changing requests to TODO routes.
// CORS alone does not prevent requests from being sent.
func RequireTodoOrigin(allowedOrigins []string) echo.MiddlewareFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			method := c.Request().Method
			path := c.Request().URL.Path
			if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions ||
				(path != "/todos" && !strings.HasPrefix(path, "/todos/")) {
				return next(c)
			}
			if _, ok := allowed[c.Request().Header.Get(echo.HeaderOrigin)]; !ok {
				return c.NoContent(http.StatusForbidden)
			}
			return next(c)
		}
	}
}
