package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/labstack/echo/v4"

	"github.com/catechlounge/kyoya67-todo/backend/internal/openapi"
)

type RegistrationService interface {
	BeginRegistration(context.Context) (string, *protocol.CredentialCreation, error)
	FinishRegistration(context.Context, string, *protocol.ParsedCredentialCreationData) error
}

type Handler struct {
	service      RegistrationService
	loginService LoginServiceAPI
	origin       string
}

func NewHandler(service RegistrationService, loginService LoginServiceAPI, origin string) *Handler {
	return &Handler{service: service, loginService: loginService, origin: origin}
}

func (h *Handler) BeginRegistration(c echo.Context) error {
	if h.service == nil {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	if err := h.checkOrigin(c); err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	token, options, err := h.service.BeginRegistration(c.Request().Context())
	if err != nil {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	return c.JSON(http.StatusOK, openapi.RegistrationOptions{RegistrationId: token, Options: *options})
}

func (h *Handler) FinishRegistration(c echo.Context) error {
	if h.service == nil {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	if err := h.checkOrigin(c); err != nil {
		return err
	}
	mediaType, _, err := mime.ParseMediaType(c.Request().Header.Get(echo.HeaderContentType))
	if err != nil || mediaType != echo.MIMEApplicationJSON {
		return c.NoContent(http.StatusUnsupportedMediaType)
	}
	var request openapi.FinishRegistrationJSONRequestBody
	decoder := json.NewDecoder(http.MaxBytesReader(c.Response(), c.Request().Body, 64<<10))
	if err := decoder.Decode(&request); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return c.NoContent(http.StatusBadRequest)
	}
	if request.RegistrationId == "" {
		return c.NoContent(http.StatusBadRequest)
	}
	parsed, err := request.Credential.Parse()
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := h.service.FinishRegistration(c.Request().Context(), request.RegistrationId, parsed); err != nil {
		if errors.Is(err, ErrInvalidRegistration) {
			return c.NoContent(http.StatusBadRequest)
		}
		return c.NoContent(http.StatusServiceUnavailable)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) checkOrigin(c echo.Context) error {
	if h.origin == "" || c.Request().Header.Get(echo.HeaderOrigin) != h.origin {
		return echo.NewHTTPError(http.StatusForbidden)
	}
	return nil
}

type LoginServiceAPI interface {
	BeginLogin(context.Context) (string, *protocol.CredentialAssertion, error)
	FinishLogin(context.Context, string, *protocol.ParsedCredentialAssertionData) (string, error)
}

func (h *Handler) BeginLogin(c echo.Context) error {
	if err := h.checkOrigin(c); err != nil {
		return err
	}
	if h.loginService == nil {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	token, options, err := h.loginService.BeginLogin(c.Request().Context())
	if err != nil {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	return c.JSON(http.StatusOK, openapi.LoginOptions{LoginId: token, Options: *options})
}

func (h *Handler) FinishLogin(c echo.Context) error {
	if err := h.checkOrigin(c); err != nil {
		return err
	}
	if h.loginService == nil {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	mediaType, _, err := mime.ParseMediaType(c.Request().Header.Get(echo.HeaderContentType))
	if err != nil || mediaType != echo.MIMEApplicationJSON {
		return c.NoContent(http.StatusUnsupportedMediaType)
	}
	var request openapi.FinishLoginJSONRequestBody
	decoder := json.NewDecoder(http.MaxBytesReader(c.Response(), c.Request().Body, 64<<10))
	if err := decoder.Decode(&request); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return c.NoContent(http.StatusBadRequest)
	}
	if request.LoginId == "" {
		return c.NoContent(http.StatusBadRequest)
	}
	parsed, err := request.Credential.Parse()
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	token, err := h.loginService.FinishLogin(c.Request().Context(), request.LoginId, parsed)
	if errors.Is(err, ErrInvalidLogin) {
		return c.NoContent(http.StatusBadRequest)
	}
	if err != nil {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	http.SetCookie(c.Response(), &http.Cookie{
		Name: "todo_session", Value: token, Path: "/",
		MaxAge: int(sessionLifetime.Seconds()), HttpOnly: true,
		Secure: strings.HasPrefix(h.origin, "https://"), SameSite: http.SameSiteStrictMode,
	})
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}
