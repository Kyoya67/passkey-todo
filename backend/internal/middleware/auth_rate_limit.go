package middleware

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

const rateWindowSeconds = 60

// This script ensures the increment and initial expiry are atomic.
var authRateScript = redis.NewScript(`local count = redis.call("INCR", KEYS[1])
if count == 1 then redis.call("EXPIRE", KEYS[1], ARGV[1]) end
return count`)

type authRateCounter interface {
	Run(context.Context, *redis.Client, []string, ...interface{}) *redis.Cmd
}

// AuthRateLimit limits sensitive unauthenticated auth routes by client IP.
// The app must only be reachable through an ALB using the default append
// X-Forwarded-For mode; its final XFF value is added by the trusted ALB.
func AuthRateLimit(client *redis.Client, localDevelopment bool) echo.MiddlewareFunc {
	return authRateLimit(client, authRateScript, localDevelopment)
}

func authRateLimit(client *redis.Client, script authRateCounter, localDevelopment bool) echo.MiddlewareFunc {
	limits := map[string]int64{
		"/auth/registration/begin":  10,
		"/auth/registration/finish": 20,
		"/auth/login/begin":         20,
		"/auth/login/finish":        20,
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Method != http.MethodPost {
				return next(c)
			}
			max, ok := limits[c.Request().URL.Path]
			if !ok {
				return next(c)
			}
			ip := albClientIP(c.Request())
			if ip == "" && localDevelopment {
				host, _, splitErr := net.SplitHostPort(c.Request().RemoteAddr)
				if splitErr == nil {
					if parsed := net.ParseIP(host); parsed != nil { ip = parsed.String() }
				}
			}
			if ip == "" {
				return c.NoContent(http.StatusServiceUnavailable)
			}
			// Distinct fixed route buckets prevent a flood of one route
			// from consuming the allowance of another.
			key := "auth_rate:" + c.Request().URL.Path + ":" + ip
			count, err := script.Run(c.Request().Context(), client, []string{key}, rateWindowSeconds).Int64()
			if err != nil {
				return c.NoContent(http.StatusServiceUnavailable)
			}
			if count > max {
				c.Response().Header().Set("Retry-After", strconv.Itoa(rateWindowSeconds))
				return c.NoContent(http.StatusTooManyRequests)
			}
			return next(c)
		}
	}
}

func albClientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return ""
	}
	parts := strings.Split(xff, ",")
	last := strings.TrimSpace(parts[len(parts)-1])
	if ip := net.ParseIP(last); ip != nil {
		return ip.String()
	}
	return ""
}
