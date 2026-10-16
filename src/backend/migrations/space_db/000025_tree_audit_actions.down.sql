-- Preserve append-only tree audit and outbox evidence during downgrade.
BEGIN;

LOCK TABLE audit_log IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM audit_log
        WHERE action IN ('tree_node_upserted','tree_node_removed','tree_reordered')
    ) THEN
        RAISE EXCEPTION 'cannot remove Space tree audit registry actions while audit evidence exists';
    END IF;
END
$$;

ALTER TABLE audit_log
    DROP CONSTRAINT audit_log_closed_registry,
    ADD CONSTRAINT audit_log_closed_registry CHECK (
        jsonb_typeof(details) = 'object'
        AND (
            (action = 'invite_revoked' AND target_type = 'invite' AND details = '{}'::jsonb)
            OR (action = 'member_kicked' AND target_type = 'profile' AND details = '{}'::jsonb)
            OR (
                action = 'member_banned' AND target_type = 'account'
                AND details - 'reason' = '{}'::jsonb
                AND (NOT details ? 'reason' OR jsonb_typeof(details -> 'reason') = 'string')
            )
            OR (action = 'member_unbanned' AND target_type = 'account' AND details = '{}'::jsonb)
            OR (
                action = 'member_timed_out' AND target_type = 'profile'
                AND details - ARRAY['duration_seconds','reason'] = '{}'::jsonb
                AND details ? 'duration_seconds'
                AND jsonb_typeof(details -> 'duration_seconds') = 'number'
                AND (NOT details ? 'reason' OR jsonb_typeof(details -> 'reason') = 'string')
            )
            OR (action = 'member_timeout_removed' AND target_type = 'profile' AND details = '{}'::jsonb)
            OR (action = 'ownership_transferred' AND target_type = 'profile' AND details = '{}'::jsonb)
        )
    );

COMMIT;
