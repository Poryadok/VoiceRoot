-- SDK authority schema is forward-only; recover a verified matching DB/runtime backup.
DO $$ BEGIN
 RAISE EXCEPTION 'Auth SDK downgrade requires matching verified backup recovery' USING ERRCODE='55000';
END $$;
