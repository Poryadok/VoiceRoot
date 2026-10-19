ALTER TABLE chats ADD COLUMN member_additions_enabled BOOLEAN NOT NULL DEFAULT true;
-- The grant disappears atomically with its membership, including profile cleanup.
-- Historical role/feed/settings values never backfill this separate capability.
ALTER TABLE chat_members ADD COLUMN post_as_chat BOOLEAN NOT NULL DEFAULT false;
