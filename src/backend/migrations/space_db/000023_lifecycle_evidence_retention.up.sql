-- Compact coordinator evidence contains no account/profile IDs or wire bodies.
-- READY delivery must outlive tombstone expiry without retaining account HMACs.
ALTER TABLE space_lifecycle_outbox DROP CONSTRAINT space_lifecycle_outbox_space_id_fkey;
ALTER TABLE space_lifecycle_outbox ADD COLUMN purge_decided_at TIMESTAMPTZ;
UPDATE space_lifecycle_outbox o SET purge_decided_at=a.purge_decided_at FROM space_lifecycle_aggregates a WHERE a.space_id=o.space_id AND a.deletion_operation_id=o.deletion_operation_id AND o.event_type='space.deleted';
CREATE INDEX space_lifecycle_delivered_expiry_idx ON space_lifecycle_outbox(delivered_at) WHERE state='DELIVERED';
CREATE TABLE space_lifecycle_completions (
 space_id UUID NOT NULL REFERENCES space_lifecycle_aggregates(space_id) ON DELETE CASCADE,
 deletion_operation_id UUID NOT NULL,
 generation BIGINT NOT NULL CHECK(generation>0),
 participant_id SMALLINT NOT NULL CHECK(participant_id BETWEEN 1 AND 10),
 request_kind TEXT NOT NULL CHECK(request_kind IN ('PURGE','ROLE_RETIREMENT')),
 request_sha256 BYTEA NOT NULL CHECK(octet_length(request_sha256)=32),
 receipt_sha256 BYTEA NOT NULL CHECK(octet_length(receipt_sha256)=32),
 manifest_sha256 BYTEA NOT NULL CHECK(octet_length(manifest_sha256)=32),
 completed_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(space_id,generation,participant_id),
 CHECK((participant_id=1 AND request_kind='ROLE_RETIREMENT') OR (participant_id<>1 AND request_kind='PURGE'))
);
ALTER TABLE space_lifecycle_aggregates ADD COLUMN evidence_compacted BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE space_lifecycle_aggregates ADD CONSTRAINT space_lifecycle_compacted_terminal CHECK(NOT evidence_compacted OR (phase='PURGED' AND local_purge_completed));
CREATE INDEX space_lifecycle_evidence_expiry_idx ON space_lifecycle_aggregates(completed_at,space_id) WHERE phase='PURGED' AND NOT evidence_compacted;
