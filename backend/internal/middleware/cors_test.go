package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestCORSAllowsConfiguredCredentialOrigin(t *testing.T) {
	e := echo.New()
	e.Use(CORS([]string{"http://localhost:3000"}))
	e.POST("/auth/login/finish", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	for _, tc := range []struct{ origin, want string }{
		{"http://localhost:3000", "http://localhost:3000"},
		{"https://evil.example", ""},
	} {
		r := httptest.NewRequest(http.MethodOptions, "/auth/login/finish", nil)
		r.Header.Set(echo.HeaderOrigin, tc.origin)
		r.Header.Set(echo.HeaderAccessControlRequestMethod, http.MethodPost)
		res := httptest.NewRecorder()
		e.ServeHTTP(res, r)
		if got := res.Header().Get(echo.HeaderAccessControlAllowOrigin); got != tc.want {
			t.Fatalf("origin=%s allow=%s", tc.origin, got)
		}
		if tc.want != "" && res.Header().Get(echo.HeaderAccessControlAllowCredentials) != "true" {
			t.Fatal("credentials not allowed")
		}
	}
}
