-- Draining epoch-aware replicas is required. An old binary does not preserve
-- credential revocation; downgrade is an explicitly controlled recovery action.
DROP INDEX IF EXISTS bot_event_session_idx;
DROP INDEX IF EXISTS bot_autocomplete_result_idx;
ALTER TABLE bot_event_log DROP COLUMN session_epoch;
ALTER TABLE bot_event_log DROP COLUMN interaction_completed_at;
ALTER TABLE bots DROP COLUMN session_epoch;
