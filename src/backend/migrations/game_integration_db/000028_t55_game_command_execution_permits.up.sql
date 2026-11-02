CREATE TABLE IF NOT EXISTS gis_game_command_execution_permits (
  permit_id UUID PRIMARY KEY,
  command_id UUID NOT NULL UNIQUE REFERENCES gis_game_command_outbox(command_id),
  operation_id UUID NOT NULL REFERENCES gis_game_action_operations(operation_id),
  binding_id UUID NOT NULL REFERENCES player_bindings(binding_id),
  application_id UUID NOT NULL,
  environment_id UUID NOT NULL,
  binding_revision BIGINT NOT NULL CHECK (binding_revision > 0),
  permit_issued_at TIMESTAMPTZ NOT NULL,
  start_before TIMESTAMPTZ NOT NULL,
  complete_before TIMESTAMPTZ NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('issued', 'committed', 'aborted', 'expired')),
  completion_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
  CHECK (start_before > permit_issued_at AND start_before <= permit_issued_at + interval '10 seconds'),
  CHECK (complete_before > start_before AND complete_before <= permit_issued_at + interval '60 seconds'),
  CHECK ((status = 'issued' AND completion_at IS NULL) OR
         (status <> 'issued' AND completion_at IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS gis_game_command_permit_drain_idx
  ON gis_game_command_execution_permits(binding_id, complete_before)
  WHERE status = 'issued';
