ALTER TABLE todos ADD COLUMN priority BIGINT NOT NULL DEFAULT 0;

WITH ranked AS (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY user_id, status ORDER BY updated_at DESC, id DESC) AS rank
    FROM todos
)
UPDATE todos SET priority = ranked.rank FROM ranked WHERE todos.id = ranked.id;

CREATE INDEX idx_todos_owner_status_priority ON todos (user_id, status, priority, id);
