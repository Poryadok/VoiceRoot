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
- **Историю чатов не хранит**; пропущенные сообщения клиент догружает через **Messaging API** (Gateway → REST/gRPC), см. [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) (Reconnect)
- Heartbeat / ping-pong для детекции разрыва

## Протокол WebSocket

### Подключение
```
GET /ws
Headers:
  Authorization: Bearer <access_token>
  X-Profile-Id: <active_profile_id>
```

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

### Операции (Client → Server)
| op             | Описание                                              |
|----------------|-------------------------------------------------------|
| `heartbeat`    | Keepalive (каждые 30 сек)                             |
| `subscribe`    | Подписка на чат: `d.chat_id` — UUID чата (RFC 4122)                    |
| `unsubscribe`  | Отписка: `d.chat_id` — UUID чата                                      |
| `typing_start` | Начал печатать                                        |
| `typing_stop`  | Перестал печатать                                     |
| `resume`       | После reconnect: `d.last_s` = последний известный `s` |

### Операции (Server → Client)
| op                   | Описание                                                            |
|----------------------|---------------------------------------------------------------------|
| `hello`              | Инициализация после подключения (начало новой сессии нумерации `s`); `d.conn_id` — server-assigned id сессии WebSocket для корреляции логов (опционально для клиента) |
| `heartbeat_ack`      | Подтверждение heartbeat                                             |
| `subscription_sync`  | Снимок подписок DM после `hello` (см. раздел «Подписки»): `d.scope` = `dm`, `d.chat_ids`, `d.source` = `chat`, `d.degraded` при ошибке S2S к Chat |
| `subscribe_ack`      | Подтверждение `subscribe`: `d.chat_id`                              |
| `unsubscribe_ack`    | Подтверждение `unsubscribe`: `d.chat_id`                          |
| `error`              | Ошибка разбора клиентской операции, напр. `d.code` = `invalid_subscribe` / `invalid_unsubscribe` |
| `message_create`     | Новое сообщение                                                     |
| `message_update`     | Сообщение отредактировано                                           |
| `message_delete`     | Сообщение удалено                                                   |
| `reaction_add`       | Реакция добавлена                                                   |
| `reaction_remove`    | Реакция удалена                                                     |
| `typing`             | Кто-то печатает                                                     |
| `presence_update`    | Смена статуса пользователя                                          |
| `chat_update`        | Изменение чата/группы                                               |
| `member_add`         | Новый участник                                                      |
| `member_remove`      | Участник удалён                                                     |
| `call_incoming`      | Входящий DM-звонок: `room_id`, `chat_id`, `initiator_profile_id`, `callee_profile_id`, `media_kind`, `expires_at` |
| `call_accepted`      | Звонок принят: `room_id`, `chat_id`, `accepted_by_profile_id`, `profile_ids`, `media_kind` |
| `call_declined`      | Звонок отклонён: `room_id`, `chat_id`, `declined_by_profile_id`, `profile_ids` |
| `call_missed`        | Входящий DM-звонок истёк по таймауту: `room_id`, `chat_id`, `initiator_profile_id`, `callee_profile_id` |
| `call_ended`         | Звонок завершён: `room_id`, `profile_ids`, `reason`, `ended_by_profile_id` |
| `voice_state_update` | Изменение voice-состояния                                           |
| `notification`       | In-app уведомление                                                  |
| `match_found`        | Найден матч (matchmaking)                                           |

## Конфигурация (NATS / JetStream)

- **`NATS_URL`** — URL NATS Server с JetStream (порт **4222**). В Compose: `nats://nats:4222`; с хоста: `nats://127.0.0.1:${NATS_PORT:-4222}` (см. [`docker-compose.yml`](../../docker-compose.yml)).
- Подписки на доменные потоки для fan-out в WebSocket — в первую очередь **`message.events`**, **`chat.events`** и с Фазы 2 **`voice.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)); детали subject/consumer — в реализации сервиса.
- **`REALTIME_CHAT_GRPC_ADDR`** (опционально) — gRPC адрес **Chat Service** для bootstrap списка DM при открытии WebSocket (например `chat:50051` в compose). Если не задан, сервер **не** вызывает Chat и **не** шлёт `subscription_sync`; клиент может подписываться через `subscribe` (lazy). TLS/insecure — как принято в окружении (локально часто plaintext внутри mesh).

## Архитектура fan-out

```
NATS (message.sent) ──► Realtime Instance A ──► Client 1
                    ──► Realtime Instance B ──► Client 2, Client 3

Realtime Instance A ──Redis Pub/Sub──► Realtime Instance B
   (typing event)                      (forward to subscribers)
```

1. Сервис (Messaging, Voice, etc.) публикует событие в NATS
2. Все инстансы Realtime подписаны на релевантные NATS subjects
3. Каждый инстанс доставляет событие своим подключённым клиентам
4. Typing indicators — через Redis Pub/Sub (не персистентные, не нужен NATS)

## Подписки

При подключении клиент автоматически подписывается на:
- Все свои активные чаты (DM, группы)
- Все свои пространства и подписки на узлы дерева (текстовые чаты / голос)
- Presence друзей
- Персональные уведомления

### DM ([text-chat.md](../features/text-chat.md)): список из Chat vs lazy `subscribe`

Требование выше («все активные чаты») для **DM** в реализации app stack разбивается так:

| Подход | Описание |
|--------|----------|
| **Bootstrap из Chat (основной)** | После `hello`, если задан `REALTIME_CHAT_GRPC_ADDR`, Realtime вызывает Chat Service **`ListChats`** (постранично), собирает чаты с типом **`CHAT_TYPE_DM`** и регистрирует их в локальном наборе подписок соединения. Клиент получает **`subscription_sync`** с отсортированным `chat_ids`. Источник истины по членству в чатах — **Chat**; так не пропускаются события по DM, в которые пользователь вступил, но UI ещё не открывал. |
| **Lazy `subscribe`** | Клиент шлёт `subscribe` с `chat_id` (например гонка сразу после `CreateDM`, пока список не обновился, или вспомогательный чат вне первой страницы `ListChats` до доработки пагинации на стороне bootstrap). Подписки суммируются с bootstrap. |
| **Только lazy** | Если Chat gRPC **не** сконфигурирован, bootstrap не выполняется — подписки только через `subscribe` / `unsubscribe`. Это сознательная деградация для dev/частичного деплоя; для продакшена DM MVP ожидается заданный адрес Chat. |
| **Ошибка Chat при bootstrap** | Всё равно отправляется `subscription_sync` с `degraded: true` и пустым `chat_ids`; клиенту следует опереться на REST список чатов и при необходимости прислать `subscribe` по известным `chat_id`. |

Группы/каналы и прочие scope — вне этого чанка; по мере готовности Chat/Realtime их bootstrap расширяется по той же схеме (источник списка в Chat, не выдумывать членство в Realtime).

## Зависимости

- **Redis** — Pub/Sub, registry подключений `{profile_id → [instance_id, ws_conn_id]}`
- **NATS** — получение событий от всех сервисов

Догрузка пропущенных **сообщений** не через Realtime: клиент обращается к **Messaging Service** через API Gateway (без обязательного gRPC Realtime → Messaging для catch-up).

## Метрики (→ Analytics)

- `realtime.connections.active` — текущие WebSocket соединения
- `realtime.events.delivered` — доставленных событий/сек
- `realtime.events.fanout_latency` — задержка fan-out (p50/p95)
- `realtime.reconnects` — количество reconnect

## Масштабирование

- **Балансировка**: клиент подключается по WSS через **L7 load balancer** (или L4 с TLS на LB). Запрос уходит на **любой** инстанс Realtime; **sticky sessions не нужны** — после reconnect клиент может оказаться на другом инстансе.
- **Несколько инстансов**: каждый подписан на NATS; между инстансами **Redis Pub/Sub** и общий **registry** подключений (см. выше), чтобы fan-out доходил до клиента независимо от того, на каком инстансе открыт сокет.
- **Падение инстанса**: соединения на нём обрываются; клиент переподключается с exponential backoff ([ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md)). Пропущенные **сообщения** догружаются через **Messaging** и API Gateway, а не через «догон» в Realtime.
- **Эфемерные события** (typing, часть presence): гарантии catch-up как у сообщений **нет** — после reconnect состояние восстанавливается из следующих live-событий или снимка из других API.


