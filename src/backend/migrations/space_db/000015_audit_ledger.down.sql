BEGIN;

LOCK TABLE spaces IN ACCESS EXCLUSIVE MODE;
LOCK TABLE audit_log IN ACCESS EXCLUSIVE MODE;
LOCK TABLE audit_outbox IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM audit_outbox) THEN
        RAISE EXCEPTION 'cannot remove audit outbox delivery evidence';
    END IF;
END
$$;

DROP TRIGGER spaces_hard_delete_disabled ON spaces;
DROP FUNCTION guard_space_hard_delete_until_p3();
DROP TRIGGER audit_log_immutable ON audit_log;
DROP FUNCTION guard_space_audit_immutability();
DROP TRIGGER audit_log_queue_outbox ON audit_log;
DROP FUNCTION queue_space_audit_outbox();
DROP TABLE audit_compensation_authorizations;
DROP TABLE audit_outbox;
DROP TABLE audit_cursor_signing_key;
DROP INDEX audit_log_space_action_created_idx;
DROP INDEX audit_log_space_actor_created_idx;
ALTER TABLE audit_log
    DROP CONSTRAINT audit_log_closed_registry,
    DROP CONSTRAINT audit_log_details_4k;

COMMIT;
