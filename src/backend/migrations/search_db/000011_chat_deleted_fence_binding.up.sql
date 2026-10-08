-- Retain only the immutable, non-payload binding needed to validate a terminal
-- ChatDeleted event after the larger P3 manifest has expired.
ALTER TABLE search_space_purged_chat_fences
    ADD COLUMN deletion_operation_id UUID,
    ADD COLUMN event_generation BIGINT,
    ADD COLUMN manifest_id TEXT,
    ADD COLUMN manifest_sha256 BYTEA,
    ADD CONSTRAINT search_purged_chat_binding_complete_or_legacy CHECK (
        (deletion_operation_id IS NULL AND event_generation IS NULL AND manifest_id IS NULL AND manifest_sha256 IS NULL)
        OR
        (deletion_operation_id IS NOT NULL AND event_generation IS NOT NULL AND event_generation >= 2
            AND manifest_id IS NOT NULL AND manifest_id <> ''
            AND manifest_sha256 IS NOT NULL AND octet_length(manifest_sha256) = 32)
    );
