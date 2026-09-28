DROP TABLE managed_chat_operations;
DROP INDEX chats_managed_resource_key_uq;
ALTER TABLE chats
    DROP CONSTRAINT chats_managed_ownership_check,
    DROP COLUMN external_chat_key,
    DROP COLUMN managed_environment_id,
    DROP COLUMN managed_by_application_id;

-- The migration down path is only valid after all managed chats have been
-- retired or removed because the original schema required a human creator.
ALTER TABLE chats ALTER COLUMN creator_profile_id SET NOT NULL;
