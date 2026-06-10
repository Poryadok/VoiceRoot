UPDATE roles
SET
    permissions = permissions & ~(1::bigint << 25),
    updated_at = now()
WHERE
    is_system = true
    AND name IN ('Owner', 'Admin', 'Moderator');
