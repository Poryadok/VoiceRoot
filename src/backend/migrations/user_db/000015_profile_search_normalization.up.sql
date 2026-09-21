-- Expand-only rollout for User-owned anti-spoof profile search keys.
ALTER TABLE profiles
    ADD COLUMN username_search_key TEXT NULL,
    ADD COLUMN display_name_search_key TEXT NULL,
    ADD COLUMN search_normalization_version INTEGER NULL;

CREATE INDEX profiles_username_search_key_idx
    ON profiles (username_search_key, discriminator, id)
    WHERE search_normalization_version IS NOT NULL;
CREATE INDEX profiles_display_name_search_key_idx
    ON profiles (display_name_search_key, discriminator, id)
    WHERE search_normalization_version IS NOT NULL;

CREATE TABLE profile_search_key_backfill_checkpoints (
    normalization_version INTEGER PRIMARY KEY,
    last_profile_id UUID NULL,
    scanned_count BIGINT NOT NULL DEFAULT 0,
    updated_count BIGINT NOT NULL DEFAULT 0,
    skipped_concurrent_count BIGINT NOT NULL DEFAULT 0,
    collision_count BIGINT NOT NULL DEFAULT 0,
    completed_at TIMESTAMPTZ NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE profile_search_key_collisions (
    normalization_version INTEGER NOT NULL,
    key_kind TEXT NOT NULL CHECK (key_kind IN ('username', 'display_name')),
    search_key TEXT NOT NULL,
    profile_id UUID NOT NULL,
    conflicting_profile_id UUID NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (normalization_version, key_kind, search_key, profile_id, conflicting_profile_id),
    CHECK (profile_id <> conflicting_profile_id)
);
