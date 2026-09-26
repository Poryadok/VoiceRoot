BEGIN;

LOCK TABLE sdk_author_tombstones IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM sdk_author_tombstones LIMIT 1) THEN
        RAISE EXCEPTION 'cannot roll back SDK author tombstones after receipts have been committed';
    END IF;
END
$$;

DROP TRIGGER IF EXISTS sdk_author_tombstones_no_truncate ON sdk_author_tombstones;
DROP TRIGGER IF EXISTS sdk_author_tombstones_no_update_delete ON sdk_author_tombstones;
DROP FUNCTION IF EXISTS reject_sdk_author_tombstone_mutation();
DROP TABLE IF EXISTS sdk_author_tombstones;
DROP TRIGGER IF EXISTS profiles_sdk_eligibility_revision ON profiles;
DROP FUNCTION IF EXISTS bump_sdk_eligibility_revision();
ALTER TABLE profiles DROP COLUMN IF EXISTS sdk_eligibility_revision;

COMMIT;
