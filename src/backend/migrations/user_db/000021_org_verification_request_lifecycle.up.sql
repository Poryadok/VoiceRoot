-- Keep at most the latest unexpired legacy challenge per profile. Verified
-- history and the existing badge source are preserved.
WITH ranked AS (
    SELECT id, created_at, row_number() OVER (
        PARTITION BY profile_id ORDER BY created_at DESC, id DESC
    ) AS position
    FROM organization_verification_requests
    WHERE status = 'pending'
)
UPDATE organization_verification_requests AS request
SET status = 'expired'
FROM ranked
WHERE request.id = ranked.id
  AND (ranked.position > 1 OR ranked.created_at <= statement_timestamp() - interval '24 hours');

CREATE UNIQUE INDEX organization_verification_one_pending_profile_idx
    ON organization_verification_requests (profile_id) WHERE status = 'pending';
