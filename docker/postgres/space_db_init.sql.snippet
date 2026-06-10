-- space_db v1 — docs/microservices/space-service.md (Phase 5: space creation)
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE spaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    icon_url TEXT NULL,
    banner_url TEXT NULL,
    visibility VARCHAR(16) NOT NULL DEFAULT 'private' CHECK (visibility IN ('public', 'invite_only', 'private')),
    owner_profile_id UUID NOT NULL,
    member_count INTEGER NOT NULL DEFAULT 1 CHECK (member_count >= 0),
    is_verified BOOLEAN NOT NULL DEFAULT false,
    verification_type VARCHAR(16) NOT NULL DEFAULT 'none' CHECK (verification_type IN ('none', 'personal', 'organization')),
    entry_requirement VARCHAR(16) NOT NULL DEFAULT 'none' CHECK (entry_requirement IN ('none', 'phone', 'captcha', 'questions', 'manual')),
    entry_questions JSONB NULL,
    mm_config JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX spaces_owner_profile_id_idx ON spaces (owner_profile_id);
CREATE INDEX spaces_public_name_idx ON spaces (name) WHERE visibility = 'public';

CREATE TABLE space_members (
    space_id UUID NOT NULL REFERENCES spaces (id) ON DELETE CASCADE,
    profile_id UUID NOT NULL,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    nickname TEXT NULL,
    PRIMARY KEY (space_id, profile_id)
);

CREATE INDEX space_members_profile_id_idx ON space_members (profile_id, joined_at DESC);
