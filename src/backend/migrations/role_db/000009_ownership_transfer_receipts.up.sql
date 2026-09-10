CREATE TABLE ownership_transfer_role_receipts (
    operation_id UUID NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('apply', 'compensate')),
    space_id UUID NOT NULL,
    old_owner_profile_id UUID NOT NULL,
    new_owner_profile_id UUID NOT NULL,
    request_hash TEXT NOT NULL,
    current_owner_profile_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (operation_id, action)
);

CREATE INDEX ownership_transfer_role_receipts_operation_idx
    ON ownership_transfer_role_receipts (operation_id);
