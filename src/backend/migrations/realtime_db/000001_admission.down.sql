-- Removing terminal denies or unresolved admissions is unsafe.
DO $$ BEGIN RAISE EXCEPTION 'Realtime lifecycle downgrade requires an explicit reviewed recovery plan'; END $$;
