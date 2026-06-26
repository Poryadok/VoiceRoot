# Observability — наблюдаемость (soft launch)

Единая спецификация для релиза наблюдаемости: логи, метрики, дашборды, алерты. Цель — **до прихода пользователей** иметь ответы на вопросы «что сломалось», «где тормозит», «сколько ошибок» без ручного `kubectl logs` по десятку подов.

Стек (канон): [MICROSERVICES.md](../MICROSERVICES.md) — **Prometheus + Grafana + Loki**. SLO и деградация: [OPERATIONS.md](../OPERATIONS.md). Окружения: [DEPLOYMENT.md](../DEPLOYMENT.md).

---

## Цель и границы релиза

### Цель

После внедрения фичи команда (1–2 человека на дежурстве) за **≤ 5 минут** может:

1. Увидеть **рост ошибок** и **деградацию latency** по пользовательским путям из SLO-таблицы.
2. По **`request_id`** (из клиента или заголовка ответа) пройти цепочку: Gateway → gRPC → NATS → Realtime WS.
3. Понять **состояние инфраструктуры** Tier 0: Postgres, Redis, NATS, диск, память подов.
4. Получить **алерт** до того, как пользователи массово пишут в поддержку.

### In scope (soft launch)

| Область | Что входит |
|---------|------------|
| **Окружения** | **staging** (обязательно до soft launch), **local** compose (желательно, паритет с staging) |
| **Сервисы** | Весь стек из [`deploy/staging/`](../deploy/staging/): Gateway, Auth, User, Social, Chat, Messaging, Realtime, Space, Role, Voice, File, Search, Matchmaking, Notification + Postgres, Redis, NATS, LiveKit |
| **Логи** | Сбор stdout всех подов в Loki; единый контракт полей; поиск по `request_id` |
| **Метрики** | RED на Gateway и Tier 0; gRPC server metrics на всех Go/Java gRPC-сервисах; exporters для Postgres, Redis, NATS; базовые k8s/node метрики |
| **Визуализация** | Grafana: Overview, Tier-0 paths, Logs explorer, Infra |
| **Алерты** | Alertmanager → Telegram или email; P1/P2 правила для Tier 0 |

### Out of scope (не блокирует soft launch)

| Область | Почему отложено |
|---------|-----------------|
| **Distributed tracing** (OTel + Jaeger/Tempo) | Correlation по `request_id` в логах достаточен для v1; трейсы — отдельная фаза |
| **Клиентские RUM-метрики** (Flutter → backend) | SLO измеряются серверно; клиент — позже |
| **Analytics Service / ClickHouse** | Продуктовая аналитика ≠ operational monitoring |
| **Полные runbooks и chaos-тесты** | [OPERATIONS.md](../OPERATIONS.md) — вне scope до стабильных прод-метрик |
| **Log-based alerting на каждую ошибку** | Шум; алерты на метрики + узкий набор log patterns (panic, JetStream stalled) |
| **Production HA** для Prometheus/Loki | Soft launch: single replica + PVC достаточно; HA — при росте нагрузки |
| **Централизованный audit log** модерации | Отдельная фича |

---

## Архитектура

```
┌─────────────┐     scrape      ┌──────────────┐     query    ┌─────────┐
│ App pods    │ ──────────────► │ Prometheus   │ ◄─────────── │ Grafana │
│ /metrics    │                 └──────────────┘              │         │
└─────────────┘                        ▲                     │ datasrc │
                                       │ alerts              │ Prom+Loki│
┌─────────────┐     push/read   ┌──────┴───────┐             └────▲────┘
│ stdout JSON │ ──► Promtail/   │ Alertmanager │                  │
│ (all pods)  │     Alloy       └──────────────┘                  │
└─────────────┘         │                                         │
                        └─────────────────────────────────────────┘
                                    Loki
```

**Namespace staging:** `voice-staging` (как в deploy). Observability-компоненты — отдельный namespace `voice-observability` или `monitoring` (зафиксировать один вариант при внедрении).

**Доступ:** Grafana — internal VPN или SSO; **не** публичный ingress без auth. Prometheus/Loki API — только внутри кластера.

---

## Контракт логов

### Общие правила

- Формат: **одна JSON-строка на событие** в stdout (уже так в Go; Auth — `logback-spring.xml`).
- Обязательные поля на **каждой** строке:

| Поле | Тип | Описание |
|------|-----|----------|
| `time` | ISO8601 | Время события |
| `level` | string | `debug` / `info` / `warn` / `error` |
| `service` | string | Имя сервиса: `gateway`, `auth`, `messaging`, … |
| `msg` | string | Человекочитаемое сообщение |
| `request_id` | string | `X-Request-Id`; пусто допустимо только для фоновых воркеров |

- **Не логировать:** пароли, токены (access/refresh), содержимое сообщений, PII сверх `profile_id` / `chat_id` / `message_id` (UUID).
- Уровень: env **`LOG_LEVEL`** (Go); Auth — `logging.level.root` / профиль Spring.
- Loki labels (на стороне collector): `namespace`, `service` (из label pod или парсинг JSON), `level` — **не** высококардинальные поля (`request_id`, `profile_id` — только в JSON, не labels).

### Типы событий (`event`)

Сквозная корреляция (уже в коде или **обязательно довести** до релиза):

| `event` | Где | Ключевые поля |
|---------|-----|----------------|
| `http_access` | Gateway, все Go HTTP; **Auth REST — добавить** | `method`, `path`, `status`, `duration_ms`, `request_id`; Gateway additionally: `route_group`, `remote_addr` |
| `grpc_call` | Все gRPC (Go `grpcmw`, Auth interceptor) | `grpc_method`, `grpc_code`, `duration_ms`, `request_id`, `error` (если есть) |
| `nats_publish` | Publishers (Messaging, Chat, User, Space, Voice, …) | `subject`, `request_id`, `event_id`, доменные id |
| `nats_consume` | Consumers (Realtime, Notification, Search, …) | `subject`, `request_id`, `event_id` |
| `ws_connect` | Realtime | `conn_id`, `profile_id`, `instance_id`, `request_id` |
| `ws_disconnect` | Realtime | то же |
| `ws_subscribe` / `ws_unsubscribe` | Realtime | `conn_id`, `chat_id`, `profile_id` |
| `ws_fanout` | Realtime | `op`, `chat_id`, `recipient_count`, `request_id`, `conn_ids` (cap 8) |

**Цепочка отладки** (из `.cursor/rules/debug-backend-logs.mdc`):

`http_access` (gateway) → `grpc_call` (messaging) → `nats_publish` → `nats_consume` (realtime) → `ws_fanout`

Запрос в Loki:

```logql
{namespace="voice-staging"} | json | request_id="<id>"
```

Сортировка по `time` — полный путь одного пользовательского действия.

### Пробелы в коде (закрыть в рамках фичи)

| Пробел | Требование |
|--------|------------|
| Auth REST без `http_access` | Фильтр как в Go `AccessLog`: method, path, status, duration_ms, event=`http_access` |
| `log.Printf` / неструктурированные логи | Заменить на `slog` с `service` + контекстом (Voice publish errors, и т.п.) |
| Scaffold-сервисы без gRPC access log | Подключить `grpcmw.ServerOptions` везде, где есть gRPC server |
| Flutter / Developer Portal | Вне scope server-side фичи; клиент уже шлёт `X-Request-Id` |

---

## Контракт метрик

### Принципы

- Экспорт: HTTP **`GET /metrics`** (Prometheus text format 0.0.4).
- Go: библиотека **`github.com/prometheus/client_golang`** (заменить ручную реализацию в Gateway).
- Java Auth: **Spring Boot Actuator** + `micrometer-registry-prometheus`; endpoint `/actuator/prometheus` (или `/metrics` — один путь на сервис, зафиксировать).
- Имена: `voice_<subsystem>_<name>_<unit>` или стандартные `http_*`, `grpc_*` от middleware.
- Labels: **низкая кардинальность** — не `user_id`, не полный `path` с id; на Gateway — `route_group` (как сейчас), не raw URL.

### Gateway (edge)

| Метрика | Тип | Labels | Назначение |
|---------|-----|--------|------------|
| `gateway_http_requests_total` | Counter | `route_group`, `method`, `status` | Availability, error rate |
| `gateway_http_request_duration_seconds` | Histogram | `route_group`, `method` | p95/p99 SLO |
| `gateway_ratelimit_hits_total` | Counter | `group` | Злоупотребления, misconfig |
| `gateway_force_update_blocks_total` | Counter | `platform` | Force-update policy |
| `gateway_ws_proxy_active` | Gauge | — | Активные WS через proxy (добавить; в доке gateway было `gateway.ws.connections`) |

Buckets histogram: `0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10` (секунды).

### Go gRPC-сервисы (все с gRPC в staging)

Стандартный набор через interceptors (общий пакет `pkg/grpcmw` или prometheus grpc middleware):

| Метрика | Labels |
|---------|--------|
| `grpc_server_handled_total` | `grpc_service`, `grpc_method`, `grpc_code` |
| `grpc_server_handling_seconds` | `grpc_service`, `grpc_method` |

Минимум на **Tier 0**: messaging, chat, user, realtime (gRPC если есть), social.

### Realtime (дополнительно к gRPC)

| Метрика | Тип | Назначение |
|---------|-----|------------|
| `realtime_ws_connections_active` | Gauge | Нагрузка, утечки |
| `realtime_ws_connect_total` | Counter | `code` (success/fail) |
| `realtime_ws_hello_duration_seconds` | Histogram | SLO WS handshake |
| `realtime_nats_consume_lag` | Gauge | `stream`, `consumer` — если доступно из NATS API |

### Auth (Java)

| Метрика | Назначение |
|---------|------------|
| `http_server_requests_*` | Micrometer default — login/refresh latency |
| `jvm_*`, `process_*` | Память, GC |
| `hikaricp_connections_*` | Пул БД |
| Custom: `auth_login_total` | Counter `result=success|failure` (без email в labels) |

### Инфраструктура (exporters)

| Компонент | Exporter | Ключевые метрики |
|-----------|----------|------------------|
| Postgres | `postgres_exporter` | connections, slow queries flag, up |
| Redis | `redis_exporter` | connected_clients, memory, commands/sec, **up** |
| NATS | NATS `/varz`, `/connz`, `/jsz` или `nats-exporter` | connections, JetStream stream lag, **up** |
| Kubernetes | kube-prometheus-stack default | pod restart, CPU, memory, not ready |
| LiveKit | LiveKit prometheus endpoint | rooms, participants (Tier 1 voice SLO) |
| Traefik (ingress) | встроенный metrics | 5xx на ingress |

### SLO ↔ метрики (soft launch)

| Путь ([OPERATIONS.md](../OPERATIONS.md)) | Метрика / запрос |
|------------------------------------------|------------------|
| Login / refresh | `gateway` route_group `auth` + `auth` http histogram; error rate |
| Send message | `gateway` namespace `messages` p95; `messaging` grpc `SendMessage` |
| WS hello | `realtime_ws_hello_duration_seconds` p95 |
| Join voice | `gateway` + `voice` grpc latency |
| List chats / history | `gateway` `chats` / `messages` GET p95 |

Recording rules в Prometheus (примеры):

- `slo:gateway:availability:5m` = ratio 2xx/все по route_group
- `slo:gateway:latency:p95:5m` = histogram_quantile по route_group

---

## Grafana — дашборды (минимальный набор)

### 1. Voice Overview

- Stat: общий RPS, global 5xx %, active WS
- Graph: p95 latency по route_group (top 5)
- Table: pods not ready, restart count за 1h
- Links → Tier-0 Paths, Logs

### 2. Tier-0 User Paths

Панели по каждому SLO-пути: availability, p95, p99, error budget burn (упрощённо — % 5xx за 1h vs цель).

### 3. Infrastructure

- Postgres: connections, disk
- Redis: memory, hit rate, **up**
- NATS: JetStream pending messages per stream (`chat.events`, `message.events`, …)
- Node: CPU/memory

### 4. Logs / Trace by Request ID

- Variable: `request_id`
- Log panel: Loki query с json parse
- Optional: timeline по `event` type

### 5. Voice & LiveKit (Tier 1)

- Voice gRPC errors, call setup latency
- LiveKit rooms/participants

Дашборды хранить в репозитории: `deploy/observability/grafana/dashboards/` (JSON), provisioning через ConfigMap.

---

## Алерты (Alertmanager)

### P1 — немедленная реакция

| Алерт | Условие (черновик) | Действие |
|-------|-------------------|----------|
| GatewayDown | `up{job="gateway"} == 0` 2m | Проверить pod, ingress |
| Tier0High5xx | 5xx rate > 5% за 5m на route_group ∈ {auth, messages, chats, ws} | Logs по route_group |
| RedisDown | `redis_up == 0` 1m | OPERATIONS: rate limit, blacklist, WS fan-out |
| NATSDown | NATS up == 0 1m | Live delivery degraded |
| PostgresDown | primary up == 0 1m | Полная недоступность |
| PodCrashLooping | restart > 3 за 15m | Конкретный сервис |

### P2 — в течение часа

| Алерт | Условие |
|-------|---------|
| GatewayLatencyHigh | p95 > SLO × 1.5 за 15m |
| JetStreamLag | pending messages > threshold per stream |
| DiskSpaceLow | < 15% на PVC postgres/loki |
| RateLimitSpike | `gateway_ratelimit_hits` аномалия |

### Notification

- Канал: Telegram bot или email (один на soft launch).
- **Inhibition:** если `PostgresDown` — не слать 20 производных алертов.
- Runbook link в annotation → раздел «Tier 0: отказ Redis и NATS» в [OPERATIONS.md](../OPERATIONS.md).

---

## Инфраструктура наблюдаемости

### Staging (обязательно)

| Компонент | Развёртывание | Хранение |
|-----------|---------------|----------|
| Prometheus | Helm `kube-prometheus-stack` или отдельный chart | PVC 20–50Gi, retention **15d** |
| Grafana | В составе stack | PVC для dashboards/preferences |
| Loki | Helm `loki-stack` или `grafana/loki` single binary | PVC 50Gi, retention **7d** |
| Promtail / Alloy | DaemonSet | — |
| Alertmanager | В составе kube-prometheus-stack | — |

**Scrape config:**

- `ServiceMonitor` / аннотации `prometheus.io/scrape` на каждый Deployment с `/metrics`
- static scrape для exporters (postgres, redis, nats)
- не скрейпить `/health` как метрики

### Local (желательно)

- Compose profile `observability`: Prometheus, Grafana, Loki, Promtail
- Scrape targets: host gateway port + compose service names
- Паритет LogQL запросов со staging

### Retention (soft launch)

| Данные | Срок |
|--------|------|
| Loki logs | 7 дней |
| Prometheus metrics | 15 дней |
| Alert history | 30 дней |

---

## Безопасность и compliance

- Grafana: admin password из Secret; смена дефолта обязательна.
- `/metrics` и Loki **не** выставлять на публичный ingress.
- В логах и метриках **нет** содержимого сообщений, email, телефонов.
- `profile_id` / `chat_id` — UUID, допустимы для отладки.

---

## Критерии готовности (Definition of Done)

Фича считается **готовой к soft launch**, когда выполнено всё ниже на **staging**:

### Логи

- [ ] Все поды приложения и infra пишут в Loki
- [ ] Поиск по `request_id` от Gateway до `ws_fanout` работает на E2E сценарии (отправка DM-сообщения)
- [ ] Auth REST пишет `http_access` в том же контракте, что Go
- [ ] Нет неструктурированного `log.Printf` на горячих путях Tier 0

### Метрики

- [ ] Gateway `/metrics` на client_golang + histogram
- [ ] Tier 0 сервисы отдают gRPC + process metrics
- [ ] Auth `/actuator/prometheus` в scrape
- [ ] Redis, Postgres, NATS exporters в scrape и на дашборде Infra
- [ ] Recording rules для SLO-путей созданы

### Grafana

- [ ] 4 обязательных дашборда (Overview, Tier-0, Infra, Logs) импортированы из git
- [ ] Datasource Prometheus + Loki настроены

### Алерты

- [ ] P1 правила активны, тестовый firing → сообщение в канал
- [ ] Inhibition для infra cascade

### Документация и процесс

- [ ] Краткий runbook «как дебажить по request_id» в [TESTING.md](../TESTING.md) или README staging
- [ ] `make compose-logs-collect` остаётся для local; не конфликтует с Loki profile

### Smoke после деплоя observability

1. `kubectl get pods -n voice-observability` — все Running
2. Grafana → Overview — видны targets UP
3. Отправить сообщение на staging → найти `request_id` в Gateway → цепочка в Loki
4. Prometheus → `gateway_http_requests_total` растёт
5. Synthetic alert test (Alertmanager amtool)

---

## Зависимости от других фич

- **Gateway** — owner edge metrics и `route_group` taxonomy
- **Realtime** — WS metrics и ws_* log events
- **Messaging** — publish/consume log attrs с `message_id`, `chat_id`
- **DEPLOYMENT** — namespace, ingress; observability не в публичном FQDN
- **OPERATIONS** — пороги алертов сверять с SLO-таблицей

---

## Связанные документы

- [MICROSERVICES.md](../MICROSERVICES.md) — целевой стек
- [OPERATIONS.md](../OPERATIONS.md) — SLO, Tier 0/1/2, Redis/NATS degraded
- [DEPLOYMENT.md](../DEPLOYMENT.md) — staging/prod
- [microservices/api-gateway.md](../microservices/api-gateway.md) — метрики Gateway
- [TESTING.md](../TESTING.md) — smoke после деплоя
