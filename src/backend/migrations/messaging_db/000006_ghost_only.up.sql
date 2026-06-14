-- Phase 14: shadow-banned senders' messages visible only to sender (ghost delivery).
ALTER TABLE messages ADD COLUMN IF NOT EXISTS ghost_only BOOLEAN NOT NULL DEFAULT false;
