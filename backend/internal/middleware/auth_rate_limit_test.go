package middleware

import (
	"context"
	"crypto/rand"
	"errors"
	"net"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

type fakeAuthRateScript struct {
	count int64
	err   error
	keys  []string
}

func (f *fakeAuthRateScript) Run(_ context.Context, _ redis.Scripter, keys []string, _ ...interface{}) *redis.Cmd {
	f.keys = keys
	cmd := redis.NewCmd(context.Background())
	if f.err != nil {
		cmd.SetErr(f.err)
	} else {
		cmd.SetVal(f.count)
	}
	return cmd
}

func TestAuthRateLimit(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, xff string
		count     int64
		scriptErr bool
		want      int
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
			if tc.scriptErr {
				fake.err = errors.New("redis down")
			}
			e.Use(authRateLimit(nil, fake, false))
			e.Add(tc.method, tc.path, func(c echo.Context) error { return c.NoContent(204) })
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d, want %d", rec.Code, tc.want)
			}
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

func TestAuthRateLimitRedisIntegration(t *testing.T) {
	address := os.Getenv("REDIS_TEST_ADDR")
	if address == "" {
		t.Skip("REDIS_TEST_ADDR is not set")
	}
	client := redis.NewClient(&redis.Options{Addr: address})
	defer client.Close()
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}

	ipBytes := make([]byte, net.IPv6len)
	if _, err := rand.Read(ipBytes); err != nil {
		t.Fatal(err)
	}
	ip := net.IP(ipBytes).String()
	key := "auth_rate:/auth/registration/begin:" + ip
	t.Cleanup(func() { _ = client.Del(context.Background(), key).Err() })

	e := echo.New()
	e.Use(AuthRateLimit(client, false))
	e.POST("/auth/registration/begin", func(c echo.Context) error { return c.NoContent(204) })
	for attempt := 1; attempt <= 11; attempt++ {
		req := httptest.NewRequest("POST", "/auth/registration/begin", nil)
		req.Header.Set("X-Forwarded-For", ip)
		res := httptest.NewRecorder()
		e.ServeHTTP(res, req)
		want := 204
		if attempt == 11 {
			want = 429
		}
		if res.Code != want {
			t.Fatalf("attempt %d: got %d, want %d", attempt, res.Code, want)
		}
	}
	if ttl := client.TTL(context.Background(), key).Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("unexpected TTL: %s", ttl)
	}
}
