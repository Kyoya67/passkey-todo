package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	sqlc "github.com/catechlounge/kyoya67-todo/backend/internal/database/sqlc"
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

type todoRequest struct {
	Title string `json:"title"`
}

type todoResponse struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type todoListResponse struct {
	Todos []todoResponse `json:"todos"`
}

func (h *TodoHandler) Create(c echo.Context) error {
	var request todoRequest
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

func (h *TodoHandler) List(c echo.Context) error {
	limit, offset, err := parsePagination(c)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	todos, err := h.service.List(c.Request().Context(), c.QueryParam("title"), limit, offset)
	if errors.Is(err, service.ErrInvalidPage) {
		return c.NoContent(http.StatusBadRequest)
	}
	if err != nil {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	response := todoListResponse{Todos: make([]todoResponse, 0, len(todos))}
	for _, todo := range todos {
		response.Todos = append(response.Todos, toTodoResponse(todo))
	}
	return c.JSON(http.StatusOK, response)
}

func (h *TodoHandler) Update(c echo.Context) error {
	id, err := parseID(c)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	var request todoRequest
	if err := c.Bind(&request); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	err = h.service.Update(c.Request().Context(), id, request.Title)
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

func (h *TodoHandler) Delete(c echo.Context) error {
	id, err := parseID(c)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		if errors.Is(err, service.ErrTodoNotFound) {
			return c.NoContent(http.StatusNotFound)
		}
		return c.NoContent(http.StatusServiceUnavailable)
	}
	return c.NoContent(http.StatusNoContent)
}

func parseID(c echo.Context) (int64, error) { return strconv.ParseInt(c.Param("id"), 10, 64) }

func parsePagination(c echo.Context) (int32, int32, error) {
	var limit, offset int32
	if raw := c.QueryParam("limit"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return 0, 0, errors.New("invalid limit")
		}
		limit = int32(parsed)
	}
	if raw := c.QueryParam("offset"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return 0, 0, errors.New("invalid offset")
		}
		offset = int32(parsed)
	}
	return limit, offset, nil
}

func toTodoResponse(todo sqlc.Todo) todoResponse {
	return todoResponse{ID: todo.ID, Title: todo.Title, CreatedAt: todo.CreatedAt.Time, UpdatedAt: todo.UpdatedAt.Time}
}
