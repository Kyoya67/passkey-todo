package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/labstack/echo/v4"

	"github.com/catechlounge/kyoya67-todo/backend/internal/auth"
	"github.com/catechlounge/kyoya67-todo/backend/internal/openapi"
	"github.com/catechlounge/kyoya67-todo/backend/internal/todo"
)

type registrationService struct{}

func (registrationService) BeginRegistration(context.Context) (string, *protocol.CredentialCreation, error) {
	return "token", &protocol.CredentialCreation{}, nil
}
func (registrationService) FinishRegistration(context.Context, string, *protocol.ParsedCredentialCreationData) error {
	return nil
}

func TestRegister(t *testing.T) {
	e := echo.New()
	Register(e, todo.NewHandler(nil), auth.NewHandler(registrationService{}, "http://localhost:3000"))
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/health", http.StatusOK},
		{http.MethodPost, "/auth/registration/begin", http.StatusOK},
		{http.MethodPost, "/auth/registration/finish", http.StatusUnsupportedMediaType},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.Header.Set("Origin", "http://localhost:3000")
		res := httptest.NewRecorder()
		e.ServeHTTP(res, req)
		if res.Code != tc.status {
			t.Fatalf("%s: status %d, want %d", tc.path, res.Code, tc.status)
		}
	}
	spec, err := openapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	if err := spec.Validate(context.Background()); err != nil {
		t.Fatalf("invalid OpenAPI: %v", err)
	}
	for _, route := range e.Routes() {
		if route.Path == "/todos" && route.Method == http.MethodGet {
			return
		}
	}
	t.Fatal("TODO routes missing")
}
