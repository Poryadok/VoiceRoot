# Analytics Service

## Обзор

Сбор, хранение и визуализация аналитических данных со всех микросервисов.

**Язык**: Go
**OLAP БД**: ClickHouse
**Буфер**: Redis (batch accumulator)

## Ответственность

- Консьюмер всех NATS-потоков (universal subscriber)
- Трансформация событий в аналитические записи
- Batch-запись в ClickHouse (каждые 5 сек или 1000 событий)
- Предагрегация метрик (DAU/MAU, retention, funnel)
- API для дашбордов (admin panel)
- Экспорт данных (CSV/JSON)
- Не хранит PII (только anonymized/hashed идентификаторы где возможно)
- Retention: raw events 90 дней, агрегаты бессрочно

## Архитектура

```
NATS (all streams) ──► Analytics Service
                          │
                    ┌─────┴──────┐
                    │ Redis      │  batch buffer
                    │ accumulator│
                    └─────┬──────┘
                          │ flush every 5s / 1000 events
                          ▼
                    ┌─────────────┐
                    │ ClickHouse  │
                    │ (OLAP)      │
                    └─────┬───────┘
                          │
                    ┌─────▼──────┐
                    │ Pre-agg    │  materialized views
                    │ tables     │
                    └─────┬──────┘
                          │
                    ┌─────▼──────┐
                    │ Admin API  │──► React Admin Panel
                    └────────────┘
```

## API (gRPC + REST)

```protobuf
// gRPC (internal)
service AnalyticsIngest {
  rpc IngestEvent(AnalyticsEvent) returns (Empty);
  rpc IngestBatch(AnalyticsEventBatch) returns (Empty);
}
```

```
// REST (admin panel)
GET  /api/v1/analytics/dashboard/{type}  — product, engagement, revenue, health, moderation
GET  /api/v1/analytics/metrics           — custom query
GET  /api/v1/analytics/funnel/{name}     — registration, onboarding, conversion
GET  /api/v1/analytics/retention         — D1, D7, D30 cohorts
GET  /api/v1/analytics/export            — CSV/JSON export
```

## ClickHouse схема

### Raw events table
```sql
CREATE TABLE events (
  event_id UUID,
  event_type String,
  source_service String,
  timestamp DateTime64(3),
  user_id String,       -- hashed
  profile_id String,    -- hashed
  properties String,    -- JSON
  session_id Nullable(String),
  platform Nullable(String),
  app_version Nullable(String),
  region Nullable(String)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (event_type, timestamp)
TTL timestamp + INTERVAL 90 DAY;
```

### Materialized views (примеры)

```sql
-- DAU
CREATE MATERIALIZED VIEW dau_mv
ENGINE = AggregatingMergeTree()
ORDER BY (date)
AS SELECT
  toDate(timestamp) AS date,
  uniqState(user_id) AS unique_users
FROM events
GROUP BY date;

-- Events per type per day
CREATE MATERIALIZED VIEW events_by_type_mv
ENGINE = SummingMergeTree()
ORDER BY (date, event_type)
AS SELECT
  toDate(timestamp) AS date,
  event_type,
  count() AS event_count
FROM events
GROUP BY date, event_type;
```

## Дашборды

| Дашборд     | Метрики                                                                    |
|-------------|----------------------------------------------------------------------------|
| Product     | DAU, MAU, WAU, new registrations, registration funnel, onboarding completion |
| Engagement  | Messages/day, voice minutes, MM sessions, active spaces, stories created   |
| Revenue     | MRR, churn rate, free→paid conversion, ARPU, LTV, payment failures        |
| Health      | p50/p95/p99 API latency, error rate, WS connections, uptime, queue depth   |
| Moderation  | Reports/day, avg resolution time, auto-block rate, appeals rate            |
| Federation  | Connected nodes, event sync lag, sync failures, notification relay latency |
| Search      | Queries/day, zero-result rate, avg result click position                   |
| Voice       | Concurrent calls, avg call duration, screen shares, codec distribution     |

## Публикуемые события

Analytics Service — чистый consumer, не публикует события в NATS.

Экспортирует метрики в Prometheus для:
- `analytics.ingest.events_per_second`
- `analytics.ingest.batch_size`
- `analytics.ingest.lag_seconds`
- `analytics.clickhouse.insert_latency`

## Зависимости

- **NATS** — подписка на все потоки событий
- **Redis** — batch buffer перед записью в ClickHouse
- **ClickHouse** — хранение и агрегация
- **Prometheus** — экспорт operational метрик
- **Grafana** — визуализация (подключение к ClickHouse + Prometheus)
