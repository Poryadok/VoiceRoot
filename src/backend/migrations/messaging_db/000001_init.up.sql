-- messaging_db v1 — docs/microservices/messaging-service.md
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE messages (
    id UUID PRIMARY KEY,
    chat_id UUID NOT NULL,
    chat_type VARCHAR(16) NOT NULL CHECK (chat_type = 'dm'),
    sender_profile_id UUID NOT NULL,
    posted_as_chat BOOLEAN NOT NULL DEFAULT false CHECK (posted_as_chat = false),
    display_chat_id UUID NULL,
    content TEXT NOT NULL CHECK (char_length(content) BETWEEN 1 AND 4000),
    type VARCHAR(16) NOT NULL DEFAULT 'regular' CHECK (type IN ('regular', 'system', 'forward')),
    thread_parent_id UUID NULL,
    forward_from_id UUID NULL,
    forward_from_sender TEXT NULL,
    attachments JSONB NOT NULL DEFAULT '[]'::jsonb,
    mentions JSONB NOT NULL DEFAULT '[]'::jsonb,
    edited_at TIMESTAMPTZ NULL,
    deleted_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX messages_chat_id_id_desc_idx ON messages (chat_id, id DESC);
CREATE INDEX messages_sender_profile_id_idx ON messages (sender_profile_id, created_at DESC);
CREATE INDEX messages_chat_id_created_at_idx ON messages (chat_id, created_at DESC);

CREATE TABLE read_receipts (
    chat_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    last_read_message_id UUID NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (chat_id, profile_id)
);

CREATE INDEX read_receipts_profile_id_idx ON read_receipts (profile_id);
