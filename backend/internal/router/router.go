package router

import (
	"github.com/labstack/echo/v4"

	"github.com/catechlounge/kyoya67-todo/backend/internal/auth"
	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
	"github.com/catechlounge/kyoya67-todo/backend/internal/openapi"
	"github.com/catechlounge/kyoya67-todo/backend/internal/todo"
)

type handlers struct {
	*todo.TodoHandler
	*auth.Handler
}

var _ openapi.ServerInterface = (*handlers)(nil)

func Register(e *echo.Echo, queries *sqlc.Queries, authHandler *auth.Handler) {
	todoRepository := todo.NewTodoRepository(queries)
	todoService := todo.NewTodoService(todoRepository)
	todoHandler := todo.NewTodoHandler(todoService)

	openapi.RegisterHandlers(e, &handlers{TodoHandler: todoHandler, Handler: authHandler})
}
