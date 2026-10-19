BEGIN;

-- NULL is historical evidence, never a native account-owner lookup fallback.
-- Existing 019 captures must be reset and freshly captured; do not backfill.
ALTER TABLE parties
    ADD COLUMN voice_member_authority_version SMALLINT,
    ADD COLUMN voice_member_authority BYTEA,
    ADD COLUMN voice_member_authority_sha256 BYTEA,
    ADD COLUMN voice_source_authority_sha256 BYTEA,
    ADD CONSTRAINT parties_private_member_authority CHECK (
        (voice_member_authority_version IS NULL AND voice_member_authority IS NULL
         AND voice_member_authority_sha256 IS NULL AND voice_source_authority_sha256 IS NULL)
        OR
        (search_operation_id IS NOT NULL
         AND voice_member_authority_version IS NOT NULL AND voice_member_authority_version=1
         AND voice_member_authority IS NOT NULL AND octet_length(voice_member_authority)>=48
         AND voice_member_authority_sha256 IS NOT NULL AND octet_length(voice_member_authority_sha256)=32
         AND voice_source_authority_sha256 IS NOT NULL AND octet_length(voice_source_authority_sha256)=32)
    );

-- The existing to_jsonb(NEW/OLD) source trigger protects these columns too.
COMMIT;
