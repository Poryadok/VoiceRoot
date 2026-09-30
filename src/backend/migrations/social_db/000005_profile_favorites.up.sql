CREATE TABLE profile_favorites (
    owner_profile_id UUID NOT NULL,
    favorite_profile_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (owner_profile_id, favorite_profile_id),
    CHECK (owner_profile_id <> favorite_profile_id)
);

CREATE INDEX profile_favorites_owner_updated_idx
    ON profile_favorites (owner_profile_id, updated_at DESC, favorite_profile_id);

INSERT INTO profile_favorites (owner_profile_id, favorite_profile_id, created_at, updated_at)
SELECT owner_profile_id, contact_profile_id, created_at, updated_at
FROM contacts
WHERE is_favorite
ON CONFLICT (owner_profile_id, favorite_profile_id) DO NOTHING;
