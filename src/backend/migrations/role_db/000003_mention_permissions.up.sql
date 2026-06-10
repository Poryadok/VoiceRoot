-- Phase 6 mentions: bits 23–24 (TEXT_CHAT_MENTION_ALL_ONLINE, TEXT_CHAT_MENTION_ALL_IN_CHAT).
UPDATE roles
SET
    permissions = permissions | ((1::bigint << 23) | (1::bigint << 24)),
    updated_at = now()
WHERE
    is_system = true
    AND name IN ('Owner', 'Admin', 'Moderator');
