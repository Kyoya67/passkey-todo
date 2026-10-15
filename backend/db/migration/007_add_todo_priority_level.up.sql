ALTER TABLE todos ADD COLUMN priority_level TEXT NOT NULL DEFAULT 'medium'
    CHECK (priority_level IN ('high', 'medium', 'low'));

WITH ranked AS (
    SELECT id, ROW_NUMBER() OVER (
        PARTITION BY user_id, status, priority_level ORDER BY priority, id
    ) AS rank
    FROM todos
)
UPDATE todos SET priority = ranked.rank FROM ranked WHERE todos.id = ranked.id;

DROP INDEX idx_todos_owner_status_priority;
CREATE INDEX idx_todos_owner_status_level_priority
    ON todos (user_id, status, priority_level, priority, id);
