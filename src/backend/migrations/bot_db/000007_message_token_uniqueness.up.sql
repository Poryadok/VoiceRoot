BEGIN;
-- Empty legacy message tokens carry no interaction identity. Keep real slash
-- tokens unique while allowing multiple independent polling message events.
DROP INDEX bot_event_log_interaction_token_idx;
CREATE UNIQUE INDEX bot_event_log_interaction_token_idx
    ON bot_event_log (bot_id, interaction_token)
    WHERE interaction_token IS NOT NULL AND interaction_token <> '';
COMMIT;
