DO $$ BEGIN
    RAISE EXCEPTION USING ERRCODE='55000', MESSAGE='terminal operation fences are forward-only';
END $$;
