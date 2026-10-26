-- Source-owned write provenance, not account eligibility or CURRENT authority.
CREATE TABLE subscription_kernel_writes (
    aggregate_kind SMALLINT NOT NULL CHECK (aggregate_kind IN (1,2)),
    aggregate_id UUID NOT NULL,
    aggregate_revision BIGINT NOT NULL CHECK (aggregate_revision>0),
    generation UUID NOT NULL,
    event_id UUID NOT NULL UNIQUE,
    payload_hash BYTEA NOT NULL CHECK (octet_length(payload_hash)=32),
    provider TEXT NOT NULL,
    provider_event_id TEXT NOT NULL,
    provider_version BIGINT NOT NULL CHECK (provider_version>0),
    state TEXT NOT NULL DEFAULT 'WRITTEN' CHECK (state='WRITTEN'),
    written_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (aggregate_kind,aggregate_id,aggregate_revision),
    FOREIGN KEY (event_id) REFERENCES subscription_event_outbox(event_id)
);
REVOKE ALL ON subscription_kernel_writes FROM PUBLIC;
