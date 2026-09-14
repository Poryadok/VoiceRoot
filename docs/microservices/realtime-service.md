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
- T056-P1 account session-epoch enforcement: fail-closed upgrade/operation/fan-out checks
- **Не хранит inbox или историю чатов**; после reconnect клиент делает глобальную REST-сверку inbox через Chat `ListChats`, а сообщения догружает через Messaging API (Gateway → REST/gRPC) только per selected `chat_id`, см. [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) (Reconnect)
- Heartbeat / ping-pong для детекции разрыва
- На client **`delivery_ack`**: ephemeral `message_delivered` fan-out **и** publish JetStream **`message.delivery_ack`** на `message.events` (Messaging consumer → durable cursor) — см. § `delivery_ack` op
- Social block closure для DM: bootstrap, lazy `subscribe` и каждая client side effect (`typing`, `mark_read`, `delivery_ack`) проверяют account pair fail-closed; `social.user_blocked` ускоряет отзыв уже открытых DM-подписок

## Протокол WebSocket

### Подключение
```
GET /ws
Headers:
  Authorization: Bearer <access_token>
  X-Profile-Id: <active_profile_id>
```

### T056-P1: session epoch

В Compose после Auth migration, startup seed и issuance preparation включены
strict-проверки Realtime. Он проверяет положительный JWT claim `session_epoch` и Auth-owned Redis
minimum-epoch floor при upgrade, на каждой inbound operation и перед outbound
fan-out. Stale/missing/corrupt claim или floor, а также Redis error, дают
fail-closed; missing floor не преобразуется в epoch `1`.

Account-targeted закрытие соединений через Redis Pub/Sub не реализовано.
Проверка floor остаётся enforcement path для stale сокета; это не заменяет
`jti`-проверку Gateway для single-session logout и не вводит глобальный event
replay. Compose strict proof не завершает rollout во всех окружениях; compatibility
допустим только в явно настроенном окружении вне доказанного strict deployment.

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
| **Fast path** | `message.sent` on NATS | Realtime emits `notification` parallel to `message_create`: `message_request` for a recipient in the requests inbox, otherwise `new_message` |
| **Policy path** | After Notification `DecideRouting` | Notification → NATS → Realtime fan-out (presence, mute, quiet hours, `send_silent`; push vs in-app split) |

**Normative rule:** Notification Service owns **routing policy** (which channel, sound, grouping). Realtime fast path is **latency optimization** for subscribed in-app sessions — must not bypass mute/type suppress. When both paths fire, client keeps one row per dedupe key. Payload schema — § **`notification` op payload** below. Push — always Notification Service (FCM/APNs), never Realtime direct.

### Phase-0 Space-room roster fan-out (target; не реализовано)

Voice publishes versioned room lifecycle events only after its authoritative
mutation commits. Before publish it resolves current Space membership and
`SPACE_VIEW_MEMBER_LIST` through server-owned Space/Role dependencies and writes
two explicit recipient sets into the trusted event: `full_profile_ids` and
`occupancy_profile_ids`. Dependency uncertainty drops the event fail-closed and
requires snapshot reconciliation; Realtime never computes or widens either set.
It delivers each already-shaped payload to every active tab of the named profiles.
No client-selected `space_id`, profile ID, subscription target or Redis registry
entry is audience authority.

Full recipients receive participant identity and voice/screen state. Occupancy
recipients receive only `room_id`, `roster_epoch`, `projection=occupancy`,
`projection_version`, `event_id`, `occurred_at` and `occupant_count`; they never
receive a participant ID, display snapshot, mute/deafen/video/speaking state,
screen-share owner/stream, role, or an internal recipient list. A mutation that
does not change occupancy emits no occupancy event and does not advance its
projection version.

`roster_epoch` is a Voice-owned disclosure/snapshot generation, distinct from
LiveKit `media_epoch`, Space access generation and Role policy epoch. Permission,
membership-authority or room-incarnation changes that can move a viewer between
disclosure classes start a new positive `roster_epoch`; the client discards the
old projection and fetches a new signed snapshot. Within one epoch, Voice keeps
independent contiguous counters per `(room_id, projection)`:

- `projection=full` advances for participant join/leave and every disclosed
  voice/screen state change;
- `projection=occupancy` advances only when `occupant_count` changes.

Thus aggregate viewers need no redacted no-op for a full-only state mutation and
cannot see a false gap. Dedupe is by UUID `event_id`; ordering/recovery key is
`(room_id, roster_epoch, projection, projection_version)`. Clients buffer the
matching live projection during snapshot, then apply only contiguous versions.
A duplicate event ID is ignored; a version gap, epoch change, reconnect or
projection-class change fetches the matching signed Voice snapshot. `occurred_at`
is diagnostic only. The ordinary WebSocket `s` remains connection-local and does
not replace roster recovery.

### Voice participant lifecycle fan-out (shipped compatibility contract)

This contract applies only to the existing participant-targeted Voice events. It
does **not** implement or weaken the Phase-0 Space watcher/roster contract above.
Voice is the recipient authority: Realtime uses only the explicit recipient IDs
inside the trusted `VoiceStreamEvent`, delivers to every active WebSocket tab of
each named profile, and never expands the audience from `chat_id`, `room_id`,
`space_id`, a client subscription, or the Redis connection registry. Duplicate
recipient IDs are compacted before fan-out. Each connection still passes the
normal session-epoch write guard.

Every delivered payload includes the outer Voice envelope's UUID `event_id` and
UTC RFC 3339 `occurred_at`. Clients deduplicate Voice lifecycle frames by
`event_id`; `occurred_at` is diagnostic chronology, not an ordering authority.
An envelope without a valid UUID event ID, valid timestamp, or the operation's
authoritative recipient set is discarded fail-closed and ACKed as malformed
after the bounded local attempt; Realtime does not synthesize an audience.

| WS op | Voice-authoritative recipients | Public `d` fields in addition to `event_id`, `occurred_at` |
|---|---|---|
| `call_incoming` | `callee_profile_id` only | `room_id`, `chat_id`, `initiator_profile_id`, `callee_profile_id`, `media_kind`, `livekit_room_name`, `expires_at` |
| `call_accepted` | every unique `profile_ids` entry | `room_id`, `chat_id`, `accepted_by_profile_id`, `profile_ids`, `media_kind`, `livekit_room_name` |
| `call_declined` | every unique `profile_ids` entry | `room_id`, `chat_id`, `declined_by_profile_id`, `profile_ids` |
| `call_missed` | unique non-empty `initiator_profile_id`, `callee_profile_id` | `room_id`, `chat_id`, `initiator_profile_id`, `callee_profile_id` |
| `call_ended` | every unique `profile_ids` entry | `room_id`, `duration_seconds`, `profile_ids`, `reason`, `ended_by_profile_id` |
| `call_started` | every unique `profile_ids` entry | current call/session locator fields; the complete `voice_room_id` / `space_id` pair is included only for `room_type=voice_room` |
| `voice_state_update` | every unique `profile_ids` entry | `room_id`, changed `profile_id`, compatibility `profile_ids`, and only the optional fields present in the source: `is_muted`, `is_deafened`, `is_video_on`, `is_commander`, `hand_raised`, `has_floor`, `is_broadcasting` |
| `screen_share_started`, `screen_share_stopped` | every unique `profile_ids` entry | `room_id`, sharing `profile_id`, `stream_id`; routing `profile_ids` are not copied into `d` |
| `voice_member_joined` | every unique `notify_profile_ids` entry; never the joined profile merely because it joined | `room_id`, `voice_room_id`, `space_id`, `joined_profile_id`, compatibility `profile_ids` snapshot |

The table freezes disclosure for the existing participant audience only. A
future Space watcher event must carry the Phase-0 audience-specific redacted or
full payload plus `(roster_epoch, projection, projection_version, event_id)` from
Voice; Realtime must not reuse the participant list as an inferred Space audience.

JetStream preserves delivery to the consumer, but this compatibility stream has
no durable client replay and no cross-connection total order. WebSocket `s` is
the delivery order of one connection only. Duplicate/redelivered `event_id`
frames may occur across reconnects. After each new `hello`, the client reconciles
`GetActiveCall`; `GetVoiceStates` owns the current room state/screen-share
projection. Space roster recovery continues to use the Phase-0 projection key,
not `s` or `occurred_at`.

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
| `archive_activity`   | Тихий badge update для archived chat: `d.chat_id`; только incoming `message.sent`, без notification-center row, навигации или звука |
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
| `role_update`        | Доставка изменения role policy из `role.events`; `role.chat_override_set` и `role.chat_override_removed` доставляются только текущим подписчикам указанного `d.chat_id`. Payload сохраняет `subject`, `space_id`, `chat_id`, `role_id`. Voice-room override events не имеют WS fan-out, пока не определён authoritative индекс voice-room подписок. |
| `member_add`         | Новый участник                                                      |
| `member_remove`      | Участник удалён                                                     |
| `dm_peer_deleted`    | Удалён второй участник уже известного DM; `d.chat_id` + `d.recipient_profile_id`, только для designated surviving profile, без deleted identity; live-ускорение, не durable history/replay |
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
| `message_request` | Stranger / requests inbox — title «Незнакомец» / «Message request» |
| `mention` | Group/channel @mention |
| `reply` | Thread reply — **not yet in code** (interim: `new_message`) |

Routing rules (presence, quiet hours, `send_silent`, mute) — [notification-service.md](notification-service.md), [features/notifications.md](../features/notifications.md). Producers — see § In-app notification fan-out above.

**Code gaps (in-app type):**

| Gap | Location | Spec |
|-----|----------|------|
| Thread replies use `type=new_message` | `in_app_notification_fanout.go` | Emit canonical **`reply`** after the reply producer contract is implemented — [notification-service.md](notification-service.md) § Types |
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
| Active voice / screen share | After every newly accepted `hello`, request Voice `GetActiveCall`; `null` closes stale LiveKit binding and clears the screen projection, while active response replaces the local session. `GetVoiceStates` owns the current sharer list and current LiveKit tracks gate renderability. REST error retains state and retries; stale hello/profile result is ignored. |

### Fan-out pressure

Each connection has a bounded queue and fan-out never waits on a slow recipient.
Ordinary ephemeral operations, including `voice_state_update`, are lossy when
the queue is full. For profile lifecycle `call_incoming`, `call_accepted`,
`call_declined`, `call_missed`, `call_ended`, `call_started`,
`screen_share_started`, and `screen_share_stopped`, overflow closes **only** the
full connection with WebSocket code `1013` and reason `fanout_overflow`; healthy
recipients continue receiving the same fan-out in their own order. There is no
retry, eviction, or reorder. The JetStream consumer completes all local enqueue
attempts and ACKs the source message afterward, including a malformed or
unsupported payload that it intentionally does not fan out.

### Операции (Client → Server)

## Конфигурация (NATS / JetStream)

- **`NATS_URL`** — URL NATS Server с JetStream (порт **4222**). В Compose: `nats://nats:4222`; с хоста: `nats://127.0.0.1:${NATS_PORT:-4222}` (см. [`docker-compose.yml`](../../docker-compose.yml)).
- Подписки на доменные потоки для fan-out и отзыва доступа — в первую очередь **`message.events`** (consume: `message.sent`, …; **publish:** client `delivery_ack` → `message.delivery_ack`), **`chat.events`**, **`social.user_blocked`** из `social.events`, **`role.events`** для role policy и с Фазы 2 **`voice.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)); детали subject/consumer — в реализации сервиса.
- **`REALTIME_CHAT_GRPC_ADDR`** (опционально) — gRPC адрес **Chat Service** для bootstrap списка DM при открытии WebSocket и проверки lazy `subscribe` через `GetChat` (например `chat:50051` в compose). Если не задан, сервер **не** вызывает Chat и **не** шлёт `subscription_sync`; valid lazy `subscribe` fail-closed с generic `permission_denied`, а не создаёт неподтверждённую подписку. TLS/insecure — как принято в окружении (локально часто plaintext внутри mesh).
- **`REALTIME_USER_GRPC_ADDR`** (опционально) — gRPC-адрес User Service для записи presence при WS `presence_update`, разрешения `dm_peer_profile_id → account_id` перед DM block decision и viewer-aware `GetPresence` перед fan-out приватного presence. Realtime передаёт в `GetPresence` identity и account type конкретного получателя, а правила аудитории применяет User; локально Realtime их не воспроизводит. Если адрес не задан, viewer-aware presence fan-out не выполняется; ошибка или пустой ответ User подавляет только затронутое эфемерное обновление этого получателя (fail-closed).
- **`REALTIME_SOCIAL_GRPC_ADDR`** (опционально) — Social Service `ListFriends` для fan-out `user.presence_changed` и `IsBlocked` в обе стороны для DM subscription policy. Если User/Social policy dependency отсутствует или ошибается, DM bootstrap/lazy subscribe fail-closed.

## Архитектура fan-out

```
NATS (message.sent) ──► Realtime Instance A ──► Client 1
                    ──► Realtime Instance B ──► Client 2, Client 3

Realtime Instance A ──Redis Pub/Sub──► Realtime Instance B
   (typing event)                      (forward to subscribers)
```

Для T056-P1 account-targeted close через Redis Pub/Sub не реализован. Floor в
Redis и проверка JWT остаются correctness path.

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
| **Bootstrap из Chat (основной)** | После `hello`, если задан `REALTIME_CHAT_GRPC_ADDR`, Realtime вызывает Chat Service **`ListChats`** (постранично), затем повторно авторизует каждый chat через `GetChat`. Для **DM** он получает peer через `ListMembers`, разрешает peer account через User `GetProfile` и вызывает Social `IsBlocked` в обе стороны. Block даёт чистый deny; ошибка Chat/User/Social исключает chat и выставляет `degraded=true`. Клиент получает **`subscription_sync`** только с разрешёнными отсортированными `chat_ids`. |
| **Lazy `subscribe`** | Клиент шлёт `subscribe` с `chat_id`. Перед `subscribe_ack` Realtime применяет тот же Chat membership и DM Social account-pair policy. Block, unknown, nonmember, deleted-for-self, dependency failure или timeout возвращают только generic `permission_denied`; внутренние причины не раскрываются. Non-DM сохраняет Chat membership semantics и не применяет DM block pair как взаимный запрет общего канала. |
| **Chat не сконфигурирован** | Bootstrap не выполняется; lazy `subscribe` **не** служит fallback для ACL и fail-closed с generic `permission_denied`. Для продакшена DM MVP ожидается заданный адрес Chat. |
| **Ошибка Chat при bootstrap** | Всё равно отправляется `subscription_sync` с `degraded: true` и пустым `chat_ids`; клиенту следует опереться на REST список чатов и при необходимости прислать `subscribe` по известным `chat_id`. |

`chat.member_changed` c `removed` или `left` отзывает все локальные подписки profile/chat; `joined` не создаёт подписку автоматически. Для DM Realtime держит bounded локальный индекс `account pair → local chat IDs` только пока существует подписка или выполняется authorization check; последняя `unsubscribe`/disconnect/revoke удаляет chat и пустую pair. Каждый instance имеет собственный durable consumer `social.user_blocked`: событие по этому индексу удаляет DM из локальных tabs обоих accounts, не сканируя глобальную историю пар. In-flight generation barrier не позволяет гонке `check → event → add` восстановить подписку.

Событие остаётся revoke-оптимизацией, а не correctness path: перед `typing`, `mark_read` и `delivery_ack` для уже открытого DM Realtime синхронно вызывает Social `IsBlocked` в обе стороны по сохранённой pair. Block или ошибка Social дают существующий generic deny до fan-out/Redis/JetStream side effects. Поэтому успешный `BlockAccount` закрывает старый socket даже если `PublishUserBlocked` завершился ошибкой или event не был доставлен; non-DM сохраняет local-subscription semantics без Social round-trip.

## Зависимости

- **Redis** — Pub/Sub, registry подключений `{profile_id → [instance_id, ws_conn_id]}` и minimum-epoch floor для fail-closed проверок аккаунта
- **NATS** — получение событий от всех сервисов
- **Chat / User / Social gRPC** — membership/type, DM peer account resolution и двунаправленный account-level block decision

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
