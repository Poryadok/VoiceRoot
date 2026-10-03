DELETE FROM managed_chat_operations WHERE method='add_members';

ALTER TABLE managed_chat_operations
    DROP CONSTRAINT managed_chat_operations_method_check,
    ADD CONSTRAINT managed_chat_operations_method_check
        CHECK (method IN ('create', 'sync_members', 'retention'));
