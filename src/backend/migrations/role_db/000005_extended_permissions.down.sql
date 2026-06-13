-- Roll back extended permission bits 26–41 from system roles (best-effort).
UPDATE roles
SET
    permissions = permissions & ~(
        (1::bigint << 26) | (1::bigint << 27) | (1::bigint << 28) | (1::bigint << 29)
        | (1::bigint << 30) | (1::bigint << 31) | (1::bigint << 32) | (1::bigint << 33)
        | (1::bigint << 34) | (1::bigint << 35) | (1::bigint << 36) | (1::bigint << 37)
        | (1::bigint << 38) | (1::bigint << 39) | (1::bigint << 40) | (1::bigint << 41)
    ),
    updated_at = now()
WHERE is_system = true;
