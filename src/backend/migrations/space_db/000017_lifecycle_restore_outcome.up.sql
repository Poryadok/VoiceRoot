-- A distinct restore operation shares the global lifecycle operation ID space.
ALTER TABLE space_lifecycle_operations
    DROP CONSTRAINT space_lifecycle_operations_method_check,
    DROP CONSTRAINT space_lifecycle_operations_state_check,
    ALTER COLUMN confirmation_name_sha256 DROP NOT NULL,
    ALTER COLUMN proof_digest_sha256 DROP NOT NULL,
    ADD COLUMN deletion_operation_id UUID,
    ADD COLUMN generation BIGINT,
    ADD CONSTRAINT space_lifecycle_operations_method_check CHECK (method IN ('DELETE','RESTORE')),
    ADD CONSTRAINT space_lifecycle_operations_state_check CHECK (state IN ('SCHEDULE_PENDING','RESTORE_PENDING','COMPLETED')),
    ADD CONSTRAINT space_lifecycle_operations_method_shape CHECK (
        (method='DELETE' AND confirmation_name_sha256 IS NOT NULL AND proof_digest_sha256 IS NOT NULL
         AND deletion_operation_id IS NULL AND generation IS NULL AND state IN ('SCHEDULE_PENDING','COMPLETED'))
        OR
        (method='RESTORE' AND confirmation_name_sha256 IS NULL AND proof_digest_sha256 IS NULL
         AND auth_receipt_bytes IS NULL AND auth_receipt_sha256 IS NULL
         AND deletion_operation_id IS NOT NULL AND deletion_operation_id<>operation_id
         AND generation IS NOT NULL AND generation>0 AND state IN ('RESTORE_PENDING','COMPLETED')
         AND (state='COMPLETED' OR (outcome_bytes IS NULL AND completed_at IS NULL)))
    );
CREATE UNIQUE INDEX space_lifecycle_restore_generation_idx
    ON space_lifecycle_operations(space_id,deletion_operation_id,generation) WHERE method='RESTORE';
