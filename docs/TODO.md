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

## Batch 3 — Phase 15 E2E (хвосты)

Критерии приёмки §15 закрыты; ниже — усиления и тесты.

- [x] **Opt-out search hardening** — compose live + Flutter require search hit when Search healthy; skip on 503/500 degradation.
- [x] **Matcher `IncludeGuests` only** — stranger/friend denied, guest allow/deny tests in `matcher_test.go` + `TestGetPresence_IncludeGuestsOnly_GuestVisibleOthersDenied`.
- [x] **Key backup Flutter live** — `phase15_e2e_key_backup_live_test.dart` + `compose-e2e-live.sh`.

### Audit 2026-06-19 (verification)

- [ ] **Compose live not re-run locally** — gateway phase15 optout/key-backup and Flutter `phase15_e2e_*_live_test` require `make compose-e2e-live`; run before merge when stack available.

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

- [x] **Bot gRPC mTLS (prod)** — NetworkPolicy template [`deploy/templates/network-policy-voice-bot.yaml`](../deploy/templates/network-policy-voice-bot.yaml) + TLS/mTLS notes in [`DEPLOYMENT.md`](DEPLOYMENT.md). **Staging-blocked:** apply policy + service-mesh mTLS on prod cluster when CNI/mesh ready.
- [x] **Bot Service prod rollout** — skeleton [`deploy/prod/services.yaml`](../deploy/prod/services.yaml) + `bot_db` migrate Job template/doc in [`DEPLOYMENT.md`](DEPLOYMENT.md). **Staging-blocked:** first prod cutover (namespace secrets, GHCR `bot` image, run migrate Job on cluster).
- [x] **Developer Portal staging k8s** — [`deploy/staging/developer-portal.yaml`](../deploy/staging/developer-portal.yaml), Auth OAuth env on staging `voice-auth` (ConfigMap + services.yaml), doc in [`DEPLOYMENT.md`](DEPLOYMENT.md). **Staging-blocked:** DNS `developers.tastytest.online`, portal image in GHCR, manual apply until next deploy.
- [x] **Daily chat limit: increment-after-success** — `IncrementDailyChatCreates` after successful `CreateChat`; `TestCreateBotChat_failedCreateDoesNotConsumeQuota`.
- [x] **Stale `src/backend/bot/README.md`** — updated; points to `docs/microservices/bot-service.md`.
- [x] **Webhook E2E on staging** — opt-in `TestStagingPhase16BotsWebhook_live` (`VOICE_STAGING_API_URL`, `VOICE_STAGING_WEBHOOK_PING_URL`) in gateway. **Staging-blocked:** run against live staging with a public webhook echo URL reachable from Bot pod.

### Audit 2026-06-19 (verification)

- [ ] **Bot grpcsvc test duration** — `go test ./...` in `bot/internal/grpcsvc` ~336s on Windows host; watch CI wall time / consider `-short` split if job regresses.

**Промпт-якорь:** `Phase 16 bots from docs/TODO.md Batch 4`.

---

## Batch 5 — Phase 17 Stories

MVP backend + partial Flutter; пробелы vs [stories.md](features/stories.md) и PLAN §17. AR, algorithmic feed, post-match auto-story, monetization — out of phase.

### Backend — events & integrations

- [x] **Expiry worker → `story.expired` NATS** — `MarkExpiredStoriesReturning` + per-row `PublishStoryExpired` in expiry worker.
- [x] **`story_events_consumer` в Notification** — JetStream subscribe `story.>` on `story_events`; `StoryPusher` for `TypeMention`.
- [ ] **`story.lfp_created` → Matchmaking** — deferred per [story-service.md](microservices/story-service.md) (moved to Post-MVP below).
- [x] **File `context_story` lifecycle** — `story_id` in `file_db`; `RequestUpload` stores context; Flutter passes `context_story`.

### Backend — API & privacy

- [x] **User privacy policy** — `CreateStory` default from `ShowStoriesAudience` when visibility unset.
- [x] **`custom` / `close_friends` audiences** — `privacy.Matcher` + `visibility_audience` JSON; Flutter maps FoF → `close_friends`.
- [x] **Feed prefilter** — `ListActiveStoriesForAuthorsPaginated` via `FeedAuthors` (friends + self).
- [x] **Feed shape** — `groupStoriesByAuthorCentric` ordered by latest story per author.

### Frontend — UX

- [x] **Text story styling** — `textStyleJson` round-trip; viewer applies `VoiceColors` background.
- [x] **Highlights owner UX** — `StoryHighlightsScreen` + `HighlightEditSheet`; archive add-to-highlight.
- [x] **Owner archive screen** — `StoryArchiveScreen` + `/stories/archive`.
- [x] **Author viewers list** — tappable view count → `StoryViewersSheet` via `getViewers`.
- [x] **Game tag plashka** — `StoryGameTagChip` on non-LFP stories.
- [x] **Video duration cap ≤60s** — client pick validation + backend `CreateStory` file duration check.

### Tests

- [x] **Expiry E2E** — `jobs_batch5_test.go` integration for `MarkExpiredStoriesReturning`; compose degradation test stub.

### Post-MVP (не в Batch 5)

- [ ] **Story editor v2** — stickers, doodle, filters, clip trim ([stories.md](features/stories.md) §Редактор / §Клип).
- [ ] **`story.lfp_created` → Matchmaking subscriber** — auto-application from LFP story (deferred).
- [ ] **Feed space-member prefilter** — bulk space co-member author list (currently friends + self only).
- [ ] **Full per-story `PrivacyAudiencePicker`** — space multiselect on create (create uses privacy-derived default + simplified picker).
- [ ] **Story reactions UI** — backend exists; viewer emoji reactions not wired in Flutter.
- [ ] **Anonymous view (Premium)** — backend `MarkViewed.anonymous`; client UX deferred.
- [ ] **Compose expiry full chain live test** — worker → archive → purge → `DeleteFile` with `STORY_TTL_DEV` in compose.

**Промпт-якорь:** `Phase 17 Stories from docs/TODO.md Batch 5`.

---

## Batch 6 — Phase 18 Growth & Accessibility

PLAN §18 client/backend `[x]` — baseline; остаток vs [deep-links.md](features/deep-links.md), [onboarding.md](features/onboarding.md), [accessibility.md](features/accessibility.md).

### PLAN baseline vs spec

- [ ] **Onboarding one-liner** — PLAN: register → space → first message; код: coach-mark flow ([onboarding.md](features/onboarding.md)).
- [ ] **Приёмка #1 invite→join** — prod AASA/`assetlinks.json` на `voice.gg` (Gateway — dev placeholders); iOS associated-domains entitlement — placeholder в Runner, TEAMID перед prod.

### Deep links

- [ ] **Prod universal links** — real `voice.gg` AASA + `assetlinks.json` (Gateway — dev placeholders).
- [ ] **Mobile device E2E** — App Links / custom scheme Android/iOS.

### Audit 2026-06-19 (verification)

- [ ] **Well-known placeholders** — Gateway serves AASA `TEAMID.gg.voice.app` and assetlinks `PLACEHOLDER` SHA-256; iOS `Runner.entitlements` has associated-domains but needs real Team ID before prod ([`DEPLOYMENT.md`](DEPLOYMENT.md)).
- [ ] **Deep link test depth** — `phase18_deeplink_web_test.dart` is parser smoke only; no `integration_test` web driver or on-device App Links in CI.
- [ ] **Push → navigation** — `push_notification_handler_test.dart` covers `deep_link` parsing; no FCM/APNs device E2E to chat/message route.
- [ ] **A11y shortcuts** — `voice_shortcuts_test.dart` asserts focus-request providers only; Semantics / keyboard traversal still manual ([`accessibility.md`](features/accessibility.md)).

### Onboarding & a11y

- [ ] **Manual TalkBack / VoiceOver** — pre-release checklist.

**Промпт-якорь:** `Phase 18 Growth/A11y from docs/TODO.md Batch 6`.

---

## Batch 7 — Guest accounts (остаток)

Baseline закрыт (2026-06): register guest, JWT, guards, convert-guest, TTL, Flutter auto-guest/convert. См. [auth-and-contacts.md](features/auth-and-contacts.md).

- [x] **Messaging `allow_guest_dm`** — guest reply в существующем DM: `messaging_grpc.go` + `dm_guest_privacy_integration_test.go`; Flutter live `guest_restrictions_e2e_live_test.dart`.
- [ ] **Guest audience (Flutter settings UX)** — backend enforcement для `show_game_status` / `show_mm_rating` / `show_stories` через `IncludeGuests` (User presence, Matchmaking `rating_privacy`, Story guest test); в Flutter нет отдельного multiselect «Гостевые аккаунты» на каждое поле (только общий `include_guests` в `PrivacyAudiencePicker`).
- [ ] **Onboarding coach-marks E2E** — [x] widget якорей: `guest_onboarding_anchor_keys_test.dart`; [ ] полный tour step-through + live API `guest_onboarding_e2e_live_test` (compose).

### Audit 2026-06-19 (verification)

- [ ] **Auth phone-hash S2S** — `ResolvePhoneHashes` gRPC + Social `auth_phone_hash.go` unit-tested; staging phone-sync live (`compose_phase11_phone_sync_live_test`) not re-run locally.
- [ ] **Guest restrictions live** — `guest_restrictions_e2e_live_test.dart` not re-run (compose); run in `compose-e2e-live` before release.

**Промпт-якорь:** `Guest accounts from docs/TODO.md Batch 7`.

---

## Batch 9 — QA polish

- [x] **Bundle emoji font for web** — Noto Color Emoji in `assets/fonts/NotoColorEmoji.ttf`; `VoiceEmojiStyle` on reactions/picker; `test/voice_emoji_style_test.dart`.

### Audit 2026-06-19 (verification)

- [ ] **Flutter analyze** — `flutter analyze` reports 35 info/warnings (unused imports in `guest_entry_test.dart`, deprecated form/Radio APIs); not merge-blocking but clean up before release polish.

**Промпт-якорь:** `QA polish from docs/TODO.md Batch 9`.

---

## Сводка: что отдать агенту

| Приоритет / готовность | Batch |
|------------------------|-------|
| E2E хвосты | **Batch 3 — Phase 15** |
| Боты: deploy, hardening, live tests | **Batch 4 — Phase 16** |
| Сторис | **Batch 5 — Phase 17** |
| Deep links, onboarding, a11y | **Batch 6 — Phase 18** |
| Гостевые ограничения + UX | **Batch 7 — Guest accounts** |
| Web emoji font | **Batch 9 — QA** |

Фазовые критерии «готово» — [PLAN.md](PLAN.md) §11–18, [encryption.md](features/encryption.md), [bots.md](features/bots.md).
