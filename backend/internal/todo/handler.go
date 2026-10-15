package todo

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
	"github.com/catechlounge/kyoya67-todo/backend/internal/middleware"
	"github.com/catechlounge/kyoya67-todo/backend/internal/openapi"
)

type todoService interface {
	Create(context.Context, [16]byte, string, string) (sqlc.Todo, error)
	List(context.Context, [16]byte, string, int32, int32) ([]sqlc.Todo, error)
	Update(context.Context, [16]byte, int64, string, string) (sqlc.Todo, error)
	Delete(context.Context, [16]byte, int64) error
	Move(context.Context, [16]byte, int64, *int64) error
}

type Handler struct{ service todoService }

// serviceUnavailable records only the operation and failure stage. The raw error
// may contain SQL details or input data, so it must not be logged or returned.
func serviceUnavailable(c echo.Context, operation string) error {
	c.Logger().Errorf("todo request failed: operation=%s stage=service", operation)
	return c.NoContent(http.StatusServiceUnavailable)
}

func NewHandler(service todoService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) CreateTodo(c echo.Context) error {
	userID, ok := middleware.UserID(c.Request().Context())
	if !ok {
		return c.NoContent(http.StatusUnauthorized)
	}
	var request openapi.CreateTodoJSONRequestBody
	if err := c.Bind(&request); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if request.Status != nil && *request.Status == "" {
		return c.NoContent(http.StatusBadRequest)
	}
	_, err := h.service.Create(c.Request().Context(), userID, request.Title, requestStatus(request.Status))
	if err != nil {
		if errors.Is(err, ErrInvalidTitle) || errors.Is(err, ErrInvalidStatus) {
			return c.NoContent(http.StatusBadRequest)
		}
		return serviceUnavailable(c, "create")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) ListTodos(c echo.Context, params openapi.ListTodosParams) error {
	userID, ok := middleware.UserID(c.Request().Context())
	if !ok {
		return c.NoContent(http.StatusUnauthorized)
	}
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
	todos, err := h.service.List(c.Request().Context(), userID, title, limit, offset)
	if errors.Is(err, ErrInvalidPage) {
		return c.NoContent(http.StatusBadRequest)
	}
	if err != nil {
		return serviceUnavailable(c, "list")
	}
	response := openapi.TodoListResponse{Todos: make([]openapi.Todo, 0, len(todos))}
	for _, todo := range todos {
		response.Todos = append(response.Todos, toTodo(todo))
	}
	return c.JSON(http.StatusOK, response)
}

func (h *Handler) UpdateTodo(c echo.Context, id openapi.TodoID) error {
	userID, ok := middleware.UserID(c.Request().Context())
	if !ok {
		return c.NoContent(http.StatusUnauthorized)
	}
	var request openapi.UpdateTodoJSONRequestBody
	if err := c.Bind(&request); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if request.Status != nil && *request.Status == "" {
		return c.NoContent(http.StatusBadRequest)
	}
	_, err := h.service.Update(c.Request().Context(), userID, id, request.Title, requestStatus(request.Status))
	if errors.Is(err, ErrInvalidTitle) || errors.Is(err, ErrInvalidStatus) {
		return c.NoContent(http.StatusBadRequest)
	}
	if errors.Is(err, ErrNotFound) {
		return c.NoContent(http.StatusNotFound)
	}
	if err != nil {
		return serviceUnavailable(c, "update")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) MoveTodo(c echo.Context, id openapi.TodoID) error {
	userID, ok := middleware.UserID(c.Request().Context())
	if !ok {
		return c.NoContent(http.StatusUnauthorized)
	}
	var request openapi.MoveTodoJSONRequestBody
	if err := c.Bind(&request); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := h.service.Move(c.Request().Context(), userID, id, request.BeforeId); err != nil {
		if errors.Is(err, ErrInvalidPosition) {
			return c.NoContent(http.StatusBadRequest)
		}
		if errors.Is(err, ErrNotFound) {
			return c.NoContent(http.StatusNotFound)
		}
		return serviceUnavailable(c, "move")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) DeleteTodo(c echo.Context, id openapi.TodoID) error {
	userID, ok := middleware.UserID(c.Request().Context())
	if !ok {
		return c.NoContent(http.StatusUnauthorized)
	}
	if err := h.service.Delete(c.Request().Context(), userID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.NoContent(http.StatusNotFound)
		}
		return serviceUnavailable(c, "delete")
	}
	return c.NoContent(http.StatusNoContent)
}

func toTodo(todo sqlc.Todo) openapi.Todo {
	return openapi.Todo{Id: todo.ID, Title: todo.Title, Status: openapi.TodoStatus(todo.Status), Priority: todo.Priority, CreatedAt: todo.CreatedAt.Time, UpdatedAt: todo.UpdatedAt.Time}
}

func requestStatus(status *openapi.TodoStatus) string {
	if status == nil {
		return ""
	}
	return string(*status)
}
