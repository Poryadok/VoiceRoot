-- Forward-only safety: an older runtime cannot preserve this coverage.
-- Recover the complete database/runtime bundle from its verified backup.
DO $$ BEGIN
 RAISE EXCEPTION 'role authority revision coverage cannot be downgraded' USING ERRCODE='55000';
END $$;
