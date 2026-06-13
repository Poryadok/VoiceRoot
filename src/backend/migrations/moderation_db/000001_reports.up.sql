-- Phase 11 moderation_db — docs/microservices/moderation-service.md
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_profile_id UUID NOT NULL,
    target_type VARCHAR(16) NOT NULL CHECK (target_type IN ('user', 'message', 'space')),
    target_id UUID NOT NULL,
    category VARCHAR(32) NOT NULL CHECK (category IN ('spam', 'harassment', 'offensive', 'fake', 'cheating', 'other')),
    description TEXT NULL,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'reviewing', 'resolved', 'dismissed')),
    assigned_to UUID NULL,
    resolved_at TIMESTAMPTZ NULL,
    resolution JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX reports_target_created_idx ON reports (target_type, target_id, created_at DESC);
CREATE INDEX reports_status_created_idx ON reports (status, created_at DESC);

CREATE TABLE auto_mod_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_profile_id UUID NOT NULL,
    trigger VARCHAR(32) NOT NULL,
    action VARCHAR(32) NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reverted_at TIMESTAMPTZ NULL
);

CREATE INDEX auto_mod_log_target_created_idx ON auto_mod_log (target_profile_id, created_at DESC);
