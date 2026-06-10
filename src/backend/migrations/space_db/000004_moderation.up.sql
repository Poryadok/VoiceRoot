-- Phase 5 space moderation — docs/microservices/space-service.md
CREATE TABLE space_bans (
    space_id UUID NOT NULL REFERENCES spaces (id) ON DELETE CASCADE,
    account_id UUID NOT NULL,
    banned_by_profile_id UUID NOT NULL,
    reason TEXT NULL,
    banned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (space_id, account_id)
);

CREATE INDEX space_bans_space_id_idx ON space_bans (space_id, banned_at DESC);

CREATE TABLE space_member_timeouts (
    space_id UUID NOT NULL REFERENCES spaces (id) ON DELETE CASCADE,
    profile_id UUID NOT NULL,
    timed_out_until TIMESTAMPTZ NOT NULL,
    timed_out_by_profile_id UUID NOT NULL,
    reason TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (space_id, profile_id)
);

CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL REFERENCES spaces (id) ON DELETE CASCADE,
    actor_profile_id UUID NOT NULL,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id UUID NOT NULL,
    details JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX audit_log_space_created_idx ON audit_log (space_id, created_at DESC);
