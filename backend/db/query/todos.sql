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
WHERE title ILIKE '%' || sqlc.arg(title)::text || '%'
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg(page_limit)::int
OFFSET sqlc.arg(page_offset)::int;

-- name: UpdateTodo :one
UPDATE todos
SET
    title = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING id, title, created_at, updated_at;

-- name: DeleteTodo :one
DELETE FROM todos
WHERE id = $1
RETURNING id;
