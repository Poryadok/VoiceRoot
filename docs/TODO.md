# Пробелы и открытые вопросы (документация)

Здесь — **вне дорожной карты** [PLAN.md](PLAN.md). Продуктовые и инфраструктурные чеклисты по фазам, остаток Фазы 0 (Auth: smoke контейнера в CI, порядок миграций `auth_db`), UUIDv7 для сообщений — в плане.

## Как пользоваться

| Метка | Кому | Смысл |
|-------|------|--------|
| **Agent batch** | Cursor / агент | Один PR или одна сессия: общий контекст, TDD, без ваших секретов |
| **Вы** | Человек | Решение, ключи, аккаунт, юридическое — агент ждёт ввод или не может закрыть |

**Порядок:** сначала закройте пункты в «Только вы», затем давайте агенту batch целиком (копируйте заголовок batch в промпт).

---

## Только вы (не делегировать агенту целиком)

### Решения и политика

- [x] **Лицензия crypto (P0 E2E)** — решено: оставляем `libsignal_protocol_dart` (GPL-3.0); Flutter-клиент — open source под GPL-3.0, бэкенд проприетарный.
- [x] **Offline cache (P2 E2E)** — решено: encrypted SQLite + ключ в secure storage; decrypted bodies в кэше для локального поиска ([encryption.md](features/encryption.md) § Offline cache).
- [x] **Safety number / TOFU (P2 E2E)** — решено: v1 — implicit TOFU; баннер при смене identity key; короткий код верификации `XX-XX-XX` (6 символов) в инфо DM, опционально ([encryption.md](features/encryption.md) § Доверие к ключам).
- [x] **Public webhook ingress (dev, P2 боты)** — решено: локально — только polling; webhook остаётся для staging/prod, E2E webhook проверяется на staging (tunnel/ngrok в dev не требуется).

### Секреты, аккаунты, внешние сервисы

- [ ] **Developer Portal OAuth** — завести OAuth/OIDC app (или временно оставить paste JWT): `client_id`, `client_secret`, redirect URI (`http://localhost:9082/callback` в dev), issuer. Агент может сделать flow **после** того как ключи лежат в `.env` / compose secrets.
- [ ] **Buf Schema Registry (позже)** — аккаунт BSR, имя модуля, `BUF_TOKEN` в CI. Агент обновит workflow **после** создания registry.

### Запуск стенда (вы, не агент)

- [x] **Compose + миграции Phase 15** — incremental snippets + `make compose-migrate-phase15`; Auth Flyway V4 (`IF NOT EXISTS`).
- [ ] **Реальные ключи dev** — если тестируете File/R2/FCM в compose: скопировать `.env.example` → `.env`, заполнить `FILE_R2_*`, `USER_R2_*` и т.д. ([`.env.example`](../.env.example)).

---

## Agent batches

### Batch E2E-A — «закрыть P0 E2E в коде» (Flutter + Messaging + Chat)

*Зависимости: GPL-решение закрыто (open source клиент).*

- [x] **Persistent Signal store** — `flutter_secure_storage` / web encrypted store вместо `InMemorySignalProtocolStore`.
- [x] **Pre-key sync по сети** — login / перед send: `UploadPreKeyBundle`, `GetPreKeyBundle(peer)`, X3DH; убрать in-process bilateral sessions из prod-пути.
- [x] **UI opt-in/opt-out** — wire `E2eEnableConfirmDialog` / `E2eDisableConfirmDialog` / `E2eKeyBackupSheet` в `ChatInfoPanel`.
- [x] **Запрет plaintext в E2E-чате** — Messaging: reject `is_e2e=false` и plain `EditMessage` при `e2e_enabled`; клиент: не plain send при ошибке encrypt.
- [x] **E2E enable gate** — оба peer с pre-keys перед enable (согласовать с Chat/Messaging API).

**Промпт-якорь:** `Phase 15 E2E P0 — batch E2E-A, TDD, docs/features/encryption.md`.

---

### Batch E2E-A audit follow-ups (post-implementation)

- [x] **Key backup codec (E2E-B)** — `E2eKeyBackupCodecV2` (PBKDF2 + AES-GCM); full `SecureSignalStore` export/restore wired.
- [x] **EditMessage E2E ciphertext** — Messaging allows ciphertext edit when `is_e2e`; client encrypts before API call.
- [x] **Pre-key validation / OTPK consume** — structural bundle validation + OTPK consume on `GetPreKeyBundle`.
- [x] **Verification code UI (P2)** — `XX-XX-XX` Crockford block in DM info (`e2e_chat_settings.dart`).
- [x] **Identity key change banner (P2)** — `E2eIdentityChangeBanner` + trust state in `e2e_identity_trust.dart`.
- [x] **File E2E upload gate** — `RequestUpload` cross-checks `chat.e2e_enabled` + DM-only for `is_e2e`.
- [x] **Two-device libsignal live test** — `phase15_e2e_dm_live_test.dart` B-side `decryptForChat` roundtrip.
- [x] **applySQLFile path normalization** — shared `integrationtest.ApplySQLFile` + `MigrationSuffixMatches`.

**Остаточные риски (см. E2E-B/C ниже):** сервер не проверяет libsignal signature на signed pre-key; ротация OTPK pool; Auth Flyway для backup blob.

---

### Batch E2E-B — «E2E backend: backup, миграции, edit/file»

*После или параллельно E2E-A; Auth Flyway — согласовать с Path A/B для `auth_db`.*

- [x] **Key backup по спеке** — client AEAD + KDF + export identity/sessions/pre-keys + restore (Auth хранит opaque blob).
- [x] **Auth Flyway** — `V4__e2e_key_backups.sql` (+ паритет `auth_db/000005` golang-migrate); `CREATE TABLE IF NOT EXISTS` для compose volumes с Path B.
- [x] **Compose migrations Phase 15** — `incremental_chat_db` / `incremental_messaging_db` snippets + `make compose-migrate-phase15`.
- [x] **Pre-key directory** — Ed25519 signed-pre-key verify + multi-OTPK pool + client replenishment on bootstrap.
- [x] **EditMessage E2E** — ciphertext edit path for `is_e2e` messages.
- [x] **File E2E** — client AES-GCM + `is_e2e` upload; 90d UX in enable/settings copy (server gate done).

**Промпт-якорь:** `Phase 15 E2E — batch E2E-B backend`.

---

### Batch E2E-B audit follow-ups (post-implementation)

- [x] **Pre-key signature cross-check** — `curve25519sig.go` + committed `prekey_libsignal_golden.b64` (from `export_prekey_golden_test.dart`); upload rejects invalid sig.
- [x] **E2E file live compose test** — `phase15_e2e_file_live_test.dart` (encrypt upload → message → peer decrypt); wired in `scripts/ci/compose-e2e-live.sh`.
- [x] **E2E attachment download** — non-image E2E: decrypt + tap-to-save via `e2e_attachment_actions_*` in `chat_room_panel.dart`.
- [x] **E2E file thumbnails** — server imgproc skipped for `is_e2e`; client `e2e_image_thumb.dart` + `e2eDecryptedAttachmentThumbProvider`.
- [x] **ClamAV / scan on E2E blobs** — `scanConfirmedFile` marks `skipped` for `is_e2e` (`file_e2e_scan_test.go`).
- [x] **Auth JDBC backup IT** — `Phase15E2EKeyBackupJdbcIntegrationTest` (Flyway `e2e_key_backups`, roundtrip, oversize); CI `backend-auth` when Docker available.

**Аудит (post E2E-B follow-ups):**
- [ ] **Pre-key golden drift** — CI/Makefile check that `prekey_libsignal_golden.b64` matches `export_prekey_golden_test.dart`; retire `sign_testutil.go` ed25519 shortcut in unit fixtures.
- [ ] **E2E attachment UX tests** — widget coverage for `ChatRoomPanel` E2E download + thumb (live test covers API only).
- [ ] **E2E shared media panel** — `chat_info_panel.dart` lock placeholder only; decrypt preview/download for E2E shared media.
- [ ] **auth-service.md** — document `PutE2EKeyBackup` / `GetE2EKeyBackup` and `e2e_key_backups` table (carry-over from E2E-C).

---

### Batch E2E-C — «E2E тесты и поиск»

- [x] **Search + compose** — `x-voice-profile-id` в Search global/in-chat (live 500 `missing credentials`); E2E exclusion на 200 + empty hits.
- [x] **Тесты E2E** — gateway live edit-in-E2E step; `MessageEdited` indexer skip; coverage ≥80% `lib/e2e/` + E2E backend packages (prekey **80%+** done; grpcsvc ~73% — add edge-case tests).
- [x] **Key backup limits (P2)** — max blob, rate limit put/get pre-keys (лимиты можно взять из спеки или дефолты в коде).
- [x] **docs/microservices/messaging-service.md** — добавить секцию E2E/pre-keys/edit policy.
- [x] **Flutter web docker image** — `flutter build web` падает на `dart:ffi` (sqlite3mc/native); smoke через `flutter run -d edge` на хосте.

**Промпт-якорь:** `Phase 15 E2E — batch E2E-C tests and search`.

**Аудит (post E2E-C):**
- [x] **Auth JDBC backup IT** — `Phase15E2EKeyBackupJdbcIntegrationTest` in CI `backend-auth` (Testcontainers); see E2E-B.
- [ ] **auth-service.md** — документировать `PutE2EKeyBackup` / `GetE2EKeyBackup` и таблицу `e2e_key_backups`.
- [x] **Pre-key signature cross-check** — libsignal golden + `curve25519sig.go`; see E2E-B.

---

### Batch BOT-A — «Bot API и Gateway parity»

- [x] **Gateway REST parity** — `GET/PATCH/DELETE` bot, uninstall, `ListInstalledBots`, webhook URL (`transcode_bots.go`).
- [x] **Autocomplete** — RPC + Gateway; клиент при `autocomplete: true`, до 25 вариантов.
- [x] **Subcommands** — `/queue join`, валидация манифеста, группировка в `/`-меню.
- [x] **`bot_event_log` delivery failures** — failed/timeout rows при webhook delivery.
- [x] **Миграции bot_db** — golang-migrate вместо `Exec` всего `000001` на старт.
- [x] **`EditBotMessage`** — реализация в Bot Service.

**Промпт-якорь:** `Phase 16 bots — batch BOT-A API/gateway`.

**Аудит (post BOT-A):**
- [ ] **grpcsvc coverage ≥80%** — **80.4%** после BOT-C; закрыть оставшиеся ветки в BOT-D.
- [ ] **Autocomplete polling mode** — webhook-only sync path; polling боты получают пустой список (док/бот API).
- [ ] **EditBotMessage** — нет integration test с Messaging mock; ownership edge cases.
- [x] **Hub deferred persistence** — `MarkEventDeferred` + `RehydrateDeferred` (BOT-C).
- [ ] **docs/features/bots.md** — polling path `/api/bots/...` vs фактический `/api/v1/bots/...`.
- [ ] **buf generate BSR** — `make buf-generate` 403; локально `buf generate --template buf.gen.local-go.yaml`.
- [ ] **Gateway Docker build** — `go mod download` в образе gateway (missing messaging context) — блокирует `compose-app-up` на чистой машине.

---

### Batch BOT-B — «Bot UX во Flutter»

- [x] **Display name бота в Messaging** — `sender_profile_id` → имя в пузырях.
- [x] **Slash UI** — опции команд; не глотать `BotsApiFailure`; greyout по offline (см. BOT-C для backend online).
- [x] **Per-chat bot toggles** — RPC `enabled` + UI в настройках текстового чата.
- [x] **Install bot в клиенте** — scopes human-readable, whitelist, `SPACE_MANAGE_BOTS`.
- [x] **Uninstall cleanup** — роли, pins, команды из меню при uninstall.

**Промпт-якорь:** `Phase 16 bots — batch BOT-B Flutter UX`.

**Аудит (post BOT-B):**
- [x] **Uninstall roles/pins** — `UninstallBotFromSpace`: whitelist + `Role.DeleteRolesCreatedByProfile` + `Messaging.UnpinMessagesBySenderInChats` + `Space.RemoveBotMember`.
- [x] **Bot online heartbeat** — `SlashCommand.online` из `bot_presence`; greyout UI на клиенте (BOT-C).
- [x] **Slash option types P2** — Flutter pickers `user`/`channel`/`role`/`attachment` (`slash_command_options_sheet.dart`).
- [x] **grpcsvc coverage ≥80%** — **80.7%** после BOT-B audit; оставшиеся ветки — BOT-D.
- [x] **Gateway bot routes в api-gateway.md** — таблица `/api/v1/bots/**` по `transcode_bots.go` (вкл. `GetBotBySlug`, `ListBotsInChat`, `SetBotChatEnabled`).
- [ ] **Flutter live E2E install+pong** — `phase16_bots_slash_live_test` + install flow с `VOICE_RUN_LIVE_INTEGRATION=true` (BOT-D).
- [x] **Страница бота `voice.app/bots/{slug}`** — deep link + `BotInstallPage` (portal polish — BOT-D P2).
- [x] **bot-service.md stale** — полный gRPC surface, `actor_profile_id`, `slug`, install/uninstall, presence.

**Аудит (post BOT-B, открытые пробелы):**
- [ ] **Bot `CreateRole` API** — нет RPC для бота создавать роли; uninstall чистит только роли с `created_by_profile_id` (Role `000007`).
- [ ] **role_db duplicate `000006`** — две миграции с версией `000006` блокируют `golang-migrate` на чистом `role_db`.
- [ ] **docs/features/bots.md** — polling path `/api/bots/...` vs фактический `/api/v1/bots/...` (дублирует BOT-A audit).
- [ ] **Rate limits / REST heartbeat / `GetChatMessagesForBot`** — см. BOT-C audit ниже.
- [ ] **Developer Portal OAuth** — BOT-D; ключи в секции «Только вы».
- [ ] **ru l10n bot scopes** — `BotScopeLabels` в `bot_scopes.dart` только EN; arb-ключи для scope labels не добавлены.

---

### Batch BOT-C — «Bot backend: надёжность и scopes»

- [x] **Bot online / offline** — `bot_presence`, `TouchPresence`, poll/webhook touch; `online` в `ListSlashCommandsForChat`; Gateway `EmitUnpopulated`; Flutter greyout + `bot_unavailable`/`bot_timeout`.
- [x] **`TEXT_CHAT_READ_HISTORY`** — `acknowledge_privileged_scopes` при install (backend + Flutter); scope gate на `GetChatMessagesForBot`.
- [x] **`MEMBER_ASSIGN_ROLES` runtime** — `AssignBotRole`/`RevokeBotRole` gRPC + делегирование Role Service.
- [ ] **Остальные scopes runtime (P2)** — `DM_SEND` (interaction-only), `TEXT_CHAT_CREATE_IN_SPACE`, `SPACE_VIEW_MEMBER_LIST` — gRPC есть; Gateway REST + slash pickers — нет.
- [x] **Hub in-memory only** — `MarkEventDeferred`, `ListDeferredTokens`, `RehydrateDeferred` при старте.
- [x] **InstallBotInSpace + space channel** — `Space.AddBotMember`; channel без `Chat.AddMembers`; group с `AddMembers`.
- [x] **Role gRPC на InstallBotInSpace** — `CheckPermission` + `s2s.ForwardIncomingMetadata`.

**Промпт-якорь:** `Phase 16 bots — batch BOT-C backend hardening`.

**Аудит (post BOT-C):**
- [x] **Online в списке ботов спейса** — `InstalledBot.online` в `ListInstalledBots`; `space_bots_sheet` показывает online/offline.
- [ ] **Gateway REST для scope RPC** — нет transcoding: `TouchPresence`, `AssignBotRole`/`RevokeBotRole`, `ListSpaceMembersForBot`, `CreateBotChat`, `GetChatMessagesForBot`.
- [ ] **`GetChatMessagesForBot`** — `Unimplemented`; нет Messaging read + Gateway route.
- [x] **Uninstall roles/pins** — реализовано в BOT-B (`DeleteRolesCreatedByProfile`, `UnpinMessagesBySenderInChats`).
- [ ] **Deferred TTL** — нет expiry/abandon для `delivery_status=deferred` в `bot_event_log`.
- [ ] **Rate limits** — 5000 req/min и 100 role ops/min + `429 Retry-After` не реализованы (`bots.md` §Rate Limiting).
- [ ] **REST heartbeat для webhook-ботов** — `POST /api/v1/bots/me/presence` или документировать gRPC-only.
- [ ] **Live compose BOT-C** — `compose_phase16_bots_slash_live_test` (greyout, privileged install); прогон с `VOICE_RUN_LIVE_COMPOSE=true`.

---

### Batch BOT-D — «Bot тесты и портал (код после OAuth)»

*Developer Portal login — после ключей OAuth (секция «Только вы»).*

- [ ] **Developer Portal auth** — login/OAuth flow, rotate `webhook_secret`, revoke token, list apps (ключи из `.env`).
- [ ] **Тесты** — Flutter live install+channel pong; Bot `grpcsvc` coverage ≥80% — grpcsvc **80.4%** (BOT-C); live compose — audit.
- [ ] **Страница бота (P2)** — `voice.app/bots/{slug}` (если есть маршрутизация в клиенте).

**Промпт-якорь:** `Phase 16 bots — batch BOT-D portal and tests`.

---

### Batch Infra — «репо, миграции, доки»

- [ ] **Политика `protos/` и генерации** — [REPOSITORIES.md](REPOSITORIES.md) + Makefile/CI: что коммитим (Go `*.pb.go`, Dart gen), что генерим в CI.
- [ ] **Скрипт стенд + миграции** — Makefile/scripts поверх [migrations README](../src/backend/migrations/README.md).
- [ ] **PLAN.md §15 / §16** — отметить чеклисты после закрытия соответствующих batch.
- [ ] **Аудит консистентности доков** — [DOCS_CONSISTENCY_AUDIT.md](DOCS_CONSISTENCY_AUDIT.md) после крупного контрактного PR.

**Промпт-якорь:** `Infra batch — protos policy and migration script`.

---

## Phase 17 — Stories (follow-ups)

*Audit vs [stories.md](features/stories.md) and [PLAN.md](PLAN.md) §17. MVP backend + partial Flutter landed; items below are genuine gaps, not out-of-phase scope (AR, algorithmic feed, post-match auto-story, monetization).*

### Backend — events & integrations

- [ ] **`story.events` NATS publisher** — JetStream stream per [story-service.md](microservices/story-service.md): publish on create, view, react, expire, highlight create, LFP create.
- [ ] **Mention notifications** — on story create with `mention_profile_ids`, event → Notification Service.
- [ ] **Reply-to-story → DM** — gRPC + Gateway route: private reply opens/sends DM thread ([stories.md](features/stories.md) §Ответы).
- [ ] **Archive purge → File Service** — `PurgeArchivedStories` deletes orphaned `media_file_id` in R2.
- [ ] **Story media upload context** — File `RequestUpload` purpose/lifecycle for story attachments.

### Backend — API & privacy

- [ ] **Highlight privacy** — enforce `highlights.visibility` in `GetHighlights`; expose on proto + create/update.
- [ ] **User privacy policy** — resolve story audience from User/Social privacy settings ([privacy.md](features/privacy.md)).
- [ ] **`custom` / `close_friends` audiences** — proto enums exist; handler only supports `everyone`/`friends`.
- [ ] **Author-only view stats** — hide `view_count` from non-authors; `GetStoryReactions` (author-only).
- [ ] **Premium anonymous views** — gate `MarkViewed(anonymous=true)` via Subscription Service.
- [ ] **Feed pagination** — cursor/`HasMore` in `GetStoryFeed`.
- [ ] **Feed shape** — friends + space members grouped by author for ring UX.
- [ ] **CreateStory: mentions & game_tag** — wire proto fields + validation.
- [ ] **LFP visibility floor** — LFP visibility ≥ user story privacy setting.

### Frontend — UX & editor (Phase 17 basics)

- [ ] **Story create entry point** — shell affordance calling `StoriesRoutes.openCreate`.
- [ ] **Media picker** — wire `image_picker` / file upload for photo/video in `story_create_screen.dart`.
- [ ] **Video playback** — replace viewer placeholder with player (≤60s).
- [ ] **Text story styling** — colored background / `text_style_json` minimal editor.
- [ ] **Highlights on profile** — mount `HighlightsSection` on profile; archive → add-to-highlight for owner.
- [ ] **Profile story ring** — tap avatar on profile opens viewer.
- [ ] **Author insights** — viewers list + view count UI for own active stories.
- [ ] **Reply-to-story UI** — DM compose from viewer (depends on backend reply RPC).
- [ ] **LFP actions** — Join / Write navigation stubs on `LfpStoryCard` per [matchmaking.md](features/matchmaking.md).
- [ ] **Game tag on create** — catalog picker → `game_tag`.

### Tests, CI & docs

- [ ] **Story `grpcsvc` coverage ≥80%** — jobs workers, `privacy.FriendChecker` with Social mock.
- [ ] **Expiry acceptance test** — publish → expire → archive → purge integration.
- [ ] **Flutter widget tests** — viewer, create, highlights screens.
- [ ] **`story-service.md` parity** — document REST map; note deferred LFP→Matchmaking NATS.
- [ ] **`make compose-migrate-story` in CI** — `story_db` migrations on fresh compose volumes.

**Промпт-якорь:** `Phase 17 Stories — follow-ups from docs/TODO.md §Phase 17`.

---

### Batch Phase-18 audit follow-ups (Growth & Accessibility)

*Post Phase 18 implementation — gaps vs full spec.*

#### Deep links

- [ ] **Prod universal links** — real `voice.gg` AASA + `assetlinks.json` with production app IDs / SHA256 (Gateway serves dev placeholders).
- [ ] **Share buttons** — copy `https://voice.gg/...` from space, chat, message, profile (invite only today).
- [ ] **Push payload migration** — FCM/APNs use canonical `deep_link` URL instead of raw `chat_id` only.
- [ ] **Message anchor scroll/highlight** — client scroll-to-message for `/m/{messageId}` routes.
- [ ] **DM / profile deep link UI** — navigate to DM compose and public profile from `/dm/` and `/u/` links.
- [ ] **Mobile device E2E** — real App Links / custom scheme on Android/iOS (CI skips device).

#### Onboarding

- [ ] **Coach-mark anchors** — steps 2–4 tooltips pinned to nav/search/MM widgets (modals today, not anchored overlays).
- [x] **Guest nickname first-run** — auto-register guest + `GuestNicknameScreen` (`guest_entry_test`, `guest_nickname_screen_test`).
- [ ] **Onboarding after guest register** — E2E: guest nickname → shell → onboarding step 1 coach-marks (not verified end-to-end).

#### A11y

- [ ] **Message list keyboard nav** — `↑/↓`, `R`, `E` per [accessibility.md](features/accessibility.md).
- [ ] **Focus trap in all modals** — onboarding uses `AlertDialog`; bottom sheets need explicit trap audit.
- [ ] **aria-live for new messages** — web semantics region (Flutter web).
- [ ] **Manual TalkBack / VoiceOver** — pre-release checklist (not automated).
- [ ] **Axe / contrast CI** — automated contrast ratio checks on token pairs.

#### Tests & ops

- [ ] **User store onboarding coverage** — run `onboarding_test.go` in CI with testcontainers.
- [ ] **Web driver E2E** — `integration_test/phase18_deeplink_web_test.dart` for Edge/Chrome path routing.

**Промпт-якорь:** `Phase 18 Growth/A11y — batch Phase-18 audit follow-ups`.

---

## Сводка: что отдать агенту следующим

| Если готовы… | Дайте агенту batch |
|--------------|-------------------|
| E2E P0 (GPL ок) | **E2E-A** (самый заметный user-facing сдвиг) |
| Нужен быстрый bot win | **BOT-C** (online heartbeat) или **BOT-D** (live E2E) |
| Есть OAuth keys в `.env` | **BOT-D** |
| Только доки/онбординг | **Infra** |

Фазовые детали и критерии «готово» — [PLAN.md](PLAN.md) §15–16, [encryption.md](features/encryption.md), [bots.md](features/bots.md).

---

## Bug batch #1–15 (QA web pass, 2026-06) — resolved

*Original repro list removed after fix batch. Widget/integration coverage noted where added.*

- [x] **#1 Dual screen share in DM** — remote stream auto-select + local preview (`screen_share_providers`, `screen_share_panel_test`).
- [x] **#2 In-chat message search** — `InChatSearch` wired to API / E2E-local (`in_chat_search_test`, `chat_room_panel` header + overlay).
- [x] **#3 In-chat search UX** — inline header field left of search icon; results overlay above chat.
- [x] **#4 Nav column width (web)** — 320px expanded rail + `FittedBox` labels (`navigation_width_test`).
- [x] **#5 Remove friend** — `isFriendProvider` + remove button in `ProfileDetailSheet`.
- [x] **#6 Matchmaking search 500 (`nudged_at`)** — `incremental_matchmaking_db.sql.snippet` via `compose-db-init`; `auth` in `GATEWAY_GRPC_UPSTREAMS_JSON` (`compose_matchmaking_search_live_test`).
- [x] **#7 MM criteria from game profile** — `QueueSearchScreen._tryPrefillFromProfile`.
- [x] **#8 Fullscreen video / screen share** — video + active screen share use fullscreen overlay + minimize bar (`active_call_panel_test`).
- [x] **#9 Emoji / Noto font warning** — `GoogleFonts.notoSansTextTheme` for UI text (emoji picker glyphs may still need bundled color font).
- [x] **#10 DM broadcast mentions 400** — mention picker removed; DM compose strips `@everyone`/`@here` from `mentions_json`.
- [x] **#11 Remove @mention picker button** — composer button removed (`chat_room_panel_test`).
- [x] **#12 Mention styling** — blue underline user links (`mention_message_content_test`).
- [x] **#13 Duplicate profile-search spinner** — single loader in `SocialPanel` (`social_panel_test`).
- [x] **#14 Global search in nav** — compact mode uses bounded `ConstrainedBox` instead of unbounded `Expanded` (`global_search_panel_test`).
- [x] **#15 Guest-first entry** — auto guest register + nickname screen + backend/gateway `ConvertGuest`; `isGuest` restored when guest password in storage.

### Bug batch #1–15 — remaining follow-ups

- [ ] **Guest convert-to-regular UI** — settings flow: email + password → `POST /api/v1/auth/convert-guest`; optional email verification step.
- [ ] **Bundle emoji font for web** — ship Noto Color Emoji for reaction/emoji picker; avoid runtime Noto fallback warning.
- [ ] **Docs: guest onboarding flow** — short spec in `docs/features/` linking guest type → nickname → convert-guest.
