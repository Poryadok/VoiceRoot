DROP INDEX IF EXISTS gis_game_command_result_operation_idx;
DROP TRIGGER IF EXISTS gis_game_command_results_truncate_guard ON gis_game_command_results;
DROP TRIGGER IF EXISTS gis_game_command_results_delete_guard ON gis_game_command_results;
DROP TRIGGER IF EXISTS gis_game_command_results_update_guard ON gis_game_command_results;
DROP FUNCTION IF EXISTS gis_game_command_result_reject_mutation();
DROP TABLE IF EXISTS gis_game_command_results;
ALTER TABLE gis_game_action_operations
  DROP CONSTRAINT IF EXISTS gis_game_action_operations_operation_status_check;
ALTER TABLE gis_game_action_operations
  ADD CONSTRAINT gis_game_action_operations_operation_status_check
  CHECK (operation_status IN ('accepted','cancelled','reconciliation_required'));
