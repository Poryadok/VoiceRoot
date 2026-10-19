BEGIN;

DO $$ BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'Voice admission ownership DOWN requires READ COMMITTED'
            USING ERRCODE = '55000';
    END IF;
END $$;
-- Serialize the retention probe with every owner writer. A first owner may
-- still be uncommitted when DOWN starts; wait for it before checking retention.
LOCK TABLE voice_space_media_admission_owners IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM voice_space_media_admission_owners) THEN
        RAISE EXCEPTION 'cannot remove admission ownership while durable ownership exists';
    END IF;
END $$;
DROP TABLE voice_space_media_admission_owners;
COMMIT;
