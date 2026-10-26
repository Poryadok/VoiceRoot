-- An installation-time privilege fence, never account CURRENT certification.
-- Runtime roles can read this evidence but cannot create or change it.
CREATE TABLE subscription_authority_fence (
    singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
    generation UUID NOT NULL,
    source_commit TEXT NOT NULL CHECK (source_commit ~ '^[0-9a-f]{40}$'),
    writer_role OID NOT NULL,
    reader_role OID NOT NULL,
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash)=32),
    state TEXT NOT NULL CHECK (state='FENCED'),
    fenced_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK (writer_role<>reader_role)
);
REVOKE ALL ON subscription_authority_fence FROM PUBLIC;
