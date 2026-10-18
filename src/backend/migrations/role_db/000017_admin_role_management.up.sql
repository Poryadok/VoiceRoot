-- SPACE_MANAGE_ROLES (bit 2) is ordinary lower-role CRUD, not Owner identity.
-- Restore only missing system Admin grants; preserve all other role state.
BEGIN;
LOCK TABLE public.roles IN ACCESS EXCLUSIVE MODE;
UPDATE public.roles
SET permissions = permissions | 4::bigint, updated_at = now()
WHERE is_system = true AND name = 'Admin' AND (permissions & 4::bigint) = 0;
-- Existing Role policy triggers durably invalidate every changed Space.
COMMIT;
