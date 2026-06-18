# Пробелы и открытые вопросы (документация)

Здесь — **вне дорожной карты** [PLAN.md](PLAN.md). Продуктовые и инфраструктурные чеклисты по фазам, остаток Фазы 0 (Auth: smoke контейнера в CI, порядок миграций `auth_db`), UUIDv7 для сообщений — в плане.

## Как пользоваться

| Метка | Кому | Смысл |
|-------|------|--------|
| **Agent batch** | Cursor / агент | Один PR или одна сессия: общий контекст, TDD, без ваших секретов |
| **Вы** | Человек | Решение, ключи, аккаунт, юридическое — агент ждёт ввод или не может закрыть |

**Порядок:** сначала закройте пункты в «Только вы», затем давайте агенту batch целиком (копируйте заголовок секции в промпт).

Агенту: удаляй отсюда все выполненные пункты.

---

## Только вы (не делегировать агенту целиком)

### Отложено (staging недоступен, локально — compose)

- [ ] **Buf Schema Registry** — аккаунт BSR, имя модуля, `BUF_TOKEN` в CI (после появления staging).
- [ ] **Реальные ключи dev** — File/R2/FCM: `.env.example` → `.env`, `FILE_R2_*`, `USER_R2_*` и т.д.

---

## Batch 1 — Docs sync (PLAN + DATA_STORES)

Сверка PLAN §11–§18 и `DATA_STORES.md` с кодом (аудит 2026-06-17). Не менять границы фаз.

- [ ] **PLAN.md** — сводная таблица и §11–§18 чеклисты:
  - **11** — строку фазы можно `[x]`; §11 чеклисты — после остатка privacy (см. Batch 2).
  - **15** — строку фазы и §15 backend `[x]` (критерии приёмки закрыты в коде).
  - **16** — строку `[ ]` до остатка ботов; §16 Developer Portal `[x]` завышен.
  - **18** — сводную `[ ]`; §18 `[x]` — baseline, не полное [accessibility.md](features/accessibility.md).
- [ ] **DATA_STORES.md** — добавить `e2e_key_backups` в инвентарь `auth_db` (Flyway `V4__e2e_key_backups.sql`, [auth-service.md](microservices/auth-service.md)).

**Промпт-якорь:** `Docs sync — PLAN.md and DATA_STORES from docs/TODO.md Batch 1`.

---

## Batch 2 — Phase 11 Trust (остаток)

Расхождения с [reports.md](features/reports.md), [privacy.md](features/privacy.md).

### Репорты

- [ ] **API shape** — один gRPC `CreateReport` + `target_type` вместо отдельных `ReportUser` / `ReportMessage` / `ReportSpace` (функционально ок; синхронизация docs-only).

### Приватность — enforcement

- [ ] **`allow_phone_search` / `allow_calls` / `allow_files` / `allow_voice_messages` / `allow_chat_space_invites`** — store+validate есть; полный gate: Calls/Space invite RPC, Messaging attachments MIME guard.
- [ ] **show_avatar / show_bio** — не в [privacy.md](features/privacy.md); PLAN §11 упоминает — doc gap.

**Промпт-якорь:** `Phase 11 Trust — privacy and reports from docs/TODO.md Batch 2`.

---

## Batch 3 — Phase 15 E2E (хвосты)

Критерии приёмки §15 закрыты; ниже — усиления и тесты.

- [ ] **Opt-out search hardening** — compose live не требует search hit (tier-2); при стабильном Search в CI усилить assert.
- [ ] **Matcher `IncludeGuests` only** — без `IsEveryoneShortcut` гости видят поле, но не strangers; edge cases guests-only.
- [ ] **Key backup Flutter live** — REST покрыт compose; нет opt-in `phase15_e2e_key_backup_live_test.dart`.

**Промпт-якорь:** `Phase 15 E2E follow-ups from docs/TODO.md Batch 3`.

---

## Batch 4 — Phase 16 Bots (остаток)

Критерии приёмки §16 (`/ping`→pong, ephemeral, 3s timeout) закрыты в коде.

- [x] **Bot Service staging deploy** — `voice-bot` в [`deploy/staging/services.yaml`](../deploy/staging/services.yaml), `"bots"` upstream, `BOT_DATABASE_URL` в secret.example; prod rollout отдельно.
- [x] **Bot gRPC mTLS v1** — `BOT_GRPC_GATEWAY_ONLY=true` в compose + staging; `x-voice-internal` от Gateway. Полный mTLS / k8s NetworkPolicy — prod hardening (ниже).
- [x] **Flutter BOT-C live** — `test/phase16_bots_botc_live_test.dart`, harness BOT-C helpers, `compose-e2e-live.sh`.
- [x] **TEXT_CHAT_CREATE_IN_SPACE 10/day** — live `TestComposePhase16BotsDailyChatCreateLimit_live`, `TestCreateBotChat_concurrentDailyLimit`, store concurrent upsert test.
- [x] **Developer Portal production OAuth** — секция в [`DEPLOYMENT.md`](DEPLOYMENT.md) (Auth env, portal build args, prod checklist).

### Phase 16 — остаток после Batch 4

- [ ] **Bot gRPC mTLS (prod)** — TLS между сервисами, k8s NetworkPolicy для `voice-bot:9090`.
- [ ] **Bot Service prod rollout** — образ + миграции `bot_db` в prod namespace; smoke webhook E2E на staging.
- [ ] **Developer Portal staging/prod k8s** — Deployment/Ingress для portal; Auth OAuth env на staging `voice-auth`.
- [ ] **Daily chat limit: increment-after-success** — счётчик растёт до `CreateChat`; failed create всё равно съедает квоту ([`bot_c.go`](../src/backend/bot/internal/grpcsvc/bot_c.go)).
- [ ] **Stale `src/backend/bot/README.md`** — всё ещё «scaffold only»; обновить или удалить.
- [ ] **Webhook E2E on staging** — polling-only в compose; production webhook path на staging не покрыт live-тестом.

**Промпт-якорь:** `Phase 16 bots from docs/TODO.md Batch 4`.

---

## Batch 5 — Phase 17 Stories

MVP backend + partial Flutter; пробелы vs [stories.md](features/stories.md) и PLAN §17. AR, algorithmic feed, post-match auto-story, monetization — out of phase.

### Backend — events & integrations

- [ ] **Expiry worker → `story.expired` NATS** — `jobs.StartExpiryWorker` вызывает только `MarkExpiredStories`; publish per row через `Events.PublishStoryExpired`.
- [ ] **`story_events_consumer` в Notification** — JetStream subscribe на `story.created` (как `message_events_consumer.go`); `StoryEventHandler` → push/in-app для `TypeMention`.
- [ ] **`story.lfp_created` → Matchmaking** — Story публикует; Matchmaking без subscriber (deferred в [story-service.md](microservices/story-service.md)).
- [ ] **File `context_story` lifecycle** — proto `RequestUploadRequest.context_story`; `file_grpc.go` игнорирует; Flutter `files_client` не передаёт; нет story-scoped TTL в `file_db`.

### Backend — API & privacy

- [ ] **User privacy policy** — audience из User/Social privacy ([privacy.md](features/privacy.md)); `CreateStory` default — не hardcoded `friends`.
- [ ] **`custom` / `close_friends` audiences** — proto есть; handler только `everyone`/`friends`; `StoryAudiencePicker` шлёт `close_friends` → deny.
- [ ] **Feed prefilter** — заменить global `ListActiveStoriesPaginated` на friends + space members до visibility filter.
- [ ] **Feed shape** — группировка по author для ring UX (`FeedGroups` есть; query — global scan + in-memory filter).

### Frontend — UX

- [ ] **Text story styling** — `text_style_json` в create; viewer не применяет background.
- [ ] **Highlights owner UX** — create/edit (name, privacy); archive → add-to-highlight; `HighlightsSection` read-only.
- [ ] **Owner archive screen** — `getArchive` в client; нет Flutter screen.
- [ ] **Author viewers list** — `getViewers` в client; viewer показывает только view count chip.
- [ ] **Game tag plashka** — non-LFP stories с `game_tag` — chip в viewer ([stories.md](features/stories.md) §Game tag).
- [ ] **Video duration cap ≤60s** — validate на pick и/или `CreateStory` / File upload.

### Tests

- [ ] **Expiry E2E** — publish → worker expire → archive → purge → File `DeleteFile` (compose live или dedicated job test).

### Post-MVP (не в Batch 5)

- [ ] **Story editor v2** — stickers, doodle, filters, clip trim ([stories.md](features/stories.md) §Редактор / §Клип).

**Промпт-якорь:** `Phase 17 Stories from docs/TODO.md Batch 5`.

---

## Batch 6 — Phase 18 Growth & Accessibility

PLAN §18 client/backend `[x]` — baseline; остаток vs [deep-links.md](features/deep-links.md), [onboarding.md](features/onboarding.md), [accessibility.md](features/accessibility.md).

### PLAN baseline vs spec

- [ ] **Onboarding one-liner** — PLAN: register → space → first message; код: 5 modal hints ([onboarding.md](features/onboarding.md)).
- [ ] **Приёмка #1 invite→join** — backend HTML+resolve OK; `DeepLinkListener` только в authed shell; `flushPendingAfterAuth()` не из auth flow; iOS без associated domains; prod AASA placeholders.
- [ ] **Приёмка #2 keyboard nav** — только Ctrl+K / Ctrl+,; нет Alt+↑/↓, Escape→composer, message keys.

### Deep links

- [ ] **Prod universal links** — real `voice.gg` AASA + `assetlinks.json` (Gateway — dev placeholders).
- [ ] **Share buttons** — copy `https://voice.gg/...` из space, chat, message, profile (сейчас только invite).
- [ ] **Push payload migration** — FCM/APNs canonical `deep_link` вместо raw `chat_id`.
- [ ] **Message anchor scroll/highlight** — `/m/{messageId}`.
- [ ] **DM / profile deep link UI** — `/dm/`, `/u/`.
- [ ] **Mobile device E2E** — App Links / custom scheme Android/iOS.

### Onboarding & a11y

- [ ] **Coach-mark anchors** — steps 2–4 pinned к nav/search/MM (сейчас modals).
- [ ] **Message list keyboard nav** — `↑/↓`, `R`, `E`.
- [ ] **Focus trap in all modals** — bottom sheets audit.
- [ ] **aria-live for new messages** — Flutter web semantics.
- [ ] **Manual TalkBack / VoiceOver** — pre-release checklist.
- [ ] **Axe / contrast CI** — token pairs.

### Tests

- [ ] **User store onboarding coverage** — `onboarding_test.go` в CI с testcontainers.
- [ ] **Web driver E2E** — `integration_test/phase18_deeplink_web_test.dart`.

**Промпт-якорь:** `Phase 18 Growth/A11y from docs/TODO.md Batch 6`.

---

## Batch 7 — Guest accounts (остаток)

Baseline закрыт (2026-06): register guest, JWT, guards, convert-guest, TTL, Flutter auto-guest/convert. См. [auth-and-contacts.md](features/auth-and-contacts.md).

- [ ] **Messaging `allow_guest_dm`** — guest reply/send в существующем DM: проверять `allow_guest_dm` у получателя; Messaging не читает `account_type`.
- [ ] **Guest audience (остальные поля)** — v1 `show_online_include_guests` есть; нет multiselect «Гостевые аккаунты» для `show_game_status` / `show_mm_rating` / `show_stories` и enforcement.
- [ ] **Onboarding coach-marks E2E** — widget-тест якорей после guest nickname; API live: `guest_onboarding_e2e_live_test`.

**Промпт-якорь:** `Guest accounts from docs/TODO.md Batch 7`.

---

## Batch 8 — Infra & repo hygiene

- [ ] **Политика `protos/` и генерации** — [REPOSITORIES.md](REPOSITORIES.md) + Makefile/CI: что коммитим (Go `*.pb.go`, Dart gen), что генерим в CI.
- [ ] **Скрипт стенд + миграции** — Makefile/scripts поверх [migrations README](../src/backend/migrations/README.md).
- [ ] **Аудит консистентности доков** — [DOCS_CONSISTENCY_AUDIT.md](DOCS_CONSISTENCY_AUDIT.md) после крупного контрактного PR.

**Промпт-якорь:** `Infra batch from docs/TODO.md Batch 8`.

---

## Batch 9 — QA polish

- [ ] **Bundle emoji font for web** — Noto Color Emoji для reaction/emoji picker; убрать runtime Noto fallback warning.

**Промпт-якорь:** `QA polish from docs/TODO.md Batch 9`.

---

## Сводка: что отдать агенту

| Приоритет / готовность | Batch |
|------------------------|-------|
| Синхронизировать план с кодом | **Batch 1 — Docs sync** |
| Trust privacy/reports | **Batch 2 — Phase 11** |
| E2E хвосты | **Batch 3 — Phase 15** |
| Боты: deploy, hardening, live tests | **Batch 4 — Phase 16** |
| Сторис | **Batch 5 — Phase 17** |
| Deep links, onboarding, a11y | **Batch 6 — Phase 18** |
| Гостевые ограничения + UX | **Batch 7 — Guest accounts** |
| Репо / protos / миграции | **Batch 8 — Infra** |
| Web emoji font | **Batch 9 — QA** |

Фазовые критерии «готово» — [PLAN.md](PLAN.md) §11–18, [encryption.md](features/encryption.md), [bots.md](features/bots.md).
