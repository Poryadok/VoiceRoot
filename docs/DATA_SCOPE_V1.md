# Скоуп данных — персистентная модель

Документ фиксирует **текущий объём** персистентной модели: что уже в миграциях, что требует новых DDL, и что **deferred** (только федерация). Каталог фич: [FEATURES.md](FEATURES.md), статус реализации — [PLAN.md](PLAN.md). Общие правила ID, ссылок между БД и полей: [DATA_MODEL.md](DATA_MODEL.md). Инвентарь БД: [DATA_STORES.md](DATA_STORES.md).

---

## 1. Ядро (shipped migrations)

**Ядро** = инфраструктурный фундамент + **MVP личных сообщений** ([auth-and-contacts.md](features/auth-and-contacts.md), [friends.md](features/friends.md), [text-chat.md](features/text-chat.md)): регистрация/логин, один активный профиль на аккаунт (без полноценного мульти-профиля как продукта), друзья и блокировки, DM-чаты, сообщения с **базовым** edit/delete на бэкенде (разветвление UX «для всех/себя» — позже в text-chat), отметка прочитанного, базовое presence, профиль с полем аватара (URL в R2 — [user-profile.md](features/user-profile.md)), Realtime без собственной PostgreSQL.

Упоминания `*_db` в [MICROSERVICES.md](MICROSERVICES.md) и `docs/microservices/*.md` — целевая модель сервиса; часть таблиц уже в коде сверх «ядра» (группы, пины, E2E и т.д.).

Цель ядра по данным: пользователь с аккаунтом и профилем может состоять в дружбе, иметь DM, отправлять и читать сообщения, видеть список диалогов с превью и непрочитанным.

---

## 2. Расширения и отдельные сервисы

Ниже — области вне базового «ядра»; реализуются по [FEATURES.md](FEATURES.md) и [MICROSERVICES.md](MICROSERVICES.md). **Федерация — единственный deferred.**

| Область                                                            | Статус / ориентир                                                                                           |
|--------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------|
| Групповые чаты, спейсы, каналы, роли в спейсах                     | [spaces.md](features/spaces.md), [roles.md](features/roles.md)                                              |
| Реакции, треды, пины, пересылка, Markdown, вложения                | [text-chat.md](features/text-chat.md), [forward-messages.md](features/forward-messages.md)                  |
| Typing indicator                                                   | text-chat (эфемерно в Realtime)                                                                             |
| Push, Notification Service                                         | [notifications.md](features/notifications.md)                                                               |
| Голос/видео, полноценный File Service, вложения в чате             | [voice-chat.md](features/voice-chat.md), [file-storage.md](features/file-storage.md) (аватар — в ядре)    |
| Matchmaking, Search (отдельная `search_db`), Analytics, ClickHouse | [matchmaking.md](features/matchmaking.md), [search.md](features/search.md), [analytics.md](features/analytics.md) |
| Базовые репорты, 2FA, гранулярная приватность профиля              | [reports.md](features/reports.md), [privacy.md](features/privacy.md)                                        |
| Subscription                                                       | [subscription.md](features/subscription.md)                                                                 |
| Мульти-профиль (продукт), верификация значков                      | [multi-profile.md](features/multi-profile.md), [verification.md](features/verification.md)                  |
| **Федерация**                                                      | **deferred** — [federation.md](features/federation.md); `federation_db` не провижинится                     |
| Авто-мод, E2E (DM), боты, сторис                                  | Частично в коде; см. [encryption.md](features/encryption.md), [bots.md](features/bots.md), [stories.md](features/stories.md) |

Для **Messaging** в ядре достаточно «сообщение в чате» + **read receipts**. Таблицы `reactions`, `pins`, треды и вложения — расширения сверх ядра (часть уже в коде). **`scheduled_messages`** — current send options; DDL и worker — backlog [todo/backend.md](todo/backend.md).

Для **Chat** в ядре — тип **`dm`** и участники. **Папки** (`folders`, `folder_chats`) и **Quick Access** (`quick_access_chats`) — **current** IA ([navigation.md](features/navigation.md)); отдельные миграции Chat Service — backlog [chat-service.md](microservices/chat-service.md#модель-данных), [todo/backend.md](todo/backend.md). Поле `chat_members.is_archived` — shipped; list/filter archived — current, RPC backlog.

Для **User** в ядре — **профиль** и базовые поля (в т.ч. URL аватара в R2 и «О себе» — [user-profile.md](features/user-profile.md)); **мульти-профиль** как продукт и верификация — [multi-profile.md](features/multi-profile.md), [verification.md](features/verification.md); расширенные Premium-поля — [subscription.md](features/subscription.md); технически одна строка `profiles` с `is_primary = true` на аккаунт достаточна для DM core.

**Аватар и R2 без `file_db`:** загрузка **только** статичного аватара профиля — presigned upload в R2 и запись ссылки в `profiles`; без таблиц File Service и без общего пайплайна вложений (детали — [user-profile.md](features/user-profile.md), [PLAN.md](PLAN.md)). Вложения в чате, метаданные файлов в БД, антивирус и т.д. — [file-storage.md](features/file-storage.md), см. [file-service.md](microservices/file-service.md).

Для **Social** в ядре — заявки в друзья, дружба, блокировки на уровне **аккаунта** ([social-service.md](microservices/social-service.md)). Полный **контакт-лист / телефонная синхронизация** — current scope; отдельная миграция, если ещё не в коде.

Решения по спорным зонам из фич:
- **Onboarding**: backend-флаг завершения обязателен для повторяемого UX; минимально включаем в `user_db` (например поле/таблица состояния онбординга).
- **Privacy**: гранулярная модель из `features/privacy.md` — current scope; в ядре достаточен минимальный enforceable baseline для DM (allow-by-default + блокировки из Social).
- **Verification**: OAuth/DNS-артефакты и cron re-check — current scope; в `user_db` допускается компактный статус/тип badge как nullable-поля.
- **Version policy (`/version`)**: source of truth у Gateway (config store/control-plane таблица), без отдельной service-owned PostgreSQL БД.

---

## 3. Сервисы и БД

| Сервис                                        | PostgreSQL     | Статус              |
|-----------------------------------------------|----------------|---------------------|
| API Gateway                                   | —              | Нет таблиц          |
| Auth Service                                  | `auth_db`      | Shipped             |
| User Service                                  | `user_db`      | Shipped             |
| Social Service                                | `social_db`    | Shipped             |
| Chat Service                                  | `chat_db`      | Shipped (+ backlog) |
| Messaging Service                             | `messaging_db` | Shipped (+ backlog) |
| Realtime Service                              | —              | Redis only          |
| Federation Service                            | `federation_db`| **deferred**        |
| Остальные из [DATA_STORES.md](DATA_STORES.md) | соотв. `*_db`  | По фичам            |

---

## 4. Минимальные сущности по сервисам (черновик для ER)

Ниже — **не финальная схема**, а согласование с `docs/microservices/*.md` и сужение под ядро. Детальные колонки, индексы и миграции — в соответствующих карточках сервисов (секция «Модель данных») и в коде миграций.

### 4.1 Auth (`auth_db`)

Соответствие [auth-service.md](microservices/auth-service.md), именование — [DATA_MODEL.md](DATA_MODEL.md), [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md):

- `accounts` — учётная запись, `deleted_at` для soft delete.
- `refresh_tokens` — ссылка на аккаунт колонкой **`account_id`** (не `user_id`).
- `otp_codes` — верификация email / сброс пароля, привязка к `account_id`.

JWT: claim **`user_id`** = `accounts.id` (то же значение, что логическое **`account_id`**).

### 4.2 User (`user_db`)

По [user-service.md](microservices/user-service.md), минимум ядра:

- `profiles` — хотя бы один профиль на аккаунт; `account_id` как UUID без FK на другой кластер ([DATA_MODEL.md](DATA_MODEL.md)).
- **Presence** ([presence.md](features/presence.md)): онлайн/оффлайн и last seen — допустим **Redis** как в доке User Service; персистентный `last_seen` в PostgreSQL — current scope для «hours ago»; зафиксировать выбор в [user-service.md](microservices/user-service.md#модель-данных).
- `onboarding_completed` (или эквивалентное состояние шагов) — минимально включаем в ядре, так как это backend-персистентный флаг продуктового поведения.

Таблица **`privacy_settings`** — current scope; в ядре достаточно базовых правил доступа к DM через Social-блокировки и дефолтную политику.

### 4.3 Social (`social_db`)

По [social-service.md](microservices/social-service.md):

- `friendships` — `pending` / `accepted` / `declined`, связь по **`profile_id`** сторон.
- `blocks` — **`blocker_account_id`**, **`blocked_account_id`** (блок на уровне аккаунта).

Таблица **`contacts`** — опционально для ядра (см. п. 2).

### 4.4 Chat (`chat_db`)

По [chat-service.md](microservices/chat-service.md), ядро:

- `chats` — `type = dm` для MVP; перечисление типов можно заложить в схеме наперёд.
- `chat_members` — связь чат ↔ профиль, уникальность пары `(chat_id, profile_id)`; `is_archived` (write path live).

**Backlog (current IA)** — отдельные миграции Chat Service:

| Таблица | Назначение |
|---------|------------|
| `folders` | Папки per profile: system + custom, `sort_order`, `filter_config_json` |
| `folder_chats` | Членство чата в папке: `folder_id`, `chat_id`, `sort_order`, pin-in-folder (`is_pinned`, `pin_order`) |
| `quick_access_chats` | Shortcuts per profile (≤15 `chat_id`), отдельно от pin чата и Social favourites; `sort_order` |

Контракты RPC — [navigation.md](features/navigation.md), [chat-service.md](microservices/chat-service.md); не смешивать Quick Access с polymorphic space-icon targets без ADR.

### 4.5 Messaging (`messaging_db`)

По [messaging-service.md](microservices/messaging-service.md), ядро:

- `messages` — идентификатор, `chat_id`, отправитель `sender_profile_id`, текст, метки времени; базовое редактирование и удаление: как минимум `edited_at` и `deleted_at` (soft delete) или согласованный эквивалент; без тредов/реакций/пинов/`forward` в первой миграции (nullable под будущее — решение в [messaging-service.md](microservices/messaging-service.md#модель-данных)).
- `read_receipts` — `chat_id`, `profile_id`, `last_read_message_id` (или согласованный эквивалент для списка диалогов и `mark_read`).

**Backlog (current spec):**

| Таблица | Назначение |
|---------|------------|
| `scheduled_messages` | Отложенная отправка / send-when-online: `chat_id`, `sender_profile_id`, payload, `scheduled_at`, timezone, status, idempotency key |

**Pin messages:** таблица `pins` может быть в коде раньше полного соответствия спеке; лимит **= 5** per chat ([text-chat.md](features/text-chat.md)); код сейчас **50** — [todo/backend.md](todo/backend.md).

`chat_type` в сообщениях для ядра может быть фиксирован **`dm`** до появления групп и каналов.

### 4.6 Realtime

Без PostgreSQL; Redis Pub/Sub и реестр соединений — вне ER PostgreSQL ([realtime-service.md](microservices/realtime-service.md)). Эфемерные WS-операции (`delivery_ack` → `message_delivered`, `mark_read` → `message_read`) — cross-instance fanout через Redis (`redis_fanout.go`); **не** персистентное состояние list preview (durable delivery — Messaging, см. [messaging-service.md](microservices/messaging-service.md)).

---

## 5. Проход по фичам (трассировка на сущности)

Используется как чеклист «ничего не забыть» при детализации схем в `docs/microservices/*.md` и миграциях:

| Фича (файл)                                           | Затрагиваемые сервисы / БД | Заметки                                                                                                                                                          |
|-------------------------------------------------------|----------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [auth-and-contacts.md](features/auth-and-contacts.md) | Auth, User, Social         | Регистрация/логин → `accounts`, OTP; профиль → `profiles`                                                                                                        |
| [friends.md](features/friends.md)                     | Social, User               | Заявки и друзья → `friendships`; блоки → `blocks`; поиск пользователя для заявки — выдача по `user_db`/API User, не отдельная `search_db` ([search.md](features/search.md)) |
| [user-profile.md](features/user-profile.md)           | User                       | Отображаемое имя; **аватар** — URL после загрузки в R2 (presigned-контур без `file_db`); **вложения в чате и полный File Service** — [file-storage.md](features/file-storage.md) |
| [presence.md](features/presence.md)                   | User, Realtime             | online/offline + last seen — см. п. 4.2                                                                                                                          |
| [text-chat.md](features/text-chat.md)                 | Chat, Messaging, Realtime  | **edit/delete на API** — базовая политика ([PLAN.md](PLAN.md)); полнотекст и глобальный поиск — не `search_db`, см. [search.md](features/search.md)             |
| [navigation.md](features/navigation.md)               | Chat (`chat_db`)           | **current** IA требует `folders`, `folder_chats`, `quick_access_chats`; archive — `chat_members.is_archived` (shipped) + list RPC backlog                        |
| [onboarding.md](features/onboarding.md)               | User                       | Нужен backend-state (`onboarding_completed` или эквивалент), чтобы туториал не сбрасывался после переустановки                                                   |
| [privacy.md](features/privacy.md)                     | User, Social               | current scope; в ядре обязательны блокировки и базовый DM-enforcement                                                                                            |
| [verification.md](features/verification.md)           | User, Auth                 | current scope; допускается placeholder-статус на профиле                                                                                                         |
| [deep-links.md](features/deep-links.md)               | Chat, Messaging, User      | Для DM в данных фиксируем адресацию по `profile_id`/`username`, не по alias `user_id(account_id)`                                                                |
| [updates.md](features/updates.md)                     | API Gateway                | `/api/v1/version` обязателен; хранение policy — config-store/control-plane, не отдельная PostgreSQL БД Gateway                                                   |
| [federation.md](features/federation.md)               | Federation                 | **deferred** — `federation_db` не провижинится                                                                                                                   |

Фичи вне ядра (спейсы, роли, поиск, MM, уведомления, подписка и т.д.) — свои `*_db` по [DATA_STORES.md](DATA_STORES.md); при появлении событий NATS не смешивать с обязательным набором ядра.

---

## 6. События и ссылки между сервисами

- Публикации из Auth / User / Social / Chat / Messaging — по таблицам в соответствующих `microservices/*.md`.
- **Удаление аккаунта** (soft delete в `accounts`): обработка в остальных сервисах (анонимизация, запрет входа, политика данных) должна быть описана в продуктовых/операционных доках; на уровне схемы — везде, где есть `account_id` / `profile_id`, заложить ожидаемое поведение в карточках `docs/microservices/*.md` и миграциях (без FK между БД, см. [DATA_MODEL.md](DATA_MODEL.md)).

---

## 7. Следующие шаги

1. Детализация таблиц — в секциях «Модель данных» [docs/microservices/](microservices/); ядро — этот документ (п. 4) + [OPERATIONS.md](OPERATIONS.md) (миграции).
2. При реализации — сверять миграции с [OPERATIONS.md](OPERATIONS.md) (expand/contract, владелец миграций).
3. Сверка с [PLAN.md](PLAN.md) и [DATA_SCOPE_V1.md](DATA_SCOPE_V1.md): чеклист уже ссылается на этот скоуп и перечисленные `*_db`; при появлении устаревших имён схем в коде — не смешивать с целевыми миграциями per service.

---

## 8. Матрица ядра (фикс для генерации DDL)

Чтобы исключить смешение shipped backlog и deferred, для DDL **ядра** используется только подмножество ниже:

| Сервис | Таблицы ядра | Backlog (current spec) | Что явно не входит в ядро |
|--------|--------------|------------------------|---------------------------|
| Auth (`auth_db`) | `accounts`, `refresh_tokens`, `otp_codes` | — | — |
| User (`user_db`) | `profiles`, `onboarding_state` | `privacy_settings`, расширенные Premium-настройки | — |
| Social (`social_db`) | `friendships`, `blocks` | `contacts` | — |
| Chat (`chat_db`) | `chats` (в т.ч. group/channel types в коде), `chat_members` (+ `is_archived`) | `folders`, `folder_chats`, `quick_access_chats` | folder pin/order RPC без DDL |
| Messaging (`messaging_db`) | `messages`, `read_receipts`; `pins` если уже в миграциях | `scheduled_messages` | `reactions`, `message_attachments`, полноценные треды/forward |
| Federation (`federation_db`) | — | — | **deferred** — не провижинится |

Все детальные типы, CHECK/PK/UNIQUE и индексы фиксируются в карточках:
- [microservices/auth-service.md](microservices/auth-service.md)
- [microservices/user-service.md](microservices/user-service.md)
- [microservices/social-service.md](microservices/social-service.md)
- [microservices/chat-service.md](microservices/chat-service.md)
- [microservices/messaging-service.md](microservices/messaging-service.md)

## 9. Закрытые TBD

- `folders`, `folder_chats`, `quick_access_chats` в Chat: **current** UX; DDL и RPC — backlog [todo/backend.md](todo/backend.md).
- `scheduled_messages` в Messaging: **current** send options; DDL backlog.
- Pin limit: **= 5** per chat; код `MaxPinsPerChat = 50` — выровнять в [todo/backend.md](todo/backend.md).
- `last_seen`: в ядре хранится в Redis presence; персистентная колонка PostgreSQL — current scope для «hours ago» (audit C06).
- Onboarding: в ядре хранится персистентно в `user_db.onboarding_state`.
- Federation: **deferred** — `federation_db` не провижинится до явного решения.


