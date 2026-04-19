-- user_db v1 — docs/microservices/user-service.md (profiles, onboarding_state)
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL,
    username VARCHAR(32) NOT NULL,
    discriminator CHAR(4) NOT NULL CHECK (discriminator ~ '^[0-9]{4}$'),
    display_name VARCHAR(64) NOT NULL,
    avatar_url TEXT NULL,
    bio TEXT NULL CHECK (bio IS NULL OR char_length(bio) <= 500),
    locale VARCHAR(8) NOT NULL DEFAULT 'ru' CHECK (locale IN ('ru', 'en')),
    theme VARCHAR(32) NOT NULL DEFAULT 'dark' CHECK (theme IN ('light', 'dark', 'high_contrast')),
    is_primary BOOLEAN NOT NULL DEFAULT true,
    verification_type VARCHAR(32) NOT NULL DEFAULT 'none' CHECK (verification_type IN ('none', 'personal', 'organization')),
    verification_badge VARCHAR(32) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX profiles_username_discriminator_uq ON profiles (username, discriminator);
CREATE UNIQUE INDEX profiles_one_primary_per_account_uq ON profiles (account_id) WHERE is_primary = true;
CREATE INDEX profiles_account_id_idx ON profiles (account_id);
CREATE INDEX profiles_created_at_idx ON profiles (created_at DESC);

CREATE TABLE onboarding_state (
    profile_id UUID PRIMARY KEY,
    completed_steps JSONB NOT NULL DEFAULT '[]'::jsonb,
    completed BOOLEAN NOT NULL DEFAULT false,
    completed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
