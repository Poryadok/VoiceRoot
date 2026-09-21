-- Auth owns account state. User stores the durable inactive overlay so every
-- profile projection writer can turn later mutations into revisioned deletes.
CREATE TABLE user_account_lifecycle (
    account_id UUID PRIMARY KEY,
    state TEXT NOT NULL CHECK (state = 'ACCOUNT_INACTIVE'),
    source_event_id UUID NOT NULL UNIQUE,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_account_lifecycle_inbox (
    event_id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
