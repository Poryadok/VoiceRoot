DO $$ BEGIN
 RAISE EXCEPTION 'membership authority floors and publication identities cannot be removed; forward repair required' USING ERRCODE='55000';
END $$;
