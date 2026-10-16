CREATE TABLE gis_app_notification_consents (
    binding_id UUID PRIMARY KEY REFERENCES player_bindings(binding_id),
    account_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    categories TEXT[] NOT NULL DEFAULT '{}',
    channels TEXT[] NOT NULL DEFAULT '{}',
    revision BIGINT NOT NULL CHECK (revision > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    revoked_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK (categories <@ ARRAY['game_activity','game_social','game_actions']::TEXT[]),
    CHECK (channels <@ ARRAY['push','in_app']::TEXT[]),
    CHECK ((cardinality(categories) = 0) = (cardinality(channels) = 0))
);

CREATE INDEX gis_app_notification_consents_scope_idx
    ON gis_app_notification_consents(application_id,environment_id,profile_id)
    WHERE revoked_at IS NULL;
