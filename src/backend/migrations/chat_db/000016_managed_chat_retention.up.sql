CREATE TABLE managed_chat_retention (
    chat_id UUID PRIMARY KEY REFERENCES chats(id) ON DELETE CASCADE,
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    purge_after TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX managed_chat_retention_deadline_idx
    ON managed_chat_retention (purge_after, chat_id);

ALTER TABLE managed_chat_operations
    DROP CONSTRAINT managed_chat_operations_method_check,
    ADD CONSTRAINT managed_chat_operations_method_check
        CHECK (method IN ('create', 'sync_members', 'retention'));
