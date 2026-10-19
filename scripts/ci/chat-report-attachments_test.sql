-- Run after the canonical chat_db migrations in a disposable PostgreSQL.
-- This is a native producer proof, not protected RPC or local-report acceptance.
\set ON_ERROR_STOP on
DO $$
DECLARE c UUID := 'b0066000-0000-4000-8000-000000000001';
        a UUID := 'b0066000-0000-4000-8000-000000000002';
        b UUID := 'b0066000-0000-4000-8000-000000000003';
        creator UUID := 'b0066000-0000-4000-8000-000000000004';
        g BIGINT; g2 BIGINT; observed TEXT; r BIGINT; n BIGINT;
BEGIN
    INSERT INTO chats(id,type,space_id,creator_profile_id) VALUES(c,'channel',a,creator);
    SELECT to_jsonb(chats)->>'report_attachment_generation' INTO observed FROM chats WHERE id=c;
    IF observed IS NULL THEN
        RAISE EXCEPTION 'REPORT_ATTACHMENT_MISSING: native Chat needs a separate positive report binding';
    END IF;
    g := observed::BIGINT;
    IF g <= 0 THEN RAISE EXCEPTION 'invalid generation'; END IF;
    SELECT revision INTO r FROM chat_scope_authority_heads WHERE chat_id=c;
    -- Neither a current type/creator change nor a forged column can retire it.
    UPDATE chats SET type='group',creator_profile_id=gen_random_uuid(),report_attachment_generation=999 WHERE id=c;
    SELECT report_attachment_generation INTO g2 FROM chats WHERE id=c;
    IF g2 <> g THEN RAISE EXCEPTION 'type/creator/supplied generation changed report binding'; END IF;
    IF EXISTS(SELECT 1 FROM chat_report_attachment_retirements WHERE chat_id=c) THEN
        RAISE EXCEPTION 'type/creator change falsely proves move';
    END IF;
    IF (SELECT revision FROM chat_scope_authority_heads WHERE chat_id=c) <> r+1 THEN
        RAISE EXCEPTION 'existing native authority revision behavior changed';
    END IF;
    UPDATE chats SET name='cosmetic',topic='cosmetic' WHERE id=c;
    IF (SELECT report_attachment_generation FROM chats WHERE id=c) <> g THEN
        RAISE EXCEPTION 'cosmetic change changed attachment';
    END IF;
    UPDATE chats SET space_id=b WHERE id=c;
    IF NOT EXISTS(SELECT 1 FROM chat_report_attachment_retirements
        WHERE chat_id=c AND attachment_generation=g AND space_id=a AND reason='TARGET_MOVED'
          AND owner_revision>0 AND owner_event_id IS NOT NULL AND acknowledged_at IS NULL) THEN
        RAISE EXCEPTION 'native move lacks exact positive old binding';
    END IF;
    UPDATE chats SET space_id=a WHERE id=c;
    SELECT report_attachment_generation INTO g2 FROM chats WHERE id=c;
    IF g2 <= g THEN RAISE EXCEPTION 'ABA reused old binding'; END IF;
    IF (SELECT count(*) FROM chat_report_attachment_retirements WHERE chat_id=c) <> 2 THEN
        RAISE EXCEPTION 'ABA lost retirement history';
    END IF;
    -- Rollback cannot invent a positive receipt or advance either producer.
    SELECT revision INTO r FROM chat_scope_authority_heads WHERE chat_id=c;
    BEGIN
        UPDATE chats SET space_id=b WHERE id=c;
        RAISE EXCEPTION 'forced rollback';
    EXCEPTION WHEN raise_exception THEN NULL;
    END;
    IF (SELECT revision FROM chat_scope_authority_heads WHERE chat_id=c) <> r OR
       (SELECT report_attachment_generation FROM chats WHERE id=c) <> g2 OR
       (SELECT count(*) FROM chat_report_attachment_retirements WHERE chat_id=c) <> 2 THEN
        RAISE EXCEPTION 'aborted transaction left retirement effects';
    END IF;
    -- Freeze is not a target move or delete.
    INSERT INTO chat_space_lifecycle_fences(space_id,generation,state,deletion_operation_id,
        schedule_generation,manifest_id,manifest_sha256,manifest_item_count,
        source_manifest_id,source_manifest_sha256,source_manifest_item_count)
        VALUES(a,1,'FROZEN',gen_random_uuid(),1,gen_random_uuid(),decode(repeat('00',32),'hex'),0,
               gen_random_uuid(),decode(repeat('00',32),'hex'),0);
    IF (SELECT report_attachment_generation FROM chats WHERE id=c) <> g2 OR
       (SELECT count(*) FROM chat_report_attachment_retirements WHERE chat_id=c) <> 2 THEN
        RAISE EXCEPTION 'freeze was falsely terminal';
    END IF;
    DELETE FROM chats WHERE id=c;
    IF NOT EXISTS(SELECT 1 FROM chat_report_attachment_retirements
        WHERE chat_id=c AND attachment_generation=g2 AND space_id=a AND reason='TARGET_DELETED') THEN
        RAISE EXCEPTION 'deletion lost exact old binding';
    END IF;
    INSERT INTO chats(id,type,space_id,creator_profile_id) VALUES(c,'channel',a,creator);
    IF (SELECT report_attachment_generation FROM chats WHERE id=c) <= g2 THEN
        RAISE EXCEPTION 'same-ID recreation reused a retired binding';
    END IF;
    -- Pending receipts cannot be removed, rewritten or silently acknowledged.
    BEGIN
        DELETE FROM chat_report_attachment_retirements WHERE chat_id=c;
        RAISE EXCEPTION 'TEST: pending receipts were deleted';
    EXCEPTION WHEN raise_exception THEN
        IF SQLERRM LIKE 'TEST:%' THEN RAISE; END IF;
    END;
    BEGIN
        UPDATE chat_report_attachment_retirements SET reason='TARGET_DELETED' WHERE chat_id=c AND attachment_generation=g;
        RAISE EXCEPTION 'TEST: receipt authority was rewritten';
    EXCEPTION WHEN raise_exception THEN
        IF SQLERRM LIKE 'TEST:%' THEN RAISE; END IF;
    END;
    SELECT count(*) INTO n FROM chat_report_attachment_retirements WHERE chat_id=c;
    IF n<>3 THEN RAISE EXCEPTION 'receipt guard lost history'; END IF;
END $$;
\echo REPORT_ATTACHMENT_NATIVE_CORE_PASS

-- Trigger-writer serialisation: a failed overflow never resets either head.
DO $$
DECLARE c UUID := 'b0066000-0000-4000-8000-000000000011';
        a UUID := 'b0066000-0000-4000-8000-000000000012';
        g BIGINT;
BEGIN
    INSERT INTO chats(id,type,space_id,creator_profile_id) VALUES(c,'channel',a,gen_random_uuid());
    SELECT report_attachment_generation INTO g FROM chats WHERE id=c;
    UPDATE chat_scope_authority_heads SET revision=9223372036854775807 WHERE chat_id=c;
    BEGIN
        UPDATE chats SET space_id=gen_random_uuid() WHERE id=c;
        RAISE EXCEPTION 'TEST: overflow did not abort';
    EXCEPTION WHEN numeric_value_out_of_range THEN NULL;
    END;
    IF (SELECT report_attachment_generation FROM chats WHERE id=c)<>g OR
       EXISTS(SELECT 1 FROM chat_report_attachment_retirements WHERE chat_id=c) THEN
        RAISE EXCEPTION 'failed native writer partially committed';
    END IF;
END $$;
\echo REPORT_ATTACHMENT_NATIVE_OVERFLOW_PASS
