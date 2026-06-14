-- Phase 15: E2E ciphertext messages + Signal pre-key directory (docs/features/encryption.md).
ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS is_e2e BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS e2e_prekey_bundles (
    profile_id UUID PRIMARY KEY,
    bundle TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
