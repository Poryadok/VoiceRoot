DROP TABLE IF EXISTS verification_source_sync_targets;
ALTER TABLE linked_identities DROP COLUMN IF EXISTS source_revision;
DROP SEQUENCE IF EXISTS verification_source_revision_seq;
