package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
	"github.com/catechlounge/kyoya67-todo/backend/internal/openapi"
)

type fakeRegistrationService struct {
	err                     error
	beginCalls, finishCalls int
	token                   string
}

func (f *fakeRegistrationService) BeginRegistration(context.Context) (string, *protocol.CredentialCreation, error) {
	f.beginCalls++
	return "registration-token", &protocol.CredentialCreation{}, f.err
}
func (f *fakeRegistrationService) FinishRegistration(_ context.Context, token string, _ *protocol.ParsedCredentialCreationData) error {
	f.finishCalls++
	f.token = token
	return f.err
}

func TestRegistrationHandler(t *testing.T) {
	response := registrationResponse(t, "challenge", "http://localhost:3000", "localhost", 0x45)
	body, err := json.Marshal(map[string]any{"registrationId": "registration-token", "credential": response})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, endpoint, origin, contentType, body string
		err                                       error
		status, calls                             int
	}{
		{name: "begin", endpoint: "begin", origin: "http://localhost:3000", status: 200, calls: 1},
		{name: "begin internal error", endpoint: "begin", origin: "http://localhost:3000", err: errors.New("secret database password"), status: 503, calls: 1},
		{name: "missing origin", endpoint: "begin", status: 403},
		{name: "wrong origin", endpoint: "finish", origin: "https://evil.example", status: 403},
		{name: "finish", endpoint: "finish", origin: "http://localhost:3000", contentType: "application/json", body: string(body), status: 204, calls: 1},
		{name: "verification error", endpoint: "finish", origin: "http://localhost:3000", contentType: "application/json", body: string(body), err: ErrInvalidRegistration, status: 400, calls: 1},
		{name: "finish internal error", endpoint: "finish", origin: "http://localhost:3000", contentType: "application/json", body: string(body), err: errors.New("secret database password"), status: 503, calls: 1},
		{name: "wrong content type", endpoint: "finish", origin: "http://localhost:3000", contentType: "text/plain", body: string(body), status: 415},
		{name: "malformed JSON", endpoint: "finish", origin: "http://localhost:3000", contentType: "application/json", body: "{", status: 400},
		{name: "multiple JSON values", endpoint: "finish", origin: "http://localhost:3000", contentType: "application/json", body: string(body) + "{}", status: 400},
		{name: "missing registration ID", endpoint: "finish", origin: "http://localhost:3000", contentType: "application/json", body: "{}", status: 400},
		{name: "malformed credential", endpoint: "finish", origin: "http://localhost:3000", contentType: "application/json", body: `{"registrationId":"token","credential":{}}`, status: 400},
		{name: "oversized body", endpoint: "finish", origin: "http://localhost:3000", contentType: "application/json", body: `{"registrationId":"` + strings.Repeat("a", 65<<10) + `"}`, status: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeRegistrationService{err: tc.err}
			h := NewHandler(service, "http://localhost:3000")
			e := echo.New()
			e.POST("/begin", h.BeginRegistration)
			e.POST("/finish", h.FinishRegistration)
			r := httptest.NewRequest(http.MethodPost, "/"+tc.endpoint, strings.NewReader(tc.body))
			r.Header.Set(echo.HeaderOrigin, tc.origin)
			r.Header.Set(echo.HeaderContentType, tc.contentType)
			w := httptest.NewRecorder()
			e.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
			if service.beginCalls+service.finishCalls != tc.calls {
				t.Fatal("unexpected service calls")
			}
			if strings.Contains(w.Body.String(), "secret") {
				t.Fatal("internal error leaked")
			}
			if tc.name == "begin" && (w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Body.String(), "registration-token")) {
				t.Fatal("missing no-store or token")
			}
			if tc.name == "finish" && (service.token != "registration-token" || w.Body.Len() != 0) {
				t.Fatal("wrong token or nonempty response")
			}
		})
	}
}

func TestRegistrationDisabled(t *testing.T) {
	h := NewHandler(nil, "")
	e := echo.New()
	e.POST("/begin", h.BeginRegistration)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/begin", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", w.Code)
	}
}

func TestRegistrationHTTPIntegration(t *testing.T) {
	databaseURL := os.Getenv("INTEGRATION_DATABASE_URL")
	if databaseURL == "" || os.Getenv("REDIS_TEST_ADDR") == "" {
		t.Skip("INTEGRATION_DATABASE_URL and REDIS_TEST_ADDR are required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	w, err := NewWebAuthn("localhost", "http://localhost:3000")
	if err != nil {
		t.Fatal(err)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_TEST_ADDR")})
	defer redisClient.Close()
	h := NewHandler(NewService(w, NewRepository(pool), NewChallengeStore(redisClient)), "http://localhost:3000")
	e := echo.New()
	e.POST("/begin", h.BeginRegistration)
	e.POST("/finish", h.FinishRegistration)
	call := func(path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		r.Header.Set("Origin", "http://localhost:3000")
		r.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		e.ServeHTTP(res, r)
		return res
	}
	res := call("/begin", "")
	if res.Code != 200 {
		t.Fatalf("begin: %d %s", res.Code, res.Body.String())
	}
	var options openapi.RegistrationOptions
	if err := json.Unmarshal(res.Body.Bytes(), &options); err != nil {
		t.Fatal(err)
	}
	token, err := base64.RawURLEncoding.DecodeString(options.RegistrationId)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(token)
	defer func() {
		if err := redisClient.Del(ctx, registrationKey(hash)).Err(); err != nil {
			t.Error(err)
		}
	}()
	response := registrationResponse(t, options.Options.Response.Challenge.String(), "http://localhost:3000", "localhost", 0x45)
	defer func() {
		if _, err := pool.Exec(ctx, "DELETE FROM users WHERE id = (SELECT user_id FROM passkey_credentials WHERE credential_id = $1)", []byte(response.RawID)); err != nil {
			t.Error(err)
		}
	}()
	body, err := json.Marshal(openapi.RegistrationRequest{RegistrationId: options.RegistrationId, Credential: response})
	if err != nil {
		t.Fatal(err)
	}
	res = call("/finish", string(body))
	if res.Code != 204 {
		t.Fatalf("finish: %d %s", res.Code, res.Body.String())
	}
	queries := sqlc.New(pool)
	credential, err := queries.GetPasskeyCredential(ctx, []byte(response.RawID))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := queries.GetUserByID(ctx, credential.UserID); err != nil {
		t.Fatal(err)
	}
	if res := call("/finish", string(body)); res.Code != 400 {
		t.Fatalf("replay: %d", res.Code)
	}
}
