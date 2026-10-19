-- These bindings may already be saved in another owner. Local row counts do
-- not prove that discarding them or their retirement receipts is safe.
DO $$ BEGIN RAISE EXCEPTION 'Chat report attachment authority requires roll-forward recovery'; END $$;
