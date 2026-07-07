DROP INDEX IF EXISTS profile_search_documents_verified_idx;
ALTER TABLE profile_search_documents DROP COLUMN IF EXISTS verification_type;
