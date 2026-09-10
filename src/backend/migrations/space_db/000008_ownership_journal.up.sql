-- Additive v2 ownership evidence. Production callers remain disabled until
-- coordinator, authority fences and finalization are integrated.
-- No cascading FK: deleting a space must not delete an operation tombstone.
CREATE TABLE ownership_journal (
    operation_id UUID PRIMARY KEY,
    protocol_version INTEGER NOT NULL CHECK (protocol_version = 2),
    space_id UUID NOT NULL,
    account_id UUID NOT NULL,
    actor_profile_id UUID NOT NULL,
    new_owner_profile_id UUID NOT NULL,
    session_epoch BIGINT NOT NULL CHECK (session_epoch > 0),
    proof_digest TEXT NOT NULL CHECK (proof_digest ~ '^[0-9a-f]{64}$'),
    binding_bytes BYTEA NOT NULL,
    binding_hash BYTEA NOT NULL CHECK (octet_length(binding_hash) = 32),
    state TEXT NOT NULL DEFAULT 'reserved' CHECK (state IN (
        'reserved', 'proof_confirmed', 'prepared', 'commit_decided',
        'abort_decided', 'completed', 'aborted'
    )),
    audit_id UUID NOT NULL,
    event_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (actor_profile_id <> new_owner_profile_id),
    CHECK (audit_id <> event_id)
);

CREATE UNIQUE INDEX ownership_journal_one_active_space
    ON ownership_journal (space_id)
    WHERE state NOT IN ('completed', 'aborted');
