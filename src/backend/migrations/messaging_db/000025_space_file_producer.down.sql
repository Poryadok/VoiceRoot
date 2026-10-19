DO $$ BEGIN
 -- The probe must get a fresh snapshot after every admitted writer finishes.
 IF current_setting('transaction_isolation') <> 'read committed' THEN
  RAISE EXCEPTION 'immutable P3 File producer rollback requires READ COMMITTED';
 END IF;
 -- Keep the lock, probe and DROP in this one transaction even when the
 -- migration client executes top-level statements separately.
 LOCK TABLE messaging_space_file_producers IN ACCESS EXCLUSIVE MODE;
 IF EXISTS(SELECT 1 FROM messaging_space_file_producers) THEN
  RAISE EXCEPTION 'cannot remove immutable P3 File producer snapshots';
 END IF;
 DROP TABLE messaging_space_file_producers;
END $$;
