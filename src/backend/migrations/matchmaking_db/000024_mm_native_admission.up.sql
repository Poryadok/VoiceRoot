BEGIN;

-- Installer-owned cross-check, not an application-issued admission receipt.
-- An incomplete CLOSED/PREPARED row permanently records that native cut began.
CREATE TABLE public.mm_native_admission_head (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    protocol_version INTEGER NOT NULL CHECK (protocol_version=1),
    state TEXT NOT NULL CHECK (state IN ('CLOSED','PREPARED','ADMITTED')),
    environment_id UUID,
    installation_id UUID,
    authority_incarnation UUID,
    transition_id UUID,
    admission_generation BIGINT CHECK (admission_generation>0),
    credential_cohort_id UUID,
    database_binding_sha256 BYTEA CHECK (octet_length(database_binding_sha256)=32),
    schema_contract_sha256 BYTEA CHECK (octet_length(schema_contract_sha256)=32),
    source_factory_manifest_sha256 BYTEA CHECK (octet_length(source_factory_manifest_sha256)=32),
    fresh_route_manifest_sha256 BYTEA CHECK (octet_length(fresh_route_manifest_sha256)=32),
    admission_receipt_sha256 BYTEA CHECK (octet_length(admission_receipt_sha256)=32),
    launch_manifest_sha256 BYTEA CHECK (octet_length(launch_manifest_sha256)=32),
    expected_login_role_oid BIGINT CHECK (expected_login_role_oid>0 AND expected_login_role_oid<=4294967295),
    expected_login_role_name TEXT CHECK (length(expected_login_role_name)>0),
    CHECK (state<>'ADMITTED' OR (
        environment_id IS NOT NULL AND installation_id IS NOT NULL
        AND authority_incarnation IS NOT NULL AND transition_id IS NOT NULL
        AND admission_generation IS NOT NULL AND credential_cohort_id IS NOT NULL
        AND database_binding_sha256 IS NOT NULL AND schema_contract_sha256 IS NOT NULL
        AND source_factory_manifest_sha256 IS NOT NULL AND fresh_route_manifest_sha256 IS NOT NULL
        AND admission_receipt_sha256 IS NOT NULL AND launch_manifest_sha256 IS NOT NULL
        AND expected_login_role_oid IS NOT NULL AND expected_login_role_name IS NOT NULL
    ))
);
COMMENT ON TABLE public.mm_native_admission_head IS
    'Installer-owned MM native admission cross-check; serving roles must have SELECT only. Mutators first take the code-owned exclusive admission gate.';

COMMIT;
