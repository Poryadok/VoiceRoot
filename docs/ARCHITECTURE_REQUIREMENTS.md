# Технические требования к архитектуре

Этот файл — единая точка для кросс-сервисных технических решений. Фичи описаны в [features/](features/).

**Операционные цели** (SLO по пользовательским путям, порядок деградации, canary/rollback, миграции БД per service): [OPERATIONS.md](OPERATIONS.md).

---

## Аутентификация и сессии

- **Access token**: JWT, TTL 15 мин
- **Refresh token**: opaque, TTL 30 дней, хранится в DB
- **Учётная запись в БД**: таблица **`accounts`** в PostgreSQL **`auth_db`** (не `users` в смысле «вся личность»). Профили и ники — **User Service** / `user_db`, см. [DATA_MODEL.md](DATA_MODEL.md).
- **Таблица refresh-токенов** (имена колонок выровнены с [microservices/auth-service.md](microservices/auth-service.md)):  
  `refresh_tokens(id, account_id, token_hash, device_info, expires_at, created_at, revoked_at)` — логически **`account_id`** = `accounts.id`; в JWT claim по-прежнему **`user_id`** (историческое имя, то же значение, что у аккаунта).
- **Инвалидация refresh tokens**: смена пароля → удалить все; logout → удалить один
- **Досрочный отзыв одного access token**: Redis blacklist (ключ = jti, TTL = оставшееся время токена); `jti` остаётся per-session механизмом и не заменяется epoch.
- **Одновременных сессий**: неограниченно → страница "Активные устройства"
- **Восстановление пароля**: через email (ссылка или код)
- **Удаление аккаунта**: soft delete, поле **`deleted_at`** на таблице **`accounts`** в `auth_db` (антискам + 152-ФЗ); детали модели — [microservices/auth-service.md](microservices/auth-service.md)

### T056-P1: epoch для отзыва всех сессий

Auth DB остаётся источником истины: `accounts.session_epoch BIGINT NOT NULL
DEFAULT 1`, положительный и монотонный; операция отзыва всех сессий увеличивает
его атомарно и никогда не уменьшает. Новый access JWT обязан содержать
положительный integer claim `session_epoch`.

Auth зеркалирует в Redis minimum-epoch floor аккаунта без TTL. Gateway и Realtime
принимают токен только когда `token.session_epoch >= floor`; в strict-режиме
отсутствующий/невалидный claim, отсутствующий floor, ошибка Redis или corrupt
floor отклоняются (floor не подставляется как `1`). Redis floor может быть выше
Auth DB после сбоя/rollback: это безопасный over-revoke, который подлежит
reconcile без снижения floor.

Ключ floor имеет фиксированный Auth-owned формат
`auth:session:min_epoch:<account_id>`, содержит положительный `int64` и не имеет
TTL. Auth остаётся единственным writer; Gateway читает этот ключ через общий
Gateway Redis и тот же пароль, не меняя prefix. После успешной валидации JWT
Gateway сначала проверяет non-empty `jti` blacklist, затем floor; запрос Redis
ограничен 2 секундами. В strict missing/corrupt floor либо ошибка/timeout Redis
на Gateway boundary дают `auth_unavailable`, а не fallback к epoch `1`.

В Compose включены strict-потребители Gateway и Realtime после Auth migration,
startup seed и issuance preparation. Это локальный integration gate, а не
завершение rollout во всех окружениях. Compatibility допустим только в явно
настроенном окружении вне доказанного strict deployment. Немедленное
account-targeted закрытие через Redis Pub/Sub не реализовано; обязательными
остаются strict JWT/floor проверки на upgrade, inbound operations и outbound
fan-out.

---

## Rate Limiting

Реализация: **Redis + sliding window**.

| Точка                 | Лимит                                       | Блок                         |
|-----------------------|---------------------------------------------|------------------------------|
| Авторизация           | 5 попыток / 15 мин с одного IP              | Блок 15 мин, экспоненциально |
| OTP                   | 3 попытки / 10 мин; новый код не чаще 1/мин | —                            |
| Сообщения (глобально) | 5 сообщений / 5 сек на пользователя         | —                            |
| Slow mode текстовых чатов в спейсе | 5 сек – 6 ч, настраивается админом спейса | —                  |
| Загрузка файлов       | 10 загрузок / час на пользователя           | —                            |
| Создание спейсов      | 5 спейсов / день на аккаунт                 | —                            |
| Создание текстовых чатов в спейсе | 20 чатов (`group` \| `channel`) / день на аккаунт | —            |
| Регистрация ботов     | 5 ботов / день на аккаунт                   | —                            |
| Bot API (slash / webhook через Gateway) | 5000 запросов / 1 мин на ключ лимита (см. [microservices/api-gateway.md](microservices/api-gateway.md)) | —                            |

### Redis: API Gateway и Auth Service

Один Redis (или кластер) на окружение; **разделение префиксами ключей**, отдельной PostgreSQL у Gateway нет.

| Компонент        | Redis: что делает                                                                                                                                                                                                                                                                                                                                                                                              |
|------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **API Gateway**  | Счётчики **сквозных** лимитов из таблицы выше (например `ratelimit:{user_id}:{endpoint_group}`, где **`user_id` в ключе** = subject JWT, то есть **`accounts.id`** / логическое `account_id`; см. [DATA_MODEL.md](DATA_MODEL.md); для лимитов по IP — ключи с IP в составе). **Чтение** blacklist access token: ключ = `jti`, TTL = оставшееся время жизни токена; чтение minimum-epoch floor — strict fail-closed. |
| **Auth Service** | **Запись** в blacklist при logout и сценариях отзыва одного access token; публикация/поддержание minimum-epoch floor; состояние OTP / throttling верификации. HTTP-лимиты из таблицы (в т.ч. вход с одного IP) обрабатывает **Gateway** — дублирующие счётчики в Auth для тех же лимитов не заводим. |

### Версии клиента

`GET /api/v1/version`, флаг **`force_update`** и **`426 Upgrade Required`** на все прочие маршруты при обязательном обновлении — зона **API Gateway**; продуктовый флоу и кэш ответа — [features/updates.md](features/updates.md), маршрут — [microservices/api-gateway.md](microservices/api-gateway.md).

---

## Доставка сообщений

- **Протокол**: WebSocket, persistent connection (edge-вход через API Gateway `/ws`, прокси на Realtime Service; см. [microservices/api-gateway.md](microservices/api-gateway.md) и [microservices/realtime-service.md](microservices/realtime-service.md))
- **Reconnection**: exponential backoff (1s → 2s → 4s, cap 30s)
- **Между инстансами Realtime**: Redis Pub/Sub
- **Прочитанное (dual path)**: (1) **persist** — `Messaging.MarkRead` (REST/gRPC) пишет `read_receipts`, публикует `message.read`; (2) **fan-out** — Realtime WS `mark_read` (client) и NATS `message.read` → op `message_read` на подписчиков чата и другие устройства профиля. Список чатов и тики доставки — durable metadata из Messaging, не из WS alone.
- **Доставлено (dual path)**: WS `delivery_ack` (client) → ephemeral `message_delivered` отправителю (+ Redis cross-instance в `redis_fanout.go`) **и** JetStream `message.delivery_ack` (publisher: Realtime) → Messaging consumer → `read_receipts.last_delivered_message_id`; durable `last_message_delivery_state` для list preview — Messaging ([messaging-service.md](microservices/messaging-service.md) § Durable delivery derivation).
- **Shipped scope (durable delivery):** ephemeral WS path and the durable DM **Messaging-internal** metadata path are **shipped**: `last_delivered_message_id`, JetStream `message.delivery_ack` consumer, and `GetChatListMetadata` persistence/derivation of `last_message_delivery_state`. Exposure through the Chat list and client delivery ticks remains pending.
- **Typing indicator**: WebSocket, throttle отправки — не чаще 1 раза в 3 сек; гасить через 5 сек без обновления
- **UX при потере соединения**: баннер "Переподключение..." появляется через 2 сек после разрыва; исчезает через 1 сек после успешного reconnect
- **Аутентификация WS (web)**: браузерный WebSocket API не позволяет задать заголовок `Authorization`. **Web-клиент** запрашивает short-lived ticket через `POST /api/v1/realtime/ws-ticket` (JWT только в заголовке REST) и подключается к `/ws?ticket=…`. Gateway валидирует ticket (Redis, single-use, TTL ~60s), подставляет claims и upstream JWT для Realtime. **Нативные клиенты** используют `Authorization: Bearer` на upgrade без query. Legacy `access_token` query на `/ws` остаётся для совместимости, но web не использует.

### Reconnect: WebSocket-поток и история сообщений

Два независимых слоя — не смешивать курсоры.

**1. Поток событий (Realtime, WebSocket)**  
Каждое событие в шлюзе несёт монотонный номер **`s`** (sequence) в рамках соединения. После reconnect клиент отправляет **`resume`** с последним известным `last_s` (см. протокол в [microservices/realtime-service.md](microservices/realtime-service.md)). Нумерация привязана к сессии; при новом TCP/WebSocket-подключении сервер выдаёт новый поток `s` с `hello` — клиент не полагается на «глобальный» журнал событий в Realtime.

**2. Глобальная сверка inbox (Chat, REST через Gateway)**
После reconnect клиент получает авторитетный paginated snapshot своего inbox через `ListChats`: `main`, `requests` и `archive`; `Chat` обогащает строки durable metadata из Messaging. Первую страницу можно показать сразу, остальные страницы догружаются в фоне до конца snapshot. Ошибка страницы оставляет локальный cache и требует retry: отсутствие ответа нельзя трактовать как пустой inbox, удаление строки или `unread_count = 0`. Это **глобальный catch-up состояния списка**, а не журнал WebSocket-событий и не выгрузка истории всех чатов.

**3. История сообщений (Messaging, REST через Gateway)**
Пропущенные **сообщения** догружаются **с клиента** через публичный API Messaging (`GetMessages` с курсором **per `chat_id`**: `last_message_id` / `after_message_id`) только для открытого чата, явного перехода из notification или другого выбранного пользователем контекста. Offline queue на сервере не нужна. Если курсор не найден (сообщение удалено) — запрос последних 50 сообщений чата без курсора. Для уже выбранного known DM `GetMessagesResponse.dm_peer_state=DELETED` — durable state удаления второго участника: клиент добавляет единственный локальный неперсистентный marker «Пользователь удалён», не синтезируя message/tombstone и не раскрывая deleted identity. Детали контракта — [microservices/messaging-service.md](microservices/messaging-service.md).

**Эфемерные события** (typing, часть presence, `delivery_ack` / `message_delivered`): catch-up **не гарантируется** — после reconnect состояние «с нуля» или из следующих live-событий.

**4. Durable read / delivery (Messaging, REST — не WebSocket)**
Тики ✓/✓✎ в списке чатов и read cursor — **только** из Messaging (`MarkRead` REST/gRPC → `read_receipts`; `GetChatListMetadata` → `last_message_delivery_state`). После reconnect клиент **обязан** перезапросить `ListChats` / metadata — WS `mark_read` и `message_read` **не** заменяют REST persist. См. [messaging-service.md](microservices/messaging-service.md) § MarkRead, § Durable delivery derivation.

**Shipped scope:** `MarkRead` / durable read cursor validation — **DM today**; group/channel read parity — backlog. DM `last_message_delivery_state` persistence/derivation is shipped inside Messaging; Chat-list and client tick exposure remain pending.

### WS vs REST — граница ответственности

| Concern | REST / gRPC (durable) | WebSocket (ephemeral fan-out) |
|---------|----------------------|-------------------------------|
| История сообщений | `GetMessages` per `chat_id`, включая `dm_peer_state` для selected DM | `message_create` / `message_update` / `message_delete`; `dm_peer_deleted` только live-ускорение, без replay |
| Read cursor | `Messaging.MarkRead` → `read_receipts` | `mark_read` op + `message_read` (same-profile tabs) |
| Delivery ticks (list) | `GetChatListMetadata.last_message_delivery_state` | `delivery_ack` → `message_delivered` (live bubble only) |
| Catch-up после reconnect | `ListChats` global inbox snapshot + `GetMessages` per selected `chat_id` + metadata | `resume` + `last_s` — только live-поток новой сессии, не журнал |
| In-app `notification` | Notification routing policy (presence, mute, quiet hours, `send_silent`) | WS `notification` op — fan-out only; **не** источник durable read/unread |

**Client rule (read sync):** открытие чата / scroll → **REST** `MarkRead` (durable); WS `mark_read` — опционально для faster same-profile multi-tab sync. Badge на других устройствах — после REST + metadata refresh, не от WS alone.

## Конкурентные операции

- **Стратегия**: last write wins по `updated_at` timestamp
- Применяется к: редактированию сообщений, изменению ролей, модераторским действиям (бан/разбан)
- Дедупликация: idempotency key на клиенте для повторных запросов при ненадёжной сети

## gRPC: ошибки между сервисами и клиентом

- **Не-OK ответы** задаются через стандартный **gRPC status**: код (`INVALID_ARGUMENT`, `NOT_FOUND`, `PERMISSION_DENIED`, `RESOURCE_EXHAUSTED`, `UNAVAILABLE`, …) и текст `message`; контракты успешных тел в `protos/voice/**` отдельно ошибки не моделируют — это принято для gRPC ([rich error model](https://grpc.io/docs/guides/error/)).
- **Уточнение причин** для отладки, корреляции или i18n — опционально через **`google.rpc.Status`** и вложения (`ErrorInfo`, `LocalizedMessage`, …): см. пакет [`google.rpc`](https://github.com/googleapis/googleapis/tree/master/google/rpc). Отдельный общий `.proto` в монорепо для ошибок не обязателен, пока коды достаточны для клиента и наблюдаемости.
- **REST через API Gateway**: маппинг gRPC→HTTP статусов и тела ошибки — ответственность Gateway; источник истины для маршрутов и префиксов — [microservices/api-gateway.md](microservices/api-gateway.md).

## Email

- **Провайдер**: Resend (до 3000 писем/мес бесплатно; $20/мес за 50k)
- **Абстракция**: `EmailSender { Send(to, template, params) }` — провайдер сменяем без изменения логики
- **Использование**: регистрация (верификация email), сброс пароля — **auth-only**; product event notifications **не** через email ([notification-service.md](microservices/notification-service.md))

## Оффлайн-режим (мобильный клиент)

- **v1**: только чтение кэша — последние 50 сообщений на канал (SQLite/Hive на Flutter)
- Отправка сообщений при отсутствии сети — блокируется с UI-сообщением
- **v2 (будущее)**: локальная очередь с отправкой при восстановлении соединения

---

## Голос и видео

- **SFU**: LiveKit, self-hosted (open source)
- **TURN/STUN**: встроенный в LiveKit Server; свои TURN-серверы на старте — избыточно
- **Кодеки**:
  - Голос: Opus 48kHz, ~32kbps, авто-адаптация по сети
  - Видео (камера / шара): VP8/VP9, simulcast для адаптации к пропускной способности

---

## Push-уведомления

| Платформа                 | Канал                                                                 |
|---------------------------|-----------------------------------------------------------------------|
| Android                   | FCM                                                                   |
| Web                       | FCM (через Service Workers)                                           |
| iOS                       | APNs (обычные уведомления) + APNs VoIP (PushKit) для входящих звонков |
| Desktop (Win/macOS/Linux) | WebSocket (приложение всегда онлайн)                                  |

- Собственный push-сервер не нужен
- **Группировка**: 1 push на чат с превью последнего сообщения и счётчиком; обновлять существующий push, не плодить новые
- **iOS звонки**: PushKit + CallKit обязательны; VoIP push требует немедленного показа CallKit UI, иначе система завершит процесс; требует отдельного APNs VoIP-сертификата
- **Presence routing**: получатель **online** (User `GetPresence`: `online`, `idle`, `in_call` on active session) → **in-app only** (Realtime WS `notification` op), push **не** шлётся; **offline** → push + in-app; **invisible** → in-app ✓, push ✓ (treat as offline for push). **Exceptions:** `match_found`, `voice_member_joined` — presence check skipped (spec). Producer — Notification Service `DecideRouting` / `EnrichDecision`. Code gaps (ONLINE-only checker, MM/voice still enrich presence) — [notification-service.md](microservices/notification-service.md) § Presence routing.
- **Stranger DM (`message_request`)**: wire type **`message_request`** (not `new_message`) for requests inbox; canonical **`new_message`** for accepted DM / group / channel. Block / `allow_dm` deny suppress at Messaging — Notification never sees event. Realtime in-app fan-out code gap (`new_message` hardcode) — [realtime-service.md](microservices/realtime-service.md) § In-app notification fan-out.
- **Тихие часы (quiet hours)**: в окне DND **push suppressed** (`ApplyQuietHours` → `Push=false`); **in-app still delivered** через WebSocket. Не формулировать как «уведомления не приходят». `@mention` может пробить push block при `override_mentions=true`. Применяется **после** base presence routing.
- **Send without sound (`send_silent`)**: wire name на `SendMessageRequest` и JetStream `message.sent` — **`send_silent`** (bool). Notification consumer: suppress push **sound** и badge increment; in-app unread/badge **как обычно**. **`send_silent` не пробивает quiet hours** — оба правила применяются. Scheduled dispatch: silent применяется в момент фактической отправки worker'ом. **Not yet in proto/code** — [todo/backend.md](todo/backend.md). См. [features/notifications.md](features/notifications.md) § Send without sound, [messaging-service.md](microservices/messaging-service.md) § `SendMessageRequest`.
- **Send options (composer)**: `scheduled_at` (UTC instant) и `send_when_online` (DM only) — Messaging `SendMessageRequest`; lifecycle `scheduled_messages` + `UpdateScheduledMessage` / cancel / send-now — [messaging-service.md](microservices/messaging-service.md) § Scheduled messages; UX — [text-chat.md](features/text-chat.md) § Send options, [screen-controls.md](design/screen-controls.md) §3.6c / #13–17. **Not yet in proto/code** — [todo/backend.md](todo/backend.md).

---

## Полнотекстовый поиск

- **Стратегия**: PostgreSQL `tsvector`/`GIN` (v1) → Meilisearch (v2) → Elasticsearch (v3) **только при явных триггерах ниже** — не переключать движок «потому что красиво».
- **v1 языки**: `simple` конфигурация на старте; `russian` + `english` при росте базы
- **Federated поиск**: master рассылает запрос на все ноды параллельно, агрегирует результаты; нода недоступна — её результаты пропускаются (graceful degradation)
- **Миграция между движками**: запись в два индекса **одновременно — только на время активного cutover** с датой отключения старого пути; бессрочная двойная запись без cutover — антипаттерн.

### Пороговая матрица: смена движка поиска

Пороги — **черновые**, пересматриваются после реальных метрик (см. [OPERATIONS.md](OPERATIONS.md)).

| Переход                                   | Когда планировать миграцию                                                                                                                                                                                                                                                                                                                                                                                                                                  |
|-------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **v1 → v2** (Postgres → Meilisearch)      | **Любой** из: (1) p95 латентности типового **глобального** поиска **> 500 ms** в согласованном окне наблюдения (например 24 ч) при репрезентативной нагрузке; (2) порядок величины **~10M+** проиндексированных сообщений или рост нагрузки на Postgres (CPU/IO на поисковых запросах, реплики) делает v1 экономически/операционно невыгодным; (3) продукт требует возможностей, нереалистичных на v1 без тяжёлых костылей (ранжирование, опечатки и т.д.). |
| **v2 → v3** (Meilisearch → Elasticsearch) | **Только** если зафиксированы требования, которые **Meilisearch не закрывает** (сложные агрегаты по поисковому индексу, специфичные операторы/DSL, корпоративный мандат на ES и т.п.). **По умолчанию ES не внедряем**; срок пересмотра — когда появится такой запрос (до этого v2 считается потолком).                                                                                                                                                     |

Детали сервиса: [microservices/search-service.md](microservices/search-service.md).

---

## Аналитика: пороги эволюции пайплайна

Пайплайн: NATS → Analytics Service → буфер (Redis) → ClickHouse ([microservices/analytics-service.md](microservices/analytics-service.md)). Ниже — **когда усложнять**, а не «вечно наращивать двойную запись без цели».

| Сигнал             | Порядок величины / условие                                                                       | Типичный следующий шаг                                                                                 |
|--------------------|--------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------|
| Объём событий      | устойчиво **~1M+ событий/день**                                                                  | Тюнинг батчей, параллелизма ingest, лимитов Redis-буфера; мониторинг lag                               |
| Объём событий      | **~10M+ событий/день** или буфер регулярно под давлением                                         | Партиционирование/шардирование в ClickHouse, выделение ingest, горизонтальное масштабирование воркеров |
| Задержка пайплайна | p95 времени от события в NATS до **доступности в ClickHouse** **> 60 s** при нормальной нагрузке | Разбор узкого места (буфер, CH insert, диск); масштабирование до укладывания в целевой SLO             |
| Запросы к CH       | Тяжёлые отчёты мешают ingest или дают p95 **> согласованного** порога для admin API              | Реплики для чтения, отдельные витрины/materialized views, ограничение ad-hoc                           |

Цифры (1M / 10M / 60 s) — стартовые ориентиры; уточнять по Prometheus после запуска продакшена.

---

## Файловое хранилище

- **Провайдер**: Cloudflare R2 (S3-совместимый, $0 исходящий трафик)
- **Абстракция**: `FileStorage { Upload, Delete, GeneratePresignedUrl }` — провайдер сменяем без изменения логики
- **Антивирус**: ClamAV при загрузке; на старте — только `.exe` / `.zip` / `.bat`, остальное по mime-type
- **Превью документов**: изображения → thumbnail; PDF → первая страница; остальное (DOCX, ZIP) → иконка + имя + размер; генерация превью — async worker

---

## Законодательство и приватность трафика

**Намеренное решение:** компания не записывает и не хранит медиатрафик пользователей. Запись войс-сессий возможна только **локально на устройстве инициатора** — сервер в процессе не участвует. Текстовые сообщения хранятся на сервере в зашифрованном виде; E2E-чаты сервером не читаются.

В данной модели платформа не является оператором связи в смысле СОРМ, а 152-ФЗ регулирует только хранение пользовательских данных (имя, email, телефон) — не содержимое переписки. Этот вывод **не пересматривается** при ревью без явного юридического заключения.

---

## Протоколы: gRPC / REST / WebSocket

| Протокол      | Где используется                                                           |
|---------------|----------------------------------------------------------------------------|
| **gRPC**      | Сервис ↔ сервис (внутренние вызовы); S2S Federation (нода ↔ master)        |
| **REST**      | Клиент ↔ сервер (CRUD-операции, авторизация, загрузка файлов)              |
| **WebSocket** | Клиент ↔ сервер (real-time fan-out: live сообщения, typing, presence, ephemeral delivery/read sync). **Durable** read/delivery/list ticks — REST/gRPC Messaging, не WS alone |

Клиент **никогда** не обращается к сервисам напрямую по gRPC — только через API Gateway (REST/WS).

---

## Service Discovery и инфраструктура

- **Local dev**: Docker Compose (одна команда для локального подъёма сервисов и зависимостей)
- **Staging**: **k3s** (лёгкий Kubernetes), максимально близко к production
- **Production**: Kubernetes (k8s) или managed k8s (Yandex Managed Kubernetes / Hetzner)
- **Service discovery**: Kubernetes ClusterDNS — сервисы находят друг друга по DNS-именам (`voice-message-service.default.svc.cluster.local`)
- Consul, Eureka — избыточны, не используем

Окружения (local / staging / prod) и поток деплоя: [DEPLOYMENT.md](DEPLOYMENT.md).

---

## Федерация (S2S API)

- **Real-time синхронизация**: gRPC bidirectional stream — нода открывает persistent stream на master, подписывается на события своих спейсов (`RoleChanged`, `UserBanned`, `Defederated`)
- **Fallback**: auth-токен с TTL 5-10 мин; если стрим упал — кэш действует до TTL, затем форс ре-авторизация
- **Маршрутизация уведомлений**: нода → `POST /s2s/notify { user_id, type, space_id, preview }` → master доставляет через FCM/APNs; при недоступности master — in-memory retry ~5 мин, затем дроп
- **Отказоустойчивость**:
  - Нода недоступна: пользователь видит "спейс недоступен", потери данных нет
  - Reconnect ноды: snapshot re-sync — нода запрашивает текущие роли/баны у master (event log не нужен)
  - Master недоступен: нода работает с TTL-кэшем, новых пользователей не пропускает
  - Kafka / очереди сообщений — не нужны в V1
- **Канонический контракт S2S**: [`protos/voice/s2s/v1/s2s.proto`](../protos/voice/s2s/v1/s2s.proto)
- **Документация semantics/ownership**: [microservices/federation-service.md](microservices/federation-service.md)

---

## Канонический клиентский API-контракт

Источник истины для клиентских API находится в двух местах:
- **Маршрутизация и публичные namespace'ы**: [microservices/api-gateway.md](microservices/api-gateway.md)
- **Предметная семантика endpoint'ов**: документы сервисов в [microservices/](microservices/)

Минимальные требования к описанию каждого публичного endpoint в документации:
- HTTP method + route + auth requirement
- request/response schema (обязательные поля и типы)
- error model (status code + `error_code`)
- pagination/курсоры (если применимо)
- idempotency/повтор запроса (если применимо)
