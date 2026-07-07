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
TTL timestamp + INTERVAL 90 DAY;

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
