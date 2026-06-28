# Voice — План разработки

> Каждая фаза — **один ощутимый сдвиг** для пользователя или для команды (релизный инкремент). Ниже — **развёрнутые чеклисты с границами**: что именно строить, до какого уровня, какой сервис полноценный vs минимальный. Продуктовые детали — в `docs/features/` и `docs/microservices/`; здесь — **критерии «готово»** для разработки без догадок.

---

## Текущее состояние кодовой базы

Репозиторий: **фазы 0–10 закрыты на локальном `make compose-app-up`** (см. чеклисты ниже). **Локальный стенд:** Auth, User, Social, Chat, Messaging, Realtime, Gateway, Space, Role, Voice, File, Search, Matchmaking, Notification и Flutter web ([README.md](../README.md)). **Staging:** полный k8s-стек — [`deploy/staging/`](../deploy/staging/), `scripts/staging/render-and-apply.sh`; FCM/APNs на устройстве требуют секретов в кластере ([DEPLOYMENT.md](DEPLOYMENT.md)).

**Auth (Java):** register/login/refresh/logout/validate (REST+gRPC), JWT/JWKS, PostgreSQL + Flyway, Redis blacklist, стабильный JWKS из PKCS#8; Testcontainers `AuthJdbcRedisIntegrationTest`.

**API Gateway (Go):** REST proxy к Auth; **REST→gRPC transcoding** для Phase-1 namespace (`/api/v1/users`, `/friends`, `/chats`, `/messages`); прокси **`/ws`** к Realtime; JWT/JWKS, Redis blacklist/rate limit (в compose dev — `GATEWAY_RATE_LIMIT_RULES_JSON`, отключение bucket `Auth`), CORS, тесты in-process и opt-in live против compose.

**Сервисы Фазы 1 (Go, gRPC + Postgres/Redis/NATS где нужно):** основные потоки реализованы с тестами: **User** — профили, `SearchProfiles`, `ListMyProfiles`, код R2 avatar presigned upload, presence/last_seen через User API; **Social** — друзья (invite/accept/decline/list/block); **Chat** — DM `CreateDM`, `ListChats`, preview/unread через S2S enrichment к Messaging (degraded при недоступности Messaging); **Messaging** — send/history/cursor, UUIDv7 `messages.id`, базовое edit/delete, read receipts (+ `message.read` в NATS), публикация в NATS; **Realtime** — JWT WS `hello`/`subscribe`, `message_create`, `s`/`resume`, `mark_read`/`presence_update`, Redis fan-out, NATS `chat.events` → `chat_update`, WS presence → User при `REALTIME_USER_GRPC_ADDR`. Остальные backend-сервисы из [MICROSERVICES.md](MICROSERVICES.md) — по-прежнему scaffold (`GET /health`, Dockerfile, CI matrix).

**Flutter-клиент** ([`src/frontend/`](../src/frontend/)): shell + **основные экраны Фазы 1 реализованы** — login/register (локализованные ошибки auth), social panel (поиск, друзья, профиль, presence, DM без дружбы), список чатов и комната с WS catch-up; widget/unit/i18n tests и opt-in live API-тесты (`phase1_two_users_e2e_live_test`, `gateway_dm_ws_live_integration_test`) против локального gateway — [integration_test/README.md](../src/frontend/integration_test/README.md). До полного закрытия клиентского чеклиста нужен UI-level realtime сценарий.

| Компонент | Состояние |
|-----------|-----------|
| Документация `docs/` | Актуальные спеки, план фаз, микросервисы |
| `protos/` | Protobuf (S2S + сервисные API); **buf** — [buf.work.yaml](../buf.work.yaml), [protos/buf.yaml](../protos/buf.yaml), CI |
| `src/backend/gateway/` | Edge: health/metrics/version, Auth REST upstream, Phase-1 gRPC transcoding, `/ws`→Realtime, JWT, rate limits; compose + `phase1_rest_integration_test` / live compose tests |
| `src/backend/user`, `social`, `chat`, `messaging`, `realtime` | Доменная логика Фазы 1 в основном реализована; для медиа в dev нужен R2 в `.env` (см. `.env.example`) |
| `src/backend/<остальные>/` | Scaffold: health, smoke, Dockerfile, `make build-all` |
| `src/frontend`, `src/admin` | **Frontend:** Phase-1 UI (auth, social, chat, WS) поверх shell; widget/live tests, `make flutter-ci`. **Admin:** зарезервировано |
| `src/backend/migrations/` | SQL v1 + init в compose для локального dev |
| Docker Compose | Infra: Postgres (5 БД), Redis, NATS (JetStream). **`--profile app`:** Phase-1 app stack — [README.md](../README.md), `make compose-app-up` |
| Локальные проверки | **`make build-all`** + **`make flutter-ci`**; opt-in live: gateway compose tests, Flutter `VOICE_RUN_LIVE_INTEGRATION` — [TESTING.md](TESTING.md) |
| CI / staging | [ci.yml](../.github/workflows/ci.yml), [staging-deploy.yml](../.github/workflows/staging-deploy.yml); staging FQDN `voice.tastytest.online` — пока в основном gateway |

Целевые сервисы и БД по-прежнему описаны в [MICROSERVICES.md](MICROSERVICES.md) и `docs/microservices/*`; ниже — **куда класть код** при появлении реализации.

### План размещения кода → целевой микросервис

| Целевой сервис | Стек (цель) | Каталог / примечание |
|----------------|-------------|----------------------|
| Auth Service | Java/Spring | `src/backend/auth/` или согласованное имя; миграции `auth_db` (Flyway рядом с модулем) |
| API Gateway | Go | `src/backend/gateway/` |
| Realtime Service | Go | `src/backend/realtime/` — WebSocket за Gateway ([ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md)) |
| Messaging Service | Go | `src/backend/messaging/` — `messaging_db` |
| Chat Service | Go | `src/backend/chat/` — `chat_db` |
| User Service | Go | `src/backend/user/` — `user_db` |
| Social Service | Go | `src/backend/social/` — `social_db` |
| Space / Voice и др. | Go (+ LiveKit) | по мере фаз из таблицы ниже |
| Клиент | Flutter | `src/frontend/` |
| Admin | Web | `src/admin/` |

Прогресс scaffold: каталоги выше созданы для всех backend-сервисов; это **техническая инициализация проектов**, не закрытие продуктовых задач сервисов.

---

## Сводная таблица фаз

Отметьте строку, когда **вся фаза** закрыта. Детальные чеклисты — в секциях «Фаза N» ниже.

| Готово | Фаза | Фокус                                              | Платформы             |
|--------|------|----------------------------------------------------|-----------------------|
| - [x]  | 0    | Инфраструктура, CI, схемы БД, i18n-каркас          | —                     |
| - [x]  | 1    | DM, друзья, профиль, presence                      | Web                   |
| - [x]  | 2    | Звонки 1:1                                         | Web                   |
| - [x]  | 3    | Вложения, typing, статусы, DM «запросы», edit/delete UX | Web                   |
| - [x]  | 4    | Группы, групповой голос, реакции                   | Web                   |
| - [x]  | 5    | Спейсы: структура и клиент                         | Web                   |
| - [x]  | 6    | Markdown, @mentions, пины, FCM (space channels OK; staging Firebase optional) | Web + Android (push)  |
| - [x]  | 7    | Матчмейкинг (squad voice UI; catalog/match_found E2E) | Web (+ push где есть) |
| - [x]  | 8    | Мобильные клиенты, APNs/VoIP prod creds, Win, кэш | Все                   |
| - [x]  | 9    | Поиск                                              | Все                   |
| - [x]  | 10   | Треды, шаринг экрана, shared media, кастомные роли | Все                   |
| - [x]  | 11   | Репорты (без панели), 2FA, приватность             | Все                   |
| - [ ]  | 12   | Подписки и платежи                                 | Все                   |
| - [ ]  | 13   | Мульти-профиль, верификация                        | Все                   |
| - [ ]  | 14   | Авто-мод, панель модераторов                       | Все                   |
| - [x]  | 15   | E2E DM (opt-in)                                    | Все                   |
| - [ ]  | 16   | Боты + Developer Portal (staging OK; prod partial) | Все                   |
| - [ ]  | 17   | Сторис                                             | Все                   |
| - [ ]  | 18   | Deep links, онбординг, a11y (baseline)             | Все                   |
| - [ ]  | 19   | Федерация, self-hosting                            | Все                   |

---

## Как читать фазы (легенда)

### Уровни реализации сервиса

| Уровень | Что это | Достаточно для закрытия фазы? |
|---------|---------|-------------------------------|
| **Scaffold** | `GET /health`, Dockerfile, CI matrix, пустой gRPC-сервер | **Нет** — только подготовка каталога |
| **Минимальный (MVP фазы)** | Реальная БД + миграции, gRPC/REST для сценариев **этой** фазы, интеграционные тесты на критический путь, сервис в `docker-compose` профиле `app` | **Да**, если фаза от него зависит |
| **Полный (целевой)** | Всё из спеки микросервиса: все RPC, edge cases, observability, rate limits по доку | **Нет** — дозакрывается в следующих фазах, если не указано иное |
| **Внешняя интеграция** | LiveKit, R2, ClamAV, FCM/APNs, Paddle и т.д. | Нужна **рабочая** связка в dev/staging, не mock в прод-пути |

**Правило:** если пункт фазы помечен сервисом (например, «Messaging»), для закрытия пункта нужен уровень **минимальный** для этого сценария. Scaffold **не засчитывается**, кроме явно помеченных «заглушка допустима».

### Структура пункта чеклиста

- **Сервис(ы)** — куда класть код ([таблица размещения](#план-размещения-кода--целевой-микросервис) выше).
- **Объём** — API, таблицы, NATS streams ([CONTRACT_MATRIX.md](CONTRACT_MATRIX.md)).
- **Не делать** — что сознательно отложено (ссылка на фазу).
- **Готово когда** — проверяемые критерии (ручной сценарий + автотесты).
- **Тесты** — минимум по [TESTING.md](TESTING.md).

### Когда фаза считается закрытой

1. Все чекбоксы секции «Фаза N» отмечены.
2. `make build-all` и `make flutter-ci` зелёные; для фазы добавлены opt-in live-тесты, если есть сквозной сценарий.
3. Затронутые сервисы в compose `app` (или документированы env для внешних зависимостей).
4. Gateway transcoding / маршруты для новых RPC добавлены в [CONTRACT_MATRIX.md](CONTRACT_MATRIX.md).
5. Критерии приёмки в конце фазы выполняются на локальном стенде двумя тестовыми пользователями (если фаза про multi-user).

### Заглушка vs минимальный сервис — типичные решения

| Ситуация | Решение |
|----------|---------|
| Фаза требует отправку сообщений | **Messaging** — минимальный, не stub |
| Фаза требует только проверку JWT | **Auth** уже есть с Фазы 0 |
| Новый домен (спейс, ММ, биллинг) | Новый сервис **минимальный** + миграции своей БД |
| Admin UI для модераторов | Отдельное веб-приложение `src/admin/`, не заглушка в Flutter |
| Analytics, Meilisearch v2 | **Не в фазах 0–12** — scaffold достаточен |
| S2S между сервисами | Реальный gRPC client; сквозной путь — против реального сервиса в compose |

---

## Фаза 0 — Фундамент

**Цель для пользователя:** пока нет — инфраструктура для команды.

**Цель для команды:** любой разработчик поднимает локальный стенд, логинится через бэкенд, CI ловит регрессии до merge.

### Границы фазы

| Входит | Не входит |
|--------|-----------|
| Compose infra + профиль `app` для Phase-1 стека | Продуктовые фичи (DM, звонки…) |
| Auth, Gateway, миграции v1, buf/CI | Реализация Space, Voice, File, Notification и др. |
| Scaffold **всех** backend-сервисов | Бизнес-логика в scaffold-сервисах |
| Flutter shell + i18n каркас | Полноценный UI чатов |

### Уровень сервисов

| Сервис | Уровень в Фазе 0 |
|--------|------------------|
| Auth | **Минимальный** — register/login/refresh/logout/validate, JWT, JWKS, Postgres, Redis blacklist |
| Gateway | **Минимальный** — JWT, CORS, rate limit, `/metrics`, `/api/v1/version`, прокси Auth |
| User, Social, Chat, Messaging, Realtime | Scaffold или начало Фазы 1 (вертикальный срез) |
| Остальные 15 сервисов | **Scaffold** только |

### Бэкенд

- [x] **Схемы БД (первая волна)** — [DATA_SCOPE_V1.md](DATA_SCOPE_V1.md), [DATA_STORES.md](DATA_STORES.md)
  - **Объём:** `auth_db`, `user_db`, `social_db`, `chat_db`, `messaging_db`; init в `docker/postgres/`; Flyway для Auth.
  - **Готово когда:** чистый compose на новом volume создаёт все схемы.

- [x] **API Gateway** — [api-gateway.md](microservices/api-gateway.md)
  - **Объём:** REST edge, JWT/JWKS, Redis blacklist + rate limit, CORS, logging, `/metrics`, `/api/v1/version`, health; позже transcoding Phase-1.
  - **Тесты:** in-process integration tests в `src/backend/gateway/`.

- [x] **CI/CD** — [DEPLOYMENT.md](DEPLOYMENT.md)
  - **Объём:** Actions PR/push, `make build-all`, GHCR, staging gateway, smoke после деплоя.

- [x] **Docker Compose (dev)** — Postgres, Redis, NATS; профиль `app`; `make compose-app-up`.

- [x] **Backend project scaffolds** — все `src/backend/<service>/`: health, Dockerfile, CI matrix. **Не** закрывает продуктовые задачи.

- [x] **Auth (Java)** — [auth-service.md](microservices/auth-service.md)
  - **Объём:** REST+gRPC; refresh opaque 30d; Flyway; Redis blacklist; JWKS PKCS#8; Testcontainers.
  - **Готово когда:** register → login → refresh → logout; validate для Gateway.

- [x] **Auth — smoke контейнера в CI** — health + минимальный REST/gRPC против Postgres+Redis.

- [x] **Общая библиотека Go** — `src/backend/pkg/`: JWT, middleware, logging, config, integrationtest.

### Клиент

- [x] **Flutter скелет** — layout, backend client, DI — [navigation.md](features/navigation.md)
- [x] **i18n-каркас** — ARB EN+RU — [i18n.md](features/i18n.md)

### Критерии приёмки

1. `make build-all` зелёный с Docker.
2. Регистрация/логин через Gateway → валидный JWT.
3. Все backend-модули в CI; scaffold отвечает health.

**Результат:** репозиторий готов к вертикальным срезам; локальный стенд и CI — единый контур. Продуктового чата ещё нет. Процесс: [CONTRIBUTING.md](CONTRIBUTING.md), [TESTING.md](TESTING.md), [DEPLOYMENT.md](DEPLOYMENT.md).

### Первый вертикальный срез (до полного закрытия Фазы 1)

JWT + `user_db.profiles` — [EXEC_PLAN.md](EXEC_PLAN.md). Не заменяет чеклист Фазы 1.

---

## Фаза 1 — MVP: личные сообщения

**Цель для пользователя:** зарегистрироваться, найти человека, написать в личку, видеть онлайн и переписку в реальном времени.

**Цель для команды:** сквозной DM-поток через Gateway → gRPC → Postgres/NATS → WS; основа для всех последующих типов чатов.

### Границы фазы

| Входит | Не входит (фаза) |
|--------|------------------|
| DM 1-на-1 без обязательной дружбы | Папка «запросы» от незнакомцев (**3**) |
| Текст, базовый edit/delete (одна политика, без UX «(ред.)») | Полный edit/delete UX (**3**) |
| Друзья: invite/accept/decline/block | Группы (**4**) |
| Профиль: имя, статичный аватар (presigned R2 в User), bio | File Service, вложения в чат (**3**) |
| Presence online/offline, last_seen | Кастомные статусы DND (**позже**) |
| Web-клиент EN+RU | Звонки (**2**), push (**6/8**) |

**Скоуп DM:** любой может написать первым; **один** список диалогов (без папки «запросы»).

### Уровень сервисов

| Сервис | Уровень |
|--------|---------|
| User | **Минимальный** — profiles, SearchProfiles, avatar presigned PUT, presence API |
| Social | **Минимальный** — friends flow + block |
| Chat | **Минимальный** — DM only: CreateDM, ListChats, members, unread enrichment |
| Messaging | **Минимальный** — send/history/cursor, UUIDv7, basic edit/delete, read receipts, NATS publish |
| Realtime | **Минимальный** — WS JWT, subscribe, message_create, s/resume, mark_read, presence → User |
| Gateway | Transcoding `/api/v1/users`, `/friends`, `/chats`, `/messages`; `/ws` → Realtime |
| File | **Не нужен** — аватар через User + R2 presigned |
| Notification | Scaffold |

### Бэкенд

- [x] **Compose: NATS / JetStream** — `NATS_URL`; streams `message.events`, `chat.events` по [CONTRACT_MATRIX.md](CONTRACT_MATRIX.md).

- [x] **Messaging** — [messaging-service.md](microservices/messaging-service.md)
  - **API:** SendMessage, GetHistory (cursor per `chat_id`), EditMessage, DeleteMessage, MarkRead.
  - **БД:** `messages` UUIDv7, `message_reads`; курсор по `(created_at, id)`.
  - **События:** publish `message.events` (create/edit/delete/read).
  - **Не делать:** вложения, reactions, threads, pins.
  - **Готово когда:** два пользователя обмениваются текстом; history после reconnect через REST cursor.
  - **Тесты:** store integration + gRPC handler tests.

- [x] **UUIDv7 для `messages.id`** — генерация по спеке messaging-service.

- [x] **Realtime** — [realtime-service.md](microservices/realtime-service.md), [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md)
  - **WS:** hello, subscribe(chat_ids), message_create fan-out, `s`/`resume`, mark_read, presence_update.
  - **Догрузка:** только через Messaging REST, не «догнать всё по WS».
  - **Redis:** pub/sub между инстансами; NATS consumer `chat.events`.
  - **Тесты:** WS protocol unit + integration.

- [x] **Чаты (DM)** — [chat-service.md](microservices/chat-service.md)
  - CreateDM, ListChats с preview/unread (S2S Messaging; degraded без Messaging).
  - **Не делать:** groups, channels, space folders.

- [x] **Друзья** — [social-service.md](microservices/social-service.md)
  - Invite, accept, decline, list, block.

- [x] **Профиль** — [user-service.md](microservices/user-service.md)
  - Display name, bio, avatar: presigned PUT R2, URL в `profiles`; max 2–5 MB, image MIME whitelist.
  - **Не делать:** animated avatar, banner (**12**).

- [x] **Presence** — [presence.md](features/presence.md)
  - Online/offline via WS → User gRPC; last_seen в профиле.

### Клиент (Web)

- [x] Login/register с локализованными ошибками auth.
- [x] Список диалогов, комната чата, realtime через WS + catch-up.
- [x] SearchProfiles, friend request, профиль, online indicator.
- [x] Локализация EN+RU всех экранов фазы.

### Критерии приёмки

1. Два аккаунта: A находит B, пишет DM; B видит сообщение <2s по WS.
2. Reconnect: `resume` + history cursor восстанавливает ленту.
3. Friend flow: invite → accept → в списке друзей.
4. Avatar upload через presigned URL отображается у собеседника.
5. Opt-in live: `phase1_two_users_e2e_live_test`, `gateway_dm_ws_live_integration_test`.

**Результат:** веб-мессенджер 1-на-1 для первых тестеров; архитектурный паттерн «Messaging + Realtime + Chat» зафиксирован для всех типов чатов.

---

## Фаза 2 — Звонки 1:1

**Цель для пользователя:** позвонить собеседнику в DM — входящий/исходящий, принять/отклонить, говорить, положить трубку.

### Границы фазы

| Входит | Не входит |
|--------|-----------|
| Голос 1:1 в DM | Видео в DM (опционально позже в voice-chat.md) |
| LiveKit SFU self-hosted | Групповой голос (**4**) |
| Signaling через WS + Voice Service tokens | PushKit/CallKit (**8**) |
| Mute, speaker, hangup в UI | Screen share (**10**) |

### Уровень сервисов

| Сервис | Уровень |
|--------|---------|
| Voice | **Минимальный** — CreateCall, Accept, Decline, End; LiveKit room + token; lifecycle в Postgres/Redis |
| Realtime | WS events: `call_incoming`, `call_accepted`, `call_ended` |
| LiveKit | **Внешняя интеграция** в compose |

### Бэкенд

- [x] **LiveKit** — сервер в compose; Voice Service выдаёт JWT room token с TTL.
- [x] **1:1 signaling** — исходящий/входящий через Realtime WS; callee accept/decline; один активный call per pair policy.
- [x] **Signaling** — LiveKit-native WebRTC; backend только lifecycle + tokens, **без** Firebase.
- **Готово когда:** A звонит B → B принимает → оба слышат друг друга → hangup освобождает room.

### Клиент

- [x] Экран звонка: incoming overlay, outgoing state, mute, speaker, hangup.
- [x] Интеграция LiveKit SDK web.

### Критерии приёмки

1. Полный цикл call на локальном compose с LiveKit.
2. Decline не оставляет «висящую» room.
3. Widget/integration test на signaling state machine (минимум).

**Результат:** голос поверх DM; Voice Service и LiveKit в стеке — база для группового войса.

---

## Фаза 3 — Медиа и «живой» чат

**Цель для пользователя:** отправлять картинки/файлы, видеть «печатает…» и галочки доставки; незнакомцы — в папке «запросы».

### Границы фазы

| Входит | Не входит |
|--------|-----------|
| File Service **минимальный** для вложений чата | Квоты Premium 200MB (**12**) |
| ClamAV scan после upload | Голосовые сообщения, GIF, стикеры (backlog text-chat) |
| Typing ~3s throttle | @mentions (**6**) |
| Delivery: sent/delivered/read в DM | Счётчик просмотров в группах (уже в **4** если группы есть) |
| Edit/delete UX: «(ред.)», for all / for self | Markdown (**6**) |
| DM inbox «запросы» | Privacy presets (**11**) |

### Уровень сервисов

| Сервис | Уровень |
|--------|---------|
| File | **Минимальный** — presigned upload, metadata, ClamAV hook, thumb WebP, `file_db`, NATS `file.events` |
| Messaging | Attachments в messages, edit/delete policies |
| Chat | Request folder model для DM от незнакомцев |
| Realtime | `typing_start`/`typing_stop` events |

### Бэкенд

- [x] **R2 + presigned** — 50MB free tier; [file-service.md](microservices/file-service.md).
- [x] **Вложения** — image thumb+full; file icon+name+size в message payload.
- [x] **WebP thumbnails** — server-side или worker в File Service.
- [x] **ClamAV** — async scan; block/quarantine infected; compose service.
- [x] **Typing** — Realtime broadcast, throttle 3s, expire 5s — [text-chat.md](features/text-chat.md).
- [x] **Статусы доставки** — sent/delivered/read в DM; WS + Messaging state.
- [x] **Edit/delete UX API** — dual policy for all/self; `edited_at`; согласовано с клиентом.
- [x] **DM «запросы»** — [friends.md](features/friends.md): Chat flags + отдельный список; accept → main inbox.

### Клиент

- [x] Attachment picker, preview, progress upload.
- [x] Typing indicator в шапке чата.
- [x] Checkmarks delivered/read.
- [x] Edited label, delete for all/self dialogs.
- [x] Requests folder UI.

### Критерии приёмки

1. Image + file в DM; thumb отображается; infected file rejected.
2. Typing visible to peer, не спамит WS.
3. Stranger DM попадает в «запросы» до accept.
4. Integration tests File + Messaging attachment path.

**Результат:** базовый паритет с Telegram по медиа и сигналам в DM; File Service в прод-пути (не stub).

---

## Фаза 4 — Групповые чаты

**Цель для пользователя:** группа до ~500 человек, текст, групповой голос, реакции, пересылка.

### Границы фазы

| Входит | Не входит |
|--------|-----------|
| Group chat type в Chat | Space channels (**5**) |
| Owner + members (простые роли) | Кастомные роли 32+ (**10**) |
| Group voice до 32 | Space voice rooms (**5**) |
| Reactions unicode | Custom emoji space (**12**) |
| Forward с атрибуцией | Копия без атрибуции (backlog) |
| In-app notification badge | FCM (**6**) |

### Уровень сервисов

| Сервис | Уровень |
|--------|---------|
| Chat | Groups: create, invite, kick, avatar, member list |
| Messaging | Reactions, forward metadata |
| Voice | Group room attach to `chat_id`, join active call |
| Role | **Минимальный** — owner flag only или built-in owner/member |

### Бэкенд

- [x] **Группы** — max ~500; create from DM/contacts; group avatar via File/User pattern.
- [x] **Роли простые** — owner (creator) + member; owner can kick.
- [x] **Групповой голос** — temporary/adhoc room; max 32; join mid-call.
- [x] **Forward** — [forward-messages.md](features/forward-messages.md); attribution block in message.
- [x] **Реакции** — add/remove, counts per emoji; WS fan-out.
- [x] **In-app notifications** — unread badge; web sound = NoOp until web audio (**6**).

### Клиент

- [x] Create group, member management, group chat room.
- [x] Group call UI, participant list.
- [x] Reaction picker, forward flow.
- [x] In-app notification center/badge.

### Критерии приёмки

1. 3+ users group messaging realtime.
2. Group call: 4 users simultaneous.
3. Reaction counts consistent after reconnect.
4. Forward shows original author attribution.

**Результат:** «пати в одном чате» без спейсов; паттерны group reuse в space channels.

---

## Фаза 5 — Спейсы: структура

**Цель для пользователя:** создать сообщество как Discord-сервер: дерево текстовых чатов и голосовых комнат, инвайты, базовые роли и модерация.

### Границы фазы

| Входит | Не входит |
|--------|-----------|
| Space CRUD, icon, description | Space Pro лимиты (**12**) |
| `space_tree_nodes`: categories, text chats, voice rooms | Markdown, pins (**6**) |
| Invites: link, expiry, max uses | Verification on join (капча, screening) — backlog spaces.md |
| Built-in roles Owner→Guest | Custom roles granular (**10**) |
| Kick, ban, timeout mute, slow mode | Global moderation reports (**11**) |
| Client: space sidebar, tree, voice join | Push (**6**) |
| Text in space channels — **plain text** как в DM | @mentions (**6**) |

### Уровень сервисов

| Сервис | Уровень |
|--------|---------|
| Space | **Минимальный** — spaces, tree, invites, member join/leave |
| Role | **Минимальный** — 5 built-in roles, permission checks for mod actions |
| Chat | `type=group|channel` linked to space; space channels |
| Voice | Persistent `voice_rooms` in space tree |
| Search | Публичный каталог спейсов — может быть stub до **9** |

### Бэкенд

- [x] **Спейс** — create, update icon/description, visibility public/invite/private.
- [x] **Дерево** — `space_tree_nodes`: `text_chat`, `voice_room`, `category`; sort order; CRUD + reorder.
- [x] **Инвайты** — token, expires_at, max_uses, revoke; join by link.
- [x] **Роли** — Owner, Admin, Mod, Member, Guest; assign on join default Member.
- [x] **Модерация** — kick, ban (`space_bans`), timeout (`space_member_timeouts`); slow mode per channel; `MuteChat` proto может быть stub если timeout покрывает MVP.
- [x] **Gateway routes** для Space + Role gRPC.

### Клиент

- [x] Space list sidebar, switch active space.
- [x] Channel tree expand/collapse, select text channel → chat room (plain).
- [x] Voice room: join/leave, participant strip, speaking indicator.

### Критерии приёмки

1. Create space → add text channel + voice room → second user joins via invite → both chat and voice work.
2. Mod kicks user → user loses access to space channels.
3. Slow mode blocks rapid sends server-side.
4. Integration tests space membership + role checks.

**Результат:** можно «жить» в спейсе; текстовый чат канала — без форматирования и пинов (сознательно до Фазы 6).

---

## Фаза 6 — Спейсы: чат и FCM

**Цель для пользователя:** нормальный чат в каналах/группах спейса (markdown, @mentions, пины) + push на Web/Android когда офлайн.

### Границы фазы

| Входит | Не входит |
|--------|-----------|
| Markdown subset — [text-chat.md](features/text-chat.md) | Полный markdown (спойлер, H1–H3) — дозакрыть по text-chat |
| @user, @everyone, @here с проверкой прав Role | iOS APNs (**8**) |
| Pins per channel | Thread defaults (**10**) |
| FCM Web + Android: register token, new msg, @mention | VoIP push (**8**) |
| Push grouping per chat | Granular notification settings per channel — backlog notifications.md |
| Notification Service **минимальный** | Email event notifications |

### Уровень сервисов

| Сервис | Уровень |
|--------|---------|
| Messaging | Markdown storage/render hints, mentions parsing, pins |
| Role | Permissions `TEXT_CHAT_MENTION_ALL_*` |
| Notification | **Минимальный** — FCM sender, token registry, consumer `message.events`, enriched body via User/Messaging gRPC |
| Chat | Mention guard (membership) |

### Бэкенд

- [x] **Markdown** — bold, italic, code, blockquote, links; server stores raw; client renders.
- [x] **Упоминания** — parse `@userId`, `@everyone`, `@here`; WS `mention` event online; offline → Notification.
  - @here = online in **this text chat** only; requires `TEXT_CHAT_MENTION_ALL_ONLINE`.
  - @everyone = all chat members; requires `TEXT_CHAT_MENTION_ALL_IN_CHAT`.
- [x] **Пины** — PinMessage, ListPins, max pins per policy; permission mod+.
- [x] **FCM** — [notifications.md](features/notifications.md): RegisterDeviceToken; push on new message (muted chats respect settings v1); @mention breaks mute default.
- [x] **Группировка push** — collapse key per `chat_id`; update existing notification.

### Клиент

- [x] Markdown composer + renderer in space channels.
- [x] Mention autocomplete @.
- [x] Pinned messages panel.
- [x] FCM web SW + Android firebase; tap → open chat.
- [x] Permission errors for @everyone/@here shown inline.
- [x] FCM push delivery proof on compose (`phase6_fcm_delivery_e2e_live_test`, notification debug recorder).

### Критерии приёмки

1. @mention offline user → FCM received with sender name + excerpt.
2. @everyone without permission → server error, no fan-out.
3. Pin visible to all channel members after reload.
4. Live E2E optional: `phase6_space_channel_mentions_e2e_live_test`.
5. Staging Firebase optional but documented in DEPLOYMENT.

**Результат:** спейс «готов» на web/Android; iOS push — Фаза 8.

---

## Фаза 7 — Матчмейкинг

**Цель для пользователя:** найти тиммейтов по игре/рангу/региону; принять матч → временный squad с войсом и чатом.

### Границы фазы

| Входит | Не входит |
|--------|-----------|
| **Глобальный** MM only | Space-level MM (после глобального) |
| Каталог: browse + **4 seed games** | User-submitted game constructor + moderation queue |
| Player profile: games, rank, roles, region | Rank verification Steam/Riot |
| Queue + matchmaking algorithm (exact/range criteria) | Cross-region match |
| Match squad: temp voice + chat | Permanent squad |
| Accept/decline popup; party reset rules | Stories LFG (**17**) |
| 30 min timeout; 15 min nudge | |
| Post-match 1–5 stars + MM-only ban | |
| Match history | |

### Уровень сервисов

| Сервис | Уровень |
|--------|---------|
| Matchmaking | **Минимальный** — catalog, queue, matcher, squad lifecycle, rating, history, MM ban |
| Chat + Voice | S2S create ephemeral squad `chat_id` + `voice_room_id` |
| Notification | FCM `match_found` |

### Бэкенд

- [x] **Каталог игр** — [game-catalog.md](features/game-catalog.md), [matchmaking.md](features/matchmaking.md)
  - **Объём:** `ListGames`, `GetGame`; JSON schema modes/roles/ranks; **4 seeds** (Dota 2, CS2, Valorant, PUBG) в migration/seed.
  - **Не делать:** user game constructor, admin moderation UI (можно SQL seed + API read).
  - **Готово когда:** клиент browse/search catalog; select game → queue form fields from schema.
  - **Тесты:** catalog CRUD read integration.

- [x] **Профиль игрока** — per-game rank/roles self-reported; store in `matchmaking_db`.

- [x] **Очередь** — Enqueue/Leave; criteria per game schema; one active queue per user; party from voice snapshot.

- [x] **Подбор** — matcher forms squads; on all accept → CreateSquad (Chat+Voice); WS + FCM `match_found`.
  - **Готово когда:** 2 parties matched → popup → accept → voice+chat joinable; decline rules per matchmaking.md.
  - **Тесты:** matcher unit tests; live E2E with FCM optional flag.

- [x] **Таймаут** — 30 min stop; 15 min «долго ищем» notification.

- [x] **Рейтинг** — 1–5 on squad leave; optional skip; MM ban on 1–2 stars flow.

- [x] **История** — list past squads + participants; add friend / MM ban from history.

### Клиент

- [x] MM panel (desktop right / mobile tab): catalog, queue form, searching state.
- [x] Match found modal accept/decline.
- [x] Squad voice UI (reuse voice room).
- [x] Live E2E FCM match_found (device registration + `phase7_match_fcm_e2e_live_test`; delivery via `NOTIFICATION_RECORD_PUSHES` on compose).

### Критерии приёмки

1. Two users queue same game/region/criteria → matched → squad voice works.
2. Party member decline resets whole party search.
3. Opponent decline → continue search.
4. MM ban prevents future match pairing only (not DM block).

**Результат:** ключевое УТП; открытые пункты: полный каталог UX + live FCM E2E.

---

## Фаза 8 — Мобильные клиенты и push

**Цель для пользователя:** тот же Voice на Android/iOS/Windows; офлайн-история; iOS push и входящие звонки.

### Границы фазы

| Входит | Не входит |
|--------|-----------|
| Flutter Android + iOS builds CI | macOS/Linux desktop priority |
| APNs alert push | Rich notification extensions |
| VoIP PushKit + CallKit **scaffold** | Полный prod VoIP без credentials |
| FCM mobile polish (enriched body, tap→chat) | Shorebird OTA (**updates.md** backlog) |
| SQLite offline cache last N msgs read-only | Full offline write/sync |
| Windows desktop + auto-updater | Game overlay |

### Уровень сервисов

| Сервис | Уровень |
|--------|---------|
| Notification | APNs sender path; VoIP push type separate |
| Client | Platform channels, secure token storage |

### Бэкенд

- [x] **APNs** — Notification Service: device token type `apns`; sandbox + prod config; message + mention payloads (код готов; sandbox delivery — секреты staging).
  - **Готово когда:** staging iOS device receives push; documented entitlements + Apple keys in DEPLOYMENT.
  - **Не путать с:** FCM on iOS if used as fallback — primary per notifications.md is APNs.

- [x] **VoIP** — PushKit incoming call payload; client CallKit UI; requires VoIP cert (scaffold + token registration E2E).
  - **MVP фазы:** scaffold compiles; E2E with prod creds may remain staging-only if documented.

- [x] **FCM mobile** — token refresh; enriched body via pushenrich resolver; deep link to chat route.

### Клиент

- [x] Responsive mobile layout (not just stretched web).
- [x] `flutter build apk/appbundle`; `flutter build ios --no-codesign` in CI.
- [x] SQLite cache: last N messages per chat, read on startup offline.
- [x] Windows installer + version_update_launcher / auto-updater — [updates.md](features/updates.md).

### Критерии приёмки

1. Android APK installs and completes Phase 1–6 smoke manually.
2. iOS build CI green; APNs on staging device (when creds present).
3. Offline: open chat → see cached history; send queued or blocked with clear UX.
4. Windows updater detects new version from `/api/v1/version`.

**Результат:** мультиплатформа; iOS push/VoIP may be staging-gated but code path complete.

---

## Фаза 9 — Поиск

**Цель для пользователя:** найти сообщение в чате или глобально; найти пользователя и публичный спейс.

### Границы фазы

| Входит | Не входит |
|--------|-----------|
| PostgreSQL tsvector/GIN v1 | Meilisearch/ES (**v2+**) |
| Search in chat + global messages | E2E encrypted DM content (**15**) |
| User search (existing SearchProfiles + search svc) | Verified badge boost (**13**) |
| Public space catalog search | Federated space index (**19**) |

### Уровень сервисов

| Сервис | Уровень |
|--------|---------|
| Search | **Минимальный** — indexer consumer `message.events`, query API |
| Messaging | S2S fetch message for context/snippet |

### Бэкенд

- [x] **Полнотекст** — [search.md](features/search.md): index on send/edit; `SearchInChat`, `SearchGlobal`; pagination.
- [x] **Reindex** — admin/repair RPC or job for backfill.
- [x] **Users and spaces** — `SearchUsers`, `SearchPublicSpaces`; pg_trgm optional for fuzzy names.

### Клиент

- [x] In-chat search field → results jump to message.
- [x] Global search from shell: tabs messages / users / spaces.

### Критерии приёмки

1. Send message → searchable within seconds in-chat.
2. Global search finds message across chats user is member of.
3. Cannot search chats user is not in.
4. Integration test indexer + query.

**Результат:** продукт масштабируется по истории; Search Service не stub.

---

## Фаза 10 — Глубина сообщества

**Цель для пользователя:** треды в каналах, screen share в войсе, медиа-вкладки в инфо чата, кастомные роли с оверрайдами.

### Границы фазы

| Входит | Не входит |
|--------|-----------|
| Threads — reply chains / thread channel mode | Forum channels advanced backlog |
| Screen share до 3 streams | System audio on web (platform limit) |
| Shared media tabs in chat info | Server-wide audit log UI (space backlog) |
| Custom roles 32+ permissions | Bot role assignments (**16**) |
| Overrides per `chat_id` and voice room | Commander broadcast mode (voice-chat backlog) |

### Уровень сервисов

| Сервис | Уровень |
|--------|---------|
| Messaging | Threads: `parent_message_id`, thread metadata, list threads |
| Role | **Полный для community** — custom roles, permission bitmap, chat_overrides, voice_room_overrides |
| Voice | Screen share tracks via LiveKit |
| Chat | Channel setting: thread-first default |

### Бэкенд

- [x] **Треды** — create thread from message; channel default thread-oriented; group configurable.
- [x] **Screen share** — [screen-share.md](features/screen-share.md); max 3 simultaneous; LiveKit screen track.
- [x] **Shared media** — aggregate attachments/links query per chat_id.
- [x] **Кастомные роли** — [roles.md](features/roles.md); Role Service CRUD; assign members; override sheets in client.

### Клиент

- [x] Thread panel / side view; post in thread.
- [x] Screen share button in voice; viewer tiles.
- [x] Chat info → Media / Files / Links tabs.
- [x] Role editor + per-channel override UI.

### Критерии приёмки

1. Channel default: user cannot post to main lane without permission; thread create works.
2. 3 users screen share capped at 3.
3. Custom role denies `TEXT_CHAT_SEND` in one channel only (override).
4. Role changes propagate via WS within seconds.

**Результат:** крупные сообщества комфортны; Role Service production-grade for space admins.

---

## Фаза 11 — Доверие (базовая)

**Цель для пользователя:** пожаловаться на контент; включить 2FA; настроить кто видит профиль.

**Блокер для Фазы 12** (платежи).

### Границы фазы

| Входит | Не входит |
|--------|-----------|
| Report UI + API + queue in DB | Moderator admin panel (**14**) |
| Auto-mod thresholds (basic) | Shadow ban sophistication (**14**) |
| 2FA TOTP enroll/verify | SMS 2FA |
| Privacy presets + per-field | Federation privacy |

### Уровень сервисов

| Сервис | Уровень |
|--------|---------|
| Moderation | **Минимальный** — SubmitReport, list queue (internal API only), store `moderation_db` |
| Auth | TOTP 2FA enroll, challenge on login |
| User | Privacy settings on profile fields |

### Бэкенд

- [x] **Репорты** — [reports.md](features/reports.md)
  - **API:** один gRPC `CreateReport` + `target_type` (`user` | `message` | `space` | `story`); HTTP `POST /api/v1/moderation/reports` → 202 Accepted (см. [reports.md](features/reports.md), [moderation-service.md](microservices/moderation-service.md)).
  - **Категории:** spam, harassment, offensive, fake, mm_toxic, other+comment.
  - **БД:** reports table, status enum, target reference.
  - **Не делать:** moderator UI, sanctions execution (**14**); auto-block may log only in **11**.
  - **Готово когда:** user submits → 202 + «принята»; row in DB; no status updates to reporter.
  - **Тесты:** submit integration; authz (cannot report self).

- [x] **2FA** — [auth-service.md](microservices/auth-service.md): TOTP secret, backup codes, require on login when enabled.

- [x] **Приватность** — [privacy.md](features/privacy.md): who can DM, who sees online/game/MM/phone/stories; presets; avatar/bio пока без privacy-контролов (все видят); runtime gates incl. phone-hash sync (Auth S2S) и group attachment policy.

### Клиент

- [x] Report flow from message context menu, profile, space.
- [x] Security settings: enable 2FA, show QR.
- [x] Privacy settings screen.

### Критерии приёмки

1. Report message → stored with category + reporter id.
2. 2FA enabled → login requires TOTP.
3. Privacy «friends only DM» blocks stranger message at Chat API.

**Результат:** минимальная безопасность перед монетизацией; модерация обработки — Фаза 14.

---

## Фаза 12 — Монетизация: биллинг

**Цель для пользователя:** купить Premium или Space Pro; получить лимиты и косметику.

**Требует закрытую Фазу 11.**

### Границы фазы

| Входит | Не входит |
|--------|-----------|
| Subscription Service **минимальный** | Инапп на mobile stores (use Paddle/CP web) |
| Premium + Space Pro entitlements | One-off username shop |
| Paddle + CloudPayments webhooks | Apple/Google IAP |
| Grace period 7 days | Trial period |
| Enforce limits: file 200MB, profiles 5, space members 5000 | CDN priority streaming |

### Уровень сервисов

| Сервис | Уровень |
|--------|---------|
| Subscription | **Минимальный** — products, checkout session, webhook, entitlements cache, NATS `subscription.events` |
| User | Premium flag, profile count limit |
| Space | Space Pro flag, member cap enforcement |
| File | Upload size check against entitlement |

### Бэкенд

- [ ] **Premium** — [subscription.md](features/subscription.md): status on account; animated avatar MIME; banner; 200MB upload; 5 profiles max.
- [ ] **Space Pro** — 5000 members, 128 voice, 500 channels, custom emoji.
- [ ] **Платежи** — Paddle (non-CIS), CloudPayments (RU); recurring; webhook signature verify.
- [ ] **Годовая** — −20% price SKU.

### Клиент

- [ ] Subscription settings, checkout redirect, manage subscription link.
- [ ] Premium badge in chat name.
- [ ] Downgrade profile picker when subscription ends.

### Критерии приёмки

1. Test webhook activates Premium → upload 100MB succeeds, 250MB rejected.
2. Space Pro raises member cap; cancel allows existing members, blocks new over free cap.
3. Grace period: features work 7 days after failed payment.

**Результат:** revenue stream; freemium base unchanged.

---

## Фаза 13 — Профили и верификация

**Цель для пользователя:** несколько «личностей» на аккаунте; значки верификации для публичных фигур и организаций.

### Границы фазы

| Входит | Не входит |
|--------|-----------|
| Multi-profile: 2 free / 5 premium | Per-profile separate billing |
| Switch profile in client | Profile merge |
| Verification badges + search boost | Manual verification admin at scale (**14** tooling) |

### Уровень сервисов

| Сервис | Уровень |
|--------|---------|
| User | **Расширенный** — multi-profile CRUD, active profile session, isolated friend lists per profile |
| Auth | Profile switch token claims `profile_id` |

### Бэкенд

- [ ] **Мульти-профиль** — [multi-profile.md](features/multi-profile.md): separate contacts, spaces, DMs per profile; limit enforced via Subscription.

- [ ] **Верификация** — [verification.md](features/verification.md): OAuth Twitch Partner / YouTube YPP; org DNS TXT; badge in UI; search ranking weight.

### Критерии приёмки

1. Two profiles → different display names in different spaces; no cross-leak in list APIs.
2. Verified badge visible in chat + search results ordered higher.

**Результат:** идентичность и доверие, не только косметика Premium.

---

## Фаза 14 — Модерация платформы

**Цель для команды модераторов:** разбирать очередь жалоб, применять санкции, аудит.

Граница с **11**: там submit; здесь process.

### Границы фазы

| Входит | Не входит |
|--------|-----------|
| Moderation admin web `src/admin/` | In-app mod tools for space admins (already **5**) |
| Auto-mod: thresholds, shadow ban | ML classifiers |
| Sanctions: warn, temp ban, perm ban | Appeals workflow UI (API stub OK) |
| Separate queues: users/messages vs public spaces | Story moderation if **17** not done |

### Уровень сервисов

| Сервис | Уровень |
|--------|---------|
| Moderation | **Полный** для platform mod |
| Auth | Ban account flag enforcement Gateway-wide |

### Бэкенд

- [ ] **Авто-мод** — [reports.md](features/reports.md): 1% threshold min 10/24h; shadow ban; spam pattern mute.

- [ ] **Панель модераторов** — React admin: queue list, filters, assign, sanction actions, audit log export.

### Критерии приёмки

1. Moderator resolves report → user sanction → Gateway rejects JWT or marks shadow banned.
2. Space reports queue separate from user reports.
3. All actions audited with actor id.

**Результат:** модерация выдерживает рост аудитории.

---

## Фаза 15 — E2E в DM (опционально)

**Цель для пользователя:** opt-in сквозное шифрование в личке.

### Границы фазы

| Входит | Не входит |
|--------|-----------|
| Signal Protocol DM opt-in per chat | E2E groups/space |
| Key exchange via server (encrypted envelopes) | Full Olm/Megolm spaces |
| Search excludes E2E ciphertext | Server-side message content for E2E |

### Уровень сервисов

| Сервис | Уровень |
|--------|---------|
| Messaging | E2E flag on chat; store ciphertext only when enabled |
| Client | libsignal or equivalent; key backup UX minimal |

### Бэкенд

- [x] **Signal Protocol** — [encryption.md](features/encryption.md): pair-wise sessions; disable server search index for E2E messages; key backup in Auth (`e2e_key_backups`).

### Критерии приёмки

1. Two users enable E2E → server DB shows non-plaintext payload.
2. Global search does not return E2E message body.
3. Opt-out reverts to plain with clear UX warning.

**Результат:** privacy option без ломки продукта для остальных.

---

## Фаза 16 — Боты

**Цель для разработчиков:** бот в спейсе со slash-командами.

### Границы фазы

| Входит | Не входит |
|--------|-----------|
| Developer Portal (может отдельный deploy) | Bot marketplace |
| Bot Service: manifest, slash, webhook | Socket mode gateway |
| Polling mode for local dev | Full OAuth for users |
| Scopes + per-channel whitelist | Bot reads all messages without scope |

### Уровень сервисов

| Сервис | Уровень |
|--------|---------|
| Bot | **Минимальный** — register app, manifest validate, interaction dispatch, 3s timeout |
| Gateway | Public bot webhook route with signature |

### Бэкенд

- [x] **Developer Portal** — apps, secrets, manifest upload, revoke token — [bots.md](features/bots.md). Minimal: [`src/developer-portal/`](../src/developer-portal/).

- [x] **Runtime** — slash commands in client; webhook + polling dev; rate limits per bots.md; TEXT_CHAT_SEND in whitelisted channels only.

### Критерии приёмки

1. Register bot → add to space → `/ping` returns pong in channel.
2. Ephemeral response visible only to caller.
3. 3s timeout shows user-friendly error.

**Результат:** сторонние интеграции в спейсах на compose/staging; prod — `voice-bot` manifest и gateway upstream есть, rollout/mTLS/webhook E2E и k8s Developer Portal — частично ([DEPLOYMENT.md](DEPLOYMENT.md), [TODO.md](TODO.md) Batch 4).

---

## Фаза 17 — Сторис

**Цель для пользователя:** stories 24h, highlights, LFG story type для матчмейкинга.

### Границы фазы

| Входит | Не входит |
|--------|-----------|
| Story Service **минимальный** | TikTok-style algorithmic feed |
| Photo/video/text stories, editor basics | AR filters |
| 24h expiry, 30d archive | Permanent posts |
| Highlights on profile | |
| «Ищу пати» story type (not MM queue) | |

### Уровень сервисов

| Сервис | Уровень |
|--------|---------|
| Story | **Минимальный** — CRUD, views, reactions, replies |
| File | Media upload for stories |

### Бэкенд

- [ ] **Контент + редактор** — [stories.md](features/stories.md): R2 media; overlays text/stickers v1 minimal.
- [ ] **Лента** — friends+space ring on avatar; views count; replies as DM thread optional.
- [ ] **Highlights** — curated collections on profile.
- [ ] **ММ связь** — «Ищу пати» structured fields link to game catalog display only.

### Критерии приёмки

1. Post story → visible to friends 24h → expires.
2. Highlight persists on profile after expiry.
3. Report story flows to Moderation (**11/14**).

**Результат:** контентный слой удержания + LFG discovery.

---

## Фаза 18 — Рост и доступность

**Цель для пользователя:** зайти по ссылке, пройти онбординг, пользоваться с клавиатуры и screen reader.

### Границы фазы

| Входит | Не входит |
|--------|-----------|
| Deep links universal links | Full marketing site |
| Onboarding 4–5 steps | Interactive tutorial gamification |
| WCAG-oriented a11y pass on main flows | Certified compliance audit |

### Бэкенд

- [x] **Deep links** — [deep-links.md](features/deep-links.md): `voice://` + `https://voice.gg/invite|space|chat|user|message`; Gateway redirect rules.

### Клиент

- [x] **Онбординг** — [onboarding.md](features/onboarding.md): 5 contextual coach-mark шагов (save account → chats → spaces → MM → wrap-up); канон vs PLAN one-liner — см. [TODO.md](TODO.md) Batch 5.
- [x] **A11y (baseline)** — focus order, Semantics labels, contrast tokens, reduced motion на основных потоках; полный чеклист [accessibility.md](features/accessibility.md) (TalkBack/VO, prod universal links) — [TODO.md](TODO.md) Batch 6.

### Критерии приёмки

1. Click invite link on mobile → app opens join flow (prod AASA/`assetlinks.json` — Batch 6).
2. Main navigation operable keyboard-only.
3. `flutter test` semantics tests for login + chat shell.

**Результат:** меньше трения для новичков; a11y — baseline, не полное закрытие accessibility.md.

---

## Фаза 19 — Федерация и self-hosting

**Цель для оператора:** поднять federated node; хостить спейсы на своём железе; оставаться в глобальном каталоге.

### Границы фазы (V1 federation)

| Входит | Не входит |
|--------|-----------|
| S2S gRPC bidi stream master↔node | Open node registration |
| Node Docker image | Accounts on node |
| Auth always via master | DM on node |
| Role/ban sync | Local MM on node |
| Defederation | Cross-server DM |
| Push: node→master→FCM | |

### Уровень сервисов

| Сервис | Уровень |
|--------|---------|
| Federation | **Минимальный V1** — [federation.md](features/federation.md) |
| Space/Voice/File on node | Same stack, `NODE_MODE=federated` |

### Бэкенд

- [ ] **S2S API** — `protos/voice/s2s/v1/s2s.proto`: RegisterNode, AuthUser, stream events.
- [ ] **Нода** — Docker compose/k8s manifest; env for master URL + node cert.
- [ ] **Синхронизация** — RoleChanged, UserBanned snapshot on reconnect.
- [ ] **Дефедерация** — master marks node banned; catalog hides spaces.
- [ ] **Уведомления** — FederationService.NotifyUser → master Notification.

### Критерии приёмки

1. Node hosts space → appears in master catalog → user auth via master joins.
2. Ban on master → node rejects within TTL/stream latency.
3. Master down → node serves cached members only; no new users.

**Результат:** Voice на своей инфраструктуре без форка продукта.

