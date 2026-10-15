-- name: CreateTodo :one
INSERT INTO todos (
    title, user_id, status
) VALUES (
    sqlc.arg(title), sqlc.arg(owner_id)::uuid, sqlc.arg(status)
)
RETURNING id, title, created_at, updated_at, user_id, status;

-- name: ListTodos :many
SELECT
    id,
    title,
    created_at,
    updated_at,
    user_id,
    status
FROM todos
WHERE user_id = sqlc.arg(owner_id)::uuid
  AND title ILIKE '%' || sqlc.arg(title)::text || '%'
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg(page_limit)::int
OFFSET sqlc.arg(page_offset)::int;

-- name: UpdateTodo :one
UPDATE todos
SET
    title = sqlc.arg(title),
    status = COALESCE(NULLIF(sqlc.arg(status)::text, ''), status),
    updated_at = NOW()
WHERE id = sqlc.arg(todo_id) AND user_id = sqlc.arg(owner_id)::uuid
RETURNING id, title, created_at, updated_at, user_id, status;

-- name: DeleteTodo :one
DELETE FROM todos
WHERE id = sqlc.arg(todo_id) AND user_id = sqlc.arg(owner_id)::uuid
RETURNING id;
