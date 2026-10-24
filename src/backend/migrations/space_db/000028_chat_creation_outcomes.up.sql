-- Private evidence has no ordinary TTL. Terminal P3 fixes a separate 30-day
-- deadline; compact resource fences survive evidence and domain-row deletion.
CREATE TABLE space_chat_creation_fences (
 operation_id UUID PRIMARY KEY,
 protocol_version INTEGER NOT NULL CHECK(protocol_version=1),
 execution_kind TEXT NOT NULL DEFAULT 'space_composite_v1' CHECK(execution_kind='space_composite_v1'),
 space_id UUID NOT NULL,
 chat_id UUID NOT NULL UNIQUE,
 chat_type INTEGER NOT NULL CHECK(chat_type IN (2,3)),
 node_id UUID NOT NULL UNIQUE,
 audit_event_id UUID NOT NULL UNIQUE,
 terminal_state INTEGER NOT NULL CHECK(terminal_state IN (1,2)),
 failure_code INTEGER NOT NULL,
 deletion_operation_id UUID,
 purge_generation BIGINT,
 purge_completed_at TIMESTAMPTZ,
 full_evidence_expires_at TIMESTAMPTZ,
 CHECK((terminal_state=1 AND failure_code=0) OR (terminal_state=2 AND failure_code IN (5,7,8,9))),
 CHECK((deletion_operation_id IS NULL)=(purge_generation IS NULL)),
 CHECK((deletion_operation_id IS NULL)=(purge_completed_at IS NULL)),
 CHECK((deletion_operation_id IS NULL)=(full_evidence_expires_at IS NULL)),
 CHECK(purge_generation IS NULL OR purge_generation>0),
 CHECK(full_evidence_expires_at IS NULL OR full_evidence_expires_at=purge_completed_at+interval '30 days')
);
CREATE INDEX space_chat_creation_fences_space_idx ON space_chat_creation_fences(space_id);
CREATE TABLE space_chat_creation_evidence (
 operation_id UUID PRIMARY KEY REFERENCES space_chat_creation_fences(operation_id),
 outcome_bytes BYTEA NOT NULL CHECK(octet_length(outcome_bytes)>0)
);
-- Contains no account/profile/actor, content or identity-derived hash.
CREATE TABLE space_permanent_purge_fences (
 space_id UUID PRIMARY KEY,
 deletion_operation_id UUID NOT NULL,
 purge_generation BIGINT NOT NULL CHECK(purge_generation>0),
 purged_at TIMESTAMPTZ NOT NULL
);
