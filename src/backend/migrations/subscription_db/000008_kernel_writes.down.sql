-- Persisted writer provenance must not silently disappear during rollback.
DO $$ BEGIN RAISE EXCEPTION 'Subscription kernel write rollback requires a reviewed forward migration' USING ERRCODE='55000'; END $$;
