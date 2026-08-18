-- LFP JOIN accept builds a 2-member party before enqueue; party_size_max must allow 2.
UPDATE games
SET config = jsonb_set(
    config,
    '{modes,0,party_size_max}',
    '2'::jsonb,
    true
)
WHERE lower(name) = lower('MM Duo Live')
  AND status = 'active'
  AND (config->'modes'->0->>'party_size_max')::int < 2;
