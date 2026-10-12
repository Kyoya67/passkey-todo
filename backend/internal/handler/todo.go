package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
	"github.com/catechlounge/kyoya67-todo/backend/internal/openapi"
	"github.com/catechlounge/kyoya67-todo/backend/internal/service"
)

type TodoService interface {
	Create(context.Context, string) error
	List(context.Context, string, int32, int32) ([]sqlc.Todo, error)
	Update(context.Context, int64, string) error
	Delete(context.Context, int64) error
}

type TodoHandler struct{ service TodoService }

func NewTodoHandler(service TodoService) *TodoHandler {
	return &TodoHandler{service: service}
}

func (h *TodoHandler) GetHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *TodoHandler) CreateTodo(c echo.Context) error {
	var request openapi.CreateTodoJSONRequestBody
	if err := c.Bind(&request); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := h.service.Create(c.Request().Context(), request.Title); err != nil {
		if errors.Is(err, service.ErrInvalidTitle) {
			return c.NoContent(http.StatusBadRequest)
		}
		return c.NoContent(http.StatusServiceUnavailable)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *TodoHandler) ListTodos(c echo.Context, params openapi.ListTodosParams) error {
	var title string
	if params.Title != nil {
		title = *params.Title
	}
	var limit, offset int32
	if params.Limit != nil {
		limit = *params.Limit
	}
	if params.Offset != nil {
		offset = *params.Offset
	}
	todos, err := h.service.List(c.Request().Context(), title, limit, offset)
	if errors.Is(err, service.ErrInvalidPage) {
		return c.NoContent(http.StatusBadRequest)
	}
	if err != nil {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	response := openapi.TodoListResponse{Todos: make([]openapi.Todo, 0, len(todos))}
	for _, todo := range todos {
		response.Todos = append(response.Todos, toTodo(todo))
	}
	return c.JSON(http.StatusOK, response)
}

func (h *TodoHandler) UpdateTodo(c echo.Context, id openapi.TodoID) error {
	var request openapi.UpdateTodoJSONRequestBody
	if err := c.Bind(&request); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	err := h.service.Update(c.Request().Context(), id, request.Title)
	if errors.Is(err, service.ErrInvalidTitle) {
		return c.NoContent(http.StatusBadRequest)
	}
	if errors.Is(err, service.ErrTodoNotFound) {
		return c.NoContent(http.StatusNotFound)
	}
	if err != nil {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *TodoHandler) DeleteTodo(c echo.Context, id openapi.TodoID) error {
	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		if errors.Is(err, service.ErrTodoNotFound) {
			return c.NoContent(http.StatusNotFound)
		}
		return c.NoContent(http.StatusServiceUnavailable)
	}
	return c.NoContent(http.StatusNoContent)
}

func toTodo(todo sqlc.Todo) openapi.Todo {
	return openapi.Todo{Id: todo.ID, Title: todo.Title, CreatedAt: todo.CreatedAt.Time, UpdatedAt: todo.UpdatedAt.Time}
}
