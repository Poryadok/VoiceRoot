BEGIN;
LOCK TABLE roles,chat_overrides,voice_room_overrides IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM roles WHERE (permissions & (1::bigint << 42))<>0 AND NOT(is_system AND name IN ('Owner','Admin','Moderator')))
       OR EXISTS(SELECT 1 FROM chat_overrides WHERE ((allow | deny) & (1::bigint << 42))<>0)
       OR EXISTS(SELECT 1 FROM voice_room_overrides WHERE ((allow | deny) & (1::bigint << 42))<>0) THEN
        RAISE EXCEPTION 'join approval configured delegation prevents downgrade';
    END IF;
END $$;
UPDATE roles SET permissions=permissions & ~(1::bigint << 42),updated_at=clock_timestamp()
WHERE is_system AND name IN ('Owner','Admin','Moderator');
COMMIT;
