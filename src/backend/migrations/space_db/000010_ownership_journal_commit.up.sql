-- Additive Role PREPARED evidence and private local commit artifacts.
ALTER TABLE ownership_journal
    ADD COLUMN role_receipt_bytes BYTEA,
    ADD COLUMN role_receipt_hash BYTEA,
    ADD COLUMN role_intent_bytes BYTEA,
    ADD COLUMN role_intent_hash BYTEA,
    ADD COLUMN pending_audit_action TEXT,
    ADD COLUMN pending_audit_target_type TEXT,
    ADD COLUMN pending_audit_target_id UUID,
    ADD COLUMN pending_audit_details JSONB,
    ADD CONSTRAINT ownership_journal_role_receipt_all_or_none CHECK (
        (role_receipt_bytes IS NULL
            AND role_receipt_hash IS NULL
            AND role_intent_bytes IS NULL
            AND role_intent_hash IS NULL)
        OR
        (role_receipt_bytes IS NOT NULL
            AND role_receipt_hash IS NOT NULL
            AND octet_length(role_receipt_hash) = 32
            AND role_intent_bytes IS NOT NULL
            AND role_intent_hash IS NOT NULL
            AND octet_length(role_intent_hash) = 32)
    ),
    ADD CONSTRAINT ownership_journal_role_receipt_requires_auth CHECK (
        role_receipt_bytes IS NULL OR auth_receipt_id IS NOT NULL
    ),
    ADD CONSTRAINT ownership_journal_prepared_has_role_receipt CHECK (
        state <> 'prepared' OR role_receipt_bytes IS NOT NULL
    ),
    ADD CONSTRAINT ownership_journal_role_receipt_state CHECK (
        role_receipt_bytes IS NULL OR state IN (
            'prepared','abort_decided','commit_decided','completed','aborted'
        )
    ),
    ADD CONSTRAINT ownership_journal_pending_audit_all_or_none CHECK (
        (pending_audit_action IS NULL
            AND pending_audit_target_type IS NULL
            AND pending_audit_target_id IS NULL
            AND pending_audit_details IS NULL)
        OR
        (pending_audit_action IS NOT NULL
            AND pending_audit_target_type IS NOT NULL
            AND pending_audit_target_id IS NOT NULL
            AND pending_audit_details IS NOT NULL
            AND pending_audit_action = 'ownership_transferred'
            AND pending_audit_target_type = 'profile'
            AND pending_audit_target_id = new_owner_profile_id
            AND pending_audit_details = '{}'::jsonb)
    ),
    ADD CONSTRAINT ownership_journal_commit_has_pending_artifacts CHECK (
        state <> 'commit_decided' OR (
            role_receipt_bytes IS NOT NULL
            AND pending_audit_action IS NOT NULL
        )
    ),
    ADD CONSTRAINT ownership_journal_pending_audit_state CHECK (
        pending_audit_action IS NULL OR state IN ('commit_decided','completed')
    ),
    ADD CONSTRAINT ownership_journal_pending_audit_requires_role CHECK (
        pending_audit_action IS NULL OR role_receipt_bytes IS NOT NULL
    );

CREATE TABLE ownership_outbox (
    event_id UUID PRIMARY KEY,
    operation_id UUID NOT NULL UNIQUE,
    space_id UUID NOT NULL,
    previous_owner_profile_id UUID NOT NULL,
    new_owner_profile_id UUID NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type = 'space.updated'),
    ready BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (previous_owner_profile_id <> new_owner_profile_id)
);
