# Операции: SLO, деградация, релизы, миграции

Минимальный слой договорённостей для продакшена. Цифры SLO — **черновые цели v1** для выравнивания команды; после появления стабильных замеров (Prometheus и т.п.) — пересмотр.

**Error budget по сервисам, полноформатный chaos engineering и полные runbooks** — вне scope до стабильных прод-метрик и отдельного решения. Ниже зафиксированы **минимальные сценарии отказа** Redis и NATS для Tier 0 (ожидаемое поведение и UX), чтобы команда могла проектировать фичи и алерты без устных договорённостей.

Архитектура сервисов и БД: [MICROSERVICES.md](MICROSERVICES.md). Голос/LiveKit: [microservices/voice-service.md](microservices/voice-service.md). Матрица маршрутов и JetStream: [CONTRACT_MATRIX.md](CONTRACT_MATRIX.md).

---

## Целевые SLO по пользовательским путям

Измерения: по возможности **серверная** латентность (Gateway / целевой сервис) и **доступность** по успешным запросам/сессиям; клиентские метрики — уточнить при внедрении наблюдаемости.

| Пользовательский путь                                      | Доступность (месяц) | p95    | p99   | Заметки                                                                                                   |
|------------------------------------------------------------|---------------------|--------|-------|-----------------------------------------------------------------------------------------------------------|
| Логин / refresh (Auth + Gateway)                           | 99.5%               | 800 ms | 2 s   | Успешный ответ API; без учёта сторонних провайдеров email при исключении из критического пути             |
| Отправка сообщения (Gateway → Messaging, 2xx)              | 99.9%               | 400 ms | 1.5 s | От приёма запроса Gateway до ответа клиенту                                                               |
| WebSocket: handshake до первого `hello` (Realtime)         | 99.9%               | 500 ms | 2 s   | Успешный upgrade + первое серверное сообщение по [realtime-service.md](microservices/realtime-service.md) |
| Join voice: выдача сессии и токена (Voice → LiveKit-ready) | 99.5%               | 600 ms | 2 s   | Успешный ответ API; медиа-путь до SFU — отдельно на стороне LiveKit/сети                                  |
| Загрузка списка чатов / истории (типовой GET)              | 99.5%               | 600 ms | 2 s   | Пагинированный запрос; кэш на клиенте не входит в SLO                                                     |

Пути можно укрупнять или добавить (например поиск, файлы) по мере готовности продуктовых приоритетов.

---

## Порядок деградации

Если ресурсы или зависимости недоступны, **сначала** сохраняем Tier 0; Tier 1 и 2 могут отдавать ошибки или упрощённый UX без остановки текстового ядра.

| Tier         | Сервисы / компоненты                                                                                                         | Комментарий                                                                                           |
|--------------|------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------|
| **0 — core** | API Gateway, Auth, User, Chat, Messaging, Realtime; Redis и NATS на критичных путях доставки                                 | Падение = продукт для основного сценария «чат» не работает                                            |
| **1**        | Voice Service + LiveKit, File, Notification, Social, **Role Service** (см. ниже)      | Голос/файлы/push/граф друзей; проверка прав в **спейсе** зависит от Role начиная с [PLAN.md](PLAN.md) Фазы 5+ |
| **2**        | Search, Analytics, Story, **Matchmaking** (по умолчанию), Federation, Bot (вспомогательный), Subscription (если допускается режим без биллинга) | Поиск, аналитика, сторис, очереди матчмейкинга — некритичны для сценария «текстовый DM + WS» |

**Role Service:** для **DM и standalone-текстовых чатов** (без спейса) Tier 0 может обходиться без Role. Как только в проде включены **спейсы с ролями и оверрайдами** ([spaces.md](features/spaces.md)), Role считается **Tier 1**: падение Role блокирует корректный вход в спейс и проверку прав в каналах/голосе, но не обязано глушить уже открытые DM-сессии, если продукт явно допускает degraded-режим без спейсов.

**Matchmaking:** **по умолчанию Tier 2** до отдельного продуктового релиза, где матчмейкинг объявлен критичным УТП. При таком релизе команда **повышает Matchmaking до Tier 1** (релиз-ноты, дашборды, алерты) на согласованный период или постоянно.

---

## Tier 0: отказ Redis и NATS (сценарии и degraded UX)

Цель — заранее согласовать **направление деградации**, а не идеальный автопилот. Конкретные код-пути (fail-open vs fail-closed) фиксируются в реализации Gateway / Auth / Realtime и в тестах; здесь — продуктово-операционный ориентир.

### Redis (кластер недоступен или партиция)

| Зона использования | Влияние на Tier 0 | Целевой degraded UX / политика |
|--------------------|-------------------|----------------------------------|
| **API Gateway**: rate limiting (sliding window) | Лимиты не считаются или отвечают ошибкой — зависит от реализации | Предпочтительно **кратковременный fail-open** с жёстким алертом (риск злоупотребления) либо **503** на записи с высоким риском (строже). Зафиксировать одно поведение в коде Gateway. |
| **API Gateway**: чтение JWT blacklist (`jti`) | Отозванные access token могут проходить до истечения TTL JWT | **Риск безопасности**: алерт P1; краткий fail-open допустим только если TTL access token короткий (см. [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md)). Долгая деградация — рассмотреть read replica / локальный кэш с оговорённым окном. |
| **Auth Service**: запись blacklist, OTP state | Logout / отзыв access может не сохраниться; OTP-throttle может «просесть» | Алерт; повтор операций пользователем; мониторинг аномалий по Auth. |
| **Realtime**: Pub/Sub между инстансами | События live **не доходят** между узлами (фрагментированный fan-out) | Клиенты в одном инстансе видят обновления, в другом — нет. **Degraded UX**: баннер «обновите чат / переподключение»; после reconnect — догрузка истории через Messaging ([ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md)). |
| **User Service**: presence (если в Redis) | Неточный или пустой presence | Некритично для доставки сообщений; показывать «статус неизвестен» или скрыть presence. |
| **User Service**: `privacy_settings` отсутствует / Social или Space S2S недоступен | Неверная видимость онлайн или DM / friend-request gate | **Fail-closed**: без строки `privacy_settings` онлайн скрыт (кроме self-view); при недоступности Social/Space S2S проверки friends/FoF/space_members **отклоняют** доступ (не открывают всем). Алерт при длительном отказе S2S. |
| **Voice lifecycle mirror** | Admission/receipt mirror недоступен | Возвращать unavailable. Только unblocked decided/replayable-completed state позже может быть rebuilt из `voice_db`; open divergence запрещает Redis access/rebuild/replay. Memory fallback и automatic key deletion запрещены. |

### Voice lifecycle Redis divergence (source-disabled)

R22.3 сохраняет durable incidents `orphan`, `decided`, `completed` и
`quarantined` с закрытым Redis class и redacted digest. Open evidence проверяется
в PostgreSQL до Redis/admission; Redis correction, equality, TTL или flush не
являются resolution. Метрики используют только closed low-cardinality labels.
Backup/restore включает incidents как часть `voice_db`; PG restore без Redis не
снимает fences, а Redis без PG никогда не становится authority.

Будущий protected operator tool может закрыть incident только как
`orphan_removed`, `mirror_restored_exact`, `mirror_reset_for_rebuild` или
`expired_operation_retired`. R22.3 не предоставляет этот tool. Business
quarantine после закрытия incident требует отдельного reviewed repair.

### NATS JetStream (брокер недоступен, переполнение, долгий NAK)

| Поток / роль | Влияние на Tier 0 | Целевой degraded UX / политика |
|--------------|-------------------|--------------------------------|
| **Publish** из Messaging / Chat / Space после успешного commit в БД | Сообщение **сохранено**, но downstream не уведомлён | REST **2xx** сохраняется. **Live** обновления (WS) могут задержаться или отсутствовать до восстановления шины. Клиент: polling не вводим по умолчанию — полагаться на **reconnect + GetMessages** по `chat_id`. |
| **Realtime** как consumer `chat.events` / `message.events` | Нет push live-событий в сокеты | Как при потере WS: догрузка истории; typing/presence — эфемерно, без гарантии catch-up. |
| **Notification / Search / Analytics** как consumers | Отложенные push, индекс, метрики | **Tier 1/2** по определению; не блокируют отправку сообщения в Tier 0. |

### Общие действия дежурного (черновик)

1. Подтвердить масштаб: только Redis, только NATS или оба (коррелировать с Dashboard Redis/NATS и 5xx Gateway).
2. Зафиксировать время начала инцидента для пост-морем и error budget.
3. Включить/проверить канарею отката недавнего деплоя, если отказ совпал с релизом.
4. Коммуникация в продукте: короткий статус + «сообщения могут приходить с задержкой; потяните вниз для обновления» при сохранении REST.

---

## Релизы: canary и rollback

- **Canary** по умолчанию для сервисов с пользовательским трафиком через Gateway или общий ingress. Стартовая доля **5–10%** трафика на новую версию — настраивается в CI/CD / mesh (конкретный инструмент не фиксируем).
- **Rollback** без обсуждения, если: резкий рост **5xx** выше согласованного порога на канаре или проде; регрессия по SLO из таблицы выше; **ошибка миграции БД** или несовместимость контракта после деплоя; критический баг с подтверждением дежурного.
- **Admin Panel**, внутренние воркеры, инструменты без пользовательского трафика — допускается **rolling update** без canary при низком риске.

---

## Миграции БД (database per service)

- **Инструменты:** Flyway (Auth / Java), golang-migrate (Go-сервисы). Каталог миграций — в репозитории рядом с кодом владельца БД; один сервис не трогает чужие `*_db`.
- **Владелец**: команда/владелец микросервиса, которому принадлежит БД из таблицы «Владение данными» в [MICROSERVICES.md](MICROSERVICES.md). Скрипты миграций живут **рядом с кодом этого сервиса** (репозиторий или модуль); нет одной общей миграции на все 20 БД.
- **Безопасный порядок изменения схемы**: фаза **expand** (добавить столбцы/таблицы, обратная совместимость) → деплой приложения, читающего/пишущего и старую, и новую форму → затем **contract** (удаление старых полей/таблиц) отдельным релизом, когда старых клиентов нет.
- **Несколько сервисов в одной фиче**: инициатор фичи задаёт **порядок деплоя** и совместимость gRPC/NATS контрактов (consumer не ломается раньше producer).
- **Удаление спейса и проекции**: публикуется доменное событие (например `space.deleted`); Role, Search и другие сервисы удаляют связанные строки **идемпотентно** (см. [microservices/role-service.md](microservices/role-service.md#модель-данных)). Порядок относительно Messaging и TTL сообщений канала — явно в дизайне фичи (сага / transactional outbox).
- **API Gateway**: обновление маршрутов и контракта REST — **после** готовности целевых бэкендов или за **feature flag** на маршруте.
- **Voice**: применять `000001_room_lifecycle`, затем
  `000002_redis_divergence` до rollout. DOWN `000002` берёт `ACCESS EXCLUSIVE`
  и отказывается при любой incident row, включая resolved history.

---

## Runtime configuration (env)

Shared timeouts for Go HTTP services and Postgres bootstrap. Values are Go `time.ParseDuration` strings (e.g. `30s`, `2m`). Invalid or empty values fall back to the default.

| Переменная | Default | Назначение |
|------------|---------|------------|
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | `net/http.Server.ReadHeaderTimeout` |
| `HTTP_READ_TIMEOUT` | `30s` | `ReadTimeout` (`0` disables; Realtime defaults to `0`) |
| `HTTP_WRITE_TIMEOUT` | `60s` | `WriteTimeout` (`0` disables; Realtime defaults to `0`) |
| `HTTP_IDLE_TIMEOUT` | `120s` | `IdleTimeout` |
| `HTTP_SHUTDOWN_TIMEOUT` | `10s` | Graceful HTTP shutdown deadline |
| `POSTGRES_CONNECT_TIMEOUT` | `15s` | `pgxpool.New` connect context |
| `GRPC_DIAL_TIMEOUT` | `15s` | Upstream gRPC ready wait at startup |

Staging overrides — `deploy/staging/configmap-app.yaml` (`GRPC_DIAL_TIMEOUT` is set; HTTP/Postgres keys are commented for optional tuning).

### Voice lifecycle database

`voice_db` is the durable source of truth for Voice room lifecycle state. Redis
contains a rebuildable projection; after Redis loss, rebuild the Redis projection
from PostgreSQL rather than treating the cache as authoritative.

Run the lifecycle migration before the Voice application rollout and require
`/ready` to pass its bounded schema check. For recovery, restore `voice_db` into an isolated database and validate the migration version, table set, invariants,
and evidence before cutover. The restore workflow must never write to the live source database. The guarded lifecycle migration DOWN refuses to remove schema
while evidence rows remain.

## Phase-0 S2S key rotation and verifier incidents (target)

Каждый issuer получает secret-mounted каталог
`<SERVICE>_PRINCIPAL_SIGNING_KEYS_DIR` с ровно двумя unencrypted PKCS#8 RSA
private keys `<kid>.pem` и выбирает active через
`<SERVICE>_PRINCIPAL_ACTIVE_KID`; aliases `S2S_SIGNING_KEY_PEM` и
`S2S_SIGNING_KID` запрещены. Для rotation подготовить peer `next` key,
опубликовать его public JWK до переключения active kid, затем хранить прежний
public key минимум 30 секунд после прекращения signing. Private key никогда не
попадает в ConfigMap, logs или JWKS.

Consumer refreshes complete JWKS every 30 seconds and may serve only a complete
last-good set for two minutes. A bad refresh does not overwrite that set. At hard
expiry, unknown `kid`, JWKS failure or TLS failure protected RPCs return deny;
they must not fall back to plaintext or legacy identity headers. Alert on a hard
cache expiry, denied unknown-kid refresh bursts, signature/temporal failures and
session-epoch lookup failures, tagging issuer, audience, RPC and request ID but
never a bearer or private key.

The TLS requirement applies in staging and production. Any plaintext gRPC profile
must be named and isolated as local development/test; its deployment manifest
cannot be promoted to staging or production.

---

## Bot Service (env)

| Переменная | Назначение |
|------------|------------|
| `BOT_DEFERRED_TTL` | TTL deferred interaction до пометки `abandoned` в `bot_event_log`; default `24h` |
| `BOT_DEFERRED_SWEEP_INTERVAL` | Интервал фонового sweeper (`RunDeferredTTLSweeper`); default `15m` |
| `BOT_RATE_LIMIT_DISABLED` | Опционально: `true` отключает rate limits в Bot Service (dev/tests) |

Подробнее deferred flow — [bot-service.md](microservices/bot-service.md).

---

## Связанные документы

- [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md) — rate limiting, протоколы, доставка сообщений; **пороги смены движка поиска и эволюции аналитики** (матрицы триггеров)
- [CONTRACT_MATRIX.md](CONTRACT_MATRIX.md) — маршруты Gateway → сервисы и таблица JetStream
- [MICROSERVICES.md](MICROSERVICES.md) — отказоустойчивость, масштабирование, перечень сервисов
- [DEPLOYMENT.md](DEPLOYMENT.md) — стенды, поток артефактов, первый выкат
- [PRODUCTION_HOME_SERVER.md](PRODUCTION_HOME_SERVER.md) — фактическая схема домашнего production, edge, backup и recovery
- [TESTING.md](TESTING.md) — тесты в CI перед выкатом
- [CONTRIBUTING.md](CONTRIBUTING.md) — merge в `master`, review

## P3 Space lifecycle operations and activation (accepted target)

Space retries every incomplete participant with exact durable request bytes, a
fresh 10-second deadline and exponential one-second-to-five-minute backoff with
bounded jitter and no attempt cap. Fifteen minutes without progress raises a
stuck-participant alert; contract mismatch pages immediately. Operators monitor
state/subphase age, participant/retry/receipt validation, Auth unacknowledged
receipts, manifest imports/seals, File zero-reference GC and physical R2 retry.

Lifecycle outbox rows start `BLOCKED` in the domain transaction and become
`READY` only after the corresponding all-participant barrier/local terminal
commit. Dispatchers claim at most 100 ordered rows with `FOR UPDATE SKIP LOCKED`,
store a random 30-second lease token, publish deterministic bytes outside the
transaction with `Nats-Msg-Id=event_id`, and mark `DELIVERED` only on the correct
JetStream PubAck and matching lease token. Failure retries from one second to
five minutes. Consumers use durable pull, explicit ACK, 30-second AckWait,
unlimited MaxDeliver within the seven-day stream MaxAge and transactional inbox
dedup; ten consecutive failures alert without discard.

Cleanup workers use PostgreSQL time, bounded batches and both age and terminal/
ack predicates. They expire Space tombstones at equality after 365 days, full
lifecycle/outbox/inbox evidence at its 30-day boundary and acknowledged Auth
receipts at the later required boundary. Permanent Role/participant/provider
fences never enter ordinary TTL cleanup. HMAC families rotate each `P90D` and
old versions wait for dependent rows plus the `P30D` restorable-backup window;
rotation/destruction is audited and missing keys fail closed.

P3 activation requires protected RPC capability on every replica, deterministic
owner backfills and dual-write catch-up, sealed File manifests from every owner,
a `LIVE` File fence for every Space-scoped reference, zero URL/metadata/bulk
file_id-only callers, generation-aware authority consumers, and drain/advance
past legacy lifecycle events. A missing method, row, participant or receipt is
never empty success. Physical R2 completion is monitored independently after
durable File GC handoff. Rollback migrations refuse while permanent/unexpired
evidence exists.
