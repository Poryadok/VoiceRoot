-- Durable send option for Notification consumption from message.sent.
-- Existing and legacy rows retain the documented false default.
ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS send_silent BOOLEAN NOT NULL DEFAULT false;
