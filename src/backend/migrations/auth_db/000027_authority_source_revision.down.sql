DO $$ BEGIN
 RAISE EXCEPTION 'Auth authority floors are forward-only; restore requires matching backup and reconciliation' USING ERRCODE='55000';
END $$;
