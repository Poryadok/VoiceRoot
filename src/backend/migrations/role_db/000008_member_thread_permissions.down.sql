UPDATE roles
SET
    permissions = permissions & ~((1::bigint << 32) | (1::bigint << 33)),
    updated_at = now()
WHERE
    is_system = true
    AND name = 'Member';
