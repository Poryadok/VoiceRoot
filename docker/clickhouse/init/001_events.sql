CREATE DATABASE IF NOT EXISTS voice;

CREATE TABLE IF NOT EXISTS voice.events (
  event_id UUID,
  event_type String,
  source_service String,
  timestamp DateTime64(3),
  user_id_hashed String DEFAULT '',
  profile_id_hashed String DEFAULT '',
  properties String,
  session_id Nullable(String),
  platform Nullable(String),
  app_version Nullable(String),
  region Nullable(String)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (event_type, timestamp)
TTL toDateTime(timestamp) + INTERVAL 90 DAY;

-- Redelivery can insert the same immutable source event more than once. Keep raw
-- evidence unchanged and expose one logical row per stable event_id to readers.
CREATE VIEW IF NOT EXISTS voice.events_logical AS
SELECT
  event_id,
  any(event_type) AS event_type,
  any(source_service) AS source_service,
  any(timestamp) AS timestamp,
  any(user_id_hashed) AS user_id_hashed,
  any(profile_id_hashed) AS profile_id_hashed,
  any(properties) AS properties,
  any(session_id) AS session_id,
  any(platform) AS platform,
  any(app_version) AS app_version,
  any(region) AS region
FROM voice.events
GROUP BY event_id;

CREATE TABLE IF NOT EXISTS voice.dau_mv (
  date Date,
  unique_users AggregateFunction(uniq, String)
) ENGINE = AggregatingMergeTree()
ORDER BY date;

CREATE MATERIALIZED VIEW IF NOT EXISTS voice.dau_mv_mv TO voice.dau_mv AS
SELECT
  toDate(timestamp) AS date,
  uniqState(user_id_hashed) AS unique_users
FROM voice.events
WHERE user_id_hashed != ''
GROUP BY date;

CREATE TABLE IF NOT EXISTS voice.events_by_type_mv (
  date Date,
  event_type String,
  event_count UInt64
) ENGINE = SummingMergeTree()
ORDER BY (date, event_type);

CREATE MATERIALIZED VIEW IF NOT EXISTS voice.events_by_type_mv_mv TO voice.events_by_type_mv AS
SELECT
  toDate(timestamp) AS date,
  event_type,
  count() AS event_count
FROM voice.events
GROUP BY date, event_type;
