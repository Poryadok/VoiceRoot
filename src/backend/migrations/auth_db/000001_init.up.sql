-- auth_db v1 — see docs/microservices/auth-service.md (V1 DDL)
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(320) NULL,
    phone VARCHAR(32) NULL,
    password_hash TEXT NOT NULL,
    type VARCHAR(16) NOT NULL CHECK (type IN ('regular', 'guest')),
    status VARCHAR(16) NOT NULL CHECK (status IN ('active', 'suspended', 'deleted')),
    totp_secret BYTEA NULL,
    totp_enabled BOOLEAN NOT NULL DEFAULT false,
    deleted_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX accounts_email_uq ON accounts (email) WHERE email IS NOT NULL;
CREATE UNIQUE INDEX accounts_phone_uq ON accounts (phone) WHERE phone IS NOT NULL;

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL,
    token_hash CHAR(64) NOT NULL,
    device_info JSONB NOT NULL DEFAULT '{}'::jsonb,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ NULL
);

CREATE INDEX refresh_tokens_account_active_idx ON refresh_tokens (account_id, expires_at DESC)
    WHERE revoked_at IS NULL;
CREATE INDEX refresh_tokens_token_hash_idx ON refresh_tokens (token_hash);

CREATE TABLE otp_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL,
    code BYTEA NOT NULL,
    type VARCHAR(32) NOT NULL CHECK (type IN ('email_verify', 'password_reset')),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX otp_codes_account_type_idx ON otp_codes (account_id, type, expires_at DESC);
