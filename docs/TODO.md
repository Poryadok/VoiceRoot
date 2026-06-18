# Пробелы и открытые вопросы (документация)

Здесь — **вне дорожной карты** [PLAN.md](PLAN.md). Продуктовые и инфраструктурные чеклисты по фазам, остаток Фазы 0 (Auth: smoke контейнера в CI, порядок миграций `auth_db`), UUIDv7 для сообщений — в плане.

## Как пользоваться

| Метка | Кому | Смысл |
|-------|------|--------|
| **Agent batch** | Cursor / агент | Один PR или одна сессия: общий контекст, TDD, без ваших секретов |
| **Вы** | Человек | Решение, ключи, аккаунт, юридическое — агент ждёт ввод или не может закрыть |

**Порядок:** сначала закройте пункты в «Только вы», затем давайте агенту batch целиком (копируйте заголовок секции в промпт).

Агенту: Удаляй отсюда все выполненные пункты.

---

## Только вы (не делегировать агенту целиком)

### Секреты и внешние сервисы

- [x] **Developer Portal OAuth** — проверить OAuth/OIDC app: `client_id`, `client_secret`, redirect URI (`http://localhost:9082/callback` в dev), issuer в `.env` / compose secrets. Агент может доделать flow после ключей.

### Отложено (staging недоступен, локально — compose)

- [ ] **Buf Schema Registry** — аккаунт BSR, имя модуля, `BUF_TOKEN` в CI (после появления staging).
- [ ] **Реальные ключи dev** — File/R2/FCM: `.env.example` → `.env`, `FILE_R2_*`, `USER_R2_*` и т.д.

---

## Документация vs код (аудит 2026-06-17)

Сверка PLAN §11–§18 и `DATA_STORES.md` с кодом. **Критерии приёмки** — ниже; **синхронизация чекбоксов PLAN** — отдельный batch после закрытия остатков.

### Вердикт по строкам из сводной таблицы PLAN

| Фаза | PLAN сейчас | Код vs критерии приёмки | Можно ставить `[x]` в PLAN? |
|------|-------------|-------------------------|----------------------------|
| **11** | `[ ]` | 3/3 критерия **DONE**; privacy **PARTIAL** vs [privacy.md](features/privacy.md) | Сводную строку — **да** (с остатком в «Phase 11» ниже); чеклисты §11 — после остатка privacy |
| **15** | `[ ]` | 3/3 критерия **DONE** (opt-out live + video tab + libsignal pin); compose key-backup live | Сводную строку — **да**; §15 backend `[x]` после PLAN sync |
| **16** | `[ ]` | Критерии 1–3 **DONE**; portal **PARTIAL** (webhook_secret); rate limits **PARTIAL** | Строку `[ ]` оставить до Phase 16 остатка; §16 `[x]` Developer Portal — **завышен** |
| **18** | `[ ]` | §18 backend/client `[x]` — **PARTIAL** (onboarding/a11y не полные spec); приёмка 1/3 | Сводную `[ ]` оставить; §18 `[x]` — baseline, не полное закрытие [accessibility.md](features/accessibility.md) |

- [ ] **Синхронизировать PLAN.md** — обновить сводную таблицу и §11–§18 чеклисты по вердикту выше (не менять границы фаз).
- [ ] **DATA_STORES.md** — добавить `e2e_key_backups` в инвентарь `auth_db` (таблица есть: Flyway `V4__e2e_key_backups.sql`, [auth-service.md](microservices/auth-service.md) L102–107).

**Промпт-якорь:** `Docs audit — sync PLAN.md and DATA_STORES with code`.

---

## Phase 11 — Trust (остаток)

*Критерии приёмки PLAN §11 закрыты в коде; ниже — расхождения с [reports.md](features/reports.md), [privacy.md](features/privacy.md), [auth-service.md](microservices/auth-service.md).*

### Репорты

- [ ] **API shape** — один gRPC `CreateReport` + `target_type` вместо отдельных `ReportUser` / `ReportMessage` / `ReportSpace` (функционально ок; PLAN/доки называют иначе; синхронизация docs-only).

### 2FA

*Закрыто для §11: TOTP enroll/verify, backup codes, login challenge, Flutter QR, live `compose_phase11_trust_live_test` + `phase11_trust_e2e_live_test.dart`.*

### Приватность

- [x] **Пресеты ≠ privacy.md** — work `show_online` = УС (`space_members` only); gaming/personal/work в `pkg/privacy/presets.go` + Flutter `privacy_presets.dart`.
- [x] **Мультиселект аудитории** — `PrivacyAudience` proto/JSONB, `PrivacyAudiencePicker`, Все/Никто, per-space picker.
- [x] **Поля действий** — proto/UI: phone search, calls, files, voice, chat/space invites; enforcement: DM + friend requests + presence; **остаток:** `allow_phone_search` / `allow_calls` / `allow_files` / `allow_voice_messages` / `allow_chat_space_invites` — store+validate, полный gate в Voice/Space/Messaging attachments — см. audit ниже.
- [ ] **show_avatar / show_bio** — не в [privacy.md](features/privacy.md); PLAN §11 упоминает — doc gap, отложено.
- [x] **FoF live E2E** — `compose_phase11_privacy_fof_live_test.go` (opt-in) + `phase11_privacy_fof_e2e_live_test.dart` (opt-in Flutter).
- [x] **Presence privacy без `privacy_settings`** — fail-closed в `user_presence.go`; строка в [OPERATIONS.md](OPERATIONS.md) §degraded UX.
- [x] **ListFriendsOfFriends cap** — SQL one-hop в `social/internal/store/friendships.go` (без 5000 loop).

**Audit (Phase 11 privacy batch):** `allow_calls` / invites — нет enforcement в Calls/Space invite RPC (сервисы частично stub). `allow_files` / `allow_voice_messages` — MIME guard в Messaging не подключён. Social/User Dockerfiles обновлены для `voice/pb/space` + chat transitive dep.

**Промпт-якорь:** `Phase 11 Trust — privacy and reports parity from docs/TODO.md`.

---

## Phase 15 — E2E (остаток)

*Batch закрыт 2026-06-18: opt-out/key-backup compose live, libsignal ciphertext golden, shared-media video E2E UI, libsignal pin в encryption.md.*

- [x] **Opt-out live test** — `compose_phase15_e2e_optout_live_test.go` + `phase15_e2e_optout_live_test.dart` (plaintext API; search best-effort при деградации Search).
- [x] **Key backup compose live** — `compose_phase15_e2e_key_backup_live_test.go`; gateway forwards `Authorization` → Auth gRPC; `resolveAccessToken()` в key-backup RPC.
- [x] **Compose Go DM test** — `e2e_ciphertext_libsignal_golden.b64` + peer pre-key golden; не full decrypt в Go (by design).
- [x] **Shared media video tab** — `_MediaTile` video affordance + `chat_info_panel_e2e_shared_media_test.dart`.
- [x] **libsignal version pin** — секция в [encryption.md](features/encryption.md); drift/export tools.

**Audit (Phase 15 batch):**

- [ ] **PLAN.md §15 sync** — сводная строка фазы 15 + чеклист backend `[x]` (docs-only).
- [ ] **DATA_STORES.md** — `e2e_key_backups` в инвентарь `auth_db` (см. также строку в «Доки» выше).
- [ ] **Opt-out search hardening** — compose live не требует search hit (tier-2); при стабильном Search в CI усилить assert в opt-out live.
- [ ] **Matcher `IncludeGuests` only** — без `IsEveryoneShortcut` гости видят поле, но не strangers; покрыто тестом shortcut, edge cases guests-only — отдельно.
- [ ] **Key backup Flutter live** — REST покрыт compose; нет opt-in `phase15_e2e_key_backup_live_test.dart`.
- [ ] **Multi-device E2E / safety number** — out of v1 per encryption.md; не в Phase 15.

**Промпт-якорь:** `Phase 15 E2E — follow-ups from docs/TODO.md`.

---

## Phase 16 — Боты (остаток)

*Критерии приёмки §16: `/ping`→pong, ephemeral, 3s timeout — **DONE** (opt-in live). Batch 2026-06-18: Redis gRPC rate limit, webhook_secret API/portal, ru scope l10n, compose webhook `extra_hosts`, opt-in GHA `compose-e2e-live`.*

**Аудит batch (2026-06-18, post-fix):**

- [ ] **Bot Service staging deploy** — сервис есть в compose, нет манифестов в `deploy/staging/`; prod rollout отдельно.
- [ ] **Bot gRPC mTLS** — v1: `BOT_GRPC_GATEWAY_ONLY` + `x-voice-internal`; полный mTLS / k8s NetworkPolicy — prod hardening.
- [ ] **PLAN.md §16 sync** — чекбокс Developer Portal `[x]` завышен до docs-sync batch; сводная строка фазы 16.
- [ ] **Flutter BOT-C live** — Go `TestComposePhase16BotsBotCRoutes_live` есть; нет opt-in Flutter live для presence/members/create chat.
- [ ] **TEXT_CHAT_CREATE_IN_SPACE 10/day** — лимит в bot-service.md; проверить enforcement + тест при multi-replica.
- [ ] **Developer Portal production OAuth** — PKCE в compose; prod redirect URIs / Auth client в DEPLOYMENT (ключи в «Только вы» закрыты).

**Промпт-якорь:** `Phase 16 bots — follow-ups from docs/TODO.md`.

---

## Инфраструктура и доки

- [ ] **Политика `protos/` и генерации** — [REPOSITORIES.md](REPOSITORIES.md) + Makefile/CI: что коммитим (Go `*.pb.go`, Dart gen), что генерим в CI.
- [ ] **Скрипт стенд + миграции** — Makefile/scripts поверх [migrations README](../src/backend/migrations/README.md).
- [ ] **Аудит консистентности доков** — [DOCS_CONSISTENCY_AUDIT.md](DOCS_CONSISTENCY_AUDIT.md) после крупного контрактного PR.

**Промпт-якорь:** `Infra batch — protos policy and migration script`.

---

## Phase 17 — Stories

*MVP backend + partial Flutter; ниже — пробелы vs [stories.md](features/stories.md) и [PLAN.md](PLAN.md) §17 (не out-of-phase: AR, algorithmic feed, post-match auto-story, monetization).*

### Backend — events & integrations

- [ ] **`story.events` NATS publisher** — JetStream stream per [story-service.md](microservices/story-service.md): create, view, react, expire, highlight create, LFP create. *Publisher wired when `NATS_URL` set; expiry worker still does not emit `story.expired`.*
- [ ] **Mention notifications** — on story create with `mention_profile_ids` → Notification Service. *`StoryEventHandler` + unit tests only; no NATS consumer in `notification/main.go`.*
- [x] **Reply-to-story → DM** — gRPC + Gateway: private reply opens/sends DM thread ([stories.md](features/stories.md) §Ответы).
- [x] **Archive purge → File Service** — `PurgeArchivedStories` deletes orphaned `media_file_id` in R2.
- [ ] **Story media upload context** — File `RequestUpload` purpose/lifecycle for story attachments.

### Backend — API & privacy

- [x] **Highlight privacy** — enforce `highlights.visibility` in `GetHighlights`; expose on proto + create/update.
- [ ] **User privacy policy** — resolve story audience from User/Social privacy settings ([privacy.md](features/privacy.md)).
- [ ] **`custom` / `close_friends` audiences** — proto enums exist; handler only supports `everyone`/`friends`.
- [x] **Author-only view stats** — hide `view_count` from non-authors; `GetStoryReactions` (author-only).
- [x] **Premium anonymous views** — gate `MarkViewed(anonymous=true)` via Subscription Service.
- [x] **Feed pagination** — cursor/`HasMore` in `GetStoryFeed`.
- [ ] **Feed shape** — friends + space members grouped by author for ring UX. *`FeedGroups` returned; feed query is global active scan + in-memory filter, no space members.*
- [x] **CreateStory: mentions & game_tag** — wire proto fields + validation.
- [x] **LFP visibility floor** — LFP visibility ≥ user story privacy setting.

### Frontend — UX & editor

- [x] **Story create entry point** — shell affordance calling `StoriesRoutes.openCreate` (route есть, нет кнопки в shell).
- [x] **Media picker** — `story_create_screen.dart`: `_pickMedia` только меняет тип, не вызывает `image_picker` / file upload.
- [x] **Video playback** — replace viewer placeholder with player (≤60s).
- [ ] **Text story styling** — colored background / `text_style_json` minimal editor. *Create picker stores `text_style_json`; viewer does not apply background.*
- [ ] **Highlights on profile** — mount `HighlightsSection`; archive → add-to-highlight for owner. *`HighlightsSection` on profile; no archive or add-to-highlight UI.*
- [x] **Profile story ring** — tap avatar on profile opens viewer.
- [ ] **Author insights** — viewers list + view count UI for own active stories. *View count only; `getViewers` unused in viewer.*
- [x] **Reply-to-story UI** — DM compose from viewer (depends on backend reply RPC).
- [x] **LFP actions** — Join / Write navigation stubs on `LfpStoryCard` per [matchmaking.md](features/matchmaking.md).
- [x] **Game tag on create** — catalog picker → `game_tag`.

### Tests, CI & docs

- [x] **Story `grpcsvc` coverage ≥80%** — jobs workers, `privacy.FriendChecker` with Social mock.
- [ ] **Expiry acceptance test** — publish → expire → archive → purge integration.
- [x] **Flutter widget tests** — viewer, create, highlights screens.
- [x] **`story-service.md` parity** — document REST map; note deferred LFP→Matchmaking NATS.
- [x] **`make compose-migrate-story` in CI** — `story_db` migrations on fresh compose volumes.

### Phase 17 — audit follow-ups (2026-06)

*Gaps found comparing `src/backend/story`, Gateway `transcode_stories.go`, Flutter `lib/ui/stories`, and compose live tests vs docs.*

- [ ] **Expiry worker → `story.expired` NATS** — `jobs.StartExpiryWorker` calls `MarkExpiredStories` only; must publish `story.expired` per expired row via `Events.PublishStoryExpired`.
- [ ] **`story_events_consumer` in Notification `main.go`** — add JetStream subscribe on `story.created` (mirror `message_events_consumer.go`); wire `StoryEventHandler.HandleStoryCreated` → push/in-app for `TypeMention`.
- [ ] **`story.lfp_created` → Matchmaking NATS consumer** — Story publishes event from `CreateLookingForParty`; Matchmaking Service has no subscriber (deferred in [story-service.md](microservices/story-service.md)).
- [ ] **File `context_story` lifecycle** — `RequestUploadRequest.context_story` in proto; `file_grpc.go` ignores field; Flutter `files_client` omits on story upload; no story-scoped TTL/link in `file_db`.
- [ ] **Feed friends/space prefilter** — replace global `ListActiveStoriesPaginated` scan with Social friends (+ space members per PLAN §17) before visibility filter; reduces feed perf cost at scale.
- [ ] **`close_friends` visibility end-to-end** — `StoryAudiencePicker` sends `close_friends`; `canViewStory` / `canViewHighlight` deny unknown values; needs Social close-friends list or map to `friends`.
- [ ] **CreateStory default visibility from User `show_stories`** — `CreateStory` defaults `friends`; should resolve audience from User Service privacy, not only LFP floor path.
- [ ] **Owner archive screen** — `VoiceStoriesClient.getArchive` exists; no Flutter screen to browse 30-day archive.
- [ ] **Add-to-highlight owner UI** — `addToHighlight` API + live test; no UI from archive or story viewer.
- [ ] **Create/edit highlights UI** — owner cannot name collections or set highlight privacy from client (only read `HighlightsSection`).
- [ ] **Author viewers list UI** — `getViewers` in client; `story_viewer_screen` shows view count chip only for active own stories.
- [ ] **Text story style in viewer** — apply `text_style_json` background tokens from create screen.
- [ ] **Game tag plashka in viewer** — non-LFP stories with `game_tag` should show catalog chip per [stories.md](features/stories.md) §Game tag.
- [ ] **Video duration cap ≤60s** — validate on client pick and/or `CreateStory` / File upload for story video.
- [ ] **Expiry E2E compose assertion** — extend `compose_phase17_stories_live_test` or dedicated job test: publish → worker expire → archive list → purge → File `DeleteFile`.
- [ ] **Story editor v2 (post-MVP)** — stickers, doodle, filters, clip trim per [stories.md](features/stories.md) §Редактор / §Клип.

**Промпт-якорь:** `Phase 17 Stories — follow-ups from docs/TODO.md`.

---

## Phase 18 — Growth & Accessibility

*PLAN §18 client/backend `[x]` — baseline; ниже — остаток vs [deep-links.md](features/deep-links.md), [onboarding.md](features/onboarding.md), [accessibility.md](features/accessibility.md).*

### PLAN §18 — расхождения baseline vs spec

- [ ] **Onboarding PLAN one-liner** — PLAN: «register → create/join space → first message»; код следует [onboarding.md](features/onboarding.md) (5 modal hints, не anchored).
- [ ] **A11y PLAN `[x]`** — semantics login/shell **DONE**; message keyboard nav, aria-live, focus trap, axe CI — **MISSING** (см. ниже).
- [ ] **Приёмка #1 invite→join** — **PARTIAL**: backend HTML+resolve OK; `DeepLinkListener` только в authed shell; `flushPendingAfterAuth()` **не вызывается** из auth flow (только unit test); iOS без associated domains; prod AASA placeholders.
- [ ] **Приёмка #2 keyboard nav** — **PARTIAL**: только Ctrl+K / Ctrl+, (`voice_shortcuts.dart`); нет Alt+↑/↓, Escape→composer, message keys.

### Deep links

- [ ] **Prod universal links** — real `voice.gg` AASA + `assetlinks.json` with production app IDs / SHA256 (Gateway serves dev placeholders).
- [ ] **Share buttons** — copy `https://voice.gg/...` from space, chat, message, profile (invite only today).
- [ ] **Push payload migration** — FCM/APNs use canonical `deep_link` URL instead of raw `chat_id` only.
- [ ] **Message anchor scroll/highlight** — client scroll-to-message for `/m/{messageId}` routes.
- [ ] **DM / profile deep link UI** — navigate to DM compose and public profile from `/dm/` and `/u/` links.
- [ ] **Mobile device E2E** — real App Links / custom scheme on Android/iOS (CI skips device).

### Onboarding

- [ ] **Coach-mark anchors** — steps 2–4 tooltips pinned to nav/search/MM widgets (modals today, not anchored overlays).

### Accessibility

- [ ] **Message list keyboard nav** — `↑/↓`, `R`, `E` per [accessibility.md](features/accessibility.md).
- [ ] **Focus trap in all modals** — onboarding uses `AlertDialog`; bottom sheets need explicit trap audit.
- [ ] **aria-live for new messages** — web semantics region (Flutter web).
- [ ] **Manual TalkBack / VoiceOver** — pre-release checklist (not automated).
- [ ] **Axe / contrast CI** — automated contrast ratio checks on token pairs.

### Tests & ops

- [ ] **User store onboarding coverage** — run `onboarding_test.go` in CI with testcontainers.
- [ ] **Web driver E2E** — `integration_test/phase18_deeplink_web_test.dart` for Edge/Chrome path routing.

**Промпт-якорь:** `Phase 18 Growth/A11y — follow-ups from docs/TODO.md`.

---

## Гостевые аккаунты (остаток)

*Baseline закрыт (2026-06): register guest, JWT `account_type`, Gateway header, guest guards (CreateDM/StartCall/SendFriendInvitation/CreateSpace/JoinByInvite+`allow_guests`), convert-guest + NATS, TTL sweeper, Flutter auto-guest/convert/reminder/greyout, integration tests. См. [auth-and-contacts.md](features/auth-and-contacts.md).*

### Backend — ограничения и privacy

- [ ] **Messaging `allow_guest_dm`** — guest **reply/send** в существующем DM: проверять `allow_guest_dm` у получателя; инициация guest по-прежнему запрещена (`CreateDM` уже blocked). Messaging Service сейчас не читает `account_type`.
- [ ] **GetBulkPresence guest filter** — `guestMayViewOnlineStatus` применяется в `GetPresence`, но **не** в `GetBulkPresence` (утечка online для guest-viewer).
- [ ] **Guest audience (остальные поля)** — v1 `show_online_include_guests` + UI есть; нет multiselect «Гостевые аккаунты» для `show_game_status` / `show_mm_rating` / `show_stories` и enforcement.

### Frontend & tests

- [ ] **Onboarding coach-marks E2E** — widget-тест якорей после guest nickname; API live: `guest_onboarding_e2e_live_test`.

**Промпт-якорь:** `Guest accounts — restrictions and UX from docs/TODO.md`.

---

## QA follow-ups (web pass 2026-06)

- [ ] **Bundle emoji font for web** — ship Noto Color Emoji for reaction/emoji picker; avoid runtime Noto fallback warning.

---

## Сводка: что отдать агенту следующим

| Если готовы… | Дайте агенту |
|--------------|--------------|
| Синхронизировать план с кодом | **Документация vs код** |
| Закрыть Trust privacy/reports | **Phase 11 — Trust (остаток)** |
| Закрыть E2E хвосты | **Phase 15 — E2E (остаток)** |
| Боты: backend hardening | **Phase 16 — Боты (остаток)** |
| Есть OAuth keys в `.env` | **Developer Portal** (внутри Phase 16) |
| Гостевые ограничения + UX | **Гостевые аккаунты (остаток)** |
| Сторис / growth | **Phase 17** или **Phase 18** |
| Только доки/репо | **Инфраструктура и доки** |

Фазовые детали и критерии «готово» — [PLAN.md](PLAN.md) §11–18, [encryption.md](features/encryption.md), [bots.md](features/bots.md).
