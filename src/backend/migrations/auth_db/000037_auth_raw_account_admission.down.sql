DO $$ BEGIN RAISE EXCEPTION USING ERRCODE='55000', MESSAGE='raw account admission is forward-only'; END $$;
