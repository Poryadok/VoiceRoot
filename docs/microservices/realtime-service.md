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
- Catch-up: при reconnect клиент отправляет `last_event_id`, сервис запрашивает пропущенные события
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
```json
{
  "op": "event_type",
  "d": { /* payload */ },
  "s": 12345  // sequence number
}
```

### Операции (Client → Server)
| op              | Описание                              |
|-----------------|---------------------------------------|
| `heartbeat`     | Keepalive (каждые 30 сек)            |
| `subscribe`     | Подписка на канал/чат/space           |
| `unsubscribe`   | Отписка                              |
| `typing_start`  | Начал печатать                       |
| `typing_stop`   | Перестал печатать                     |
| `resume`        | Reconnect с last_sequence            |

### Операции (Server → Client)
| op                    | Описание                          |
|-----------------------|-----------------------------------|
| `hello`               | Инициализация после подключения   |
| `heartbeat_ack`       | Подтверждение heartbeat           |
| `message_create`      | Новое сообщение                   |
| `message_update`      | Сообщение отредактировано         |
| `message_delete`      | Сообщение удалено                 |
| `reaction_add`        | Реакция добавлена                 |
| `reaction_remove`     | Реакция удалена                   |
| `typing`              | Кто-то печатает                   |
| `presence_update`     | Смена статуса пользователя        |
| `chat_update`         | Изменение чата/группы             |
| `member_add`          | Новый участник                    |
| `member_remove`       | Участник удалён                   |
| `voice_state_update`  | Изменение voice-состояния         |
| `notification`        | In-app уведомление                |
| `match_found`         | Найден матч (matchmaking)         |

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
- Все свои пространства и их каналы
- Presence друзей
- Персональные уведомления

## Зависимости

- **Redis** — Pub/Sub, registry подключений `{profile_id → [instance_id, ws_conn_id]}`
- **NATS** — получение событий от всех сервисов
- **Messaging Service** — catch-up при reconnect (gRPC)

## Метрики (→ Analytics)

- `realtime.connections.active` — текущие WebSocket соединения
- `realtime.events.delivered` — доставленных событий/сек
- `realtime.events.fanout_latency` — задержка fan-out (p50/p95)
- `realtime.reconnects` — количество reconnect

## Масштабирование

N инстансов за Load Balancer. Sticky sessions не нужны — reconnect создаёт новое соединение на любом инстансе. Redis Pub/Sub обеспечивает согласованность между инстансами.
