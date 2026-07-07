-- user_db v4 — Phase 13: soft-delete profiles, org DNS verification

ALTER TABLE profiles ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;

CREATE TABLE IF NOT EXISTS organization_verification_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL,
    domain TEXT NOT NULL,
    txt_token TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'verified', 'expired')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    verified_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS organization_verification_profile_idx
    ON organization_verification_requests (profile_id, created_at DESC);
