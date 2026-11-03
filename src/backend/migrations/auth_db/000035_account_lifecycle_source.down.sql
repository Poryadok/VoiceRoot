-- Permanent lifecycle authority cannot be removed by a binary rollback.
DO $$ BEGIN
  RAISE EXCEPTION 'Auth lifecycle source is forward-only; use a compatible binary' USING ERRCODE='55000';
END $$;
