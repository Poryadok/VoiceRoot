-- Source-disabled normalized provider state. No legacy billing backfill or live
-- adapter activation. Privacy purge must cover these rows before activation.
CREATE TABLE subscription_provider_bindings (
 provider TEXT NOT NULL CHECK (provider IN ('fake','paddle','cloudpayments')),
 provider_subscription_id TEXT NOT NULL CHECK (octet_length(provider_subscription_id) BETWEEN 1 AND 512),
 aggregate_kind SMALLINT NOT NULL CHECK (aggregate_kind IN (1,2)),
 aggregate_id UUID NOT NULL,
 provider_version BIGINT NOT NULL CHECK (provider_version>0),
 facts_hash BYTEA NOT NULL CHECK (octet_length(facts_hash)=32),
 current_binding BOOLEAN NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(provider,provider_subscription_id),
 FOREIGN KEY(aggregate_kind,aggregate_id) REFERENCES subscription_entitlement_aggregates
);
CREATE UNIQUE INDEX subscription_provider_current ON subscription_provider_bindings(aggregate_kind,aggregate_id) WHERE current_binding;

-- Keep every observed version fingerprint: moving the high-water mark or
-- retiring a binding must not permit conflicting facts for an older version.
CREATE TABLE subscription_provider_versions (
 provider TEXT NOT NULL,
 provider_subscription_id TEXT NOT NULL,
 provider_version BIGINT NOT NULL CHECK(provider_version>0),
 facts_hash BYTEA NOT NULL CHECK(octet_length(facts_hash)=32),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(provider,provider_subscription_id,provider_version),
 FOREIGN KEY(provider,provider_subscription_id) REFERENCES subscription_provider_bindings
);

CREATE TABLE subscription_provider_outcomes (
 provider TEXT NOT NULL CHECK (provider IN ('fake','paddle','cloudpayments')),
 provider_event_id TEXT NOT NULL CHECK (octet_length(provider_event_id) BETWEEN 1 AND 512),
 request_hash BYTEA NOT NULL CHECK (octet_length(request_hash)=32),
 disposition TEXT NOT NULL CHECK (disposition IN ('APPLIED','UNCHANGED','STALE','RETIRED')),
 payload BYTEA NOT NULL CHECK (octet_length(payload)>0 AND octet_length(payload)<=1048576),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(provider,provider_event_id)
);

CREATE TABLE subscription_provider_conflicts (
 provider TEXT NOT NULL,
 provider_event_id TEXT NOT NULL,
 request_hash BYTEA NOT NULL CHECK (octet_length(request_hash)=32),
 request_bytes BYTEA NOT NULL CHECK (octet_length(request_bytes)>0 AND octet_length(request_bytes)<=2097152),
 error_class TEXT NOT NULL CHECK(error_class='CONTRACT_MISMATCH'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(provider,provider_event_id,request_hash)
);
-- Processed outcomes and quarantine retain P400D, with account privacy P30D
-- override. Provider target/order fences survive replacement; no age-based
-- cleanup is enabled until the permanent HMAC purge transition is implemented.
