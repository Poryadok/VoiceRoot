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

- [ ] **Pre-key signature cross-check** — server Ed25519 verify vs real `libsignal_protocol_dart` bundles (not only Go test fixtures); reject upload if Dart/Go wire mismatch.
- [ ] **E2E file live compose test** — finish `phase15_e2e_file_live_test.dart` (encrypted upload roundtrip + decrypt preview) when MinIO/R2 up.
- [ ] **E2E attachment download** — non-image files: decrypt + open/save UX (images only via `Image.memory` today).
- [ ] **E2E file thumbnails** — skip server imgproc (done); optional client-side thumb after decrypt.
- [ ] **ClamAV / scan on E2E blobs** — skip or mark `skipped` for `is_e2e` ciphertext (scanner sees noise).
- [ ] **Auth JDBC backup IT** — `Phase15E2EKeyBackupJdbcIntegrationTest` in CI (Testcontainers).

---

### Batch E2E-C — «E2E тесты и поиск»

- [x] **Search + compose** — `x-voice-profile-id` в Search global/in-chat (live 500 `missing credentials`); E2E exclusion на 200 + empty hits.
- [x] **Тесты E2E** — gateway live edit-in-E2E step; `MessageEdited` indexer skip; coverage ≥80% `lib/e2e/` + E2E backend packages (prekey **80%+** done; grpcsvc ~73% — add edge-case tests).
- [x] **Key backup limits (P2)** — max blob, rate limit put/get pre-keys (лимиты можно взять из спеки или дефолты в коде).
- [x] **docs/microservices/messaging-service.md** — добавить секцию E2E/pre-keys/edit policy.
- [x] **Flutter web docker image** — `flutter build web` падает на `dart:ffi` (sqlite3mc/native); smoke через `flutter run -d edge` на хосте.

**Промпт-якорь:** `Phase 15 E2E — batch E2E-C tests and search`.

**Аудит (post E2E-C):**
- [ ] **Auth JDBC backup IT** — `Phase15E2EKeyBackupJdbcIntegrationTest` в CI (Testcontainers) — перенесено из E2E-B.
- [ ] **auth-service.md** — документировать `PutE2EKeyBackup` / `GetE2EKeyBackup` и таблицу `e2e_key_backups`.
- [ ] **Pre-key signature cross-check** — server Ed25519 vs real `libsignal_protocol_dart` bundles (E2E-B).

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
- [ ] **Uninstall roles/pins** — `UninstallBotFromSpace` очищает whitelist; Role Service + Messaging pins при удалении бота — BOT-C/отдельная задача.
- [x] **Bot online heartbeat** — `SlashCommand.online` из `bot_presence`; greyout UI на клиенте (BOT-C).
- [ ] **Slash option types P2** — `user`/`channel`/`role`/`attachment` pickers; v1: `string`/`integer`/`boolean`.
- [ ] **grpcsvc coverage ≥80%** — ~56% после BOT-B; добить в BOT-D.
- [ ] **Gateway bot routes в api-gateway.md** — `ListBotsInChat`, `SetBotChatEnabled` не в CONTRACT_MATRIX.
- [ ] **Flutter live E2E install+pong** — `phase16_bots_slash_live_test` + install flow с `VOICE_RUN_LIVE_INTEGRATION=true` (BOT-D).
- [ ] **Страница бота `voice.app/bots/{slug}`** — space sheet вместо portal (BOT-D P2).
- [ ] **bot-service.md stale** — нет `ListBotsInChat`, `SetBotChatEnabled`, `actor_profile_id`.

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
- [ ] **Online в списке ботов спейса** — `ListInstalledBots` / `space_bots_sheet` без online/offline (`bots.md` §«Статус бота»).
- [ ] **Gateway REST для scope RPC** — нет transcoding: `TouchPresence`, `AssignBotRole`/`RevokeBotRole`, `ListSpaceMembersForBot`, `CreateBotChat`, `GetChatMessagesForBot`.
- [ ] **`GetChatMessagesForBot`** — `Unimplemented`; нет Messaging read + Gateway route.
- [ ] **Uninstall roles/pins** — `UninstallBotFromSpace` не чистит роли бота и pins в Messaging (`bots.md` §«Удаление бота»).
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

## Сводка: что отдать агенту следующим

| Если готовы… | Дайте агенту batch |
|--------------|-------------------|
| E2E P0 (GPL ок) | **E2E-A** (самый заметный user-facing сдвиг) |
| Нужен быстрый bot win | **BOT-C** (online heartbeat) или **BOT-D** (live E2E) |
| Есть OAuth keys в `.env` | **BOT-D** |
| Только доки/онбординг | **Infra** |

Фазовые детали и критерии «готово» — [PLAN.md](PLAN.md) §15–16, [encryption.md](features/encryption.md), [bots.md](features/bots.md).
