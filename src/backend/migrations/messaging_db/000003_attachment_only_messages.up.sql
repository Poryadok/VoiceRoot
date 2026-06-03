ALTER TABLE messages
    DROP CONSTRAINT IF EXISTS messages_content_check;

ALTER TABLE messages
    ADD CONSTRAINT messages_content_check CHECK (char_length(content) <= 4000);
