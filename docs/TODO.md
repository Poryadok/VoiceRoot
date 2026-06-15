# Пробелы и открытые вопросы (документация)

Здесь — **вне дорожной карты** [PLAN.md](PLAN.md). Продуктовые и инфраструктурные чеклисты по фазам, остаток Фазы 0 (Auth: smoke контейнера в CI, порядок миграций `auth_db`), UUIDv7 для сообщений — в плане. Крупные темы при желании дублируйте в issue-трекере.

## Фаза 15 — E2E DM (остаток после scaffold)

Scaffold и live-smoke есть; до prod — доработки по аудиту реализации (см. [encryption.md](features/encryption.md), PLAN §15).

### P0 — блокеры prod

- [ ] **Persistent Signal store** — заменить `InMemorySignalProtocolStore` на secure storage (mobile: `flutter_secure_storage`; web: encrypted local store); ключи не терять при рестарте.
- [ ] **Pre-key sync по сети** — на login/перед E2E: `UploadPreKeyBundle`; перед первым send — `GetPreKeyBundle(peer)` + X3DH; убрать зависимость от in-process bilateral sessions (только тестовый артефакт).
- [ ] **UI opt-in/opt-out** — подключить `E2eEnableConfirmDialog` / `E2eDisableConfirmDialog` / `E2eKeyBackupSheet` в chat info (`chat_room_panel` или аналог); сейчас виджеты есть, entry point в продукте нет.
- [ ] **Запрет plaintext в E2E-чате** — Messaging: reject `is_e2e=false` (и plain `EditMessage`) когда `chats.e2e_enabled=true`; клиент: не fallback на plain send при ошибке encrypt.
- [ ] **Key backup по спеке** — заменить XOR/SHA stub на AEAD + KDF (PBKDF2/Argon2); экспорт identity/sessions/pre-keys; restore восстанавливает расшифровку истории.
- [ ] **Auth Flyway** — `V2__e2e_key_backup.sql` (сейчас только golang-migrate `auth_db/000005`; compose Auth на Flyway по умолчанию).
- [ ] **Compose migrations Phase 15** — автоматизировать `chat_db/000006`, `messaging_db/000009`, `auth_db/000005` в onboarding (см. пункт «скрипт миграций» выше); не применять SQL руками.
- [ ] **Лицензия crypto** — ревью `libsignal_protocol_dart` (GPL-3.0) vs продукт; альтернатива/исключение если нужно.

### P1 — высокий приоритет

- [ ] **Pre-key directory** — валидация bundle, consume OTPK, ротация; не отдавать один и тот же bundle бесконечно.
- [ ] **File E2E** — клиент: `is_e2e` на upload + client-side encrypt blob; File: проверка `chat.e2e_enabled`; expiry job 90d + UX-предупреждение.
- [ ] **E2E enable gate** — оба peer upload pre-keys / оба enable (сейчас флаг на чат без readiness peer).
- [ ] **Search + compose** — починить propagation `x-voice-profile-id` в Search global/in-chat (live 500 `missing credentials`); E2E exclusion тогда проверять на 200 + empty hits, не только degraded.
- [ ] **EditMessage E2E** — ciphertext edit path или reject edits for `is_e2e` messages.
- [ ] **Тесты** — two-client libsignal roundtrip over HTTP; reject plain send when `e2e_enabled`; `MessageEdited` indexer skip; покрытие ≥80% на `lib/e2e/` и E2E backend packages (сейчас ниже на полных grpcsvc).

### P2 — можно после MVP E2E

- [ ] **Key backup limits** — max blob size, rate limit на put/get pre-keys.
- [ ] **Safety number / TOFU** — identity key change UX.
- [ ] **Offline cache** — политика хранения decrypted bodies на device (spec: local search OK, но риск at-rest).
- [ ] **PLAN.md §15** — отметить чеклисты backend/client после закрытия P0.

## Сейчас (по мере работы с контрактами)

- [ ] **Политика `protos/` и генерации** — зафиксировать в [REPOSITORIES.md](REPOSITORIES.md) и в скриптах сборки: что коммитим в git (Go/Java), что генерируем при CI/локально, по мере расширения публичных gRPC.

- [ ] **`GRPC_DIAL_TIMEOUT` для S2S dial при старте** — вынести хардкод `30*time.Second` (ранее 5 с) из `user` / `chat` / `messaging` / `file` `main.go` в общий helper (`pkg/grpcconn` или аналог) и env `GRPC_DIAL_TIMEOUT` (дефолт в compose, например `15s`); один источник для bootstrap-dial к upstream gRPC.

## Фаза 16 — Боты (остаток после MVP)

Scaffold Bot Service, Gateway `/api/v1/bots/**`, polling slash E2E и минимальный Developer Portal есть; до закрытия PLAN §16 и [bots.md](features/bots.md) — доработки по аудиту реализации.

### P0 — блокеры prod / критерии приёмки

- [ ] **Bot actor profile** — `RegisterBot` вызывает `User.CreateProfile` при `USER_GRPC_ADDR`; `InstallBotInSpace` требует успешный `Chat.AddMembers` для actor. Нужен rebuild compose `bot` и live-проверка не-ephemeral `pong` в истории; display name в пузырях — отдельно.
- [ ] **Deferred follow-up end-to-end** — после `deferred` Hub снимается (`defer Cancel`); клиент не показывает «обрабатываю…»; нет согласованного пути `DeferResponse` / `SendBotMessage`+`interaction_token` / WS push для финального ответа; Gateway не экспонирует `POST /api/v1/bots/me/interactions/defer`.
- [ ] **Критерий приёмки #1** — довести compose/live сценарий до не-ephemeral `pong` в истории чата (Messaging + Realtime), не только `content` в `ExecuteSlashInteractionResponse` (сейчас E2E — ephemeral `pong`).
- [x] **`SetChatWhitelist` space_id** — `SetChatWhitelist` резолвит `space_id` через `PrimaryInstalledSpace` (одна installation).

### P1 — высокий приоритет

- [ ] **NATS `bot.events`** — публиковать `BotRegistered`, `CommandExecuted`, `WebhookDelivered`/`WebhookFailed` по [bot-service.md](microservices/bot-service.md) и `jetstream_events.proto`; сейчас publisher отсутствует.
- [ ] **Bot online / offline** — heartbeat или webhook health; в `ListSlashCommandsForChat` и Flutter `/`-меню: greyout + tooltip «Бот недоступен» ([bots.md](features/bots.md)).
- [ ] **Autocomplete** — RPC + Gateway route для partial input; клиент шлёт запросы при наборе опций с `autocomplete: true`; до 25 вариантов.
- [ ] **Subcommands** — модель команд (`/queue join`), валидация манифеста, группировка в `/`-меню ([bots.md](features/bots.md)).
- [ ] **Display name бота в Messaging** — резолв `sender_profile_id` → имя бота (User/bot profile), не UUID/unknown в пузырях чата.
- [ ] **Rate limit 5000/min per bot** — `BotAPI` сейчас bucket по user/IP; `GET/POST /api/v1/bots/me/**` не по спеке; ответ `429` + заголовок `Retry-After` ([bots.md](features/bots.md)); тесты на 429 (сейчас только mapping group в `ratelimit_test.go`).
- [ ] **Per-chat bot toggles** — RPC `enabled` на `bot_chat_whitelist` + UI в настройках текстового чата ([bots.md](features/bots.md)); колонка есть, API/UI нет.
- [ ] **`TEXT_CHAT_READ_HISTORY`** — privileged scope: предупреждение при install, enforcement в Bot/Gateway ([bots.md](features/bots.md)).
- [ ] **`MEMBER_ASSIGN_ROLES` runtime** — выдача/снятие ролей через Bot API с проверкой иерархии в Role Service ([bots.md](features/bots.md)).
- [ ] **Developer Portal auth** — заменить ручной paste JWT на login/OAuth flow; показ/rotate `webhook_secret`, revoke token, list apps.
- [ ] **Compose profile для Developer Portal** — сервис `developer-portal` в `docker-compose.yml` (сейчас только локальный `npm run dev`).
- [ ] **Gateway REST parity** — `GET/PATCH/DELETE` bot, uninstall, `ListInstalledBots`, webhook URL, ephemeral/defer bot routes; сейчас subset в `transcode_bots.go`.
- [ ] **Install bot в клиенте** — UI добавления в спейс: scopes human-readable, whitelist чатов, `SPACE_MANAGE_BOTS` ([bots.md](features/bots.md)); только slash-меню, без install flow.
- [ ] **Uninstall cleanup** — при удалении бота из спейса: роли бота, pin сообщений, команды из меню ([bots.md](features/bots.md)); сейчас только `DELETE` installation row.
- [ ] **Webhook delivery hardening** — retry 3× exponential backoff, `bot_event_log` failed/timeout ([bot-service.md](microservices/bot-service.md)); polling: не `MarkEventDelivered` до успешного `CompleteInteraction`.
- [ ] **RegisterBot token UX** — вернуть token в `RegisterBotResponse` или one-shot secret; сейчас обязателен лишний `RegenerateToken`.

### P2 — можно после MVP

- [ ] **Public webhook ingress (dev)** — опциональный Gateway reverse-tunnel для webhook без публичного URL (production — Socket Mode, post-v1).
- [ ] **`EditBotMessage`** — сейчас `Unimplemented` в Bot Service.
- [ ] **Остальные scopes runtime** — `DM_SEND`, `TEXT_CHAT_CREATE_IN_SPACE` (10/день), `SPACE_VIEW_MEMBER_LIST`; валидация типов опций slash (integer, user, channel, role, attachment).
- [ ] **Страница бота** — `voice.app/bots/{slug}`: описание, scopes, install CTA ([bots.md](features/bots.md)).
- [ ] **Slash UI** — опции команд, loading/deferred state, command greyout по offline; не глотать ошибки списка команд (`BotsApiFailure` → `[]`).
- [ ] **Миграции bot_db** — golang-migrate versioning вместо `Exec` всего `000001_init.up.sql` на каждый старт.
- [ ] **Тесты** — Gateway 429+`Retry-After` для BotAPI; deferred follow-up integration; Flutter live install+channel pong; coverage Bot `main`/grpcsvc ≥ CI порог. `scripts/dev/ping-bot/` + `ping_bot_test.go` (HMAC/pong) есть.
- [ ] **PLAN.md §16** — отметить чеклисты backend/client после закрытия P0/P1.

## Позже / по событию

- [ ] **Buf Schema Registry (BSR)** — сейчас только локальный модуль `protos/`; при введении удалённого registry обновить CI и [REPOSITORIES.md](REPOSITORIES.md).

- [ ] **Скрипт «поднять стенд + применить все миграции»** — опциональная обёртка в `Makefile` / `scripts/` поверх [src/backend/migrations/README.md](../src/backend/migrations/README.md); ускоряет onboarding, не блокирует фазы.

- [ ] **Аудит консистентности доков** — пройти [DOCS_CONSISTENCY_AUDIT.md](DOCS_CONSISTENCY_AUDIT.md) после первого значимого изменения контрактов или при закрытии крупных фаз.
