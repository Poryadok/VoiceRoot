BEGIN;
-- Lock acquisition cannot refresh an inherited transaction snapshot.
DO $$ BEGIN
  IF current_setting('transaction_isolation') <> 'read committed' THEN
    RAISE EXCEPTION 'ordinary Voice DOWN requires READ COMMITTED'
      USING ERRCODE = '55000';
  END IF;
END $$;

-- Exclude every native writer before taking the fresh retention snapshot.
-- Keep these locks through DDL so a first committed journal/snapshot cannot
-- appear between the retention check and removal.
LOCK TABLE voice_ordinary_rooms, voice_ordinary_memberships,
           voice_ordinary_outbox, voice_sdk_conversion_fence_receipts
  IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM voice_ordinary_rooms) OR
     EXISTS (SELECT 1 FROM voice_sdk_conversion_fence_receipts WHERE ordinary_source_snapshot IS NOT NULL) THEN
    RAISE EXCEPTION 'ordinary Voice journal/tombstones require an explicit retirement protocol';
  END IF;
END $$;
DROP TABLE voice_ordinary_outbox;
DROP TABLE voice_ordinary_memberships;
DROP TABLE voice_ordinary_rooms;
ALTER TABLE voice_sdk_conversion_fence_receipts DROP COLUMN ordinary_source_snapshot;
COMMIT;
