BEGIN;
LOCK TABLE space_entry_policy_operations,space_entry_policies IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM space_entry_policy_operations) THEN
        RAISE EXCEPTION 'entry policy durable operation evidence prevents downgrade';
    END IF;
END $$;
DROP TABLE space_entry_policy_operations;
DROP TABLE space_entry_policies;
COMMIT;
