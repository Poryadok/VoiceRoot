# Пробелы и открытые вопросы (документация)

Здесь — **вне дорожной карты** [PLAN.md](PLAN.md). Продуктовые и инфраструктурные чеклисты по фазам, остаток Фазы 0 (Auth: smoke контейнера в CI, порядок миграций `auth_db`), UUIDv7 для сообщений — в плане.

## Как пользоваться

| Метка | Кому | Смысл |
|-------|------|--------|
| **Agent batch** | Cursor / агент | Один PR или одна сессия: общий контекст, TDD, без ваших секретов |
| **Вы** | Человек | Решение, ключи, аккаунт, юридическое — агент ждёт ввод или не может закрыть |

**Порядок работы:** сверху вниз — сначала **Critical**, затем High → Common → Low. Внутри Critical сначала «Только вы», потом agent batch целиком (копируйте заголовок секции в промпт).

**Выполненные пункты:** удалять из этого файла **целиком** (пункт, подпункт, пустую секцию batch). **Не** помечать `[x]`, не оставлять зачёркивания и «сделано» — список только открытого. Исключение: чеклисты в других `docs/` (например [observability.md](features/observability.md)) ведутся по своим правилам.

**Приоритеты:**

| Уровень | Смысл |
|---------|--------|
| **Critical** | Блокирует софт-ланч на staging: секреты, observability, обязательные live E2E |
| **High** | Prod/mobile, deep links, регрессия гостей — до или сразу после первых пользователей |
| **Common** | Verification и UX-дыры, не ломают Tier 0 (DM + WS) |
| **Low** | Post-MVP, polish, техдолг |

Фазовые критерии «готово» — [PLAN.md](PLAN.md) §11–18, [encryption.md](features/encryption.md), [bots.md](features/bots.md). Критерии observability на staging — [observability.md](features/observability.md) §Definition of Done.

---

## Critical — до софт-ланча

### Только вы (не делегировать агенту целиком)

- [ ] **Секреты staging k8s** — `voice-app-secrets` по [`deploy/staging/secret.example.yaml`](../deploy/staging/secret.example.yaml): JWT, Postgres URLs, R2 (`FILE_R2_*`, `USER_R2_*`), FCM/APNs для Notification ([`DEPLOYMENT.md`](DEPLOYMENT.md)).
- [ ] **Observability: канал алертов** — Secret уведомлений (Telegram bot или email) для Alertmanager; без него P1-алерты уходят в null receiver ([`deploy/observability/README.md`](../deploy/observability/README.md)).

### Batch 1 — Observability staging

Проверки на живом кластере после `apply-observability.sh`; спека — [observability.md](features/observability.md).

- [ ] **Loki: все поды** — приложение + infra пишут в Loki (`kubectl get pods -n voice-observability` — Running).
- [ ] **Трассировка `request_id`** — E2E: отправка DM на staging → цепочка Gateway → gRPC → NATS → `ws_fanout` в Loki ([`TESTING.md`](TESTING.md) § Debug by request_id).
- [ ] **Grafana smoke** — Overview: targets UP; дашборды Overview / Tier-0 / Infra / Logs открываются.
- [ ] **P1 алерты** — правила активны; тестовый firing → сообщение в канал (не null receiver).
- [ ] **Prometheus scrape** — `gateway_http_requests_total` растёт при трафике на staging.

**Промпт-якорь:** `Observability staging smoke from docs/TODO.md Critical Batch 1`.

### Batch 2 — E2E verification перед релизом

Требует `make compose-e2e-live` или эквивалентный живой стек ([`TESTING.md`](TESTING.md)).

- [ ] **Phase 15 E2E live** — gateway phase15 optout/key-backup; Flutter `phase15_e2e_*_live_test`.
- [ ] **Guest restrictions live** — `guest_restrictions_e2e_live_test.dart` в `compose-e2e-live`.
- [ ] **Staging deploy smoke** — при доступном кластере: `STAGING_SMOKE_ENABLED=true` → `scripts/staging/smoke-staging.sh` после деплоя ([`DEPLOYMENT.md`](DEPLOYMENT.md)).

**Промпт-якорь:** `Pre-release compose/staging E2E from docs/TODO.md Critical Batch 2`.

---

## High

### Только вы — prod / mobile (если в scope софт-ланча)

- [ ] **Prod universal links** — реальные AASA + `assetlinks.json` на `voice.gg` (сейчас Gateway — dev placeholders).
- [ ] **iOS Team ID** — `Runner.entitlements` associated-domains: заменить `TEAMID`; SHA-256 в assetlinks вместо `PLACEHOLDER` ([`DEPLOYMENT.md`](DEPLOYMENT.md)).
- [ ] **Firebase / FCM prod** — `google-services.json`, web config в CI secrets; FlutterFire для staging/prod клиента.

### Batch 3 — Deep links & mobile acceptance

- [ ] **Приёмка invite→join** — universal link открывает приложение / web fallback ([deep-links.md](features/deep-links.md)).
- [ ] **Well-known на prod** — Gateway отдаёт валидные `/.well-known/apple-app-site-association` и `assetlinks.json` для целевого домена.
- [ ] **Mobile device E2E** — App Links / custom scheme Android/iOS (сейчас только parser smoke в `phase18_deeplink_web_test.dart`).

**Промпт-якорь:** `Deep links prod/mobile from docs/TODO.md High Batch 3`.

### Batch 4 — Guest & phone verification

- [ ] **Auth phone-hash S2S live** — `compose_phase11_phone_sync_live_test` на живом стеке (unit-тесты `ResolvePhoneHashes` / `auth_phone_hash.go` есть).
- [ ] **Onboarding coach-marks E2E** — полный tour step-through + `guest_onboarding_e2e_live_test` (compose); widget-якоря покрыты `guest_onboarding_anchor_keys_test.dart`.

**Промпт-якорь:** `Guest/phone live verification from docs/TODO.md High Batch 4`.

---

## Common

### Batch 5 — Phase 18 Growth & Accessibility

PLAN §18 baseline `[x]`; остаток vs [deep-links.md](features/deep-links.md), [onboarding.md](features/onboarding.md), [accessibility.md](features/accessibility.md).

- [x] **Onboarding one-liner** — канон: 5 coach-mark шагов [onboarding.md](features/onboarding.md); PLAN one-liner сведён к onboarding.md; step 3 «Find a space» → global search (`onboarding_overlay_test.dart`).
- [x] **Push → navigation** — `push_deep_link_navigation_test.dart`: tap payload `navigateToChat` → chat/message via `applyDeepLinkNavigation`; FCM/APNs on-device E2E — вне CI.
- [x] **Deep link test depth** — `integration_test/phase18_deeplink_web_test.dart` + CI job `flutter-web-integration` (Chrome); prod App Links / AASA → Batch 6.
- [x] **A11y shortcuts** — `voice_shortcuts_keyboard_test.dart`: Ctrl+K, Escape, Alt+↑/↓, Enter → context menu; PTT configurable key — v1 gap.
- [x] **Manual TalkBack / VoiceOver** — pre-release checklist в [accessibility.md](features/accessibility.md) §Testing.

**Промпт-якорь:** `Phase 18 Growth/A11y from docs/TODO.md Common Batch 5`.

### Batch 6 — Guest accounts UX

Baseline закрыт (2026-06): register guest, JWT, guards, convert-guest, TTL, Flutter auto-guest/convert. См. [auth-and-contacts.md](features/auth-and-contacts.md).

- [ ] **Guest audience (Flutter settings UX)** — backend enforcement для `show_game_status` / `show_mm_rating` / `show_stories` через `IncludeGuests`; в Flutter нет отдельного multiselect «Гостевые аккаунты» на каждое поле (только общий `include_guests` в `PrivacyAudiencePicker`).

**Промпт-якорь:** `Guest accounts UX from docs/TODO.md Common Batch 6`.

---

## Low / post-MVP

### Batch 7 — Phase 17 Stories (post-MVP)

MVP backend + partial Flutter; AR, algorithmic feed, post-match auto-story, monetization — out of phase ([stories.md](features/stories.md), PLAN §17).

- [ ] **Story editor v2** — stickers, doodle, filters, clip trim (§Редактор / §Клип).
- [ ] **`story.lfp_created` → Matchmaking subscriber** — auto-application from LFP story (deferred per [story-service.md](microservices/story-service.md)).
- [ ] **Feed space-member prefilter** — bulk space co-member author list (сейчас friends + self only).
- [ ] **Full per-story `PrivacyAudiencePicker`** — space multiselect on create.
- [ ] **Story reactions UI** — backend есть; emoji reactions в Flutter viewer не подключены.
- [ ] **Anonymous view (Premium)** — backend `MarkViewed.anonymous`; client UX отложен.
- [ ] **Compose expiry full chain live test** — worker → archive → purge → `DeleteFile` с `STORY_TTL_DEV` в compose.

**Промпт-якорь:** `Phase 17 Stories post-MVP from docs/TODO.md Low Batch 7`.

### Batch 8 — Phase 16 Bots (CI)

- [ ] **Bot grpcsvc test duration** — `go test ./...` в `bot/internal/grpcsvc` ~336s на Windows host; следить за wall time в CI / `-short` split при регрессии.

**Промпт-якорь:** `Phase 16 bots CI from docs/TODO.md Low Batch 8`.

### Batch 9 — QA polish

- [ ] **Flutter analyze** — ~35 info/warnings (unused imports в `guest_entry_test.dart`, deprecated form/Radio APIs); не merge-blocking, убрать перед release polish.

**Промпт-якорь:** `QA polish from docs/TODO.md Low Batch 9`.

### Batch 10 — Ops tech debt

- [ ] **Таймауты в конфиг** — хардкод вроде `context.WithTimeout(..., 15*time.Second)` и HTTP `ReadTimeout`/`WriteTimeout` в `main.go` сервисов: вынести в env/ConfigMap, единый стиль с [`OPERATIONS.md`](OPERATIONS.md).
- [ ] **Магические числа** — пройти backend/frontend: константы в коде vs настраиваемые лимиты (rate limit, pool size, retry backoff) — часть в конфиги.

**Промпт-якорь:** `Timeouts and config cleanup from docs/TODO.md Low Batch 10`.

---

## Сводка: приоритет → batch

| Приоритет | Batch | Тема |
|-----------|-------|------|
| **Critical** | Только вы | Секреты staging, алерты |
| **Critical** | Batch 1 | Observability staging |
| **Critical** | Batch 2 | Compose / staging E2E перед релизом |
| **High** | Только вы | Prod links, iOS Team ID, FCM prod |
| **High** | Batch 3 | Deep links & mobile acceptance |
| **High** | Batch 4 | Guest / phone live verification |
| **Common** | Batch 5 | Growth, push, a11y |
| **Common** | Batch 6 | Guest UX |
| **Low** | Batch 7 | Stories post-MVP |
| **Low** | Batch 8 | Bots CI time |
| **Low** | Batch 9 | Flutter analyze |
| **Low** | Batch 10 | Таймауты и magic numbers |
