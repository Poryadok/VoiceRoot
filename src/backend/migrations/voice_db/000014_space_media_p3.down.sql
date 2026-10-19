-- Rollback is supported only before any native/P3 evidence exists. A retained
-- grant, terminal admission or deny head is not proof that its old credentials
-- can no longer be used. Never erase that history to make downgrade succeed.
BEGIN;
DO $$ BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'Voice P3 DOWN requires READ COMMITTED'
            USING ERRCODE = '55000';
    END IF;
END $$;
LOCK TABLE voice_space_lifecycle_fence_heads,
    voice_space_lifecycle_fence_receipts,
    voice_space_media_room_heads,
    voice_space_media_admissions,
    voice_space_lifecycle_targets,
    voice_space_media_grants IN ACCESS EXCLUSIVE MODE;
DO $$
BEGIN
    IF to_regclass('voice_native_owner_works') IS NOT NULL THEN
        RAISE EXCEPTION 'rollback native owner migration 017 before P3 migration 014'
            USING ERRCODE = '55000';
    END IF;
    IF EXISTS (SELECT 1 FROM voice_space_media_grants)
        OR EXISTS (SELECT 1 FROM voice_space_lifecycle_targets)
        OR EXISTS (SELECT 1 FROM voice_space_media_admissions)
        OR EXISTS (SELECT 1 FROM voice_space_media_room_heads)
        OR EXISTS (SELECT 1 FROM voice_space_lifecycle_fence_heads)
        OR EXISTS (SELECT 1 FROM voice_space_lifecycle_fence_receipts) THEN
        RAISE EXCEPTION 'cannot rollback P3 migration 014: retained native or lifecycle evidence requires reconciliation'
            USING ERRCODE = '55000';
    END IF;
END
$$;
DROP TABLE voice_space_media_grants;
DROP TABLE voice_space_lifecycle_targets;
ALTER TABLE voice_space_lifecycle_fence_receipts DROP COLUMN retirement_proven,DROP COLUMN coverage_sealed;
ALTER TABLE voice_space_media_room_heads DROP COLUMN lifecycle_generation;
ALTER TABLE voice_space_media_admissions DROP COLUMN lifecycle_generation;
COMMIT;
