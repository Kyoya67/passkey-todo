-- name: CreateTodo :one
INSERT INTO todos (
    title
) VALUES (
    $1
)
RETURNING id, title, created_at, updated_at;

-- name: ListTodos :many
SELECT
    id,
    title,
    created_at,
    updated_at
FROM todos
WHERE title ILIKE '%' || $1 || '%'
ORDER BY updated_at DESC, id DESC
LIMIT $2
OFFSET $3;

-- name: GetTodo :one
SELECT
    id,
    title,
    created_at,
    updated_at
FROM todos
WHERE id = $1;

-- name: UpdateTodo :one
UPDATE todos
SET
    title = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING id, title, created_at, updated_at;

-- name: DeleteTodo :exec
DELETE FROM todos
WHERE id = $1;