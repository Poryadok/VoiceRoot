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
| `subscribe`    | Подписка на канал/чат/space                           |
| `unsubscribe`  | Отписка                                               |
| `typing_start` | Начал печатать                                        |
| `typing_stop`  | Перестал печатать                                     |
| `resume`       | После reconnect: `d.last_s` = последний известный `s` |

### Операции (Server → Client)
| op                   | Описание                                                            |
|----------------------|---------------------------------------------------------------------|
| `hello`              | Инициализация после подключения (начало новой сессии нумерации `s`) |
| `heartbeat_ack`      | Подтверждение heartbeat                                             |
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
| `voice_state_update` | Изменение voice-состояния                                           |
| `notification`       | In-app уведомление                                                  |
| `match_found`        | Найден матч (matchmaking)                                           |

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
- Все свои пространства и подписки на узлы дерева (текстовые каналы / голос)
- Presence друзей
- Персональные уведомления

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


