DO $$ BEGIN RAISE EXCEPTION 'Auth lifecycle input rollback requires a reviewed forward migration' USING ERRCODE='55000'; END $$;
