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

## Batch 3 — Phase 15 E2E (verification)

- [ ] **Compose live not re-run locally** — gateway phase15 optout/key-backup and Flutter `phase15_e2e_*_live_test` require `make compose-e2e-live`; run before merge when stack available.

**Промпт-якорь:** `Phase 15 E2E follow-ups from docs/TODO.md Batch 3`.

---

## Batch 4 — Phase 16 Bots (verification)

- [ ] **Bot grpcsvc test duration** — `go test ./...` in `bot/internal/grpcsvc` ~336s on Windows host; watch CI wall time / consider `-short` split if job regresses.

**Промпт-якорь:** `Phase 16 bots from docs/TODO.md Batch 4`.

---

## Batch 5 — Phase 17 Stories

MVP backend + partial Flutter; пробелы vs [stories.md](features/stories.md) и PLAN §17. AR, algorithmic feed, post-match auto-story, monetization — out of phase.

### Post-MVP (не в Batch 5)

- [ ] **Story editor v2** — stickers, doodle, filters, clip trim ([stories.md](features/stories.md) §Редактор / §Клип).
- [ ] **`story.lfp_created` → Matchmaking subscriber** — auto-application from LFP story (deferred per [story-service.md](microservices/story-service.md)).
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

- [ ] **Guest audience (Flutter settings UX)** — backend enforcement для `show_game_status` / `show_mm_rating` / `show_stories` через `IncludeGuests` (User presence, Matchmaking `rating_privacy`, Story guest test); в Flutter нет отдельного multiselect «Гостевые аккаунты» на каждое поле (только общий `include_guests` в `PrivacyAudiencePicker`).
- [ ] **Onboarding coach-marks E2E** — widget якорей есть (`guest_onboarding_anchor_keys_test.dart`); полный tour step-through + live API `guest_onboarding_e2e_live_test` (compose) — нет.

### Audit 2026-06-19 (verification)

- [ ] **Auth phone-hash S2S** — `ResolvePhoneHashes` gRPC + Social `auth_phone_hash.go` unit-tested; staging phone-sync live (`compose_phase11_phone_sync_live_test`) not re-run locally.
- [ ] **Guest restrictions live** — `guest_restrictions_e2e_live_test.dart` not re-run (compose); run in `compose-e2e-live` before release.

**Промпт-якорь:** `Guest accounts from docs/TODO.md Batch 7`.

---

## Batch 9 — QA polish (verification)

- [ ] **Flutter analyze** — `flutter analyze` reports 35 info/warnings (unused imports in `guest_entry_test.dart`, deprecated form/Radio APIs); not merge-blocking but clean up before release polish.

**Промпт-якорь:** `QA polish from docs/TODO.md Batch 9`.

---

## Сводка: что отдать агенту

| Приоритет / готовность | Batch |
|------------------------|-------|
| E2E verification (compose live) | **Batch 3 — Phase 15** |
| Bot grpcsvc CI wall time | **Batch 4 — Phase 16** |
| Сторис Post-MVP | **Batch 5 — Phase 17** |
| Deep links, onboarding, a11y | **Batch 6 — Phase 18** |
| Гостевые ограничения + UX | **Batch 7 — Guest accounts** |
| Flutter analyze cleanup | **Batch 9 — QA** |

Фазовые критерии «готово» — [PLAN.md](PLAN.md) §11–18, [encryption.md](features/encryption.md), [bots.md](features/bots.md).
