BEGIN;
LOCK TABLE voice_legacy_owner_works, voice_legacy_owner_acceptances IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM voice_legacy_owner_works) THEN
        RAISE EXCEPTION 'cannot discard retained legacy owner work';
    END IF;
END $$;
DROP TABLE voice_legacy_owner_acceptances, voice_legacy_owner_works;
DROP FUNCTION voice_legacy_owner_immutable_fn();
COMMIT;
