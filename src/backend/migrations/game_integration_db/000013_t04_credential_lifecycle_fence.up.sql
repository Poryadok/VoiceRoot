-- Status restoration must never revive a credential that was valid before a
-- lifecycle transition. Database triggers cover direct SQL status changes as
-- well as the registry API path; SQL writers cannot recompute the keyed seal.
-- This verifier version also changes the credential seal format. Retire every
-- preexisting credential before deploying it; migration down intentionally
-- does not restore those authorities.
UPDATE service_credentials
SET revoked_at = COALESCE(revoked_at, clock_timestamp()),
    secret_digest = decode(repeat('00', 32), 'hex');

CREATE FUNCTION revoke_credentials_on_application_lifecycle_change()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status IS DISTINCT FROM NEW.status THEN
        UPDATE service_credentials c
        SET revoked_at = COALESCE(c.revoked_at, clock_timestamp()),
            secret_digest = decode(repeat('00', 32), 'hex')
        FROM environments e
        WHERE e.application_id = NEW.id
          AND c.environment_id = e.id;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER applications_revoke_credentials_on_status_change
AFTER UPDATE OF status ON applications
FOR EACH ROW EXECUTE FUNCTION revoke_credentials_on_application_lifecycle_change();

CREATE FUNCTION revoke_credentials_on_environment_lifecycle_change()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status IS DISTINCT FROM NEW.status THEN
        UPDATE service_credentials
        SET revoked_at = COALESCE(revoked_at, clock_timestamp()),
            secret_digest = decode(repeat('00', 32), 'hex')
        WHERE environment_id = NEW.id;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER environments_revoke_credentials_on_status_change
AFTER UPDATE OF status ON environments
FOR EACH ROW EXECUTE FUNCTION revoke_credentials_on_environment_lifecycle_change();
