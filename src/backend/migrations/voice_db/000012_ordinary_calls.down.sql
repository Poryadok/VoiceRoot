BEGIN;
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM voice_ordinary_rooms) THEN
    RAISE EXCEPTION 'ordinary Voice journal/tombstones require an explicit retirement protocol';
  END IF;
END $$;
DROP TABLE voice_ordinary_outbox;
DROP TABLE voice_ordinary_memberships;
DROP TABLE voice_ordinary_rooms;
ALTER TABLE voice_sdk_conversion_fence_receipts DROP COLUMN ordinary_source_snapshot;
COMMIT;
