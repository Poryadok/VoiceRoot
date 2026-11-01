ALTER TABLE gis_game_action_operations
  DROP CONSTRAINT gis_game_action_operations_operation_status_check;
ALTER TABLE gis_game_action_operations
  ADD CONSTRAINT gis_game_action_operations_operation_status_check
  CHECK (operation_status IN ('accepted','cancelled','reconciliation_required','succeeded','failed'));

CREATE TABLE gis_game_command_results (
  command_id UUID PRIMARY KEY REFERENCES gis_game_command_outbox(command_id),
  operation_id UUID NOT NULL UNIQUE REFERENCES gis_game_action_operations(operation_id),
  result_id UUID NOT NULL UNIQUE,
  result_status TEXT NOT NULL CHECK (result_status IN ('succeeded','failed')),
  state_version TEXT NOT NULL CHECK (length(state_version) BETWEEN 1 AND 256),
  summary TEXT NOT NULL CHECK (length(summary) BETWEEN 1 AND 512),
  receipt_sha256 BYTEA NOT NULL CHECK (octet_length(receipt_sha256)=32),
  receipt_body BYTEA NOT NULL CHECK (octet_length(receipt_body) BETWEEN 1 AND 4096),
  received_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE OR REPLACE FUNCTION gis_game_command_result_reject_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'game command result receipts are immutable' USING ERRCODE = '55000';
END;
$$;
CREATE TRIGGER gis_game_command_results_update_guard BEFORE UPDATE ON gis_game_command_results
  FOR EACH ROW EXECUTE FUNCTION gis_game_command_result_reject_mutation();
CREATE TRIGGER gis_game_command_results_delete_guard BEFORE DELETE ON gis_game_command_results
  FOR EACH ROW EXECUTE FUNCTION gis_game_command_result_reject_mutation();
CREATE TRIGGER gis_game_command_results_truncate_guard BEFORE TRUNCATE ON gis_game_command_results
  FOR EACH STATEMENT EXECUTE FUNCTION gis_game_command_result_reject_mutation();

CREATE INDEX gis_game_command_result_operation_idx ON gis_game_command_results(operation_id,received_at);
