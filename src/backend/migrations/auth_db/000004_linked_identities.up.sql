-- auth_db v4 — Phase 13: OAuth linked platform identities

CREATE TABLE IF NOT EXISTS linked_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL,
    platform TEXT NOT NULL CHECK (platform IN ('twitch', 'youtube')),
    external_id TEXT NOT NULL,
    external_login TEXT NULL,
    access_token_encrypted BYTEA NULL,
    refresh_token_encrypted BYTEA NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (platform, external_id),
    UNIQUE (account_id, platform)
);

CREATE INDEX IF NOT EXISTS linked_identities_account_idx ON linked_identities (account_id);
