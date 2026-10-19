BEGIN;
-- Serialize the retention probe with every owner writer. A first owner may
-- have inserted before DOWN starts but commit only after this lock is acquired.
LOCK TABLE voice_space_media_admission_owners IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM voice_space_media_admission_owners) THEN
        RAISE EXCEPTION 'cannot remove admission ownership while durable ownership exists';
    END IF;
END $$;
DROP TABLE voice_space_media_admission_owners;
COMMIT;
