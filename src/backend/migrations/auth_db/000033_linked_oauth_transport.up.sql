BEGIN;
-- Disabled-by-default linked-provider transport. No source is created by a callback.
CREATE TABLE linked_oauth_attempts (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES accounts(id),
    profile_id UUID NOT NULL,
    provider TEXT NOT NULL CHECK (provider IN ('twitch', 'youtube')),
    original_jti TEXT NOT NULL,
    original_epoch BIGINT NOT NULL CHECK (original_epoch > 0),
    original_exp TIMESTAMPTZ NOT NULL,
    subscription_tier TEXT NOT NULL,
    registration_revision TEXT NOT NULL,
    callback_uri TEXT NOT NULL,
    return_uri TEXT NOT NULL,
    client_surface TEXT NOT NULL,
    state_hash TEXT NOT NULL UNIQUE,
    return_challenge TEXT NOT NULL,
    provider_secret BYTEA,
    expires_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending','processing','ready','consumed','cancelled','superseded','expired')),
    result_hash TEXT UNIQUE,
    result_expires_at TIMESTAMPTZ,
    result_secret BYTEA,
    outcome TEXT CHECK (outcome IN ('success','cancelled','denied','provider_error','unavailable')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX linked_oauth_attempt_owner ON linked_oauth_attempts(account_id, original_jti, profile_id, provider);
CREATE INDEX linked_oauth_attempt_cleanup ON linked_oauth_attempts(expires_at) WHERE status NOT IN ('consumed','cancelled','superseded','expired');
COMMIT;
