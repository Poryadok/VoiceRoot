-- Source-disabled producer seam. No live subscription rows are migrated or
-- published until the normalized lifecycle and consumer cutover are ready.
CREATE TABLE subscription_entitlement_aggregates (
    aggregate_kind SMALLINT NOT NULL CHECK (aggregate_kind IN (1,2)),
    aggregate_id UUID NOT NULL,
    aggregate_revision BIGINT NOT NULL CHECK (aggregate_revision > 0),
    payload BYTEA NOT NULL CHECK (octet_length(payload)>0 AND octet_length(payload)<=1048576),
    payload_hash BYTEA NOT NULL CHECK (octet_length(payload_hash) = 32),
    PRIMARY KEY (aggregate_kind, aggregate_id)
);

CREATE TABLE subscription_event_outbox (
    event_id UUID PRIMARY KEY,
    aggregate_kind SMALLINT NOT NULL,
    aggregate_id UUID NOT NULL,
    aggregate_revision BIGINT NOT NULL CHECK (aggregate_revision>0),
    event_kind TEXT NOT NULL CHECK (event_kind='entitlement_changed'),
    subject TEXT NOT NULL CHECK (subject='subscription.entitlement_changed'),
    payload BYTEA NOT NULL CHECK (octet_length(payload)>0 AND octet_length(payload)<=1048576),
    payload_hash BYTEA NOT NULL CHECK (octet_length(payload_hash)=32),
    available_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts>=0),
    last_error TEXT,
    lease_token UUID,
    lease_until TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    published_stream TEXT,
    published_sequence BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (aggregate_kind,aggregate_id,aggregate_revision),
    FOREIGN KEY (aggregate_kind,aggregate_id) REFERENCES subscription_entitlement_aggregates,
    CHECK ((lease_token IS NULL)=(lease_until IS NULL)),
    CHECK ((delivered_at IS NULL AND published_stream IS NULL AND published_sequence IS NULL)
        OR (delivered_at IS NOT NULL AND published_stream IS NOT NULL AND published_sequence>0))
);
CREATE INDEX subscription_event_outbox_ready ON subscription_event_outbox(available_at,created_at)
    WHERE delivered_at IS NULL;
-- No age-based deletion: undelivered rows are permanent, delivered evidence
-- retains P400D, overridden by the future account privacy purge at P30D.
