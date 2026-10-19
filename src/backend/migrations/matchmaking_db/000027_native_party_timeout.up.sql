BEGIN;
CREATE TABLE matchmaking_native_timeout_outbox (
    event_id UUID PRIMARY KEY,
    party_id UUID NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    search_session_id UUID NOT NULL UNIQUE REFERENCES search_sessions(id) ON DELETE RESTRICT,
    profile_id UUID NOT NULL,
    terminal_generation BIGINT NOT NULL CHECK(terminal_generation>0),
    occurred_at TIMESTAMPTZ NOT NULL,
    payload BYTEA NOT NULL CHECK(octet_length(payload)>0 AND octet_length(payload)<=16384),
    delivered_at TIMESTAMPTZ,
    lease_token UUID,
    lease_until TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK((lease_token IS NULL)=(lease_until IS NULL))
);
CREATE INDEX matchmaking_native_timeout_pending ON matchmaking_native_timeout_outbox(next_attempt_at,event_id) WHERE delivered_at IS NULL;
CREATE FUNCTION matchmaking_native_timeout_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF (to_jsonb(NEW)-'delivered_at'-'lease_token'-'lease_until'-'next_attempt_at')
       IS DISTINCT FROM (to_jsonb(OLD)-'delivered_at'-'lease_token'-'lease_until'-'next_attempt_at') THEN
        RAISE EXCEPTION 'native timeout facts are immutable';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER matchmaking_native_timeout_immutable BEFORE UPDATE ON matchmaking_native_timeout_outbox FOR EACH ROW EXECUTE FUNCTION matchmaking_native_timeout_immutable();
COMMIT;
