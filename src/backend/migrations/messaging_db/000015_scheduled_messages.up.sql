-- Durable schedule state. Worker retry/claim metadata remains internal to Messaging.
CREATE TABLE scheduled_messages (
    id UUID PRIMARY KEY,
    chat_id UUID NOT NULL,
    sender_profile_id UUID NOT NULL,
    client_message_id UUID,
    payload JSONB NOT NULL,
    schedule_kind TEXT NOT NULL CHECK (schedule_kind IN ('at', 'when_online')),
    scheduled_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'cancelled', 'failed')),
    sent_message_id UUID,
    dispatch_event_id UUID,
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    next_attempt_at TIMESTAMPTZ,
    last_error_code TEXT,
    failed_at TIMESTAMPTZ,
    dispatch_lease_owner TEXT,
    dispatch_lease_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT scheduled_messages_schedule_shape CHECK (
        (schedule_kind = 'at' AND scheduled_at IS NOT NULL)
        OR (schedule_kind = 'when_online' AND scheduled_at IS NULL)
    ),
    CONSTRAINT scheduled_messages_dispatch_lease_pair CHECK (
        (dispatch_lease_owner IS NULL) = (dispatch_lease_expires_at IS NULL)
    ),
    CONSTRAINT scheduled_messages_terminal_shape CHECK (
        (status = 'pending' AND sent_message_id IS NULL AND dispatch_event_id IS NULL AND failed_at IS NULL)
        OR (status = 'cancelled' AND sent_message_id IS NULL AND dispatch_event_id IS NULL AND failed_at IS NULL)
        OR (status = 'failed' AND sent_message_id IS NULL AND dispatch_event_id IS NULL AND failed_at IS NOT NULL)
        OR (status = 'sent' AND sent_message_id IS NOT NULL AND dispatch_event_id IS NOT NULL AND failed_at IS NULL)
    )
);

CREATE UNIQUE INDEX scheduled_messages_client_message_id_uq
    ON scheduled_messages (chat_id, sender_profile_id, client_message_id)
    WHERE client_message_id IS NOT NULL;

-- Future pollers can claim only due time schedules or released retry work.
CREATE INDEX scheduled_messages_due_pending_idx
    ON scheduled_messages (next_attempt_at, scheduled_at, id)
    WHERE status = 'pending' AND schedule_kind = 'at';

CREATE INDEX scheduled_messages_owner_pending_idx
    ON scheduled_messages (chat_id, sender_profile_id, created_at, id)
    WHERE status = 'pending';
