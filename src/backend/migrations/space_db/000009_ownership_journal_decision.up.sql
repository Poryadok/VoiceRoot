-- Additive Auth receipt evidence for the disconnected v2 ownership journal.
ALTER TABLE ownership_journal
    ADD COLUMN auth_receipt_id UUID,
    ADD COLUMN auth_account_id UUID,
    ADD COLUMN auth_profile_id UUID,
    ADD COLUMN auth_space_id UUID,
    ADD COLUMN auth_new_owner_profile_id UUID,
    ADD COLUMN auth_operation_id UUID,
    ADD COLUMN auth_session_epoch BIGINT,
    ADD COLUMN auth_consumed_at TIMESTAMPTZ,
    ADD COLUMN auth_verified_factors TEXT[],
    ADD CONSTRAINT ownership_journal_auth_receipt_all_or_none CHECK (
        (auth_receipt_id IS NULL
            AND auth_account_id IS NULL
            AND auth_profile_id IS NULL
            AND auth_space_id IS NULL
            AND auth_new_owner_profile_id IS NULL
            AND auth_operation_id IS NULL
            AND auth_session_epoch IS NULL
            AND auth_consumed_at IS NULL
            AND auth_verified_factors IS NULL)
        OR
        (auth_receipt_id IS NOT NULL
            AND auth_account_id IS NOT NULL
            AND auth_profile_id IS NOT NULL
            AND auth_space_id IS NOT NULL
            AND auth_new_owner_profile_id IS NOT NULL
            AND auth_operation_id IS NOT NULL
            AND auth_session_epoch IS NOT NULL
            AND auth_consumed_at IS NOT NULL
            AND auth_verified_factors IS NOT NULL)
    ),
    ADD CONSTRAINT ownership_journal_auth_receipt_matches_binding CHECK (
        auth_receipt_id IS NULL OR (
            auth_account_id = account_id
            AND auth_profile_id = actor_profile_id
            AND auth_space_id = space_id
            AND auth_new_owner_profile_id = new_owner_profile_id
            AND auth_operation_id = operation_id
            AND auth_session_epoch = session_epoch
        )
    ),
    ADD CONSTRAINT ownership_journal_reserved_has_no_receipt CHECK (
        state <> 'reserved' OR auth_receipt_id IS NULL
    ),
    ADD CONSTRAINT ownership_journal_proof_confirmed_has_receipt CHECK (
        state <> 'proof_confirmed' OR auth_receipt_id IS NOT NULL
    );
