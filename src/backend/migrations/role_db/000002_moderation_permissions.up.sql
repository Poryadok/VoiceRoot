-- Phase 5 moderation: bits 20–22 (TEXT_CHAT_MANAGE_SETTINGS, TEXT_CHAT_SET_SLOW_MODE, MODERATION_TIMEOUT_MEMBERS).
UPDATE roles
SET
    permissions = permissions | ((1::bigint << 20) | (1::bigint << 21) | (1::bigint << 22)),
    updated_at = now()
WHERE
    is_system = true
    AND name IN ('Owner', 'Admin', 'Moderator');
