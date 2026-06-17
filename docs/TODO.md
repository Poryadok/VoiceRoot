# Пробелы и открытые вопросы (документация)

Здесь — **вне дорожной карты** [PLAN.md](PLAN.md). Продуктовые и инфраструктурные чеклисты по фазам, остаток Фазы 0 (Auth: smoke контейнера в CI, порядок миграций `auth_db`), UUIDv7 для сообщений — в плане.

## Как пользоваться

| Метка | Кому | Смысл |
|-------|------|--------|
| **Agent batch** | Cursor / агент | Один PR или одна сессия: общий контекст, TDD, без ваших секретов |
| **Вы** | Человек | Решение, ключи, аккаунт, юридическое — агент ждёт ввод или не может закрыть |

**Порядок:** сначала закройте пункты в «Только вы», затем давайте агенту batch целиком (копируйте заголовок секции в промпт).

---

## Только вы (не делегировать агенту целиком)

### Секреты и внешние сервисы

- [ ] **Developer Portal OAuth** — проверить OAuth/OIDC app: `client_id`, `client_secret`, redirect URI (`http://localhost:9082/callback` в dev), issuer в `.env` / compose secrets. Агент может доделать flow после ключей.

### Отложено (staging недоступен, локально — compose)

- [ ] **Buf Schema Registry** — аккаунт BSR, имя модуля, `BUF_TOKEN` в CI (после появления staging).
- [ ] **Реальные ключи dev** — File/R2/FCM: `.env.example` → `.env`, `FILE_R2_*`, `USER_R2_*` и т.д.

---

## Phase 15 — E2E (остаток)

_Закрыто в batch Phase 15 E2E follow-ups (2026-06)._

### Новые пробелы (аудит)

- [ ] **DATA_STORES.md** — строка `e2e_key_backups` в инвентаре `auth_db`.
- [ ] **Shared media video tab** — E2E decrypt для video-вложений (если тип поддержан в UI).
- [ ] **Web saveDecryptedE2eAttachment** — паритет с desktop для скачивания расшифрованных файлов.
- [ ] **libsignal version pin** — документировать/зафиксировать ожидаемую версию для pre-key golden (подпись non-deterministic).

**Промпт-якорь:** `Phase 15 E2E — follow-ups from docs/TODO.md`.

---

## Phase 16 — Боты (остаток)

### Backend и Gateway

- [ ] **buf generate BSR** — `make buf-generate` 403; локально `buf generate --template buf.gen.local-go.yaml`.
- [ ] **gRPC Bot API rate limits** — 5000/min только на Gateway `/api/v1/bots/me/**`; прямой gRPC `BotService` обходит `BotAPI` limiter.
- [ ] **GetChatMessagesForBot response shape** — proto отдаёт только `message_ids`; нет тел сообщений для bot runtime без отдельного Messaging доступа.
- [ ] **Autocomplete polling UX** — `AutocompleteSlashOption` сразу возвращает пустой список для polling-ботов; Flutter не ретраит до `CompleteAutocomplete`.
- [ ] **SendEphemeral Gateway REST** — gRPC `SendEphemeral` есть; transcoding route в Gateway / `api-gateway.md` нет.
- [ ] **SPACE_MANAGE_ROLES doc drift** — privileged scope в manifest/proto; отсутствует в таблице scopes `docs/features/bots.md`.
- [ ] **BOT_DEFERRED_TTL ops doc** — env и default 24h не задокументированы в `bot-service.md` / `OPERATIONS.md`.

### Developer Portal

*После ключей OAuth (секция «Только вы»).*

- [ ] **Developer Portal auth** — login/OAuth flow, rotate `webhook_secret`, revoke token, list apps.
- [ ] **Developer Portal webhook_secret rotate** — minimal portal (`src/developer-portal/`) умеет regenerate bot token; rotate `webhook_secret` / HMAC key — нет.
- [ ] **Developer Portal production OAuth** — PKCE flow есть; production зависит от Auth OAuth client; dev — paste JWT (`oauthDisabled`).

### Клиент и локализация

- [ ] **ru l10n bot scopes** — `BotScopeLabels` в `bot_scopes.dart` только EN; arb-ключи для scope labels.

### Тесты и покрытие

- [ ] **BOT-C live tests in CI** — compose + Flutter live opt-in; не в default `make` / GitHub Actions matrix.

### Документация

- [ ] **docs/features/bots.md** — polling path `/api/bots/...` vs фактический `/api/v1/bots/...`.

**Промпт-якорь:** `Phase 16 bots — follow-ups from docs/TODO.md`.

---

## Инфраструктура и доки

- [ ] **Политика `protos/` и генерации** — [REPOSITORIES.md](REPOSITORIES.md) + Makefile/CI: что коммитим (Go `*.pb.go`, Dart gen), что генерим в CI.
- [ ] **Скрипт стенд + миграции** — Makefile/scripts поверх [migrations README](../src/backend/migrations/README.md).
- [ ] **PLAN.md §15 / §16** — отметить чеклисты после закрытия соответствующих batch.
- [ ] **Аудит консистентности доков** — [DOCS_CONSISTENCY_AUDIT.md](DOCS_CONSISTENCY_AUDIT.md) после крупного контрактного PR.

**Промпт-якорь:** `Infra batch — protos policy and migration script`.

---

## Phase 17 — Stories

*MVP backend + partial Flutter; ниже — пробелы vs [stories.md](features/stories.md) и [PLAN.md](PLAN.md) §17 (не out-of-phase: AR, algorithmic feed, post-match auto-story, monetization).*

### Backend — events & integrations

- [ ] **`story.events` NATS publisher** — JetStream stream per [story-service.md](microservices/story-service.md): create, view, react, expire, highlight create, LFP create.
- [ ] **Mention notifications** — on story create with `mention_profile_ids` → Notification Service.
- [ ] **Reply-to-story → DM** — gRPC + Gateway: private reply opens/sends DM thread ([stories.md](features/stories.md) §Ответы).
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

### Frontend — UX & editor

- [ ] **Story create entry point** — shell affordance calling `StoriesRoutes.openCreate`.
- [ ] **Media picker** — wire `image_picker` / file upload in `story_create_screen.dart`.
- [ ] **Video playback** — replace viewer placeholder with player (≤60s).
- [ ] **Text story styling** — colored background / `text_style_json` minimal editor.
- [ ] **Highlights on profile** — mount `HighlightsSection`; archive → add-to-highlight for owner.
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

**Промпт-якорь:** `Phase 17 Stories — follow-ups from docs/TODO.md`.

---

## Phase 18 — Growth & Accessibility

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

## Гостевые аккаунты (guest account type)

_Аудит 2026-06 vs [auth-and-contacts.md](features/auth-and-contacts.md), [auth-service.md](microservices/auth-service.md), [privacy.md](features/privacy.md)._

**Уже есть (baseline):** `Register` с `guest:true`, JWT `account_type`, Gateway `X-Voice-Account-Type`, guest guards (CreateDM / StartCall / SendFriendInvitation / CreateSpace / JoinByInvite+`allow_guests`), `convert-guest` + NATS `user.guest_converted`, TTL sweeper; Flutter — auto guest on web, convert modal, save-account reminder, greyout, `isGuest` из JWT.

**Остаётся:** enforcement `allow_guest_dm` при **ответе** guest в существующем DM (Messaging); полный multiselect guest-audience для всех visibility-полей (v1: только `show_online_include_guests`).

### Backend — ограничения и lifecycle

- [x] **JWT `account_type` claim** — `regular` \| `guest` в access JWT (или общий lookup); без этого Go-сервисы не различают тип аккаунта.
- [x] **Block guest-initiated DM** — Chat `CreateDM` (+ при необходимости первый send): `PermissionDenied` для guest caller ([auth-and-contacts.md](features/auth-and-contacts.md) §«не может инициировать разговор»).
- [x] **Block guest-initiated calls** — Voice `StartCall` (DM audio/video) от guest account.
- [x] **Block guest friend requests** — Social `SendFriendInvitation` от guest account.
- [x] **Block guest self-join** — запрет `CreateSpace`, discover/join без invite; guest только `JoinByInvite` / явное приглашение.
- [ ] **Enforce `allow_guest_dm`** — если caller — guest, DM к получателю только при `allow_guest_dm=true`; инициация guest всё равно запрещена (docs: действия guest не зависят от настроек получателя). *CreateDM blocked; reply/send в существующем чате — ещё нет.*
- [x] **Space/chat `allow_guests` setting** — настройка входа гостям на уровне спейса/чата (schema + enforce при join/membership).
- [x] **Guest TTL 30d** — `last_online_at` + sweeper деактивации/удаления guest после 30 дней без онлайна ([auth-and-contacts.md](features/auth-and-contacts.md) §жизненный цикл).
- [x] **NATS `user.guest_converted`** — publish при `convert-guest` ([auth-service.md](microservices/auth-service.md)).

### Backend — privacy

- [ ] **Guest audience в visibility** — [privacy.md](features/privacy.md): опция «Гостевые аккаунты» для show_online / show_game_status / show_mm_rating / show_stories; v1: `show_online_include_guests` + фильтр presence; остальные поля — follow-up.
- [x] **Reconcile `allow_guest_dm` vs privacy model** — bool в proto vs мультиселект аудитории в спеке; зафиксировать канон в `docs/features/privacy.md` (addendum).

### Frontend

- [x] **Auto guest on web** — при открытии без сессии auto `registerGuest` (docs); сейчас только кнопка (`guest_entry_test` проверяет отсутствие auto).
- [x] **Guest convert-to-regular UI** — modal (email + password) → `POST /api/v1/auth/convert-guest`; не редирект в settings; optional email verification ([auth-and-contacts.md](features/auth-and-contacts.md) §convert-guest).
- [x] **Guest save-account reminder** — не показывать на первом входе; на повторных — напоминание зарегистрироваться, max 1×/сутки ([auth-and-contacts.md](features/auth-and-contacts.md) §напоминание).
- [x] **Guest action greyout** — disable DM compose, call, friend invite для guest (defense in depth).
- [x] **Server-side `isGuest`** — из JWT/`/me`, не только heuristic `guest_password` в secure storage (новое устройство / refresh).
- [ ] **Onboarding after guest register** — E2E: nickname → shell → onboarding step 1 coach-marks. *API live: `guest_onboarding_e2e_live_test`; widget coach-marks E2E — follow-up.*

### Tests & docs

- [x] **Integration tests guest restrictions** — CreateDM / StartCall / SendFriendInvitation / CreateSpace as guest → denied.
- [x] **Docs: guest onboarding flow** — [auth-and-contacts.md](features/auth-and-contacts.md) §Гостевая учётка: nickname → повторное напоминание → convert-guest modal.

**Follow-ups:** Messaging `allow_guest_dm` для guest reply; multiselect guest-audience для остальных visibility; compose DDL `space_bans` / migrate tool version для join-by-invite live; widget E2E onboarding coach-marks.

**Промпт-якорь:** `Guest accounts — restrictions and UX from docs/TODO.md`.

---

## QA follow-ups (web pass 2026-06)

- [ ] **Bundle emoji font for web** — ship Noto Color Emoji for reaction/emoji picker; avoid runtime Noto fallback warning.

---

## Сводка: что отдать агенту следующим

| Если готовы… | Дайте агенту |
|--------------|--------------|
| Закрыть E2E хвосты | **Phase 15 — E2E (остаток)** |
| Боты: backend hardening | **Phase 16 — Боты (остаток)** |
| Есть OAuth keys в `.env` | **Developer Portal** (внутри Phase 16) |
| Гостевые ограничения + UX | **Гостевые аккаунты** |
| Сторис / growth | **Phase 17** или **Phase 18** |
| Только доки/репо | **Инфраструктура и доки** |

Фазовые детали и критерии «готово» — [PLAN.md](PLAN.md) §15–18, [encryption.md](features/encryption.md), [bots.md](features/bots.md).
