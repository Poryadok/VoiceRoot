-- A published store identity must not be silently replaced by DOWN/UP.
DO $$ BEGIN
  RAISE EXCEPTION 'moderation startup store identity is permanent';
END $$;
