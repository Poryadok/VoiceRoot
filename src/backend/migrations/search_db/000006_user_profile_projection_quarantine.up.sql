-- A conflicting same-revision payload is evidence, not a replacement for the
-- accepted revision row. Keep one active row while retaining quarantines.
DROP INDEX IF EXISTS search_user_profile_inbox_profile_revision_idx;
CREATE UNIQUE INDEX search_user_profile_inbox_active_profile_revision_idx
    ON search_user_profile_inbox (profile_id, source_revision)
    WHERE quarantined_at IS NULL;
