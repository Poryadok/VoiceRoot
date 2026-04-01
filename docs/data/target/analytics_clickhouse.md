# ClickHouse — целевая схема (Analytics)

**Сервис:** Analytics ([analytics-service.md](../../microservices/analytics-service.md)). **Шаг порядка:** 17.

OLAP-хранилище; **не PostgreSQL**. Буфер перед записью — **Redis** (батчи), не таблицы CH.

Ниже — целевая форма из документа сервиса; детальные MV и витрины наращиваются по [ARCHITECTURE_REQUIREMENTS.md](../../ARCHITECTURE_REQUIREMENTS.md).

---

## Таблица сырых событий `events`

```sql
CREATE TABLE events (
  event_id UUID,
  event_type String,
  source_service String,
  timestamp DateTime64(3),
  user_id String,
  profile_id String,
  properties String,
  session_id Nullable(String),
  platform Nullable(String),
  app_version Nullable(String),
  region Nullable(String)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (event_type, timestamp)
TTL timestamp + INTERVAL 90 DAY;
```

- `user_id` / `profile_id` — по политике **без сырого PII** (хэш/псевдоним), как в [analytics-service.md](../../microservices/analytics-service.md).
- `properties` — JSON строкой.

---

## Материализованные представления (примеры целевого набора)

- **DAU:** агрегат `uniqState(user_id)` по `toDate(timestamp)`.
- **События по типу и дню:** `SummingMergeTree` / `AggregatingMergeTree` с группировкой `(date, event_type)`.

Точные DDL дополнительных MV — по метрикам из раздела «Дашборды» в [analytics-service.md](../../microservices/analytics-service.md).

---

## Связь с микросервисами

Источник — **NATS** (все релевантные streams); Analytics не владеет PostgreSQL-БД продуктовых сервисов.
