CREATE TABLE bot_message_deliveries (
    id UUID PRIMARY KEY,
    bot_id UUID NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
    message_id UUID NOT NULL,
    chat_id UUID NOT NULL,
    payload JSONB NOT NULL,
    is_polling_mode BOOLEAN NOT NULL,
    webhook_url TEXT,
    webhook_secret TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'delivered')),
    attempts INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    claimed_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ,
    UNIQUE (bot_id, message_id)
);

CREATE INDEX bot_message_deliveries_due_idx
    ON bot_message_deliveries (next_attempt_at, created_at)
    WHERE status = 'pending';

ALTER TABLE bot_event_log ADD COLUMN source_message_id UUID;
CREATE UNIQUE INDEX bot_event_log_message_once_idx
    ON bot_event_log (bot_id, source_message_id)
    WHERE source_message_id IS NOT NULL;
