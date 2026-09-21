-- Search-owned generations isolate User-authoritative profile rebuilds. The
-- seed route keeps all pre-existing rows visible as generation 1.
CREATE TABLE search_user_profile_generations (
    generation BIGINT PRIMARY KEY CHECK (generation > 0),
    state TEXT NOT NULL CHECK (state IN ('building', 'ready')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ready_at TIMESTAMPTZ NULL
);
INSERT INTO search_user_profile_generations(generation,state,ready_at)
VALUES (1,'ready',now()) ON CONFLICT DO NOTHING;

CREATE TABLE search_user_profile_generation_route (
    singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
    active_generation BIGINT NOT NULL REFERENCES search_user_profile_generations(generation),
    rollback_generation BIGINT NULL REFERENCES search_user_profile_generations(generation),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO search_user_profile_generation_route(singleton,active_generation)
VALUES (true,1) ON CONFLICT DO NOTHING;

ALTER TABLE profile_search_documents ADD COLUMN generation BIGINT NOT NULL DEFAULT 1;
ALTER TABLE search_user_profile_inbox ADD COLUMN generation BIGINT NOT NULL DEFAULT 1;
ALTER TABLE search_user_profile_fence ADD COLUMN generation BIGINT NOT NULL DEFAULT 1;
ALTER TABLE search_user_profile_checkpoint ADD COLUMN generation BIGINT NOT NULL DEFAULT 1;

ALTER TABLE profile_search_documents DROP CONSTRAINT profile_search_documents_pkey;
ALTER TABLE profile_search_documents ADD PRIMARY KEY (generation, profile_id);
ALTER TABLE search_user_profile_inbox DROP CONSTRAINT search_user_profile_inbox_pkey;
ALTER TABLE search_user_profile_inbox ADD PRIMARY KEY (generation, event_id);
ALTER TABLE search_user_profile_fence DROP CONSTRAINT search_user_profile_fence_pkey;
ALTER TABLE search_user_profile_fence ADD PRIMARY KEY (generation, profile_id);
ALTER TABLE search_user_profile_checkpoint DROP CONSTRAINT search_user_profile_checkpoint_pkey;
ALTER TABLE search_user_profile_checkpoint DROP CONSTRAINT search_user_profile_checkpoint_singleton_check;
ALTER TABLE search_user_profile_checkpoint ADD PRIMARY KEY (generation);
ALTER TABLE search_user_profile_checkpoint ADD CONSTRAINT search_user_profile_checkpoint_singleton_check CHECK (singleton);

DROP INDEX IF EXISTS search_user_profile_inbox_active_profile_revision_idx;
CREATE UNIQUE INDEX search_user_profile_inbox_active_generation_profile_revision_idx
    ON search_user_profile_inbox (generation, profile_id, source_revision)
    WHERE quarantined_at IS NULL;
