DELETE FROM managed_chat_operations WHERE method='retention';

ALTER TABLE managed_chat_operations
    DROP CONSTRAINT managed_chat_operations_method_check,
    ADD CONSTRAINT managed_chat_operations_method_check
        CHECK (method IN ('create', 'sync_members'));

DROP TABLE IF EXISTS managed_chat_retention;
