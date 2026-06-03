ALTER TABLE messages
    DROP CONSTRAINT IF EXISTS messages_content_check;

ALTER TABLE messages
    ADD CONSTRAINT messages_content_check CHECK (char_length(content) BETWEEN 1 AND 4000);
