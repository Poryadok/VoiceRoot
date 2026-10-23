DO $$ BEGIN RAISE EXCEPTION 'native51 downgrade requires separately qualified reverse transition' USING ERRCODE='55000'; END $$;
