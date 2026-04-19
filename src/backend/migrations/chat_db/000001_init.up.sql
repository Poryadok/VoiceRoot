-- chat_db v1 DM — docs/microservices/chat-service.md
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE chats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(16) NOT NULL CHECK (type = 'dm'),
    space_id UUID NULL,
    name TEXT NULL,
    avatar_url TEXT NULL,
    topic TEXT NULL,
    creator_profile_id UUID NOT NULL,
    slow_mode_seconds INTEGER NOT NULL DEFAULT 0 CHECK (slow_mode_seconds = 0),
    last_message_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX chats_last_message_at_idx ON chats (last_message_at DESC);
CREATE INDEX chats_creator_profile_id_idx ON chats (creator_profile_id);

CREATE TABLE chat_members (
    chat_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    role VARCHAR(16) NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'admin', 'member')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    muted_until TIMESTAMPTZ NULL,
    is_archived BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (chat_id, profile_id)
);

CREATE INDEX chat_members_profile_id_idx ON chat_members (profile_id, joined_at DESC);
