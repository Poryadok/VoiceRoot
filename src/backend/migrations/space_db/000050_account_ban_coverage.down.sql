DO $$ BEGIN
 RAISE EXCEPTION 'account coverage downgrade requires a matching verified backup; evidence and replay floors must be preserved' USING ERRCODE='55000';
END $$;
