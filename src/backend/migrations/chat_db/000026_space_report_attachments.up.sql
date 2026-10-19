-- A report binding retires only on actual Space exit/delete/recreation.
-- The existing scope incarnation also changes for type/creator changes and
-- therefore cannot provide positive target-move authority (BE-066 c10).
ALTER TABLE chats ADD COLUMN report_attachment_generation BIGINT NOT NULL DEFAULT 1
    CHECK (report_attachment_generation > 0);
ALTER TABLE chat_scope_authority_heads ADD COLUMN report_attachment_generation BIGINT NOT NULL DEFAULT 1
    CHECK (report_attachment_generation > 0);

-- No FK: the exact old attachment must remain observable after Chat deletion.
-- A pending receipt has no timeout. An authenticated consumer acknowledgement
-- is a future protected protocol, not authority inferred from a missing row.
CREATE TABLE chat_report_attachment_retirements (
    chat_id UUID NOT NULL,
    attachment_generation BIGINT NOT NULL CHECK (attachment_generation > 0),
    space_id UUID NOT NULL,
    reason TEXT NOT NULL CHECK (reason IN ('TARGET_DELETED','TARGET_MOVED')),
    owner_event_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    owner_revision BIGINT NOT NULL CHECK (owner_revision > 0),
    retired_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    acknowledged_at TIMESTAMPTZ CHECK (acknowledged_at >= retired_at),
    PRIMARY KEY (chat_id, attachment_generation)
);
CREATE INDEX chat_report_attachment_retirements_pending_idx
    ON chat_report_attachment_retirements(retired_at,owner_event_id) WHERE acknowledged_at IS NULL;

-- Replace the existing native trigger body without changing its incarnation or
-- authority revision behavior. All fields and receipts share the source TX.
CREATE OR REPLACE FUNCTION maintain_chat_scope_authority() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE incarnation UUID; report_generation BIGINT; observed_revision BIGINT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        UPDATE chat_scope_authority_heads SET present=false,revision=revision+1
            WHERE chat_id=OLD.id AND present
            RETURNING revision INTO observed_revision;
        IF NOT FOUND THEN RAISE EXCEPTION 'Chat authority head unavailable'; END IF;
        IF OLD.space_id IS NOT NULL THEN
            INSERT INTO chat_report_attachment_retirements
                (chat_id,attachment_generation,space_id,reason,owner_revision)
                VALUES(OLD.id,OLD.report_attachment_generation,OLD.space_id,'TARGET_DELETED',observed_revision);
        END IF;
        RETURN OLD;
    END IF;
    IF TG_OP = 'UPDATE' THEN
        IF NEW.id IS DISTINCT FROM OLD.id THEN RAISE EXCEPTION 'Chat identity is immutable'; END IF;
        NEW.report_attachment_generation := OLD.report_attachment_generation;
        IF ROW(NEW.space_id,NEW.type,NEW.creator_profile_id,NEW.managed_by_application_id,
               NEW.managed_environment_id,NEW.external_chat_key)
           IS NOT DISTINCT FROM
           ROW(OLD.space_id,OLD.type,OLD.creator_profile_id,OLD.managed_by_application_id,
               OLD.managed_environment_id,OLD.external_chat_key) THEN
            NEW.scope_incarnation := OLD.scope_incarnation;
            RETURN NEW;
        END IF;
        report_generation := OLD.report_attachment_generation;
        IF NEW.space_id IS DISTINCT FROM OLD.space_id THEN
            report_generation := report_generation+1;
        END IF;
    ELSE
        -- Existing permanent head distinguishes same-ID recreation. A legacy
        -- absent binding is never reconstructed as a historical retirement.
        SELECT report_attachment_generation+1 INTO report_generation
            FROM chat_scope_authority_heads WHERE chat_id=NEW.id;
        IF NOT FOUND THEN report_generation := 1; END IF;
    END IF;
    incarnation := gen_random_uuid();
    INSERT INTO chat_scope_authority_heads
        (chat_id,scope_incarnation,revision,space_id,present,report_attachment_generation)
        VALUES(NEW.id,incarnation,1,NEW.space_id,true,report_generation)
    ON CONFLICT(chat_id) DO UPDATE SET scope_incarnation=EXCLUDED.scope_incarnation,
        revision=chat_scope_authority_heads.revision+1,space_id=EXCLUDED.space_id,present=true,
        report_attachment_generation=EXCLUDED.report_attachment_generation
    RETURNING revision INTO observed_revision;
    NEW.scope_incarnation := incarnation;
    NEW.report_attachment_generation := report_generation;
    IF TG_OP = 'UPDATE' AND OLD.space_id IS NOT NULL AND NEW.space_id IS DISTINCT FROM OLD.space_id THEN
        INSERT INTO chat_report_attachment_retirements
            (chat_id,attachment_generation,space_id,reason,owner_revision)
            VALUES(OLD.id,OLD.report_attachment_generation,OLD.space_id,'TARGET_MOVED',observed_revision);
    END IF;
    RETURN NEW;
END $$;

CREATE FUNCTION protect_chat_report_attachment_generation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.report_attachment_generation < OLD.report_attachment_generation THEN
        RAISE EXCEPTION 'Chat report attachment generation cannot regress';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER chat_report_attachment_generation_guard BEFORE UPDATE ON chat_scope_authority_heads
FOR EACH ROW EXECUTE FUNCTION protect_chat_report_attachment_generation();

CREATE FUNCTION protect_chat_report_attachment_receipt() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        IF OLD.acknowledged_at IS NULL THEN RAISE EXCEPTION 'Pending report attachment receipt must remain'; END IF;
        RETURN OLD;
    END IF;
    IF (to_jsonb(NEW)-'acknowledged_at') IS DISTINCT FROM (to_jsonb(OLD)-'acknowledged_at') OR
       (OLD.acknowledged_at IS NOT NULL AND NEW.acknowledged_at IS DISTINCT FROM OLD.acknowledged_at) THEN
        RAISE EXCEPTION 'Chat report attachment receipt is immutable';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER chat_report_attachment_receipt_guard BEFORE UPDATE OR DELETE ON chat_report_attachment_retirements
FOR EACH ROW EXECUTE FUNCTION protect_chat_report_attachment_receipt();
CREATE TRIGGER chat_report_attachment_receipt_truncate BEFORE TRUNCATE ON chat_report_attachment_retirements
FOR EACH STATEMENT EXECUTE FUNCTION reject_chat_scope_authority_truncate();
