-- space_db v3 — invites (docs/microservices/space-service.md)

CREATE TABLE invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL REFERENCES spaces (id) ON DELETE CASCADE,
    code VARCHAR(32) NOT NULL UNIQUE,
    creator_profile_id UUID NOT NULL,
    max_uses INTEGER NULL,
    use_count INTEGER NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ NULL
);

CREATE INDEX invites_space_id_idx ON invites (space_id);
CREATE INDEX invites_code_idx ON invites (code);
