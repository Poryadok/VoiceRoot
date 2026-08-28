# Voice — план исполнения

> Каталог фич — [FEATURES.md](FEATURES.md). Инвентарь пробелов — [TODO.md](TODO.md) и `docs/todo/`. Здесь — **честный статус кода** и **порядок работ**. Федерация вне плана.

Сверка спека↔код: 2026-08-17 (`tmp/feature-audit/synthesis.md`). Мелкие PR в `feature/` / `fix/` от `master` — [CONTRIBUTING.md](CONTRIBUTING.md).

---

## Продуктовый scope

**Текущий scope:** все фичи из [FEATURES.md](FEATURES.md) — **current**, реализуются **сейчас**. Единственное исключение — [federation.md](features/federation.md) (**deferred**).

**Федерация — deferred.** Спека и scaffold `src/backend/federation/` не трогать, пока рынок явно не попросит. Не планировать RPC, `federation_db`, Gateway upstream, Admin UI нод. См. также federated search, federated file storage, federation analytics/moderation в соответствующих feature docs.

**Спека ≠ shipped.** UX может быть в спеке и макетах, а бэкенд/Flutter — `partial`. Не писать `shipped`, если код не закрывает DoD из `docs/features/*`.

### Матрица Telegram-parity (спека vs код)

| Фича | Спека | Код (2026-08-28) | Пробел |
|------|-------|------------------|--------|
| Folders (system + custom) | current | proto CRUD only | DDL/handlers, Flutter rail |
| Quick Access (≤15, per profile) | current | нет RPC/DDL | Chat RPC + Flutter |
| Archive (`is_archived`) | current | write ✓ | `ListChats` исключает archived |
| List archived / unarchive UX | current | нет list/filter RPC | Chat RPC + archive screen |
| Pin messages (≤**5**) | current | RPC ✓, лимит **50** | выровнять — [todo/backend.md](todo/backend.md) |
| Send silent / schedule / when online | current | нет полей/RPC/таблицы | `scheduled_messages` + composer |
| List preview ✓/✓✓ | current | preview text only | delivery state в list metadata |
| Стикеры/GIF | current | 0 кода | паки + composer picker |

**IA (решено):** Quick Access — shortcuts `chat_id` per profile (≤15), не pin чата и не Social favourites. Folders — в left rail между nav-кнопками и аватарами профилей. Archive — вход через RC меню аватара профиля (не строка списка).

---

## Как читать и резать работу

| Документ | Роль |
|----------|------|
| Этот файл | Последовательность и параллельные треки |
| `docs/todo/*.md` | Полный инвентарь; не копировать сюда |
| `docs/features/*.md` | Критерии приёмки фичи |

- Один PR — одна смысловая задача ([CONTRIBUTING.md](CONTRIBUTING.md)). Пункт может закрываться несколькими PR.
- Независимые сервисы двигаются **параллельно**. Не ждать «весь пункт», если DoD соседнего трека не нужен.
- Auth — **Java**. Realtime — **WS** (`s` / `resume`). Догрузка истории — **REST Messaging per `chat_id`**, не «догнать всё по WS».
- Node в CI/фронте — **24**.
- **Вы** = секреты, DNS, аккаунты провайдеров. Агент не выдумывает продуктовое поведение: дыра в спеке → человек ([product-roadmap.md](todo/product-roadmap.md)).

---

## Честный статус

`shipped` = ядро ходит в compose/Flutter live. `partial` = ядро есть, Critical/High хвост. `stub` = happy path фейковый. `baseline` = минимум a11y. Не путать live-тест «ручка живая» с полнотой спеки.

| Фича | Статус | Сервисы | Что реально / чего нет |
|------|--------|---------|------------------------|
| [auth-and-contacts](features/auth-and-contacts.md) | partial | Auth (Java), Gateway | Email/guest/sessions/reset **REST есть**. Flutter: нет телефона/OTP, sessions, delete-account. Convert-guest ядро shipped. |
| [friends](features/friends.md) | shipped | Social, User | Запросы/блок/DM-гейт живые. Нет REST list contacts/favorites, QR, phone-book UI. |
| [text-chat](features/text-chat.md) | partial | Chat, Messaging, Realtime | DM/группы/треды/markdown/@mentions live. **Пины:** RPC есть, лимит **50** в коде vs **5** в спеке ([todo/backend.md](todo/backend.md)). **Без кода:** folders, Quick Access, list archive, send menu (silent/schedule), стикеры/GIF. Нет `DeleteChat`, view-count. |
| [forward-messages](features/forward-messages.md) | shipped | Messaging | Attribution / copy-as-new / commentary live. |
| [presence](features/presence.md) | partial | User, Realtime | REST presence + WS в общем чате. Нет idle 5 мин, game detect, live presence друзьям вне чата. |
| [user-profile](features/user-profile.md) | partial | User, File | Аватар/био/switch live. GIF-аватар отвергается; `banner_url` не в proto. |
| [voice-chat](features/voice-chat.md) | partial | Voice, Role | 1:1 / group / space join, commander/raise-hand live. `MoveToVoiceRoom` Unimplemented; `VOICE_SPEAK`/`MUTE` не на speak/mute. |
| [file-storage](features/file-storage.md) | partial | File | Upload/R2/retention/SHA verify live. **Нет** SHA-256 dedup, ffmpeg GIF/video/PDF; «WebP» = JPEG. |
| [spaces](features/spaces.md) | partial | Space, Role | Create/Join/Leave/tree/инвайты live. **Нет** `DeleteSpace` / `TransferOwnership` / `GetAuditLog` / каталога / шаблонов. Owner не может leave. |
| [roles](features/roles.md) | shipped | Role | Кастомные роли, send-deny, `VOICE_JOIN` deny live. Часть TEXT_CHAT_* / verification roles — High. |
| Групповые чаты | partial | Chat, Role | Create/kick/min-size/mute/archive live. **Read-state — DM-only** (`read_receipts` per chat); group/channel unread неполный. **Каналы** не в `ListChats` filter. |
| Треды, shared media | shipped | Chat, Messaging | Live E2E есть. |
| Markdown, @mentions | shipped | Messaging, Chat | Live E2E есть. |
| Пины сообщений | partial | Messaging, Chat | Pin/unpin/list live. **Лимит 50** (`MaxPinsPerChat`); спека = **5** — [todo/backend.md](todo/backend.md). |
| [notifications](features/notifications.md) | partial | Notification | Quiet hours в БД; FCM/APNs compose с mock. Прод: noop без секретов + **имена env не совпадают**. |
| [matchmaking](features/matchmaking.md) | partial | Matchmaking | Поиск/space-queue/LFP live. `PartyStore` stub (`partySize=1`); П.2 постматч открыт; `mm_ban` не S2S в MM. |
| [game-catalog](features/game-catalog.md) | shipped | Matchmaking | Seed + `SubmitGameRequest` + admin moderation live. Staff Add-game UI тонкий. |
| [search](features/search.md) | shipped | Search | Global/in-chat live, ACL пересечение есть. `ReindexChat` индексирует E2E ciphertext; нет historical backfill. |
| [screen-share](features/screen-share.md) | shipped | Voice | API/E2E есть. Desktop picker / system audio — Common. |
| [reports](features/reports.md) | partial | Moderation, Auth | Create report + perm_ban live. Нет user appeals HTTP/UI; Admin не resolve/dismiss; shadow-ban не режет fanout. |
| [privacy](features/privacy.md) | partial | User, Social | Настройки и DM-гейт live. Social/MM/Voice **fail-open**, если S2S client nil. |
| [subscription](features/subscription.md) | **stub** | Subscription, Auth | Space Pro sync/member-cap live на seed/webhook. Checkout = `checkout.paddle.test`; CloudPayments Unimplemented; JWT tier=`free` без `AUTH_NATS_URL`. |
| [multi-profile](features/multi-profile.md) | partial | User, Auth | Create/switch live. Нет delete-profile UI, freeze в switcher, `SetPrimaryProfile`. |
| [verification](features/verification.md) | partial | User, Auth | Twitch mock-path; YouTube/DNS/cron неполные; OAuth пишет в User DB в обход `SetVerification`. |
| [encryption](features/encryption.md) | shipped (opt-in) | Messaging, Chat, Auth | E2E DM live. UX fingerprint / key-change — Common (П.13). |
| [bots](features/bots.md) | partial | Bot, Gateway | Slash live. Нет inbound `message.events` → webhook; Portal CSRF/manifest; `GetChatMessagesForBot` дырявый. |
| [stories](features/stories.md) | partial | Story | Create/feed/LFP ядро live. Editor v2 / audience JSON / anonymous NATS leak — Common/Low. |
| [deep-links](features/deep-links.md) | shipped | Gateway | Invite compose live. Prod AASA/assetlinks — placeholders (**Вы**). |
| [onboarding](features/onboarding.md) | shipped | User, Flutter | Coach-marks / guest flow live. FAQ vs UI — Common. |
| [accessibility](features/accessibility.md) | baseline | Flutter | Semantics + Axe analog в CI. Message-list keys, TalkBack — Common. |
| [platforms](features/platforms.md) | partial | Flutter | Web+Windows CI. Mobile push/universal links, Shorebird — High/Low. |
| [i18n](features/i18n.md) | shipped | Flutter | EN+RU baseline. Language sync — Common. |
| [navigation](features/navigation.md) | partial | Flutter, Chat | Shell есть (nav + profiles). **IA в спеке (current):** folders rail, **Quick Access (≤15 `chat_id`/profile)**, archive via profile RC — **нет в коде**; Chat RPC + Flutter backlog ([todo/backend.md](todo/backend.md)). См. матрицу Telegram-parity выше. |
| [updates](features/updates.md) | partial | Flutter, CI | Force-update/version checks частично. Shorebird — явный defer или PR. |
| [observability](features/observability.md) | partial | ops | Compose метрики есть. Staging DoD (Loki/Grafana/P1) открыт. |
| [analytics](features/analytics.md) | partial | Analytics, ClickHouse | Ingest + product dashboard. PII в `properties`; ack до CH write; Admin дашборды тонкие. |
| [federation](features/federation.md) | **deferred** | Federation (scaffold) | Вне плана. |

Группы/каналы в спейсе завязаны на Space+Chat+Role: создание дерева shipped, **каталог публичных спейсов — нет**.

---

## Топ-блокеры (не параллелятся «через них»)

| Блокер | Кого стопорит | Куда |
|--------|---------------|------|
| Секреты staging (FCM/APNs, TOTP, Resend, ClickHouse, Paddle, `AUTH_NATS_URL`) | Реальный push, почта, JWT tier, биллинг | [ci.md](todo/ci.md) Critical (**Вы**) |
| Имена env FCM/APNs ≠ код | Push останется noop даже с секретами | [backend.md](todo/backend.md) Notification + [ci.md](todo/ci.md) |
| Checkout stub `checkout.paddle.test` | Premium GIF/banner, 3-й профиль, File quota, П.12/П.16 | [backend.md](todo/backend.md) Subscription |
| JWT `subscription_tier=free` без NATS | User/Chat доверяют JWT, не Subscription S2S | Auth Java + ops `AUTH_NATS_URL` |
| Нет `TransferOwnership` / `DeleteSpace` | Owner не может leave; спейс нельзя закрыть | Space Critical |
| Alertmanager → null receiver | P1 алерты никто не видит | [ci.md](todo/ci.md) (**Вы**) |
| Стикеры/GIF в чате (0 кода) | Паки + composer | Current scope ([text-chat.md](features/text-chat.md)); пункт **2** ниже |

CloudPayments — отдельный трек после Paddle (СНГ), не блокер staging rollout, если Paddle живой.

---

## Порядок

Каждый пункт: цель → параллельные треки → зависимости → Definition of Done. Инвентарь — в `docs/todo/`, здесь только **нарезка**.

### 0 — Staging не врёт

**Цель:** staging/prod rollout без fail-open privacy, без InMemory tier, без noop push при заполненных секретах, с живыми алертами.

| Трек | PR-шаги (параллельно) | Зависимости |
|------|----------------------|-------------|
| **Ops (Вы)** | `voice-app-secrets`; `AUTH_NATS_URL`; `AUTH_TOTP_ENCRYPTION_KEY` (не `DEFAULT_DEV_KEY`); `RESEND_API_KEY`; FCM/APNs; Paddle live **или** не отдавать test URL наружу; `CLICKHOUSE_DSN` + `ANALYTICS_ID_HASH_KEY`; Alertmanager канал; DNS `app`/`admin`/`livekit` | человек |
| **Backend Notification** | Выровнять `FCM_CREDENTIALS_JSON` / `APNS_AUTH_KEY` с deploy; смонтировать `APNS_*` в `services.yaml` | merge до секретов; доставка — после **Вы** |
| **Backend Auth (Java)** | Staging/prod: запрет `DEFAULT_DEV_KEY`; NATS tier store при URL | `AUTH_NATS_URL` |
| **Backend Social / Voice / Space** | Fail-open → fail-closed, если User/Blocks/SpaceMembers nil; compose MM: `USER_GRPC_ADDR` / `SOCIAL_GRPC_ADDR` / `SPACE_GRPC_ADDR` | k8s addrs уже есть |
| **Backend Analytics** | HMAC вместо plaintext `account_id`; ack NATS **после** durable CH | ключ из **Вы** |
| **Backend Search** | `ReindexChat` не индексирует `IsE2E` | нет |
| **CI** | **done (#60)** — `scripts/staging/**` / `scripts/prod/**` в `global`; matrix tests lock; `ci-gate` требует full tier-1 при GLOBAL | — |
| **Observability** | Loki все поды; `smoke-request-id.sh`; Grafana Overview; Prometheus scrape; тестовый P1 в канал | Alertmanager (**Вы**) |
| **Admin** | OAuth `assign-to-me` из session JWT, не только `VITE_STAFF_TOKEN` | нет |

**DoD:** секреты в кластере; JWT tier не всегда `free`; Notification читает те же имена, что в Secret; privacy/join не обходятся nil-клиентом; P1 алерт не в null; deploy-script PR не проходит `ci-gate` вхолостую (**#60**).

**1** не ждать полного DoD **0**: Space/Admin/Flutter Auth не зависят от Paddle.

---

### 1 — Critical продукт

**Цель:** владелец спейса не залочен; модерация закрывает жалобы; апелляции доходят до пользователя; checkout больше не test-URL.

#### Параллельные треки (стартуют сразу)

| Трек | PR-нарезка | Не ждать |
|------|------------|----------|
| **A. Space lifecycle** | 1) `TransferOwnership` (+ Gateway + Flutter leave-as-owner). 2) `DeleteSpace`. 3) `GetAuditLog` (чтение уже пишущегося `audit_log`). `entry_requirement`: пока ≠ `none` — явный контракт, не молчаливый hard-fail без UX; captcha/queue — отдельные PR | Paddle, push |
| **B. Moderation loop** | `SubmitAppeal` пишет `account_id` (не profile); Gateway `POST /appeals`; shadow-ban → Messaging/Realtime `IsShadowBanned` на send/fanout; Notification consumer `moderation.events` | Space A |
| **C. Admin queue** | resolve / dismiss (не только `reviewing`); санкции на non-user target не через `target_id` как account | REST статусов из B, если их ещё нет |
| **D. Social** | REST contacts/favorites (gRPC уже есть); `SendFriendInvitation` чтит block | 0 fail-closed — желательно до/вместе |
| **E. Billing Paddle** | `CreateCheckoutSession` / space checkout → реальный Paddle API; не шипить `checkout.paddle.test`; webhook renew/cancel после первого live payment | секреты Paddle (0 **Вы**). Не ждать CloudPayments |
| **F. Flutter Auth UI** | телефон+OTP; sessions/revoke; delete-account; password-reset экраны (REST есть) | нет |
| **G. Flutter appeals** | экран апелляции профиля | Gateway из B |
| **H. Auth→User verification** | OAuth Twitch → `SetVerification` + `user.verified`, не прямой JDBC в `user_db` | нет |
| **I. DeleteAccount хвост** | tombstone в DM / hide из `ListChats` (RPC уже есть) | F UI может идти параллельно |

**Зависимости внутри 1:** A1 (`Transfer`) **перед** owner-leave. E **перед** любой работой «Premium включает GIF/banner/лимиты» (**2**). G после B.

**PR-порядок (scope не меняется):** пункт **1** — Critical lifecycle, модерация, billing. Folders, Quick Access (≤15), archive list, стикеры/GIF, pin limit **5** — **current scope** ([FEATURES.md](FEATURES.md)); первые треки пункта **2** (можно параллельно после DoD **0**, не ждать полного **1**). Каталог спейсов, шаблоны, CloudPayments, ffmpeg, party-from-voice, Developer Portal polish — тоже пункт **2**.

**DoD:** owner transfer + delete live; Admin закрывает репорт; user appeal REST+UI; checkout URL боевой (или скрыт за флагом, не test); shadow-ban не доходит до аудитории; compose/Flutter live на эти пути зелёные.

---

### 2 — High: entitlements, чат, файлы, войс, ММ

**Цель:** оплата даёт обещанное; чат без Unimplemented на delete/folders; файлы по спеке; войс-комнаты можно менять; ММ пати из войса.

| Трек | PR-шаги | Зависимости |
|------|---------|-------------|
| **Backend Subscription** | Cancel/Resume **в Paddle**; grace sweeper 7d; `subscription.events` (не только `analytics.*`); `CheckLimit` в Chat/User/File; `GetLimits` ближе к спеке | 1 E + JWT NATS (0) |
| **Backend User** | GIF-аватар + banner expose; Premium custom status gate; `SetPrimaryProfile` | JWT tier ≠ всегда free |
| **Backend File** | SHA-256 dedup + `file_references`; ffmpeg GIF→MP4 / video 720p / PDF thumb; `CheckQuota` чтит premium | entitlements; не ждать стикеры |
| **Backend Chat** | `DeleteChat`; folders RPC+миграция (`folders`, `folder_chats`); Quick Access RPC+миграция (`quick_access_chats`); `ListArchivedChats` / archive filter; `ListChats` каналы + group `last_message_at`; view-count (Messaging) | folders + Quick Access UI — Flutter следом |
| **Stickers/GIF** | системные паки + upload своих; send/receive first-class; composer picker; GIF как first-class (не только file attach). Lives TC-MSG-09 после контрактов | [text-chat.md](features/text-chat.md); File ffmpeg для GIF; ADR 005 |
| **Backend Voice** | `MoveToVoiceRoom`; enforce `VOICE_SPEAK` / `VOICE_MUTE_OTHERS`; roster NATS join/leave | Role client уже wired |
| **Backend Matchmaking** | Party snapshot из voice roster; leave/join сбрасывает очередь; `ApplySanction(mm_ban)` → `BanFromMM`; П.2 `RateTeammates` / history UI | Voice roster events |
| **Backend Space catalog** | `SearchPublicSpaces` или честный Search hydrator (не member-only `GetSpace`); templates — отдельные PR | не блокер entitlements |
| **Backend Bot** | inbound `message.events` → webhook/poll; `GetChatMessagesForBot`; ChatRef type | Portal (ниже) независим |
| **Flutter** | folders rail; Quick Access (≤15); archive screen (profile RC entry); delete chat; quiet hours API (не SharedPreferences SoT); multi-profile delete + frozen switcher; downgrade picker **после** lifecycle events | Subscription downgrade event |
| **Developer Portal** | OAuth `state`; JWT expiry; REST export manifest / `GetCommands` | нет |
| **Deep links (Вы + Gateway)** | iOS Team ID, assetlinks SHA; prod AASA на `voice.gg` | **Вы** |
| **CI High** | `staging-stack-lock` ждёт image jobs; migrate Job re-run; compose-e2e на Go-only master — решение, не молчаливый skip | нет |

**DoD:** платный аккаунт проходит GIF/banner/лимит профилей в live; `DeleteChat` + folders + Quick Access не Unimplemented; pin limit = **5**; File dedup/ffmpeg в compose; `MoveToVoiceRoom` live; MM пати >1 из войса; Portal не логинится без CSRF state.

---

### 3 — Common: UX, аналитика, сторис, a11y

**Цель:** дыры, которые не ломают Tier 0 (DM+WS), но видны пользователю.

| Трек | Примеры PR (параллельно) |
|------|--------------------------|
| **Flutter Chat UX** | локальные черновики; OG preview; idle → `UpdatePresence`; in-chat search next/prev; E2E fingerprint banner; bones placeholder |
| **Flutter a11y** | ↑/↓/R/E в списке сообщений; text-scale smoke на settings/MM/stories; focus return; TalkBack (**Вы**) |
| **Social / contacts** | phone-book UI + REST list (если не закрыто в 1D); Favorites |
| **Stories** | anonymous NATS не светит viewer; `media_file_id` ownership; GET reactions REST; audience JSON |
| **Analytics / Admin** | retention SQL; date range на Gateway; дашборды engagement/revenue/health; ingest DoD `message.sent` → CH <60s |
| **Notification** | grouping 1 push/chat; presence-aware MM/voice push; `reply` type |
| **Realtime** | bootstrap subscribe groups/spaces; friend presence WS; `REALTIME_INSTANCE_ID` в k8s. ~~`delivery_ack` Redis fanout~~ **shipped** (`redis_fanout.go`, compose `TestComposeDeliveryReceipts_live`) |
| **Design** | Penpot · v2 missing buttons (composer/header) → Flutter parity после approve |
| **Product (Вы)** | П.5/П.6 верификация ранга и Twitch/YouTube/DNS; П.13 E2E multi-device |

**DoD:** quiet hours синхронны между устройствами; analytics не врёт retention; stories anonymous не в NATS; a11y чеклист desktop закрыт без TalkBack (mobile — **Вы**).

---

### 4 — Low / post-MVP

Не смешивать с пунктами **0**–**2** (staging-critical path).

- Windows: tray, global PTT, game detect, overlay (П.17–18); Shorebird или явный defer.
- Stories editor v2 (stickers/doodle/trim).
- Resilience: circuit breaker в `pkg/grpcclient`, NATS DLQ (сейчас док врёт).
- Compose Postgres/Redis → целевые 18/8.
- Helm/GitOps — [ADR 004](adr/004-helm-gitops-deferred.md), не планировать.
- Distributed tracing — [ADR 003](adr/003-distributed-tracing-deferred.md).
- Federation — **не делать**.

---

## Параллелизация (сводка)

```
0:  Ops(Вы) ‖ Notification env ‖ Analytics HMAC ‖ Search E2E skip ‖ Admin OAuth assign  (CI filters — done #60)
1:  Space A ‖ Moderation B ‖ Admin C ‖ Social D ‖ Paddle E ‖ Flutter Auth F ‖ User verification H
         G (appeals UI) ← B
         Premium GIF/limits ← E + AUTH_NATS_URL
2:  File ‖ Chat delete/folders ‖ Voice move ‖ MM party ‖ Portal ‖ User GIF  (GIF ← billing)
3:  почти всё независимо по сервисам
```

Независимые Go-сервисы (Space vs Notification vs Search vs Analytics) — разные PR в одни и те же дни. Auth Java не отдавать Go-агенту.

---

## Локальный стенд

| Команда | Назначение |
|---------|------------|
| `make compose-up` | Infra: Postgres, Redis, NATS JetStream |
| `make compose-app-up` | Полный app stack + Flutter web ([README.md](../README.md)) |
| `make compose-migrate-all` | golang-migrate для Go-owned БД |
| `make compose-migrate-e2e` | DDL для E2E encryption (messaging + chat) |
| `make compose-e2e-smoke` | Smoke E2E по фичам ([e2e-features.yml](../.github/ci/e2e-features.yml)) |

**Staging:** k8s — [`deploy/staging/`](../deploy/staging/), `scripts/staging/render-and-apply.sh` ([DEPLOYMENT.md](DEPLOYMENT.md)).

---

## Размещение кода

| Сервис | Каталог | БД |
|--------|---------|-----|
| Auth | `src/backend/auth/` | `auth_db` (Flyway) |
| API Gateway | `src/backend/gateway/` | — |
| Realtime | `src/backend/realtime/` | Redis (WS; не Messaging catch-up) |
| Messaging | `src/backend/messaging/` | `messaging_db` |
| Chat | `src/backend/chat/` | `chat_db` |
| User | `src/backend/user/` | `user_db` |
| Social | `src/backend/social/` | `social_db` |
| Space, Role, Voice, File, Search, Matchmaking, Notification, Bot, Story, Subscription, Moderation | `src/backend/<service>/` | [DATA_STORES.md](DATA_STORES.md) |
| Analytics | `src/backend/analytics/` | ClickHouse; буфер **in-memory** (не Redis) |
| Flutter client | `src/frontend/` | — |
| Admin | `src/admin/` | moderation queue + product analytics (не «зарезервировано») |
| Developer Portal | `src/developer-portal/` | боты / OAuth staff apps |
| Federation | `src/backend/federation/` | scaffold; **не в работе** |

Целевая карта — [MICROSERVICES.md](MICROSERVICES.md).

---

## Верификация

| Проверка | Команда |
|----------|---------|
| Backend | `make build-all` |
| Flutter | `make flutter-ci` |
| Compose smoke E2E | `make compose-e2e-smoke` (`VOICE_RUN_LIVE_COMPOSE=true`) |
| Полный compose E2E | `make compose-e2e-live` |
| Контракты | `buf lint`, `buf format -d --exit-code` |

Каталог E2E — [TESTING.md](TESTING.md#e2e-по-фичам), [`.github/ci/e2e-features.yml`](../.github/ci/e2e-features.yml).

Новые Critical/High пути из 1–2 — **live-тест в том же PR**, не «потом в nightly».

---

## Миграции

Файлы в `src/backend/migrations/` именуются по домену фичи (`NNNNNN_name.up.sql`), без номеров из этого плана. Staging migrate Jobs сейчас **не** переезжают сами после первого success — см. [ci.md](todo/ci.md) High (2 CI).
