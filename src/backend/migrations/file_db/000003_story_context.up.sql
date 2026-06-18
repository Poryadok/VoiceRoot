-- Story-scoped uploads (context_story on RequestUpload).

ALTER TABLE files ADD COLUMN IF NOT EXISTS story_id UUID NULL;

CREATE INDEX IF NOT EXISTS files_story_id_idx
    ON files (story_id)
    WHERE story_id IS NOT NULL;
