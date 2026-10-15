-- Refuse rollback if it would discard ownership information.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM todos WHERE user_id IS NOT NULL) THEN
        RAISE EXCEPTION 'cannot remove todo ownership while owned TODOs exist';
    END IF;
END
$$;

DROP INDEX IF EXISTS idx_todos_user_updated_id;
ALTER TABLE todos DROP COLUMN user_id;
