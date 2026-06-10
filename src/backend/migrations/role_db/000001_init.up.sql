-- role_db v1 — docs/microservices/role-service.md (Phase 5 space roles)
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL,
    name TEXT NOT NULL,
    color TEXT NULL,
    is_system BOOLEAN NOT NULL DEFAULT false,
    position INTEGER NOT NULL DEFAULT 0,
    permissions BIGINT NOT NULL DEFAULT 0,
    is_mentionable BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (space_id, name)
);

CREATE INDEX roles_space_id_position_idx ON roles (space_id, position DESC);

CREATE TABLE member_roles (
    space_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    role_id UUID NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    assigned_by UUID NOT NULL,
    PRIMARY KEY (space_id, profile_id, role_id)
);

CREATE INDEX member_roles_profile_idx ON member_roles (space_id, profile_id);

CREATE TABLE chat_overrides (
    chat_id UUID NOT NULL,
    role_id UUID NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    allow BIGINT NOT NULL DEFAULT 0,
    deny BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (chat_id, role_id)
);

CREATE TABLE voice_room_overrides (
    voice_room_id UUID NOT NULL,
    role_id UUID NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    allow BIGINT NOT NULL DEFAULT 0,
    deny BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (voice_room_id, role_id)
);
