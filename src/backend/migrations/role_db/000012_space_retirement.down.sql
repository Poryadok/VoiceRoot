-- Removing permanent retirement evidence is safe only before any retirement
-- has committed. Lock first so concurrent retirement is included in the check.
DO $$
BEGIN
    -- Runtime retirement reads/locks receipts before lifecycle state. Match that
    -- order so a receipt-first writer and DOWN can wait without a lock cycle.
    LOCK TABLE role_space_retirement_receipts, role_space_lifecycle IN ACCESS EXCLUSIVE MODE;
    IF EXISTS (SELECT 1 FROM role_space_retirement_receipts)
       OR EXISTS (SELECT 1 FROM role_space_lifecycle WHERE retired_at IS NOT NULL) THEN
        RAISE EXCEPTION 'refusing to remove permanent Role retirement evidence'
            USING ERRCODE = '55000';
    END IF;

    DROP TRIGGER role_space_retirement_fence_no_change ON role_space_lifecycle;
    DROP TRIGGER role_space_retirement_receipt_no_change ON role_space_retirement_receipts;
    DROP TRIGGER role_space_retirement_receipt_set_boundary ON role_space_retirement_receipts;
    DROP FUNCTION role_space_retirement_fence_immutable();
    DROP FUNCTION role_space_retirement_receipt_immutable();
    DROP FUNCTION role_space_retirement_receipt_insert();
    DROP TABLE role_space_retirement_receipts;

    CREATE OR REPLACE FUNCTION role_voice_policy_role_changed() RETURNS trigger
    LANGUAGE plpgsql AS $body$
    BEGIN
        IF TG_OP = 'DELETE' THEN
            PERFORM role_voice_policy_bump(OLD.space_id, NULL, NULL);
            RETURN OLD;
        END IF;
        PERFORM role_voice_policy_bump(NEW.space_id, NULL, NULL);
        RETURN NEW;
    END
    $body$;

    CREATE OR REPLACE FUNCTION role_voice_policy_member_changed() RETURNS trigger
    LANGUAGE plpgsql AS $body$
    DECLARE
        changed_space_id UUID;
        changed_profile_id UUID;
        changed_role_id UUID;
    BEGIN
        IF TG_OP = 'DELETE' THEN
            changed_space_id := OLD.space_id;
            changed_profile_id := OLD.profile_id;
            changed_role_id := OLD.role_id;
        ELSE
            changed_space_id := NEW.space_id;
            changed_profile_id := NEW.profile_id;
            changed_role_id := NEW.role_id;
        END IF;

        IF TG_OP = 'DELETE' AND NOT EXISTS (SELECT 1 FROM roles WHERE id = changed_role_id) THEN
            RETURN OLD;
        END IF;

        PERFORM role_voice_policy_bump(changed_space_id, NULL, changed_profile_id);
        IF TG_OP = 'DELETE' THEN
            RETURN OLD;
        END IF;
        RETURN NEW;
    END
    $body$;
END
$$;
