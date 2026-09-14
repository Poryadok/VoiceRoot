CREATE INDEX idx_highlight_stories_story_id ON highlight_stories (story_id);
CREATE INDEX idx_stories_archive_purge ON stories (archived_until, id) WHERE expired_at IS NOT NULL;

CREATE TABLE story_media_deletion_outbox (
    operation_id UUID PRIMARY KEY,
    story_id UUID NOT NULL,
    media_file_id UUID NOT NULL,
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_token UUID,
    lease_until TIMESTAMPTZ,
    attempt_count BIGINT NOT NULL DEFAULT 0,
    last_error TEXT,
    last_error_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (operation_id = story_id),
    CHECK ((lease_token IS NULL) = (lease_until IS NULL))
);

CREATE INDEX idx_story_media_deletion_outbox_ready
    ON story_media_deletion_outbox (available_at, lease_until, operation_id);
