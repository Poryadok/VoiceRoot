# Product Analytics — продуктовая аналитика

Сбор, хранение и отчётность по **продуктовым событиям** платформы: регистрации, сообщения, войс, матчмейкинг, подписки, модерация и т.д. Цель — дать команде Voice **DAU/MAU, воронки, retention и операционные product-метрики** без ручного SQL по десятку сервисов.

**Не путать с [observability.md](observability.md):** Prometheus/Loki/Grafana — operational monitoring (ошибки, latency, инфра); ClickHouse + Analytics Service — **продуктовая** телеметрия для staff и product-решений.

Стек и контракты: [MICROSERVICES.md](../MICROSERVICES.md), [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md). Техническая спека сервиса — [analytics-service.md](../microservices/analytics-service.md). Деплой — [DEPLOYMENT.md](../DEPLOYMENT.md) § Product analytics.

---

## Цель и границы

### Цель

После включения фичи команда (product / ops / staff) может:

1. Увидеть **активность платформы** (DAU, сообщения/день, регистрации) в staff UI и Grafana.
2. Построить **воронки** (регистрация → онбординг → первое сообщение) и **retention** (D1/D7/D30).
3. Экспортировать срезы в **CSV/JSON** для ad-hoc анализа с **audit log** на Gateway.
4. Отследить **здоровье ingest-пайплайна** (lag NATS → ClickHouse) через Prometheus.

### In scope (v1)

| Область | Что входит |
|---------|------------|
| **Ingest** | Dual ingest: JetStream domain streams (`message.events`, `user.events`, …) + прямые `analytics.*` от Notification, Search, Gateway, Subscription, Moderation |
| **Хранение** | ClickHouse: raw events (TTL 90 дней), materialized views для DAU и агрегатов |
| **Приватность** | `account_id` / `profile_id` — HMAC-хеш; без содержимого сообщений и PII в properties |
| **Query API** | Staff-only REST через Gateway: dashboards, metrics, funnel, retention, export |
| **Admin UI** | React Admin: `/analytics/product`, `/analytics/funnels`, `/analytics/export` |
| **Observability ingest** | Prometheus: `analytics_ingest_*`, Grafana dashboards ingest + product |

### Out of scope (не блокирует v1)

| Область | Почему отложено |
|---------|-----------------|
| **Клиентская product telemetry** (Flutter → backend напрямую) | v1 — server-side events из NATS; RUM — отдельная итерация |
| **Self-serve analytics для спейсов** | Только платформенный staff; space-owner dashboards — post-MVP |
| **Real-time streaming dashboards** | Batch flush 5s / 1000 events достаточен для product KPI |
| **Federation analytics** | Федерация deferred; дашборд Federation в CH — заготовка под будущее |
| **Полный backfill истории** | Опциональный replay JetStream по runbook; по умолчанию — с момента включения |

---

## Аудитория и доступ

| Кто | Доступ |
|-----|--------|
| Обычный пользователь (Flutter) | **Нет** — `/api/v1/analytics/**` → **403** |
| Staff / модератор с platform role | JWT + проверка роли персонала на Gateway → dashboards, export |
| S2S / сервисы | Ingest через NATS и gRPC `AnalyticsIngestService`, не через публичный REST |

Export и широкие metrics-запросы **обязательно** пишут audit log (subject, маршрут, время) на Gateway. Детали RBAC — [api-gateway.md](../microservices/api-gateway.md).

---

## Что измеряем

### Источники событий

1. **Domain streams** — Messaging, Chat, User, Social, Voice, Matchmaking, Story, Bot, … публикуют в JetStream; Analytics подписывается адаптерами и нормализует в `AnalyticsEvent`.
2. **Direct telemetry** — сервисы без отдельного domain stream шлют в `analytics.{service}.{event}`:
   - Notification — доставка push
   - Search — запросы, zero-result
   - Gateway — sampled REST latency (`GATEWAY_ANALYTICS_SAMPLE_RATE`, default off)
   - Subscription — платежи, churn-сигналы
   - Moderation — репорты, resolution

Subject pattern и матрица streams — [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md), раздел Analytics telemetry.

### Дашборды (staff)

| Дашборд | Метрики |
|---------|---------|
| **Product** | DAU, MAU, WAU, новые регистрации, воронка регистрации, completion онбординга |
| **Engagement** | Сообщения/день, минуты войса, MM-сессии, активные спейсы, сторис |
| **Revenue** | MRR, churn, free→paid, ARPU, LTV, payment failures |
| **Health** | p50/p95/p99 API latency (sampled), error rate, WS connections, queue depth |
| **Moderation** | Репорты/день, avg resolution time, auto-block rate, appeals |
| **Search** | Queries/day, zero-result rate, avg click position |
| **Voice** | Concurrent calls, avg duration, screen shares, codec mix |

Grafana: `voice-analytics-*.json` в [`deploy/observability/grafana/`](../../deploy/observability/grafana/). Admin UI дублирует ключевые product-панели через REST.

---

## Архитектура (кратко)

```
Domain NATS streams ──► stream adapters ──┐
analytics.* publishers ──► ingest gRPC ───┤
                                          ▼
                               batch buffer (5s / 1000)
                                          ▼
                                    ClickHouse
                                          ▼
                         AnalyticsQueryService → Gateway (staff REST)
                                          ▼
                              Admin UI / Grafana
```

Operational метрики ingest — Prometheus `/metrics` на `voice-analytics`. Пороги эволюции пайплайна (когда усложнять буфер/CH) — [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) § «Аналитика».

**Владелец:** [Analytics Service](../microservices/analytics-service.md) (Go, ClickHouse, in-memory batch buffer). Postgres для analytics **нет** — см. [DATA_STORES.md](../DATA_STORES.md).

---

## Локальная проверка

| Команда / тест | Назначение |
|----------------|------------|
| `make compose-app-up` | ClickHouse + analytics + admin (порт `ADMIN_PORT`, default 9081) |
| `GATEWAY_STATIC_TOKENS_JSON` → staff token | Доступ к `/api/v1/analytics/**` |
| `VOICE_RUN_LIVE_COMPOSE=true go test ./... -run TestComposeAnalytics_live` | Dashboard product: staff **200**, user **403** |
| `TestComposeAnalyticsExport_live` | Export CSV + audit log |
| `make build-all` + unit/integration analytics | CI green |

Opt-in флаги и каталог live-тестов — [TESTING.md](../TESTING.md).

---

## Критерии готовности (Definition of Done)

Фича считается **готовой**, когда на **compose** (и задокументировано для **staging**) выполнено:

1. Событие `message.sent` в compose → строка в ClickHouse **< 60s**; product dashboard отражает активность.
2. Staff JWT → `GET /api/v1/analytics/dashboard/product` **200**; обычный user → **403**.
3. Export CSV (`GET /api/v1/analytics/export`) пишет **audit log** на Gateway.
4. Метрика `analytics_ingest_lag_seconds` на `/metrics` analytics + Grafana ingest dashboard.
5. Telemetry Notification / Search / Gateway / Subscription / Moderation попадает в CH **без дублирования** domain events.
6. `make build-all` + unit/integration tests analytics green; opt-in live tests — [TESTING.md](../TESTING.md).
7. **Staging:** ClickHouse + `voice-analytics` + Grafana datasource; admin deploy описан в [DEPLOYMENT.md](../DEPLOYMENT.md).

Post-MVP gaps (ops-секреты, расширенные дашборды) — [TODO.md](../TODO.md).

---

## Зависимости от других фич

- **Gateway** — staff RBAC, transcoding REST, sampled API telemetry, audit log на export
- **Auth / Role** — claims staff role для analytics routes
- **Все domain publishers** — события в JetStream (Messaging, User, Chat, …)
- **observability** — общий Grafana stack; product CH datasource отдельно от Loki/Prometheus SLO
- **Admin (`src/admin`)** — UI для product/funnels/export

---

## Связанные документы

- [analytics-service.md](../microservices/analytics-service.md) — gRPC/REST, CH schema, Prometheus metrics
- [api-gateway.md](../microservices/api-gateway.md) — `/api/v1/analytics/**`, staff-only
- [DATA_STORES.md](../DATA_STORES.md) — ClickHouse, env vars
- [DEPLOYMENT.md](../DEPLOYMENT.md) — compose и staging rollout
- [observability.md](observability.md) — operational monitoring (отдельная фича)
