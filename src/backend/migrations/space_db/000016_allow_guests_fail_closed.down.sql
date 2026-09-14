-- The preceding migration discards whether a legacy Space happened to be
-- open. Reopening admission on rollback would be a security regression, so a
-- deploy must move forward with a reviewed replacement instead.
DO $$
BEGIN
    RAISE EXCEPTION 'refusing to roll back fail-closed Space guest admission';
END
$$;
