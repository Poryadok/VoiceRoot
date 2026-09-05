-- Auth-owned durable guest conversion state machine and retry metadata.
CREATE TABLE guest_conversion_operations (
    operation_id UUID PRIMARY KEY,
    account_id UUID NOT NULL UNIQUE,
    otp_code_id UUID NOT NULL UNIQUE,
    state VARCHAR(32) NOT NULL CHECK (state IN ('PENDING_USER', 'PENDING_EVENT', 'COMPLETED')),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_until TIMESTAMPTZ NULL,
    last_error_code TEXT NULL,
    user_marked_at TIMESTAMPTZ NULL,
    auth_promoted_at TIMESTAMPTZ NULL,
    event_published_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
