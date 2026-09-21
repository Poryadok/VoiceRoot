DROP INDEX IF EXISTS search_user_profile_inbox_active_profile_revision_idx;
-- Legacy schema permits only one row per profile/revision. Quarantine rows are
-- derived evidence and must be removed before its unconditional index returns.
DELETE FROM search_user_profile_inbox WHERE quarantined_at IS NOT NULL;
CREATE UNIQUE INDEX search_user_profile_inbox_profile_revision_idx
    ON search_user_profile_inbox (profile_id, source_revision);
