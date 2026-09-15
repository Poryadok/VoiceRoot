BEGIN;
LOCK TABLE space_lifecycle_operations IN ACCESS EXCLUSIVE MODE;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM space_lifecycle_operations WHERE method='RESTORE') THEN
        RAISE EXCEPTION 'cannot remove admitted Space restore evidence';
    END IF;
END
$$;
DROP INDEX space_lifecycle_restore_generation_idx;
ALTER TABLE space_lifecycle_operations
    DROP CONSTRAINT space_lifecycle_operations_method_shape,
    DROP CONSTRAINT space_lifecycle_operations_method_check,
    DROP CONSTRAINT space_lifecycle_operations_state_check,
    DROP COLUMN deletion_operation_id,
    DROP COLUMN generation,
    ALTER COLUMN confirmation_name_sha256 SET NOT NULL,
    ALTER COLUMN proof_digest_sha256 SET NOT NULL,
    ADD CONSTRAINT space_lifecycle_operations_method_check CHECK (method='DELETE'),
    ADD CONSTRAINT space_lifecycle_operations_state_check CHECK (state IN ('SCHEDULE_PENDING','COMPLETED'));
COMMIT;
