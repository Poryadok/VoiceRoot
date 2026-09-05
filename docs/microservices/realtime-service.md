# Realtime Service

## Обзор

WebSocket-шлюз для доставки событий в реальном времени. Не хранит бизнес-данные — только управляет соединениями и fan-out.

**Язык**: Go
**Хранилище**: Redis (Pub/Sub, connection registry)

## Ответственность

- WebSocket endpoint (`/ws`) — долгоживущие соединения с клиентами
- Подписка клиента на каналы (чаты, пространства, presence)
- Fan-out событий от сервисов к подписанным клиентам
- Redis Pub/Sub для синхронизации между инстансами
- Typing indicators
- Reconnection support (exponential backoff на клиенте)
- Нумерация событий **`s`** в рамках WebSocket-сессии, op **`resume`** с `last_s` после reconnect (см. ниже)
- T056-P1 account session-epoch enforcement (staged/WIP): fail-closed upgrade/operation/fan-out checks and account-targeted close
- **Не хранит inbox или историю чатов**; после reconnect клиент делает глобальную REST-сверку inbox через Chat `ListChats`, а сообщения догружает через Messaging API (Gateway → REST/gRPC) только per selected `chat_id`, см. [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) (Reconnect)
- Heartbeat / ping-pong для детекции разрыва
- На client **`delivery_ack`**: ephemeral `message_delivered` fan-out **и** publish JetStream **`message.delivery_ack`** на `message.events` (Messaging consumer → durable cursor) — см. § `delivery_ack` op

## Протокол WebSocket

### Подключение
```
GET /ws
Headers:
  Authorization: Bearer <access_token>
  X-Profile-Id: <active_profile_id>
```

### T056-P1: session epoch (staged/WIP)

Realtime не считается реализовавшим этот контракт до готовности Auth migration /
repository и Gateway strict consumer. После rollout `expand → seed → strict` он
проверяет положительный JWT claim `session_epoch` и Auth-owned Redis
minimum-epoch floor при upgrade, на каждой inbound operation и перед outbound
fan-out. Stale/missing/corrupt claim или floor, а также Redis error, дают
fail-closed; missing floor не преобразуется в epoch `1`.

При увеличении epoch Auth/координатор может послать account-targeted close всем
соединениям аккаунта. Redis Pub/Sub — только ускоритель доставки этого сигнала:
если событие потеряно, повторная проверка floor всё равно не позволяет stale
сокету отправлять или получать события. Это не заменяет `jti`-проверку Gateway
для single-session logout и не вводит глобальный event replay.

### Формат сообщений (JSON)

Сервер → клиент (события с sequence):

```json
{
  "op": "event_type",
  "d": { },
  "s": 12345
}
```

Клиент → сервер, пример **`resume`** (после обрыва; `last_s` — последний полученный `s`, если был):

```json
{
  "op": "resume",
  "d": { "last_s": 12345 }
}
```

Если клиент не присылал `resume` или это первое подключение — достаточно обычного потока после `hello`.

**`resume` semantics:** новое TCP-соединение → новый поток `s` с `hello`; `resume` с `last_s` **не** воспроизводит пропущенные события из прошлой сессии (Realtime не хранит журнал). Клиент сначала делает глобальную REST-сверку inbox через paginated `ListChats` (включая durable metadata), затем по необходимости догружает **сообщения** через Messaging API per selected `chat_id`. Эфемерные `delivery_ack` / `message_delivered` после reconnect не восстанавливаются — только live + durable metadata.

### Read / delivery dual path

| Action | Persist (source of truth) | WS fan-out (Realtime) |
|--------|---------------------------|------------------------|
| Mark read | `Messaging.MarkRead` REST/gRPC → `read_receipts` + `message.read` NATS | NATS → `message_read`; client `mark_read` → same-profile + chat subscribers (**no persist**) |
| Delivery ack | Messaging `last_delivered_message_id` via `message.delivery_ack` NATS (spec) | client `delivery_ack` → `message_delivered` to sender (+ Redis `redis_fanout.go`) |

**Client obligations:**

1. **Read** — call REST `MarkRead` when opening/scrolling chat; optional WS `mark_read` for multi-tab sync.
2. **Delivered** — send WS `delivery_ack` when message rendered on recipient device (DM).
3. **After reconnect** — complete global `ListChats` inbox snapshot for list ticks/unread; WS `message_delivered` / `message_read` from old session are **not replayed** via `resume`.

См. матрицу durable vs ephemeral — [messaging-service.md](messaging-service.md) § `GetChatListMetadata`, [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) § Доставка сообщений.

### In-app notification fan-out

Two producers may emit WS `notification` for the same message; clients **dedupe** by `(type, chat_id, message_id, recipient profile_id)`.

| Path | When | Owner |
|------|------|-------|
| **Fast path** | `message.sent` on NATS | Realtime emits `notification` parallel to `message_create` (type `new_message` today; `message_request` spec — code gap) |
| **Policy path** | After Notification `DecideRouting` | Notification → NATS → Realtime fan-out (presence, mute, quiet hours, `send_silent`; push vs in-app split) |

**Normative rule:** Notification Service owns **routing policy** (which channel, sound, grouping). Realtime fast path is **latency optimization** for subscribed in-app sessions — must not bypass mute/type suppress. When both paths fire, client keeps one row per dedupe key. Payload schema — § **`notification` op payload** below. Push — always Notification Service (FCM/APNs), never Realtime direct.

### Операции (Client → Server)
| op             | Описание                                              |
|----------------|-------------------------------------------------------|
| `heartbeat`    | Keepalive (каждые 30 сек)                             |
| `subscribe`    | Подписка на чат: `d.chat_id` — UUID чата (RFC 4122)                    |
| `unsubscribe`  | Отписка: `d.chat_id` — UUID чата                                      |
| `typing_start` | Начал печатать                                        |
| `typing_stop`  | Перестал печатать                                     |
| `mark_read`    | `d.chat_id`, `d.message_id` — fan-out `message_read` в чат + same-profile tabs; **persist** только через Messaging REST/gRPC `MarkRead` |
| `delivery_ack` | `d.chat_id`, `d.message_id` — fan-out `message_delivered` отправителю (+ Redis cross-instance); **publish** JetStream `message.delivery_ack` on `message.events` for Messaging durable cursor (spec — consumer not yet shipped) |
| `resume`       | После reconnect: `d.last_s` = последний известный `s`. **Server:** принимает op, **не** воспроизводит пропущенные события прошлой сессии (no event journal); новый поток `s` начинается с `hello` |

### Операции (Server → Client)
| op                   | Описание                                                            |
|----------------------|---------------------------------------------------------------------|
| `hello`              | Инициализация после подключения (начало новой сессии нумерации `s`); `d.conn_id` — server-assigned id сессии WebSocket для корреляции логов (опционально для клиента) |
| `heartbeat_ack`      | Подтверждение heartbeat                                             |
| `subscription_sync`  | Снимок подписок DM после `hello` (см. раздел «Подписки»): `d.scope` = `dm`, `d.chat_ids`, `d.source` = `chat`, `d.degraded` при ошибке S2S к Chat |
| `subscribe_ack`      | Подтверждение `subscribe`: `d.chat_id`                              |
| `unsubscribe_ack`    | Подтверждение `unsubscribe`: `d.chat_id`                          |
| `error`              | Ошибка клиентской операции: malformed UUID сохраняет `invalid_subscribe` / `invalid_unsubscribe`; valid lazy `subscribe`, который Chat не разрешил или не смог проверить, возвращает generic `d.code=permission_denied`, `d.message=chat subscription denied`, `d.chat_id` |
| `message_create`     | Новое сообщение                                                     |
| `message_update`     | Сообщение отредактировано                                           |
| `message_delete`     | Сообщение удалено                                                   |
| `message_read`       | Прочитано до `message_id` (`message.read` NATS или client `mark_read`) |
| `message_delivered`  | Доставлено получателю (`delivery_ack` → sender profile fan-out)      |
| `message_pinned`     | Сообщение закреплено (`message.pinned` NATS)                        |
| `message_unpinned`   | Сообщение откреплено (`message.unpinned` NATS)                      |
| `reaction_add`       | Реакция добавлена                                                   |
| `reaction_remove`    | Реакция удалена                                                     |
| `typing`             | Кто-то печатает                                                     |
| `presence_update`    | Смена статуса пользователя                                          |
| `chat_update`        | Изменение чата/группы                                               |
| `member_add`         | Новый участник                                                      |
| `member_remove`      | Участник удалён                                                     |
| `dm_peer_deleted`    | Удалён второй участник уже известного DM; `d.chat_id` only, только для surviving profile; live-ускорение, не durable history/replay |
| `call_incoming`      | Входящий DM-звонок: `room_id`, `chat_id`, `initiator_profile_id`, `callee_profile_id`, `media_kind`, `expires_at` |
| `call_accepted`      | Звонок принят: `room_id`, `chat_id`, `accepted_by_profile_id`, `profile_ids`, `media_kind` |
| `call_declined`      | Звонок отклонён: `room_id`, `chat_id`, `declined_by_profile_id`, `profile_ids` |
| `call_missed`        | Входящий DM-звонок истёк по таймауту: `room_id`, `chat_id`, `initiator_profile_id`, `callee_profile_id` |
| `call_ended`         | Звонок завершён: `room_id`, `profile_ids`, `reason`, `ended_by_profile_id` |
| `voice_state_update` | Изменение voice-состояния                                           |
| `notification`       | In-app уведомление (см. payload ниже)                               |
| `match_found`        | Найден матч (matchmaking)                                           |

### `notification` op payload (in-app)

Canonical identity fields use **`profile_id`** (not `user_id`). Legacy code may emit `user_id` — clients should accept both during migration; new producers **must** use `profile_id`.

```json
{
  "op": "notification",
  "d": {
    "type": "new_message",
    "chat_id": "<uuid>",
    "message_id": "<uuid>",
    "sender_profile_id": "<uuid>",
    "preview": "truncated body or media label",
    "send_silent": false
  },
  "s": 12346
}
```

| Field | Required | Notes |
|-------|----------|-------|
| `type` | ✓ | See table below |
| `chat_id` | ✓ | |
| `message_id` | ✓ | Dedupe key with `type` + recipient |
| `sender_profile_id` | ✓ | Not `user_id` |
| `preview` | optional | Truncated body or media label |
| `send_silent` | optional | Echo from `message.sent`; in-app row still shown; push policy in Notification Service |

| `d.type` | Notes |
|----------|-------|
| `new_message` | Ordinary DM after accept |
| `message_request` | Stranger / requests inbox — title «Незнакомец» / «Message request» (**code gap:** fast path emits `new_message` today) |
| `mention` | Group/channel @mention |
| `reply` | Thread reply — **not yet in code** (interim: `new_message`) |

Routing rules (presence, quiet hours, `send_silent`, mute) — [notification-service.md](notification-service.md), [features/notifications.md](../features/notifications.md). Producers — see § In-app notification fan-out above.

**Code gaps (in-app type):**

| Gap | Location | Spec |
|-----|----------|------|
| All DM in-app fan-out uses `type=new_message` | `in_app_notification_fanout.go` | Stranger / requests inbox must emit **`message_request`** — [notification-service.md](notification-service.md) § `message_request` |
| Legacy `user_id` in mention payloads | mention fan-out paths | Producers **must** use `profile_id` (see payload note above) |

### Reconnect checklist (client)

| After WS reconnect | Required action |
|--------------------|-----------------|
| Inbox state across chats | REST `ListChats` snapshot for `main` / `requests` / `archive`, paginate to completion; failed page retries without clearing cached state |
| Missed messages | REST `GetMessages` per open, notification-target or otherwise selected `chat_id` |
| Deleted peer in selected DM | REST `GetMessages.dm_peer_state`; `dm_peer_deleted` может ускорить локальный marker, но не replay-ится |
| List preview ticks / unread | Included in authoritative `ListChats` snapshot via S2S Messaging metadata |
| Read cursor | REST `MarkRead` if chat was open; do not rely on WS-only `mark_read` |
| Ephemeral delivery | Live `delivery_ack` only; list ✓✓ from durable metadata |
| Live events | New `hello` + optional `resume` (new `s` stream; no event journal replay) |

## Конфигурация (NATS / JetStream)

- **`NATS_URL`** — URL NATS Server с JetStream (порт **4222**). В Compose: `nats://nats:4222`; с хоста: `nats://127.0.0.1:${NATS_PORT:-4222}` (см. [`docker-compose.yml`](../../docker-compose.yml)).
- Подписки на доменные потоки для fan-out в WebSocket — в первую очередь **`message.events`** (consume: `message.sent`, …; **publish:** client `delivery_ack` → `message.delivery_ack`), **`chat.events`** и с Фазы 2 **`voice.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)); детали subject/consumer — в реализации сервиса.
- **`REALTIME_CHAT_GRPC_ADDR`** (опционально) — gRPC адрес **Chat Service** для bootstrap списка DM при открытии WebSocket и проверки lazy `subscribe` через `GetChat` (например `chat:50051` в compose). Если не задан, сервер **не** вызывает Chat и **не** шлёт `subscription_sync`; valid lazy `subscribe` fail-closed с generic `permission_denied`, а не создаёт неподтверждённую подписку. TLS/insecure — как принято в окружении (локально часто plaintext внутри mesh).
- **`REALTIME_USER_GRPC_ADDR`** (опционально) — User Service для записи presence при WS `presence_update`.
- **`REALTIME_SOCIAL_GRPC_ADDR`** (опционально) — Social Service `ListFriends` для fan-out `user.presence_changed` друзьям по WebSocket (без общей chat-подписки).

## Архитектура fan-out

```
NATS (message.sent) ──► Realtime Instance A ──► Client 1
                    ──► Realtime Instance B ──► Client 2, Client 3

Realtime Instance A ──Redis Pub/Sub──► Realtime Instance B
   (typing event)                      (forward to subscribers)
```

Для T056-P1 тот же Pub/Sub может ускорить account-targeted close, но floor в
Redis и проверка JWT остаются correctness path; нельзя считать доставку Pub/Sub
доказательством отзыва сессии.

1. Сервис (Messaging, Voice, etc.) публикует событие в NATS
2. Все инстансы Realtime подписаны на релевантные NATS subjects
3. Каждый инстанс доставляет событие своим подключённым клиентам
4. Typing indicators — через Redis Pub/Sub (не персистентные, не нужен NATS)

## Подписки

**Target state** при подключении клиент автоматически подписывается на:
- Все свои активные чаты (DM, группы)
- Все свои пространства и подписки на узлы дерева (текстовые чаты / голос)
- Presence друзей
- Персональные уведомления

**Shipped today:** DM bootstrap via Chat `ListChats` (см. ниже); groups/spaces/friend presence — lazy `subscribe` / partial; см. [todo/backend.md](../todo/backend.md) § Realtime subscription bootstrap.

### DM ([text-chat.md](../features/text-chat.md)): список из Chat vs lazy `subscribe`

Требование выше («все активные чаты») для **DM** в реализации app stack разбивается так:

| Подход | Описание |
|--------|----------|
| **Bootstrap из Chat (основной)** | После `hello`, если задан `REALTIME_CHAT_GRPC_ADDR`, Realtime вызывает Chat Service **`ListChats`** (постранично), собирает чаты с типом **`CHAT_TYPE_DM`** и регистрирует их в локальном наборе подписок соединения. Клиент получает **`subscription_sync`** с отсортированным `chat_ids`. Источник истины по членству в чатах — **Chat**; так не пропускаются события по DM, в которые пользователь вступил, но UI ещё не открывал. |
| **Lazy `subscribe`** | Клиент шлёт `subscribe` с `chat_id` (например гонка сразу после `CreateDM`, пока список не обновился, или вспомогательный чат вне первой страницы `ListChats` до доработки пагинации на стороне bootstrap). Перед `subscribe_ack` Realtime вызывает Chat `GetChat` c обычными user/profile metadata (не internal caller); unknown, nonmember, deleted-for-self, dependency failure или timeout возвращают только generic `permission_denied`. Подписки суммируются с bootstrap. |
| **Chat не сконфигурирован** | Bootstrap не выполняется; lazy `subscribe` **не** служит fallback для ACL и fail-closed с generic `permission_denied`. Для продакшена DM MVP ожидается заданный адрес Chat. |
| **Ошибка Chat при bootstrap** | Всё равно отправляется `subscription_sync` с `degraded: true` и пустым `chat_ids`; клиенту следует опереться на REST список чатов и при необходимости прислать `subscribe` по известным `chat_id`. |

Группы/каналы и прочие scope — вне этого чанка; по мере готовности Chat/Realtime их bootstrap расширяется по той же схеме (источник списка в Chat, не выдумывать членство в Realtime). `chat.member_changed` c `removed` или `left` отзывает все локальные подписки profile/chat; `joined` не создаёт подписку автоматически.

## Зависимости

- **Redis** — Pub/Sub, registry подключений `{profile_id → [instance_id, ws_conn_id]}` и staged minimum-epoch floor для fail-closed проверок аккаунта
- **NATS** — получение событий от всех сервисов

Ни глобальная сверка inbox, ни догрузка пропущенных **сообщений** не проходят через Realtime: клиент обращается через API Gateway к Chat `ListChats`, затем при необходимости к Messaging Service `GetMessages` (без обязательного gRPC Realtime → Messaging для catch-up).

## Метрики (→ Analytics)

- `realtime.connections.active` — текущие WebSocket соединения
- `realtime.events.delivered` — доставленных событий/сек
- `realtime.events.fanout_latency` — задержка fan-out (p50/p95)
- `realtime.reconnects` — количество reconnect

## Масштабирование

- **Балансировка**: клиент подключается по WSS через **L7 load balancer** (или L4 с TLS на LB). Запрос уходит на **любой** инстанс Realtime; **sticky sessions не нужны** — после reconnect клиент может оказаться на другом инстансе.
- **Несколько инстансов**: каждый подписан на NATS; между инстансами **Redis Pub/Sub** и общий **registry** подключений (см. выше), чтобы fan-out доходил до клиента независимо от того, на каком инстансе открыт сокет.
- **Падение инстанса**: соединения на нём обрываются; клиент переподключается с exponential backoff ([ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md)). Глобальное состояние списка сверяется через Chat `ListChats`, а выбранные пропущенные **сообщения** догружаются через Messaging и API Gateway, а не через «догон» в Realtime.
- **Эфемерные события** (typing, часть presence): гарантии catch-up как у сообщений **нет** — после reconnect состояние восстанавливается из следующих live-событий или снимка из других API.
