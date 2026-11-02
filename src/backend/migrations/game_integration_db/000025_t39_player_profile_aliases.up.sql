CREATE TABLE gis_player_profile_aliases (
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    account_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    opaque_reference UUID NOT NULL DEFAULT gen_random_uuid(),
    alias TEXT NOT NULL CHECK (length(alias) BETWEEN 1 AND 256),
    policy_revision BIGINT NOT NULL CHECK (policy_revision > 0),
    consented_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    revoked_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (application_id, environment_id, profile_id),
    UNIQUE (application_id, environment_id, opaque_reference),
    CHECK ((revoked_at IS NULL) OR revoked_at >= consented_at)
);

CREATE INDEX gis_player_profile_aliases_search_idx
    ON gis_player_profile_aliases(application_id, environment_id, lower(alias))
    WHERE revoked_at IS NULL;
