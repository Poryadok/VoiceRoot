-- match_ratings is the authoritative vote ledger; player_ratings is its
-- per-profile/game projection. Rebuild every projection with votes so a prior
-- split-write failure cannot leave its average or count permanently behind.
INSERT INTO player_ratings (
    profile_id, game_id, average_rating, total_ratings_received
)
SELECT
    mr.rated_profile_id,
    m.game_id,
    AVG(mr.score)::double precision,
    COUNT(*)::int
FROM match_ratings mr
JOIN matches m ON m.id = mr.match_id
GROUP BY mr.rated_profile_id, m.game_id
ON CONFLICT (profile_id, game_id) DO UPDATE SET
    average_rating = EXCLUDED.average_rating,
    total_ratings_received = EXCLUDED.total_ratings_received,
    updated_at = now();
