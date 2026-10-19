BEGIN;
-- A downgrade may be impossible after multiple blank-token messages. Refuse
-- transactionally; never delete events to recreate the old constraint.
DROP INDEX bot_event_log_interaction_token_idx;
CREATE UNIQUE INDEX bot_event_log_interaction_token_idx
    ON bot_event_log (bot_id, interaction_token)
    WHERE interaction_token IS NOT NULL;
COMMIT;
