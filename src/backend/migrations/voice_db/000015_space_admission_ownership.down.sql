DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM voice_space_media_admission_owners) THEN
        RAISE EXCEPTION 'cannot remove admission ownership while durable ownership exists';
    END IF;
END $$;
DROP TABLE voice_space_media_admission_owners;
