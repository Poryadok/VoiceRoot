CREATE TABLE messaging_space_lifecycle_operations (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    request_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    receipt_bytes BYTEA NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL,
    retain_until TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (space_id, deletion_operation_id, generation)
);

CREATE INDEX messaging_space_lifecycle_operations_retention_idx
    ON messaging_space_lifecycle_operations (retain_until);

CREATE OR REPLACE FUNCTION messaging_guard_space_chat_mutation() RETURNS trigger AS $$
DECLARE
    affected_chat_ids UUID[];
    lifecycle_space UUID;
    lifecycle_operation UUID;
    lifecycle_state TEXT;
    purge_space_setting TEXT;
    purge_operation_setting TEXT;
BEGIN
    IF TG_OP = 'INSERT' THEN
        affected_chat_ids := ARRAY[NEW.chat_id];
    ELSIF TG_OP = 'DELETE' THEN
        affected_chat_ids := ARRAY[OLD.chat_id];
    ELSE
        affected_chat_ids := ARRAY[OLD.chat_id, NEW.chat_id];
    END IF;

    FOR lifecycle_space IN
        SELECT DISTINCT item.space_id
        FROM messaging_space_chat_manifest_items item
        WHERE item.chat_id = ANY(affected_chat_ids)
        ORDER BY item.space_id
    LOOP
        -- Match voice/backend/pkg/spacemutationlock.Key exactly.
        PERFORM pg_advisory_xact_lock((
            'x' || encode(substring(digest(
                convert_to('voice.space.mutation.v1', 'UTF8') || decode('00', 'hex') || uuid_send(lifecycle_space),
                'sha256'
            ) FROM 1 FOR 8), 'hex')
        )::bit(64)::bigint);
        SELECT fence.state, fence.deletion_operation_id
        INTO lifecycle_state, lifecycle_operation
        FROM messaging_space_lifecycle_fences fence
        WHERE fence.space_id = lifecycle_space;

        IF lifecycle_state IN ('FROZEN', 'PURGE_DECIDED', 'PURGED') THEN
            purge_space_setting := current_setting('voice.messaging_purge_space_id', TRUE);
            purge_operation_setting := current_setting('voice.messaging_purge_operation_id', TRUE);
            IF TG_OP = 'DELETE' AND lifecycle_state = 'PURGE_DECIDED'
               AND purge_space_setting = lifecycle_space::text
               AND purge_operation_setting = lifecycle_operation::text THEN
                CONTINUE;
            END IF;
            RAISE EXCEPTION 'Messaging Space lifecycle fence blocks chat mutation'
                USING ERRCODE = '55000';
        END IF;
    END LOOP;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER messaging_space_chat_mutation_gate
    BEFORE INSERT OR UPDATE OR DELETE ON messages
    FOR EACH ROW EXECUTE FUNCTION messaging_guard_space_chat_mutation();
