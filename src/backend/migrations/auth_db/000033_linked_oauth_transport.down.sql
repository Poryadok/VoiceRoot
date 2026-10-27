-- Disable the transport before rollback; retain attempts and one-use receipts.
DO $$ BEGIN
  RAISE EXCEPTION 'linked OAuth transport downgrade requires explicit retained-state maintenance' USING ERRCODE='55000';
END $$;
