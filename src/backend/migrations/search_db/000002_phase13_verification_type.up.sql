-- search_db v2 — Phase 13: verification_type for profile ranking

ALTER TABLE profile_search_documents
    ADD COLUMN IF NOT EXISTS verification_type TEXT NOT NULL DEFAULT 'none';

CREATE INDEX IF NOT EXISTS profile_search_documents_verified_idx
    ON profile_search_documents ((CASE WHEN verification_type <> 'none' THEN 0 ELSE 1 END));
