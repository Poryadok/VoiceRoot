BEGIN;
DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM parties WHERE voice_member_authority_version IS NOT NULL
              OR voice_member_authority IS NOT NULL OR voice_member_authority_sha256 IS NOT NULL
              OR voice_source_authority_sha256 IS NOT NULL) THEN
        RAISE EXCEPTION 'private party member authority must be retired before rollback';
    END IF;
END $$;
ALTER TABLE parties
    DROP CONSTRAINT parties_private_member_authority,
    DROP COLUMN voice_member_authority_version,
    DROP COLUMN voice_member_authority,
    DROP COLUMN voice_member_authority_sha256,
    DROP COLUMN voice_source_authority_sha256;
COMMIT;
