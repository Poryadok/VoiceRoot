ALTER TABLE gis_sessions
    ADD COLUMN terminalized_at timestamptz,
    ADD COLUMN history_expires_at timestamptz;

UPDATE gis_sessions
SET terminalized_at=updated_at,
    history_expires_at=CASE WHEN kind='match' THEN updated_at+interval '30 days' ELSE NULL END
WHERE session_status IN ('closed','failed');

CREATE INDEX gis_sessions_retention_due
    ON gis_sessions(terminalized_at,id)
    WHERE session_status IN ('closed','failed') AND kind IN ('match','fleet');

ALTER TABLE gis_session_operations ADD COLUMN terminalized_at timestamptz;
UPDATE gis_session_operations SET terminalized_at=updated_at WHERE status IN ('succeeded','failed');

CREATE FUNCTION gis_session_operation_terminal_clock() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.status IN ('succeeded','failed') AND (OLD.status='pending' OR OLD.terminalized_at IS NULL) THEN
        NEW.terminalized_at := clock_timestamp();
    ELSIF NEW.status='pending' THEN
        NEW.terminalized_at := NULL;
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER gis_session_operation_terminal_clock
    BEFORE UPDATE OF status ON gis_session_operations
    FOR EACH ROW EXECUTE FUNCTION gis_session_operation_terminal_clock();

-- Deliberately contains no resource IDs, request hashes, receipts, or payload.
-- The external key remains reserved forever after content and receipts are purged.
CREATE TABLE gis_session_external_key_tombstones (
    application_id uuid NOT NULL,
    environment_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('party','match','fleet')),
    external_key text NOT NULL,
    tombstoned_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY(application_id,environment_id,kind,external_key)
);
