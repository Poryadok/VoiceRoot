DROP INDEX IF EXISTS bot_event_log_interaction_token_idx;
DROP INDEX IF EXISTS bot_event_log_slash_due_idx;
ALTER TABLE bot_event_log DROP COLUMN IF EXISTS claimed_until, DROP COLUMN IF EXISTS next_attempt_at;
