ALTER TABLE chats
    DROP COLUMN IF EXISTS allow_user_main_feed,
    DROP COLUMN IF EXISTS threads_enabled;
