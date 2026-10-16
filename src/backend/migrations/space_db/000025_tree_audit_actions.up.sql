-- BE-A2-AUDIT-TREE-WRITERS-SCHEMA: extend the closed Space audit registry
-- without rewriting the shipped 000015 migration or existing audit evidence.
BEGIN;

LOCK TABLE audit_log IN ACCESS EXCLUSIVE MODE;

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
            OR (
                action = 'tree_node_upserted' AND target_type = 'tree_node'
                AND details - ARRAY['kind','chat_id','voice_room_id','category_id','sort_order','is_pinned','pin_order'] = '{}'::jsonb
                AND details ?& ARRAY['kind','sort_order','is_pinned']
                AND jsonb_typeof(details -> 'kind') = 'string'
                AND jsonb_typeof(details -> 'sort_order') = 'number'
                AND (details ->> 'sort_order')::numeric = trunc((details ->> 'sort_order')::numeric)
                AND (details ->> 'sort_order')::numeric BETWEEN -2147483648 AND 2147483647
                AND jsonb_typeof(details -> 'is_pinned') = 'boolean'
                AND (
                    (details ->> 'kind' = 'text_chat'
                     AND details ? 'chat_id' AND NOT details ? 'voice_room_id'
                     AND details ->> 'chat_id' ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')
                    OR
                    (details ->> 'kind' = 'voice_room'
                     AND details ? 'voice_room_id' AND NOT details ? 'chat_id'
                     AND details ->> 'voice_room_id' ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')
                )
                AND (NOT details ? 'category_id' OR
                    (jsonb_typeof(details -> 'category_id') = 'string'
                     AND details ->> 'category_id' ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'))
                AND (NOT details ? 'pin_order' OR
                    (jsonb_typeof(details -> 'pin_order') = 'number'
                     AND (details ->> 'pin_order')::numeric = trunc((details ->> 'pin_order')::numeric)
                     AND (details ->> 'pin_order')::numeric BETWEEN 0 AND 2147483647
                     AND details ->> 'is_pinned' = 'true'))
            )
            OR (action = 'tree_node_removed' AND target_type = 'tree_node' AND details = '{}'::jsonb)
            OR (
                action = 'tree_reordered' AND target_type = 'space'
                AND details - 'count' = '{}'::jsonb
                AND details ? 'count'
                AND jsonb_typeof(details -> 'count') = 'number'
                AND (details ->> 'count')::numeric = trunc((details ->> 'count')::numeric)
                AND (details ->> 'count')::numeric BETWEEN 1 AND 500
            )
        )
    );

COMMIT;
