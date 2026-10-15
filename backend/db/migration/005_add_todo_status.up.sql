ALTER TABLE todos ADD COLUMN status TEXT NOT NULL DEFAULT 'not_started'
    CHECK (status IN ('not_started', 'in_progress', 'completed'));
