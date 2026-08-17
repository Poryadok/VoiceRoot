-- Compose live match tests: 2-slot solo queue game (keep in sync with docker/postgres/matchmaking_db_matches.sql.snippet).
INSERT INTO games (name, config, status)
SELECT
    'MM Duo Live',
    '{
      "regions": ["eu"],
      "modes": [{
        "name": "Duo",
        "slots": 2,
        "party_size_min": 1,
        "party_size_max": 1,
        "roles_required": false,
        "rank_required": false
      }]
    }'::jsonb,
    'active'
WHERE NOT EXISTS (
    SELECT 1 FROM games WHERE lower(name) = lower('MM Duo Live') AND status = 'active'
);
