-- name: CreateTodo :one
INSERT INTO todos (
    title, user_id, status, priority
) VALUES (
    sqlc.arg(title), sqlc.arg(owner_id)::uuid, sqlc.arg(status),
    (SELECT COALESCE(MAX(priority), 0) + 1 FROM todos WHERE user_id = sqlc.arg(owner_id)::uuid AND status = sqlc.arg(status))
)
RETURNING id, title, created_at, updated_at, user_id, status, priority;

-- name: ListTodos :many
SELECT
    id,
    title,
    created_at,
    updated_at,
    user_id,
    status,
    priority
FROM todos
WHERE user_id = sqlc.arg(owner_id)::uuid
  AND title ILIKE '%' || sqlc.arg(title)::text || '%'
ORDER BY status,
    CASE WHEN status <> 'completed' THEN priority END ASC NULLS LAST,
    CASE WHEN status = 'completed' THEN updated_at END DESC NULLS LAST,
    id
LIMIT sqlc.arg(page_limit)::int
OFFSET sqlc.arg(page_offset)::int;

-- name: UpdateTodo :one
UPDATE todos AS target
SET
    title = sqlc.arg(title),
    priority = CASE WHEN target.status <> COALESCE(NULLIF(sqlc.arg(status)::text, ''), target.status)
        THEN (SELECT COALESCE(MAX(other.priority), 0) + 1 FROM todos AS other
              WHERE other.user_id = sqlc.arg(owner_id)::uuid AND other.status = sqlc.arg(status)::text)
        ELSE target.priority END,
    status = COALESCE(NULLIF(sqlc.arg(status)::text, ''), target.status),
    updated_at = NOW()
WHERE target.id = sqlc.arg(todo_id) AND target.user_id = sqlc.arg(owner_id)::uuid
RETURNING target.id, target.title, target.created_at, target.updated_at, target.user_id, target.status, target.priority;

-- name: DeleteTodo :one
DELETE FROM todos
WHERE id = sqlc.arg(todo_id) AND user_id = sqlc.arg(owner_id)::uuid
RETURNING id;
