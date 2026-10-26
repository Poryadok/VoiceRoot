DO $$ BEGIN
    RAISE EXCEPTION 'subscription lifecycle capture rollback requires an explicit retained-fact recovery plan' USING ERRCODE='55000';
END $$;
