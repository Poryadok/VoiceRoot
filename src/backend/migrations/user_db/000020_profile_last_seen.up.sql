-- User owns durable activity; only IDs and timestamps, no presence payload.
-- Logical profile reference follows the service's existing ID conventions.
CREATE TABLE profile_last_seen (
    profile_id UUID PRIMARY KEY,
    last_seen_at TIMESTAMPTZ NOT NULL
);
