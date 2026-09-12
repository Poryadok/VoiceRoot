BEGIN;
LOCK TABLE space_deletion_proofs IN ACCESS EXCLUSIVE MODE;
-- Guarded refusal is SQLSTATE ERRCODE = '55000'.
CREATE FUNCTION auth_guard_space_deletion_proofs_down() RETURNS void LANGUAGE plpgsql AS
'BEGIN
    IF EXISTS (
        SELECT 1 FROM space_deletion_proofs
        WHERE consumed_at IS NOT NULL OR receipt_bytes IS NOT NULL
    ) THEN
        RAISE EXCEPTION ''space deletion receipt evidence prevents migration rollback''
            USING ERRCODE = ''55000'';
    END IF;
END';
SELECT auth_guard_space_deletion_proofs_down();
DROP FUNCTION auth_guard_space_deletion_proofs_down();
DROP TABLE space_deletion_proofs;
COMMIT;
