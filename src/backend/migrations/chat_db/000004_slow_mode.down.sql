ALTER TABLE chats DROP CONSTRAINT IF EXISTS chats_slow_mode_seconds_check;
ALTER TABLE chats ADD CONSTRAINT chats_slow_mode_seconds_check CHECK (slow_mode_seconds = 0);
