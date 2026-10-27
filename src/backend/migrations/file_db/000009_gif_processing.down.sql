DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM file_gif_jobs) OR EXISTS(SELECT 1 FROM file_gif_outputs)
 OR EXISTS(SELECT 1 FROM file_gif_cleanup) OR EXISTS(SELECT 1 FROM files WHERE upload_intent='gif') THEN
  RAISE EXCEPTION 'cannot discard retained GIF processing evidence';
 END IF;
END $$;
DROP TABLE file_gif_cleanup;
DROP TABLE file_gif_outputs;
DROP TABLE file_gif_jobs;
ALTER TABLE files DROP CONSTRAINT files_gif_plaintext;
ALTER TABLE files DROP CONSTRAINT files_upload_intent_check;
ALTER TABLE files ADD CONSTRAINT files_upload_intent_check CHECK (upload_intent IN ('','music'));
