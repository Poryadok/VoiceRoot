UPDATE roles
SET
    permissions = permissions & ~((1::bigint << 20) | (1::bigint << 21) | (1::bigint << 22)),
    updated_at = now()
WHERE
    is_system = true
    AND name IN ('Owner', 'Admin', 'Moderator');
