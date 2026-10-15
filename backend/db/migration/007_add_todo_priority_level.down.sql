DROP INDEX idx_todos_owner_status_level_priority;
CREATE INDEX idx_todos_owner_status_priority ON todos (user_id, status, priority, id);
ALTER TABLE todos DROP COLUMN priority_level;
