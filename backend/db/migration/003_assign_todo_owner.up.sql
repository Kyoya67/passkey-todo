-- Existing TODOs have no identifiable owner, so they remain unassigned.
ALTER TABLE todos
    ADD COLUMN user_id UUID REFERENCES users (id) ON DELETE CASCADE;

CREATE INDEX idx_todos_user_updated_id
    ON todos (user_id, updated_at DESC, id DESC);
