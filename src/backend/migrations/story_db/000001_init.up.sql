-- Story Service schema (Phase 17). See docs/microservices/story-service.md.

CREATE TABLE IF NOT EXISTS stories (
    id UUID PRIMARY KEY,
    author_profile_id UUID NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('photo', 'video', 'text')),
    media_file_id UUID,
    text_content TEXT,
    text_style JSONB,
    game_tag TEXT,
    is_looking_for_party BOOLEAN NOT NULL DEFAULT FALSE,
    lfp_criteria JSONB,
    mention_profile_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    view_count INT NOT NULL DEFAULT 0,
    visibility TEXT NOT NULL CHECK (visibility IN ('everyone', 'friends', 'custom')),
    expires_at TIMESTAMPTZ NOT NULL,
    archived_until TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    expired_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_stories_author_active
    ON stories (author_profile_id, created_at DESC)
    WHERE deleted_at IS NULL AND expired_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_stories_expires_at
    ON stories (expires_at)
    WHERE deleted_at IS NULL AND expired_at IS NULL;

CREATE TABLE IF NOT EXISTS story_views (
    story_id UUID NOT NULL REFERENCES stories (id) ON DELETE CASCADE,
    viewer_profile_id UUID NOT NULL,
    is_anonymous BOOLEAN NOT NULL DEFAULT FALSE,
    viewed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (story_id, viewer_profile_id)
);

CREATE TABLE IF NOT EXISTS story_reactions (
    story_id UUID NOT NULL REFERENCES stories (id) ON DELETE CASCADE,
    reactor_profile_id UUID NOT NULL,
    emoji TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (story_id, reactor_profile_id)
);

CREATE TABLE IF NOT EXISTS highlights (
    id UUID PRIMARY KEY,
    profile_id UUID NOT NULL,
    name TEXT NOT NULL,
    cover_file_id UUID,
    sort_order INT NOT NULL DEFAULT 0,
    visibility TEXT NOT NULL DEFAULT 'everyone',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_highlights_profile
    ON highlights (profile_id, sort_order);

CREATE TABLE IF NOT EXISTS highlight_stories (
    highlight_id UUID NOT NULL REFERENCES highlights (id) ON DELETE CASCADE,
    story_id UUID NOT NULL REFERENCES stories (id) ON DELETE CASCADE,
    sort_order INT NOT NULL DEFAULT 0,
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (highlight_id, story_id)
);
