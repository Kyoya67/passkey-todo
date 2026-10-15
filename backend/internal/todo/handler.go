package todo

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
	"github.com/catechlounge/kyoya67-todo/backend/internal/openapi"
)

type todoService interface {
	Create(context.Context, string) error
	List(context.Context, string, int32, int32) ([]sqlc.Todo, error)
	Update(context.Context, int64, string) error
	Delete(context.Context, int64) error
}

type Handler struct{ service todoService }

func NewHandler(service todoService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) CreateTodo(c echo.Context) error {
	var request openapi.CreateTodoJSONRequestBody
	if err := c.Bind(&request); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := h.service.Create(c.Request().Context(), request.Title); err != nil {
		if errors.Is(err, ErrInvalidTitle) {
			return c.NoContent(http.StatusBadRequest)
		}
		return c.NoContent(http.StatusServiceUnavailable)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) ListTodos(c echo.Context, params openapi.ListTodosParams) error {
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
	if errors.Is(err, ErrInvalidPage) {
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

func (h *Handler) UpdateTodo(c echo.Context, id openapi.TodoID) error {
	var request openapi.UpdateTodoJSONRequestBody
	if err := c.Bind(&request); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	err := h.service.Update(c.Request().Context(), id, request.Title)
	if errors.Is(err, ErrInvalidTitle) {
		return c.NoContent(http.StatusBadRequest)
	}
	if errors.Is(err, ErrNotFound) {
		return c.NoContent(http.StatusNotFound)
	}
	if err != nil {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) DeleteTodo(c echo.Context, id openapi.TodoID) error {
	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.NoContent(http.StatusNotFound)
		}
		return c.NoContent(http.StatusServiceUnavailable)
	}
	return c.NoContent(http.StatusNoContent)
}

func toTodo(todo sqlc.Todo) openapi.Todo {
	return openapi.Todo{Id: todo.ID, Title: todo.Title, CreatedAt: todo.CreatedAt.Time, UpdatedAt: todo.UpdatedAt.Time}
}
