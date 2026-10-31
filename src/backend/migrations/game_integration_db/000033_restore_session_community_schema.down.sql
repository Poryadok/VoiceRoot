DO $$ BEGIN RAISE EXCEPTION 'GIS33 downgrade refused; preserve schema, receipts and authority floor' USING ERRCODE='55000'; END $$;
