-- Phase 14 moderation_db stub — sanctions, appeals, audit log (docs/microservices/moderation-service.md).

CREATE TABLE sanctions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_account_id UUID NOT NULL,
    type VARCHAR(32) NOT NULL CHECK (type IN ('warning', 'temp_ban', 'perm_ban', 'shadow_ban', 'mm_ban')),
    reason TEXT NOT NULL,
    report_id UUID NULL REFERENCES reports (id),
    issued_by UUID NOT NULL,
    expires_at TIMESTAMPTZ NULL,
    revoked_at TIMESTAMPTZ NULL,
    revoked_by UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sanctions_target_active_idx ON sanctions (target_account_id, created_at DESC)
    WHERE revoked_at IS NULL;

CREATE TABLE appeals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sanction_id UUID NOT NULL REFERENCES sanctions (id),
    appellant_account_id UUID NOT NULL,
    reason TEXT NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'denied')),
    reviewed_by UUID NULL,
    reviewed_at TIMESTAMPTZ NULL,
    review_notes TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (sanction_id)
);

CREATE TABLE moderation_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_profile_id UUID NOT NULL,
    action VARCHAR(64) NOT NULL,
    target_type VARCHAR(32) NOT NULL,
    target_id UUID NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX moderation_audit_log_created_idx ON moderation_audit_log (created_at DESC);
