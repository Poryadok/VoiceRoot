ALTER TABLE bot_event_log
    ADD COLUMN next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN claimed_until TIMESTAMPTZ;

CREATE INDEX bot_event_log_slash_due_idx
    ON bot_event_log (next_attempt_at, created_at)
    WHERE event_type = 'interaction' AND delivery_status = 'pending';

CREATE UNIQUE INDEX bot_event_log_interaction_token_idx
    ON bot_event_log (bot_id, interaction_token)
    WHERE interaction_token IS NOT NULL;
