BEGIN;
LOCK TABLE role_space_audit_outbox IN ACCESS EXCLUSIVE MODE;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM role_space_audit_outbox) THEN
        RAISE EXCEPTION 'cannot remove retained role audit delivery evidence';
    END IF;
END;
$$;

DROP TABLE role_space_audit_outbox;
DROP FUNCTION role_space_audit_outbox_immutable();
COMMIT;
