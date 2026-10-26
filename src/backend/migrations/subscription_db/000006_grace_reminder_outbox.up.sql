-- Source-disabled revisioned producer; no legacy arrays are migrated.
CREATE TABLE subscription_grace_reminders (
 event_id UUID PRIMARY KEY,
 aggregate_kind SMALLINT NOT NULL CHECK (aggregate_kind IN (1,2)),
 aggregate_id UUID NOT NULL,
 grace_revision BIGINT NOT NULL CHECK (grace_revision>0),
 day INTEGER NOT NULL CHECK (day IN (1,3,7)),
 subject TEXT NOT NULL CHECK (subject='subscription.grace_reminder'),
 payload BYTEA NOT NULL CHECK (octet_length(payload)>0 AND octet_length(payload)<=1048576),
 payload_hash BYTEA NOT NULL CHECK (octet_length(payload_hash)=32),
 due_at TIMESTAMPTZ NOT NULL,
 window_end TIMESTAMPTZ NOT NULL CHECK (window_end>due_at),
 available_at TIMESTAMPTZ NOT NULL,
 status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','DELIVERED','SUPPRESSED','EXPIRED')),
 attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts>=0),
 lease_token UUID,
 lease_until TIMESTAMPTZ,
 published_stream TEXT,
 published_sequence BIGINT,
 delivered_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 UNIQUE (aggregate_kind,aggregate_id,grace_revision,day),
 FOREIGN KEY (aggregate_kind,aggregate_id) REFERENCES subscription_entitlement_aggregates,
 CHECK ((lease_token IS NULL)=(lease_until IS NULL)),
 CHECK ((status='DELIVERED' AND delivered_at IS NOT NULL AND published_stream='subscription_events' AND published_sequence>0)
 OR (status<>'DELIVERED' AND delivered_at IS NULL AND published_stream IS NULL AND published_sequence IS NULL))
);
CREATE INDEX subscription_grace_reminders_ready ON subscription_grace_reminders(available_at,event_id) WHERE status='PENDING';
-- Terminal outcomes retain P400D, subject to the earlier account privacy purge.
