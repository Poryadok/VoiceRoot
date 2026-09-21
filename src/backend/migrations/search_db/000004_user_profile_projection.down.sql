DROP TABLE IF EXISTS search_user_profile_checkpoint;
DROP TABLE IF EXISTS search_user_profile_inbox;
ALTER TABLE profile_search_documents
    DROP COLUMN IF EXISTS tombstoned_at,
    DROP COLUMN IF EXISTS source_revision,
    DROP COLUMN IF EXISTS normalization_version,
    DROP COLUMN IF EXISTS display_name_search_key,
    DROP COLUMN IF EXISTS username_search_key;
