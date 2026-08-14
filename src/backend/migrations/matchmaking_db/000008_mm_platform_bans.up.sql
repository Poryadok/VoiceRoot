-- Platform MM bans from Moderation (mm_ban sanction); distinct from peer mm_bans.

CREATE TABLE mm_platform_bans (
    account_id UUID PRIMARY KEY,
    reason TEXT NOT NULL,
    banned_by_profile_id UUID,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX mm_platform_bans_active_idx
    ON mm_platform_bans (account_id)
    WHERE revoked_at IS NULL;
