BEGIN;
LOCK TABLE space_join_request_groups,space_join_request_attempts,space_join_request_selections IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM space_join_request_groups) OR EXISTS(SELECT 1 FROM space_join_request_attempts)
  OR EXISTS(SELECT 1 FROM space_join_request_selections)
 THEN RAISE EXCEPTION 'cannot roll back admitted join request mappings' USING ERRCODE='55000'; END IF;
END $$;
DROP TABLE space_join_request_selections;
DROP TABLE space_join_request_attempts;
DROP TABLE space_join_request_groups;
DROP FUNCTION validate_space_join_request_selection();
DROP FUNCTION validate_space_join_request_attempt();
DROP FUNCTION immutable_space_join_request_mapping();
COMMIT;
