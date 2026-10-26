DO $$ BEGIN
 RAISE EXCEPTION 'GIS authority clock and retained floor cannot be downgraded' USING ERRCODE='55000';
END $$;
