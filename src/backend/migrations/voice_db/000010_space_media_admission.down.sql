BEGIN;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM voice_space_media_admissions)
       OR EXISTS (SELECT 1 FROM voice_space_media_admission_outbox)
       OR EXISTS (SELECT 1 FROM voice_space_media_room_heads WHERE state <> 'ENDED')
       OR EXISTS (SELECT 1 FROM voice_account_voice_fences WHERE admission_operation_id IS NOT NULL) THEN
        RAISE EXCEPTION 'space media admission migration cannot be reversed while admissions or owned fences exist';
    END IF;
END $$;

DROP TABLE voice_space_media_admission_outbox;
DROP TABLE voice_space_media_room_heads;
DROP TABLE voice_space_media_admissions;
ALTER TABLE voice_account_voice_fences
    DROP CONSTRAINT voice_account_voice_fences_admission_binding_check,
    DROP COLUMN admission_generation,
    DROP COLUMN admission_operation_id;

COMMIT;
