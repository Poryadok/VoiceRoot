CREATE TABLE gis_game_action_projection_outbox (
    operation_id UUID PRIMARY KEY REFERENCES gis_game_action_operations(operation_id),
    result_id UUID NOT NULL UNIQUE REFERENCES gis_game_command_results(result_id),
    message_id UUID NOT NULL,
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    action_id UUID NOT NULL,
    state_version TEXT NOT NULL,
    result_status TEXT NOT NULL CHECK (result_status IN ('succeeded','failed')),
    safe_summary TEXT NOT NULL,
    delivery_status TEXT NOT NULL DEFAULT 'pending' CHECK (delivery_status IN ('pending','delivered')),
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    lease_id UUID,
    lease_until TIMESTAMPTZ,
    last_error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    delivered_at TIMESTAMPTZ,
    CHECK ((lease_id IS NULL) = (lease_until IS NULL))
);
CREATE INDEX gis_game_action_projection_due_idx
    ON gis_game_action_projection_outbox(next_attempt_at,created_at)
    WHERE delivery_status='pending';

INSERT INTO gis_game_action_projection_outbox(operation_id,result_id,message_id,application_id,environment_id,action_id,state_version,result_status,safe_summary)
SELECT r.operation_id,r.result_id,o.message_id,o.application_id,o.environment_id,o.action_id,r.state_version,r.result_status,r.summary
FROM gis_game_command_results r JOIN gis_game_action_operations o ON o.operation_id=r.operation_id;
