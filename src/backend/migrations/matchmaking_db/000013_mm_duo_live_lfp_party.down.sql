UPDATE games
SET config = jsonb_set(
    config,
    '{modes,0,party_size_max}',
    '1'::jsonb,
    true
)
WHERE lower(name) = lower('MM Duo Live')
  AND status = 'active';
