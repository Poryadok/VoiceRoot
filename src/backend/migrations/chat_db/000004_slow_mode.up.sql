-- Phase 5 slow mode — docs/ARCHITECTURE_REQUIREMENTS.md (5 sec – 6 h)
ALTER TABLE chats DROP CONSTRAINT IF EXISTS chats_slow_mode_seconds_check;
ALTER TABLE chats ADD CONSTRAINT chats_slow_mode_seconds_check
    CHECK (slow_mode_seconds = 0 OR (slow_mode_seconds >= 5 AND slow_mode_seconds <= 21600));
