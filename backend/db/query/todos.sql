-- name: CreateTodo :one
INSERT INTO todos (
    title, user_id, status, priority_level, priority
) VALUES (
    sqlc.arg(title), sqlc.arg(owner_id)::uuid, sqlc.arg(status), sqlc.arg(priority_level),
    (SELECT COALESCE(MAX(priority), 0) + 1 FROM todos
     WHERE user_id = sqlc.arg(owner_id)::uuid AND status = sqlc.arg(status)
       AND priority_level = sqlc.arg(priority_level))
)
RETURNING id, title, created_at, updated_at, user_id, status, priority, priority_level;

-- name: ListTodos :many
SELECT
    id,
    title,
    created_at,
    updated_at,
    user_id,
    status,
    priority,
    priority_level
FROM todos
WHERE user_id = sqlc.arg(owner_id)::uuid
  AND title ILIKE '%' || sqlc.arg(title)::text || '%'
ORDER BY status,
    CASE WHEN status <> 'completed' THEN CASE priority_level
        WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END END ASC NULLS LAST,
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
            OR target.priority_level <> COALESCE(NULLIF(sqlc.arg(priority_level)::text, ''), target.priority_level)
        THEN (SELECT COALESCE(MAX(other.priority), 0) + 1 FROM todos AS other
              WHERE other.user_id = sqlc.arg(owner_id)::uuid
                AND other.status = COALESCE(NULLIF(sqlc.arg(status)::text, ''), target.status)
                AND other.priority_level = COALESCE(NULLIF(sqlc.arg(priority_level)::text, ''), target.priority_level))
        ELSE target.priority END,
    status = COALESCE(NULLIF(sqlc.arg(status)::text, ''), target.status),
    priority_level = COALESCE(NULLIF(sqlc.arg(priority_level)::text, ''), target.priority_level),
    updated_at = NOW()
WHERE target.id = sqlc.arg(todo_id) AND target.user_id = sqlc.arg(owner_id)::uuid
RETURNING target.id, target.title, target.created_at, target.updated_at, target.user_id, target.status, target.priority, target.priority_level;

-- name: DeleteTodo :one
DELETE FROM todos
WHERE id = sqlc.arg(todo_id) AND user_id = sqlc.arg(owner_id)::uuid
RETURNING id;
