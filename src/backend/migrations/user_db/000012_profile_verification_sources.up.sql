-- PR-003: durable source-scoped verification state owned by User Service.
CREATE SEQUENCE profile_verification_revision_seq AS BIGINT START WITH 1;

CREATE TABLE profile_verification_sources (
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    source VARCHAR(32) NOT NULL CHECK (source IN ('twitch', 'youtube', 'organization_dns', 'legacy_personal')),
    verification_type VARCHAR(32) NOT NULL CHECK (verification_type IN ('personal', 'organization')),
    badge VARCHAR(32) NOT NULL,
    verified BOOLEAN NOT NULL,
    revision BIGINT NOT NULL CHECK (revision > 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (profile_id, source)
);

CREATE INDEX profile_verification_sources_effective_idx
    ON profile_verification_sources (profile_id, verification_type, verified);

-- Preserve the visible state of profiles created before source-scoped storage.
INSERT INTO profile_verification_sources (
    profile_id, source, verification_type, badge, verified, revision)
SELECT id,
       CASE
           WHEN verification_type = 'organization' THEN 'organization_dns'
           WHEN verification_badge IN ('twitch', 'youtube') THEN verification_badge
           ELSE 'legacy_personal'
       END,
       verification_type,
       COALESCE(verification_badge,
           CASE WHEN verification_type = 'organization' THEN 'dns' ELSE 'verified' END),
       true,
       1
FROM profiles
WHERE verification_type IN ('personal', 'organization');

SELECT setval(
    'profile_verification_revision_seq',
    GREATEST(1, COALESCE((SELECT MAX(revision) FROM profile_verification_sources), 0) + 1),
    false);
