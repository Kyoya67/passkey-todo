package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

type fakeAuthRateScript struct {
	count int64
	err error
	keys []string
}
func (f *fakeAuthRateScript) Run(_ context.Context, _ *redis.Client, keys []string, _ ...interface{}) *redis.Cmd {
	f.keys = keys
	cmd := redis.NewCmd(context.Background())
	if f.err != nil { cmd.SetErr(f.err) } else { cmd.SetVal(f.count) }
	return cmd
}

func TestAuthRateLimit(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, xff string
		count int64
		scriptErr bool
		want int
	}{
		{"registration begin under limit", "POST", "/auth/registration/begin", "203.0.113.2", 10, false, 204},
		{"registration begin over limit", "POST", "/auth/registration/begin", "203.0.113.2", 11, false, 429},
		{"login begin allowed", "POST", "/auth/login/begin", "203.0.113.2", 20, false, 204},
		{"login begin denied", "POST", "/auth/login/begin", "203.0.113.2", 21, false, 429},
		{"registration finish denied", "POST", "/auth/registration/finish", "203.0.113.2", 21, false, 429},
		{"login finish denied", "POST", "/auth/login/finish", "203.0.113.2", 21, false, 429},
		{"logout not limited", "POST", "/auth/logout", "", 999, false, 204},
		{"get not limited", "GET", "/auth/login/begin", "", 999, false, 204},
		{"missing alb IP", "POST", "/auth/login/begin", "", 1, false, 503},
		{"invalid alb IP", "POST", "/auth/login/begin", "fake", 1, false, 503},
		{"redis outage", "POST", "/auth/login/begin", "203.0.113.2", 1, true, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			fake := &fakeAuthRateScript{count: tc.count}
			if tc.scriptErr { fake.err = errors.New("redis down") }
			e.Use(authRateLimit(nil, fake))
			e.Add(tc.method, tc.path, func(c echo.Context) error { return c.NoContent(204) })
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.xff != "" { req.Header.Set("X-Forwarded-For", tc.xff) }
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.want { t.Fatalf("got %d, want %d", rec.Code, tc.want) }
			if rec.Code == 429 && rec.Header().Get("Retry-After") != "60" {
				t.Fatal("Retry-After not set")
			}
		})
	}
}

func TestALBClientIPUsesLastForwardedValue(t *testing.T) {
	req := httptest.NewRequest("POST", "/auth/login/begin", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 203.0.113.99")
	if got := albClientIP(req); got != "203.0.113.99" {
		t.Fatalf("untrusted leftmost address was used: %q", got)
	}
}
