-- notification_db v1 — docs/microservices/notification-service.md
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE device_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL,
    platform VARCHAR(16) NOT NULL CHECK (platform IN ('android', 'ios', 'web', 'desktop')),
    token TEXT NOT NULL,
    push_service VARCHAR(16) NOT NULL CHECK (push_service IN ('fcm', 'apns', 'voip_apns')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX device_tokens_profile_token_uq ON device_tokens (profile_id, token);
CREATE INDEX device_tokens_profile_idx ON device_tokens (profile_id);

CREATE TABLE notification_settings (
    profile_id UUID NOT NULL,
    scope_type VARCHAR(16) NOT NULL CHECK (scope_type IN ('global', 'space', 'channel', 'chat')),
    scope_id UUID,
    enabled BOOLEAN NOT NULL DEFAULT true,
    mute_until TIMESTAMPTZ,
    suppress_types JSONB NOT NULL DEFAULT '[]'::jsonb,
    PRIMARY KEY (profile_id, scope_type, scope_id)
);

CREATE TABLE quiet_hours (
    profile_id UUID PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT false,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    override_mentions BOOLEAN NOT NULL DEFAULT true
);
