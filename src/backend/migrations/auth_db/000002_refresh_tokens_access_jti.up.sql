-- Pair opaque refresh row with access token jti (logout / rotation blacklist)
ALTER TABLE refresh_tokens
    ADD COLUMN IF NOT EXISTS access_jti VARCHAR(128) NOT NULL DEFAULT '';
