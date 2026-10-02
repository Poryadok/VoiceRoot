DO $$ BEGIN
 RAISE EXCEPTION 'User authority floors and permanent inactive fences are forward-only; restore requires matching backup and reconciliation' USING ERRCODE='55000';
END $$;
