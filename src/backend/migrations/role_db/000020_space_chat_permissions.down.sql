-- Existing explicit permission decisions cannot be distinguished from seeded
-- bits after application. A rollback must never erase a later delegation.
DO $$
BEGIN
    RAISE EXCEPTION 'space chat permission seed backfill is irreversible';
END;
$$;
