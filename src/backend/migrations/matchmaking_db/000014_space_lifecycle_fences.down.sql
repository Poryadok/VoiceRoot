DROP TRIGGER IF EXISTS matchmaking_space_match_result_fence ON matches;
DROP FUNCTION IF EXISTS matchmaking_gate_space_match_result();
DROP TRIGGER IF EXISTS matchmaking_space_search_session_fence ON search_sessions;
DROP FUNCTION IF EXISTS matchmaking_gate_space_search_session();
DROP TABLE IF EXISTS matchmaking_space_lifecycle_purge_receipts;
DROP TABLE IF EXISTS matchmaking_space_lifecycle_fence_receipts;
DROP TABLE IF EXISTS matchmaking_space_lifecycle_fence_heads;
