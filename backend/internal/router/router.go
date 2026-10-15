package router

import (
	"github.com/labstack/echo/v4"

	"github.com/catechlounge/kyoya67-todo/backend/internal/auth"
	"github.com/catechlounge/kyoya67-todo/backend/internal/openapi"
	"github.com/catechlounge/kyoya67-todo/backend/internal/todo"
)

type handlers struct {
	*todoRoutes
	*auth.Handler
}

type todoRoutes struct{ *todo.Handler }

var _ openapi.ServerInterface = (*handlers)(nil)

func Register(e *echo.Echo, todoHandler *todo.Handler, authHandler *auth.Handler) {
	openapi.RegisterHandlers(e, &handlers{todoRoutes: &todoRoutes{todoHandler}, Handler: authHandler})
}
