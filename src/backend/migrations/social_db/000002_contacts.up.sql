-- social_db v2 — contacts + favorites (docs/microservices/social-service.md)
CREATE TABLE contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_profile_id UUID NOT NULL,
    contact_profile_id UUID NOT NULL,
    source VARCHAR(32) NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'phone_sync', 'space', 'matchmaking')),
    is_favorite BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX contacts_owner_contact_uq ON contacts (owner_profile_id, contact_profile_id);
CREATE INDEX contacts_owner_favorite_idx ON contacts (owner_profile_id, is_favorite, updated_at DESC);
