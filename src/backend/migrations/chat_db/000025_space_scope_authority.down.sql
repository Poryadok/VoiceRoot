-- Removing durable incarnation/revision history would permit ABA. Roll forward
-- or use an explicitly planned offline authority migration instead.
DO $$ BEGIN RAISE EXCEPTION 'Chat scope authority migration cannot be reversed safely'; END $$;
