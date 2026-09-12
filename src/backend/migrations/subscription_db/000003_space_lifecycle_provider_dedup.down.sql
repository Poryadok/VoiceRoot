BEGIN;

LOCK TABLE subscription_provider_event_fences IN ACCESS EXCLUSIVE MODE;
LOCK TABLE subscription_space_lifecycle_fences IN ACCESS EXCLUSIVE MODE;
LOCK TABLE subscription_space_lifecycle_operations IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM subscription_provider_event_fences) THEN
        RAISE EXCEPTION USING ERRCODE = '55000', MESSAGE = 'provider-event evidence exists';
    END IF;
    IF EXISTS (SELECT 1 FROM subscription_space_lifecycle_fences) THEN
        RAISE EXCEPTION USING ERRCODE = '55000', MESSAGE = 'lifecycle fence evidence exists';
    END IF;
    IF EXISTS (SELECT 1 FROM subscription_space_lifecycle_operations) THEN
        RAISE EXCEPTION USING ERRCODE = '55000', MESSAGE = 'lifecycle operation evidence exists';
    END IF;
END
$$;

DROP TABLE subscription_space_lifecycle_operations;
DROP TABLE subscription_space_lifecycle_fences;
DROP TABLE subscription_provider_event_fences;
DROP FUNCTION subscription_reject_evidence_mutation();

COMMIT;
