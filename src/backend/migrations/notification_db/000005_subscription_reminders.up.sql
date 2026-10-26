CREATE TABLE notification_subscription_inbox (
 event_id uuid PRIMARY KEY,
 payload_hash bytea NOT NULL CHECK (octet_length(payload_hash)=32),
 outcome text NOT NULL CHECK (outcome IN ('APPLIED','STALE','QUEUED','SUPPRESSED')),
 processed_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE notification_subscription_projection (
 aggregate_kind smallint NOT NULL CHECK (aggregate_kind IN (1,2)),
 aggregate_id uuid NOT NULL,
 aggregate_revision bigint NOT NULL CHECK (aggregate_revision>0),
 snapshot bytea NOT NULL,
 snapshot_hash bytea NOT NULL CHECK (octet_length(snapshot_hash)=32),
 PRIMARY KEY (aggregate_kind,aggregate_id)
);
CREATE TABLE notification_reminder_dispatch (
 notification_id uuid NOT NULL,
 channel text NOT NULL CHECK (channel IN ('in_app','push')),
 event_id uuid NOT NULL,
 aggregate_kind smallint NOT NULL CHECK (aggregate_kind IN (1,2)),
 aggregate_id uuid NOT NULL,
 grace_revision bigint NOT NULL CHECK (grace_revision>0),
 day smallint NOT NULL CHECK (day IN (1,3,7)),
 payer_account_id uuid NOT NULL,
 payload bytea NOT NULL,
 window_start timestamptz NOT NULL,
 window_end timestamptz NOT NULL CHECK (window_end>window_start),
 status text NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','DELIVERED','SUPPRESSED')),
 available_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 attempts integer NOT NULL DEFAULT 0,
 lease_token uuid,
 lease_until timestamptz,
 completed_at timestamptz,
 PRIMARY KEY (notification_id,channel),
 UNIQUE (aggregate_kind,aggregate_id,grace_revision,day,channel),
 CHECK ((lease_token IS NULL)=(lease_until IS NULL))
);
CREATE INDEX notification_reminder_ready ON notification_reminder_dispatch (available_at) WHERE status='PENDING';
