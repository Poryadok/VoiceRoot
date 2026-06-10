-- Phase 6 pins: per-chat pinned messages (max 50 enforced in application).
CREATE TABLE pins (
    chat_id UUID NOT NULL,
    message_id UUID NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    pinned_by UUID NOT NULL,
    pinned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (chat_id, message_id)
);

CREATE INDEX pins_chat_id_pinned_at_desc_idx ON pins (chat_id, pinned_at DESC);
