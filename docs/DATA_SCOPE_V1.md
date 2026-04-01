# Скоуп данных v1 (перед проектированием таблиц)

Документ фиксирует **первую осмысленную итерацию** персистентной модели: что входит в объём схемы БД до детальных файлов `docs/data/*.md` и миграций. Источник фаз продукта: [PLAN.md](PLAN.md). Общие правила ID, ссылок между БД и полей: [DATA_MODEL.md](DATA_MODEL.md). Инвентарь БД: [DATA_STORES.md](DATA_STORES.md).

---

## 1. Определение «данные v1»

**Данные v1** = всё, что нужно для **Фазы 0 (фундамент)** и **Фазы 1 (MVP: личные сообщения)** из [PLAN.md](PLAN.md): регистрация/логин, один активный профиль на аккаунт (без полноценного мульти-профиля как продукта), друзья и блокировки, DM-чаты, сообщения с **базовым** edit/delete на бэкенде (одна политика; разветвление UX «для всех/себя» и метка «(ред.)» — Фаза 3), отметка прочитанного, базовое presence, профиль с полем аватара (URL объекта в R2 — см. PLAN Фаза 1), Realtime без собственной PostgreSQL.

Цель v1 по данным: пользователь с аккаунтом и профилем может состоять в дружбе, иметь DM, отправлять и читать сообщения, видеть список диалогов с превью и непрочитанным — в соответствии с границами «Не входит в Фазу 1» в [PLAN.md](PLAN.md).

---

## 2. Что сознательно не входит в данные v1

Ниже — **не проектируем отдельные таблицы / не разворачиваем отдельные БД** в первой волне миграций (позже — по фазам PLAN и [MICROSERVICES.md](MICROSERVICES.md)):

| Область | Отложено до (ориентир) |
|---------|-------------------------|
| Групповые чаты, спейсы, каналы, роли в спейсах | Фазы 4–5 |
| Реакции, треды, пины, пересылка, Markdown, вложения | Фазы 3–6+ (вложения/typing — 3; реакции/forward — 4; Markdown/пины — 6; треды — 10; см. [PLAN.md](PLAN.md)) |
| Typing indicator | Фаза 3 (эфемерно в Realtime; отдельная БД не требуется) |
| Push, Notification Service | FCM Web/Android — [PLAN.md](PLAN.md) Фаза 6; APNs / VoIP / доработка — Фаза 8 |
| Голос/видео, полноценный File Service, вложения в чате | Фазы 2–3+ (**аватар профиля в R2** — уже Фаза 1; `file_db` и таблицы вложений — позже) |
| Matchmaking, Search (отдельная `search_db`), Analytics, ClickHouse | Фазы 7, 9+ |
| Базовые репорты, 2FA, гранулярная приватность профиля | Фаза 11 |
| Subscription | Фаза 12 |
| Мульти-профиль (продукт), верификация значков | Фаза 13 |
| Авто-мод, панель модераторов, E2E (DM), боты, федерация, сторис | Фазы 14–19+ |

Для **Messaging** в v1 достаточно ядра «сообщение в чате» + **read receipts** (или эквивалент для `mark_read` и счётчика непрочитанных). Таблицы `reactions`, `pins`, полноценные треды и вложения — вне v1.

Для **Chat** в v1 — тип **`dm`** и участники; **папки** (`folders`, `folder_chats`) можно отложить до появления UX папок в плане или ввести минимально, если список диалогов без них невозможен — решение фиксируется в `docs/data/chat-service.md`.

Для **User** в v1 — **профиль** и базовые поля (в т.ч. URL аватара в R2 и «О себе» — PLAN Фаза 1); **мульти-профиль** как продукт и верификация — **Фаза 13**; расширенные Premium-поля — **Фаза 12**; технически одна строка `profiles` с `is_primary = true` на аккаунт достаточна для Фазы 1.

Для **Social** в v1 — заявки в друзья, дружба, блокировки на уровне **аккаунта** ([social-service.md](microservices/social-service.md)). Полный **контакт-лист / телефонная синхронизация** можно второй очередью в рамках Фазы 1, если продуктово обязателен; иначе — отдельная миграция после ядра.

---

## 3. Сервисы и БД в объёме v1

| Сервис | PostgreSQL | В объёме v1 |
|--------|------------|-------------|
| API Gateway | — | Нет таблиц |
| Auth Service | `auth_db` | Да |
| User Service | `user_db` | Да |
| Social Service | `social_db` | Да |
| Chat Service | `chat_db` | Да |
| Messaging Service | `messaging_db` | Да |
| Realtime Service | — | Redis only |
| Остальные из [DATA_STORES.md](DATA_STORES.md) | соотв. `*_db` | Нет в первой волне |

---

## 4. Минимальные сущности по сервисам (черновик для ER)

Ниже — **не финальная схема**, а согласование с `docs/microservices/*.md` и сужение под Фазу 1. Детальные колонки, индексы и миграции — в `docs/data/<service>.md`.

### 4.1 Auth (`auth_db`)

Соответствие [auth-service.md](microservices/auth-service.md), именование — [DATA_MODEL.md](DATA_MODEL.md), [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md):

- `accounts` — учётная запись, `deleted_at` для soft delete.
- `refresh_tokens` — ссылка на аккаунт колонкой **`account_id`** (не `user_id`).
- `otp_codes` — верификация email / сброс пароля, привязка к `account_id`.

JWT: claim **`user_id`** = `accounts.id` (то же значение, что логическое **`account_id`**).

### 4.2 User (`user_db`)

По [user-service.md](microservices/user-service.md), v1-минимум:

- `profiles` — хотя бы один профиль на аккаунт; `account_id` как UUID без FK на другой кластер ([DATA_MODEL.md](DATA_MODEL.md)).
- **Presence в Фазе 1** по PLAN: онлайн/оффлайн и last seen — допустим **Redis** как в доке User Service; персистентный `last_seen` в PostgreSQL — по необходимости для «последний визит» после перезапуска; зафиксировать выбор в `docs/data/user-service.md`.

Таблица **`privacy_settings`** — включать в v1, если без неё нельзя соблюсти базовые правила DM/поиска пользователей из фич; иначе дефолты в коде до появления настроек в UI.

### 4.3 Social (`social_db`)

По [social-service.md](microservices/social-service.md):

- `friendships` — `pending` / `accepted` / `declined`, связь по **`profile_id`** сторон.
- `blocks` — **`blocker_account_id`**, **`blocked_account_id`** (блок на уровне аккаунта).

Таблица **`contacts`** — опционально для v1 (см. п. 2).

### 4.4 Chat (`chat_db`)

По [chat-service.md](microservices/chat-service.md), сужение:

- `chats` — `type = dm` для MVP; перечисление типов можно заложить в схеме наперёд.
- `chat_members` — связь чат ↔ профиль, уникальность пары `(chat_id, profile_id)`.

### 4.5 Messaging (`messaging_db`)

По [messaging-service.md](microservices/messaging-service.md), v1:

- `messages` — идентификатор, `chat_id`, отправитель `sender_profile_id`, текст, метки времени; **Фаза 1** по PLAN — базовое редактирование и удаление: как минимум `edited_at` и `deleted_at` (soft delete) или согласованный эквивалент; без тредов/реакций/пинов/`forward` в первой миграции (nullable под будущее — решение в `docs/data/messaging-service.md`).
- `read_receipts` — `chat_id`, `profile_id`, `last_read_message_id` (или согласованный эквивалент для списка диалогов и `mark_read`).

`chat_type` в сообщениях для v1 может быть фиксирован **`dm`** до появления групп и каналов.

### 4.6 Realtime

Без PostgreSQL; Redis Pub/Sub и реестр соединений — вне ER PostgreSQL ([realtime-service.md](microservices/realtime-service.md)).

---

## 5. Проход по фичам (трассировка на сущности v1)

Используется как чеклист «ничего не забыть» при написании `docs/data/*.md`:

| Фича (файл) | Затрагиваемые сервисы / БД | Заметки для v1 |
|-------------|----------------------------|----------------|
| [auth-and-contacts.md](features/auth-and-contacts.md) | Auth, User, Social | Регистрация/логин → `accounts`, OTP; профиль → `profiles` |
| [friends.md](features/friends.md) | Social, User | Заявки и друзья → `friendships`; блоки → `blocks`; поиск пользователя для заявки (PLAN Фаза 1) — выдача по `user_db`/API User, не отдельная `search_db` (Фаза 9) |
| [user-profile.md](features/user-profile.md) | User | Отображаемое имя, аватар URL (загрузка файлов — Фаза 3) |
| [presence.md](features/presence.md) | User, Realtime | v1: упрощённо online/offline + last seen — см. п. 4.2 |
| [text-chat.md](features/text-chat.md) | Chat, Messaging, Realtime | Фаза 1: без Markdown в UI, реакций, тредов, пинов; **edit/delete на API** — базовая политика (PLAN); полнотекст и глобальный поиск — не `search_db`, см. Фаза 9 |
| [navigation.md](features/navigation.md) | — | Терминология UI; на схему не влияет напрямую |

Фичи вне v1 (спейсы, роли, поиск, MM, уведомления, подписка и т.д.) — не требуют таблиц в первой волне; при появлении событий NATS позже — не смешивать с обязательным набором v1.

---

## 6. События и ссылки между сервисами

- Публикации из Auth / User / Social / Chat / Messaging — по таблицам в соответствующих `microservices/*.md`.
- **Удаление аккаунта** (soft delete в `accounts`): обработка в остальных сервисах (анонимизация, запрет входа, политика данных) должна быть описана в продуктовых/операционных доках; на уровне схемы v1 — везде, где есть `account_id` / `profile_id`, заложить ожидаемое поведение в `docs/data/*.md` (без FK между БД, см. [DATA_MODEL.md](DATA_MODEL.md)).

---

## 7. Следующие шаги

1. ~~Для каждого сервиса из п. 3 — файл **`docs/data/<service>.md`**~~ — сделано: [data/README.md](data/README.md).
2. При реализации — сверять миграции с [OPERATIONS.md](OPERATIONS.md) (expand/contract, владелец миграций).
3. Сверка с [PLAN.md](PLAN.md) Фаза 0: чеклист уже ссылается на этот скоуп и перечисленные `*_db`; при появлении устаревших имён схем в коде — не смешивать с целевыми миграциями per service.
