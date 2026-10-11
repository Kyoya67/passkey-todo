package router

import (
	"github.com/labstack/echo/v4"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
	"github.com/catechlounge/kyoya67-todo/backend/internal/handler"
	"github.com/catechlounge/kyoya67-todo/backend/internal/repository"
	"github.com/catechlounge/kyoya67-todo/backend/internal/service"
)

func Register(e *echo.Echo, queries *sqlc.Queries) {
	healthHandher := handler.NewHealthHandler()

	todoRepository := repository.NewTodoRepository(queries)
	todoService := service.NewTodoService(todoRepository)
	todoHandler := handler.NewTodoHandler(todoService)

	e.GET("/health", healthHandher.Check)

	e.POST("/todos", todoHandler.Create)
	e.GET("/todos", todoHandler.List)
	e.PUT("/todos/:id", todoHandler.Update)
	e.DELETE("/todos/:id", todoHandler.Delete)
}
