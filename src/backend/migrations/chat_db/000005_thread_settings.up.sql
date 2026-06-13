-- Phase 10 threads — docs/features/text-chat.md
ALTER TABLE chats
    ADD COLUMN IF NOT EXISTS threads_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS allow_user_main_feed BOOLEAN NOT NULL DEFAULT true;

-- Channel defaults: thread-oriented, no user main-feed posts.
UPDATE chats
SET threads_enabled = true, allow_user_main_feed = false
WHERE type = 'channel';
