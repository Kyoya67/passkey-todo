package auth

import (
	"bytes"
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

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
	"github.com/catechlounge/kyoya67-todo/backend/internal/openapi"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
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
			h := NewHandler(service, nil, "http://localhost:3000")
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
	h := NewHandler(nil, nil, "")
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
	h := NewHandler(NewService(w, NewRepository(pool, sqlc.New(pool)), NewChallengeStore(redisClient)), nil, "http://localhost:3000")
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

type fakeLoginServiceAPI struct {
	finishCalls int
	token       string
	err         error
}

func (f *fakeLoginServiceAPI) BeginLogin(context.Context) (string, *protocol.CredentialAssertion, error) {
	return "login-token", &protocol.CredentialAssertion{}, f.err
}
func (f *fakeLoginServiceAPI) FinishLogin(context.Context, string, *protocol.ParsedCredentialAssertionData) (string, error) {
	f.finishCalls++
	return f.token, f.err
}

func TestLoginHandlerBegin(t *testing.T) {
	service := &fakeLoginServiceAPI{}
	h := NewHandler(nil, service, "http://localhost:3000")
	e := echo.New()
	e.POST("/begin", h.BeginLogin)
	for _, tc := range []struct {
		origin string
		status int
	}{{"", 403}, {"https://evil.example", 403}, {"http://localhost:3000", 200}} {
		r := httptest.NewRequest(http.MethodPost, "/begin", nil)
		r.Header.Set(echo.HeaderOrigin, tc.origin)
		res := httptest.NewRecorder()
		e.ServeHTTP(res, r)
		if res.Code != tc.status {
			t.Fatalf("origin=%q status=%d", tc.origin, res.Code)
		}
		if tc.status == 200 && (res.Header().Get("Cache-Control") != "no-store" || !strings.Contains(res.Body.String(), "login-token")) {
			t.Fatal("token response missing or cacheable")
		}
	}
	service.err = errors.New("redis failed")
	r := httptest.NewRequest(http.MethodPost, "/begin", nil)
	r.Header.Set(echo.HeaderOrigin, "http://localhost:3000")
	res := httptest.NewRecorder()
	e.ServeHTTP(res, r)
	if res.Code != 503 || strings.Contains(res.Body.String(), "redis failed") {
		t.Fatalf("internal error leaked: %d", res.Code)
	}
}

func TestLoginHandlerFinish(t *testing.T) {
	data := make([]byte, 37)
	data[32] = 0x05
	credential := protocol.CredentialAssertionResponse{
		PublicKeyCredential: protocol.PublicKeyCredential{Credential: protocol.Credential{ID: "Ag", Type: "public-key"}, RawID: []byte{2}},
		AssertionResponse: protocol.AuthenticatorAssertionResponse{
			AuthenticatorResponse: protocol.AuthenticatorResponse{ClientDataJSON: []byte(`{"type":"webauthn.get","challenge":"Y2hhbGxlbmdl","origin":"http://localhost:3000"}`)},
			AuthenticatorData:     data, Signature: []byte{1}, UserHandle: []byte{1},
		},
	}
	body, err := json.Marshal(map[string]any{"loginId": "token", "credential": credential})
	if err != nil {
		t.Fatal(err)
	}
	service := &fakeLoginServiceAPI{token: "session-token"}
	h := NewHandler(nil, service, "http://localhost:3000")
	e := echo.New()
	var logs bytes.Buffer
	e.Logger.SetOutput(&logs)
	e.POST("/finish", h.FinishLogin)
	request := httptest.NewRequest(http.MethodPost, "/finish", strings.NewReader(string(body)))
	request.Header.Set(echo.HeaderOrigin, "http://localhost:3000")
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	res := httptest.NewRecorder()
	e.ServeHTTP(res, request)
	if res.Code != 204 || service.finishCalls != 1 {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	cookies := res.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "todo_session" || cookies[0].Value != "session-token" || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Secure {
		t.Fatalf("insecure local cookie: %+v", cookies)
	}

	service.err = ErrInvalidLogin
	bad := httptest.NewRequest(http.MethodPost, "/finish", strings.NewReader(string(body)))
	bad.Header.Set(echo.HeaderOrigin, "http://localhost:3000")
	bad.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	res = httptest.NewRecorder()
	e.ServeHTTP(res, bad)
	if res.Code != 400 || len(res.Result().Cookies()) != 0 {
		t.Fatalf("failure issued session: %d", res.Code)
	}
	if !strings.Contains(logs.String(), "stage=verification reason=invalid_or_expired_challenge") {
		t.Fatalf("missing safe rejection label: %s", logs.String())
	}

	for _, tc := range []struct {
		name, origin, contentType, body string
		serviceErr                      error
		want                            int
	}{
		{name: "missing origin", origin: "", contentType: echo.MIMEApplicationJSON, body: string(body), want: 403},
		{name: "wrong content type", origin: "http://localhost:3000", contentType: "text/plain", body: string(body), want: 415},
		{name: "bad JSON", origin: "http://localhost:3000", contentType: echo.MIMEApplicationJSON, body: "{", want: 400},
		{name: "missing token", origin: "http://localhost:3000", contentType: echo.MIMEApplicationJSON, body: "{}", want: 400},
		{name: "internal error", origin: "http://localhost:3000", contentType: echo.MIMEApplicationJSON, body: string(body), serviceErr: errors.New("private secret"), want: 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service.err = tc.serviceErr
			r := httptest.NewRequest(http.MethodPost, "/finish", strings.NewReader(tc.body))
			r.Header.Set(echo.HeaderOrigin, tc.origin)
			r.Header.Set(echo.HeaderContentType, tc.contentType)
			res := httptest.NewRecorder()
			e.ServeHTTP(res, r)
			if res.Code != tc.want || len(res.Result().Cookies()) != 0 || strings.Contains(res.Body.String(), "private secret") {
				t.Fatalf("status=%d cookie=%v body=%s", res.Code, res.Result().Cookies(), res.Body.String())
			}
		})
	}
	service.err = nil
	secureHandler := NewHandler(nil, service, "https://ca-techlounge-todo.kyo8.dev")
	secureEcho := echo.New()
	secureEcho.POST("/finish", secureHandler.FinishLogin)
	secureReq := httptest.NewRequest(http.MethodPost, "/finish", strings.NewReader(string(body)))
	secureReq.Header.Set(echo.HeaderOrigin, "https://ca-techlounge-todo.kyo8.dev")
	secureReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	secureRes := httptest.NewRecorder()
	secureEcho.ServeHTTP(secureRes, secureReq)
	if secureRes.Code != 204 || len(secureRes.Result().Cookies()) != 1 || !secureRes.Result().Cookies()[0].Secure {
		t.Fatalf("cloud cookie must be secure: %d", secureRes.Code)
	}
	if strings.Contains(logs.String(), "private secret") || strings.Contains(logs.String(), "session-token") {
		t.Fatal("sensitive value appeared in login logs")
	}
}
