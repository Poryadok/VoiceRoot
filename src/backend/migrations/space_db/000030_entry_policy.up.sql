-- Private versioned policy and actor-scoped exact outcomes. Legacy columns remain
-- preserved for conservative migration/repair, never as a composable-policy writer.
CREATE TABLE space_entry_policies (
    space_id UUID PRIMARY KEY REFERENCES spaces(id) ON DELETE CASCADE,
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    policy JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

INSERT INTO space_entry_policies(space_id,policy)
SELECT id,CASE WHEN entry_requirement IN ('none','phone','captcha','questions','manual') THEN jsonb_build_object(
    'schema_version',1,
    'require_phone',entry_requirement='phone',
    'require_captcha',entry_requirement='captcha',
    'require_manual_approval',entry_requirement='manual',
    'questions',CASE WHEN entry_requirement='questions' THEN
        CASE WHEN jsonb_typeof(entry_questions)='array' THEN
            CASE WHEN jsonb_array_length(entry_questions)>0 THEN entry_questions ELSE 'null'::jsonb END
        ELSE 'null'::jsonb END
    ELSE '[]'::jsonb END
) ELSE '{"invalid_legacy_policy":true}'::jsonb END FROM spaces;

CREATE TABLE space_entry_policy_operations (
    actor_profile_id UUID NOT NULL,
    operation_id UUID NOT NULL,
    space_id UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    request_digest BYTEA NOT NULL CHECK (octet_length(request_digest)=32),
    outcome_version BIGINT NOT NULL CHECK(outcome_version>0),
    outcome_policy JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY(actor_profile_id,operation_id)
);
