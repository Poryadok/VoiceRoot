ALTER TABLE player_bindings DROP CONSTRAINT player_bindings_status_check;
ALTER TABLE player_bindings ADD CONSTRAINT player_bindings_status_check
    CHECK (status IN ('pending', 'active', 'revoking', 'revoked'));

CREATE TABLE player_binding_execution_permits (
    permit_id UUID PRIMARY KEY,
    binding_id UUID NOT NULL REFERENCES player_bindings(binding_id),
    operation_id UUID NOT NULL,
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    binding_revision BIGINT NOT NULL CHECK (binding_revision > 0),
    assertion_jti UUID NOT NULL,
    assertion_sha256 BYTEA NOT NULL CHECK (length(assertion_sha256) = 32),
    expires_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('issued', 'committed', 'aborted', 'expired')),
    completion_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (binding_id, operation_id),
    CHECK ((status = 'issued' AND completion_at IS NULL) OR
           (status <> 'issued' AND completion_at IS NOT NULL))
);

CREATE INDEX player_binding_execution_permits_drain_idx
    ON player_binding_execution_permits (binding_id, expires_at)
    WHERE status = 'issued';
