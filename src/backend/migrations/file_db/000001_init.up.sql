CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS files (
    id UUID PRIMARY KEY,
    uploader_profile_id UUID NOT NULL,
    original_name TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 52428800),
    sha256_hash TEXT NULL,
    r2_key TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL CHECK (status IN ('pending_upload', 'processing', 'ready', 'failed', 'deleted', 'expired')),
    file_type TEXT NOT NULL CHECK (file_type IN ('image', 'video', 'audio', 'document', 'other')),
    width INTEGER NULL,
    height INTEGER NULL,
    duration_seconds INTEGER NULL,
    thumbnail_r2_key TEXT NULL,
    converted_r2_key TEXT NULL,
    chat_id UUID NULL,
    chat_type TEXT NULL CHECK (chat_type IS NULL OR chat_type IN ('dm', 'group', 'channel')),
    is_e2e BOOLEAN NOT NULL DEFAULT false,
    expires_at TIMESTAMPTZ NULL,
    scan_result TEXT NOT NULL CHECK (scan_result IN ('pending', 'clean', 'infected', 'error', 'skipped')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS files_uploader_profile_id_idx
    ON files (uploader_profile_id, created_at DESC);

CREATE INDEX IF NOT EXISTS files_chat_id_created_at_idx
    ON files (chat_id, created_at DESC)
    WHERE chat_id IS NOT NULL;
