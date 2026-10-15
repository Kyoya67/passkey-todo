-- Existing ownerless rows must be handled explicitly before this migration.
ALTER TABLE todos
    ALTER COLUMN user_id SET NOT NULL;
