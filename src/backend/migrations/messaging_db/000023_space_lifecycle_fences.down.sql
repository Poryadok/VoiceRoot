DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM messaging_space_lifecycle_operations)
       OR EXISTS (SELECT 1 FROM messaging_space_lifecycle_fences WHERE state <> 'LIVE') THEN
        RAISE EXCEPTION 'refusing to drop admitted Messaging Space lifecycle fences';
    END IF;
END $$;

DROP TRIGGER messaging_space_chat_mutation_gate ON messages;
DROP FUNCTION messaging_guard_space_chat_mutation();
DROP TABLE messaging_space_lifecycle_operations;
