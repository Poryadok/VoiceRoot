-- Restoring a verified matching DB/runtime backup retains all revision floors.
DO $$ BEGIN
 RAISE EXCEPTION 'Role SDK authority downgrade requires matching verified backup recovery' USING ERRCODE='55000';
END $$;
