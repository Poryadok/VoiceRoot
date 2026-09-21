-- Collapse is safe only before any shadow generation has accepted data.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM search_user_profile_generations WHERE generation <> 1) THEN
        RAISE EXCEPTION 'cannot roll back Search profile generations after a shadow rebuild';
    END IF;
END $$;

DROP TABLE search_user_profile_generation_route;
DROP TABLE search_user_profile_generations;
DROP INDEX search_user_profile_inbox_active_generation_profile_revision_idx;

ALTER TABLE profile_search_documents DROP CONSTRAINT profile_search_documents_pkey;
ALTER TABLE profile_search_documents DROP COLUMN generation;
ALTER TABLE profile_search_documents ADD PRIMARY KEY (profile_id);

ALTER TABLE search_user_profile_inbox DROP CONSTRAINT search_user_profile_inbox_pkey;
ALTER TABLE search_user_profile_inbox DROP COLUMN generation;
ALTER TABLE search_user_profile_inbox ADD PRIMARY KEY (event_id);

ALTER TABLE search_user_profile_fence DROP CONSTRAINT search_user_profile_fence_pkey;
ALTER TABLE search_user_profile_fence DROP COLUMN generation;
ALTER TABLE search_user_profile_fence ADD PRIMARY KEY (profile_id);

ALTER TABLE search_user_profile_checkpoint DROP CONSTRAINT search_user_profile_checkpoint_pkey;
ALTER TABLE search_user_profile_checkpoint DROP CONSTRAINT search_user_profile_checkpoint_singleton_check;
ALTER TABLE search_user_profile_checkpoint DROP COLUMN generation;
ALTER TABLE search_user_profile_checkpoint ADD PRIMARY KEY (singleton);

CREATE UNIQUE INDEX search_user_profile_inbox_active_profile_revision_idx
    ON search_user_profile_inbox (profile_id, source_revision)
    WHERE quarantined_at IS NULL;
