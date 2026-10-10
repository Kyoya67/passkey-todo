package router

import (
	"github.com/labstack/echo/v4"

	"github.com/catechlounge/kyoya67-todo/backend/internal/handler"
)

func Register(e *echo.Echo) {
	healthHandher := handler.NewHealthHandler()

	e.GET("/health", healthHandher.Check)
}
