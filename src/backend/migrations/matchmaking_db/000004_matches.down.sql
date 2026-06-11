ALTER TABLE search_sessions DROP CONSTRAINT IF EXISTS search_sessions_match_id_fkey;

DROP TABLE IF EXISTS match_proposals;
DROP TABLE IF EXISTS matches;

DROP INDEX IF EXISTS search_sessions_one_active_per_profile;
CREATE UNIQUE INDEX search_sessions_one_active_per_profile
    ON search_sessions (profile_id)
    WHERE status = 'searching';

ALTER TABLE search_sessions DROP CONSTRAINT IF EXISTS search_sessions_status_check;
ALTER TABLE search_sessions ADD CONSTRAINT search_sessions_status_check
    CHECK (status IN ('searching', 'matched', 'timeout', 'cancelled'));
