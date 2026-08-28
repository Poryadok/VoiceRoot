# Notifications — уведомления

## Типы уведомлений

| Тип                                  | Wire / enum              | Есть |
|--------------------------------------|--------------------------|------|
| Новое сообщение в DM                 | `new_message`            | ✓    |
| **Запрос сообщения (незнакомец)**    | `message_request`        | ✓    |
| @упоминание в группе / канале        | `mention`                | ✓    |
| Ответ на моё сообщение (reply)       | `reply`                  | ✓    |
| Реакция на моё сообщение             | `reaction`               | ✓    |
| Запрос в друзья                      | `friend_request`         | ✓    |
| Матч найден в ММ                     | `match_found`            | ✓    |
| Системные (безопасность, обновления) | `system`                 | ✓    |
| Кто-то зашёл в голосовую комнату     | `voice_member_joined`    | ✓    |

### `message_request` (stranger DM)

| Аспект | Контракт |
|--------|----------|
| **Trigger** | Первое (или re-contact после decline) сообщение в DM с `inbox_bucket=requests` у получателя |
| **Push title** | «Незнакомец» (RU) / «Message request» (EN) + sender display name |
| **Push body** | Preview первого сообщения (truncated) |
| **In-app** | Badge на virtual folder «Запросы» + in-app notification row |
| **Grouping** | By sender profile (не by chat_id до accept) |
| **After accept** | Subsequent messages → ordinary `new_message` |

См. [text-chat.md](text-chat.md) § «Запросы сообщений», [friends.md](friends.md).

## Каналы доставки

| Платформа                 | Канал                                |
|---------------------------|--------------------------------------|
| Android                   | FCM                                  |
| Web                       | FCM (через Service Workers)          |
| iOS                       | APNs                                 |
| Desktop (Win/macOS/Linux) | WebSocket (приложение всегда онлайн) |

- Email — **только для авторизации** (вход, подтверждение), не для событий
- Собственный push-сервер не нужен
- **Группировка**: 1 push на чат с превью последнего сообщения и счётчиком ("Вася и ещё 4 сообщения"); обновлять существующий push, не плодить новые
- **Синхронизация прочитанного**: событие `mark_read(chat_id, message_id)` через WebSocket рассылается на все подключённые устройства пользователя

## Presence routing (online → in-app only)

Базовое правило (`DecideRouting` в Notification Service):

| Recipient presence | In-app (WebSocket) | Push (FCM/APNs) |
|--------------------|--------------------|-----------------|
| **Online** (`GetPresence` = online/idle/in-call on active session) | ✓ | **✗** |
| **Offline** | ✓ | ✓ |

- Sender **никогда** не получает push/in-app на собственное сообщение
- **Matchmaking / voice join** — presence check **пропускается** (always push when offline policy applies); см. [todo/backend.md](../todo/backend.md)
- Quiet hours и mute применяются **после** base routing (см. ниже)

## Гранулярность настроек

Можно настраивать:
- Глобально
- Отдельно для каждого спейса
- Отдельно для каждого канала / чата
- По типу события

## Тихие часы (DND по расписанию)

- Можно задать временной диапазон (например, 23:00–8:00) + timezone
- **Push:** **suppressed** в окне quiet hours (`ApplyQuietHours` → `Push=false`)
- **In-app:** **still delivered** через Realtime WebSocket — unread badge и in-app notification row обновляются как обычно
- **@mention override:** если `override_mentions=true`, mentions **пробивают** quiet-hours push block (как mute override)

Не формулировать как «уведомления не приходят» — корректно: «push suppressed; in-app may still deliver».

## @упоминания и тишина

- Заглушённый чат: @username-упоминание **пробивает тишину** — включено по умолчанию, можно отключить в настройках
- Broadcast `@everyone` в чате: только при праве `TEXT_CHAT_MENTION_ALL_IN_CHAT`; тишину пробивает по той же логике

## Send without sound (`send_silent`)

| Layer | Wire name |
|-------|-----------|
| Composer / `SendMessageRequest` | **`send_silent`** (bool) |
| JetStream `message.sent` | **`send_silent`** |
| UI label | «Send without sound» / «Отправить без звука» |

**Notification consumption:**

| Channel | Behavior when `send_silent=true` |
|---------|----------------------------------|
| **Push** | Delivered **without sound** and **without badge increment** on some platforms; no DND break (except explicit mention-break rules) |
| **In-app** | Unread badge + in-app notification **update as usual** |
| **Grouping** | Still groups by chat |

См. [text-chat.md](text-chat.md) § Send options; producer — Messaging `message.sent`; consumer — Notification Service.

## Уведомления во время войс-чата

- Показываются как оверлей внутри приложения — войс не прерывается
