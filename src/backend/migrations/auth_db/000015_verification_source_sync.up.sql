-- PR-003: monotonic linked-source revisions and durable User sync targets.
CREATE SEQUENCE verification_source_revision_seq AS BIGINT START WITH 1;

ALTER TABLE linked_identities
    ADD COLUMN source_revision BIGINT;

UPDATE linked_identities
SET source_revision = nextval('verification_source_revision_seq')
WHERE source_revision IS NULL;

ALTER TABLE linked_identities
    ALTER COLUMN source_revision SET NOT NULL,
    ALTER COLUMN source_revision SET DEFAULT nextval('verification_source_revision_seq');

CREATE TABLE verification_source_sync_targets (
    account_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    platform TEXT NOT NULL CHECK (platform IN ('twitch', 'youtube')),
    source_revision BIGINT NOT NULL CHECK (source_revision > 0),
    verified BOOLEAN NOT NULL,
    badge TEXT NOT NULL,
    synced_revision BIGINT NOT NULL DEFAULT 0 CHECK (synced_revision >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, profile_id, platform)
);

INSERT INTO verification_source_sync_targets (
    account_id, profile_id, platform, source_revision, verified, badge)
SELECT account_id, profile_id, platform, source_revision, status = 'active', platform
FROM linked_identities
WHERE profile_id IS NOT NULL;
