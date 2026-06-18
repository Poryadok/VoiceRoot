DROP INDEX IF EXISTS files_story_id_idx;
ALTER TABLE files DROP COLUMN IF EXISTS story_id;
