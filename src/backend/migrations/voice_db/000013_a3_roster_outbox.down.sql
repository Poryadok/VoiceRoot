BEGIN;

-- Exclude native membership commits between the history check and DDL.
LOCK TABLE voice_space_media_room_heads, voice_roster_outbox IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM voice_roster_outbox)
       OR EXISTS (SELECT 1 FROM voice_space_media_room_heads WHERE roster_version > 0) THEN
        RAISE EXCEPTION 'cannot remove Voice roster history';
    END IF;
END $$;

DROP TABLE voice_roster_outbox;
ALTER TABLE voice_space_media_room_heads DROP COLUMN roster_version;

COMMIT;
