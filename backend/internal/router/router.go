package router

import (
	"github.com/labstack/echo/v4"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
	"github.com/catechlounge/kyoya67-todo/backend/internal/handler"
	"github.com/catechlounge/kyoya67-todo/backend/internal/openapi"
	"github.com/catechlounge/kyoya67-todo/backend/internal/repository"
	"github.com/catechlounge/kyoya67-todo/backend/internal/service"
)

func Register(e *echo.Echo, queries *sqlc.Queries) {
	todoRepository := repository.NewTodoRepository(queries)
	todoService := service.NewTodoService(todoRepository)
	todoHandler := handler.NewTodoHandler(todoService)

	openapi.RegisterHandlers(e, todoHandler)
}
