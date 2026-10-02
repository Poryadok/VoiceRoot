DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM messaging_space_file_producers) THEN
  RAISE EXCEPTION 'cannot remove immutable P3 File producer snapshots';
 END IF;
END $$;
DROP TABLE messaging_space_file_producers;
