# Пробелы и открытые вопросы (документация)



Здесь — **вне статуса реализации** [PLAN.md](PLAN.md). Критерии «готово» по фичам — `docs/features/`; открытые инженерные задачи — ниже.



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



Критерии фич: [encryption.md](features/encryption.md), [bots.md](features/bots.md), [stories.md](features/stories.md). Observability на staging — [observability.md](features/observability.md) §Definition of Done.



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



- [ ] **E2E encryption live** — gateway `compose_e2e_optout` / `compose_e2e_key_backup`; Flutter `e2e_*_live_test`.

- [ ] **Guest restrictions live** — `guest_restrictions_e2e_live_test.dart` в `compose-e2e-live`.

- [ ] **Staging deploy smoke** — при доступном кластере: `STAGING_SMOKE_ENABLED=true` → `scripts/staging/smoke-staging.sh` после деплоя ([`DEPLOYMENT.md`](DEPLOYMENT.md)); расширить smoke проверкой Developer Portal (`https://${VOICE_DEVELOPER_PORTAL_INGRESS_HOST}/`) — см. Batch 11.



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

- [ ] **Mobile device E2E** — App Links / custom scheme Android/iOS (сейчас только parser smoke в `deeplink_web_test.dart`).



**Промпт-якорь:** `Deep links prod/mobile from docs/TODO.md High Batch 3`.



### Batch 4 — Guest & phone verification



- [ ] **Auth phone-hash S2S live** — `compose_phone_sync_live_test` на живом стеке (unit-тесты `ResolvePhoneHashes` / `auth_phone_hash.go` есть).

- [ ] **Onboarding coach-marks E2E** — полный tour step-through + `guest_onboarding_e2e_live_test` (compose); widget-якоря покрыты `guest_onboarding_anchor_keys_test.dart`.



**Промпт-якорь:** `Guest/phone live verification from docs/TODO.md High Batch 4`.



---



## Common



### Batch 5 — Growth & Accessibility



Baseline onboarding/deep-links/a11y — [PLAN.md](PLAN.md); остаток vs [deep-links.md](features/deep-links.md), [onboarding.md](features/onboarding.md), [accessibility.md](features/accessibility.md).



**Промпт-якорь:** `Growth/A11y from docs/TODO.md Common Batch 5`.



### Batch 6 — Guest accounts UX



Baseline закрыт (2026-06): register guest, JWT, guards, convert-guest, TTL, Flutter auto-guest/convert. См. [auth-and-contacts.md](features/auth-and-contacts.md).



- [ ] **Guest audience (Flutter settings UX)** — backend enforcement для `show_game_status` / `show_mm_rating` / `show_stories` через `IncludeGuests`; в Flutter нет отдельного multiselect «Гостевые аккаунты» на каждое поле (только общий `include_guests` в `PrivacyAudiencePicker`).



**Промпт-якорь:** `Guest accounts UX from docs/TODO.md Common Batch 6`.



### Batch 11 — CI/CD и deploy automation



Аудит 2026-07: [`ci.yml`](../.github/workflows/ci.yml), [`staging-deploy.yml`](../.github/workflows/staging-deploy.yml), [`compose-e2e-live.yml`](../.github/workflows/compose-e2e-live.yml), [`DEPLOYMENT.md`](DEPLOYMENT.md). Developer Portal **собирается в CI** (job `developer-portal`, push в GHCR на `master`); ручная сборка на staging — из‑за того, что **автодеплой не доезжал** (последний успешный Staging deploy ~2026-07-02; недавние push отменяли CI / deploy skipped).



#### Path-filter CI refactor (merged 2026-07-07)



Рефакторинг: job `changes`, [`.github/ci/path-filters.yml`](../.github/ci/path-filters.yml), [`scripts/ci/resolve-go-matrix.sh`](../.github/ci/resolve-go-matrix.sh), тиры 1/2/3, `local-ci-parity` только nightly. В `master` уже есть **`aa76cd4`** (compose migrate через `host.docker.internal`).



- [x] **path-filters: SQL migrations** — добавить `src/backend/migrations/**` в `compose` (или `global`). Иначе PR только с миграциями: `code=true`, но tier 1 skipped и `compose-e2e` на `master` не идёт.

- [x] **path-filters: postgres init** — добавить `docker/postgres/**` в `compose` / `global` (init DDL, `ensure-compose-schema.sh`).

- [x] **path-filters: все workflows** — `.github/workflows/**` в `global` (сейчас только `ci.yml`). Иначе правки `staging-deploy.yml` / `compose-e2e-live.yml` не расширяют blast radius tier 1.

- [x] **Docker build на PR** — в WIP образы только tier 2 (`push` в `master`). Раньше на PR был `docker build` с `push: false`. Рассмотреть `docker build` без push для затронутого сервиса на PR — иначе сломанный `Dockerfile` всплывёт только после merge.

- [x] **Path-filter blast radius: кросс-сервис** — изменение одного `svc_*` не гоняет зависимые сервисы (напр. `messaging` → `chat`). Осознанный trade-off; при необходимости — deps map в `resolve-go-matrix.sh` или расширить `global` для S2S контрактов.

- [x] **Branch protection: required checks** — обновить список в GitHub после тиров: `local-ci-parity` только tier 3 (cron / `workflow_dispatch`), новые job names (`flutter-android-smoke`, `ci-skip-gate`, …). Skipped jobs обычно не блокируют merge — проверить настройки репо.

- [x] **staging-deploy vs path filters** — auto deploy (`workflow_run: CI success`) может сработать при minimal green CI без пересборки образов. Документировать или триггерить deploy только если в run были docker push jobs / явный SHA.

- [x] **Sanity после merge path-filter CI** — один `workflow_dispatch` → profile `full` (все тиры, все сервисы).

- [x] **compose-migrate-all: локальный :5432** — `aa76cd4` использует `host.docker.internal`; при конфликте порта на хосте — `VOICE_MIGRATE_PG_HOST` / `POSTGRES_PORT` (кратко в [`TESTING.md`](TESTING.md) § compose-e2e).



- [x] **Developer Portal на staging из CI** — после зелёного CI на `master`: дождаться auto `Staging deploy` (`STAGING_DEPLOY_ENABLED=true`) или `workflow_dispatch` с тегом **git SHA** (не только `latest`). Убедиться, что `scripts/staging/render-and-apply.sh` применил `developer-portal.yaml` и pod тянет `ghcr.io/.../developer-portal:<sha>`.

- [x] **Rollout wait portal** — в `render-and-apply.sh` добавить `kubectl rollout status deployment/voice-developer-portal` (сейчас ждёт только gateway, с `|| true`).

- [x] **Prod deploy workflow** — нет `.github/workflows/prod-deploy.yml`; в репо только skeleton [`deploy/prod/`](../deploy/prod/) (bot). Нужны: environment `production` + approval, variables prod FQDN, `render-and-apply-prod.sh` по аналогии со staging.

- [x] **Миграции БД в staging deploy** — `render-and-apply.sh` не запускает migrate Jobs (`bot_db`, `story_db`, …); шаблоны в [`deploy/templates/`](../deploy/templates/) — встроить idempotent apply перед rollout app tier.

- [x] **compose-e2e-live Flutter drift** — [`compose-e2e-live.yml`](../.github/workflows/compose-e2e-live.yml) pin **3.29.3**, [`ci.yml`](../.github/workflows/ci.yml) — **3.41.7**; выровнять (общий env или reusable workflow).

- [x] **compose-e2e coverage в default CI** — job `compose-e2e` в `ci.yml` гоняет smoke по всем фичам; полный `make compose-e2e-live` — nightly / manual workflow. Решить: required check / оставить opt-in (связано с Critical Batch 2).

- [x] **Staging k8s vs CI image matrix** — в GHCR пушатся `story`, `subscription`, `moderation`, `analytics`, `federation`, но в [`deploy/staging/`](../deploy/staging/) нет Deployment'ов и нет upstream'ов в `GATEWAY_GRPC_UPSTREAMS_JSON` (в compose есть). Либо добавить в staging stack, либо зафиксировать «не на staging» в [PLAN.md](PLAN.md).

- [x] **imagePullSecrets в манифестах** — template + apply в `render-and-apply.sh`, если GHCR private ([`DEPLOYMENT.md`](DEPLOYMENT.md) § Pull из GHCR).

- [x] **Observability не в deploy pipeline** — [`deploy/observability/`](../deploy/observability/) применяется вручную; опциональный шаг в staging-deploy или отдельный workflow после app deploy.

- [x] **Единый pin Flutter/Go/Java в workflows** — в WIP `ci.yml` уже `env:` (`GO_VERSION`, `FLUTTER_VERSION`, …); закоммитить с path-filter refactor и выровнять `compose-e2e-live.yml` (см. drift ниже). Опционально: composite action.

- [x] **Ручной deploy tag `latest`** — `workflow_dispatch` default `latest`, auto deploy — SHA; документировать риск рассинхрона при partial failed matrix push.



**Промпт-якорь:** `CI/CD deploy automation from docs/TODO.md Common Batch 11`.



---



## Low / post-MVP



### Batch 7 — Stories (post-MVP)



MVP backend + partial Flutter; AR, algorithmic feed, post-match auto-story, monetization — вне MVP ([stories.md](features/stories.md)).



- [ ] **Story editor v2** — stickers, doodle, filters, clip trim (§Редактор / §Клип).

- [ ] **`story.lfp_created` → Matchmaking subscriber** — auto-application from LFP story (deferred per [story-service.md](microservices/story-service.md)).

- [ ] **Feed space-member prefilter** — bulk space co-member author list (сейчас friends + self only).

- [ ] **Full per-story `PrivacyAudiencePicker`** — space multiselect on create.

- [ ] **Story reactions UI** — backend есть; emoji reactions в Flutter viewer не подключены.

- [ ] **Anonymous view (Premium)** — backend `MarkViewed.anonymous`; client UX отложен.

- [ ] **Compose expiry full chain live test** — worker → archive → purge → `DeleteFile` с `STORY_TTL_DEV` в compose.



**Промпт-якорь:** `Stories post-MVP from docs/TODO.md Low Batch 7`.



### Batch 10b — Ops tech debt (magic numbers)



- [ ] **BOT_WEBHOOK_* retry/backoff** — hardcoded retry/backoff webhook delivery → env.

- [ ] **NATS_* connect/reconnect** — Realtime: connect/reconnect timeouts в конфиг.

- [ ] **pgxpool max conns** — лимиты пула Postgres → env/ConfigMap.

- [ ] **BOT_RATE_LIMIT_* / Gateway rate-limit JSON** — staging defaults в ConfigMap (сейчас только dev bypass).



**Промпт-якорь:** `Magic numbers config from docs/TODO.md Low Batch 10b`.



### Batch 12 — Phase→features: хвосты миграции (2026-07)

После переименования тестов и slim [PLAN.md](PLAN.md) остались ссылки на «фазы» и артефакты bulk-replace.

- [x] **Доки microservices/features** — убрать «Фаза N» / `app stackN` → ссылки на `docs/features/*.md`: [api-gateway.md](microservices/api-gateway.md), [auth-service.md](microservices/auth-service.md), [user-service.md](microservices/user-service.md), [chat-service.md](microservices/chat-service.md), [messaging-service.md](microservices/messaging-service.md), [realtime-service.md](microservices/realtime-service.md), [file-service.md](microservices/file-service.md), [bot-service.md](microservices/bot-service.md), [primary-profile-bootstrap.md](microservices/primary-profile-bootstrap.md), [friends.md](features/friends.md), [privacy.md](features/privacy.md), [encryption.md](features/encryption.md), [CONTRACT_MATRIX.md](CONTRACT_MATRIX.md), [OPERATIONS.md](OPERATIONS.md), [PROJECT.md](PROJECT.md), [design/brand.md](design/brand.md).
- [x] **DATA_SCOPE_V1.md §3–4** — дочистить таблицы/абзацы с «Фаза 1/3/9/12/13», «PLAN Фаза 0» (§1–2 уже на фичах).
- [x] **DATA_STORES.md, backend README** — «Phase 0–1» / `app stack5` → feature names.
- [x] **deploy/staging** — комментарии в [secret.example.yaml](../deploy/staging/secret.example.yaml), [gateway-deployment.yaml](../deploy/staging/gateway-deployment.yaml) (`phases 0–10` → full app stack).
- [x] **src/frontend/README.md** — «Phases 0–10», «Phase 8» → [PLAN.md](PLAN.md) / [platforms.md](features/platforms.md).
- [x] **Комментарии в src/** — остатки `Phase N` в [voice_client.dart](../src/frontend/lib/backend/voice_client.dart), [game_catalog_screen.dart](../src/frontend/lib/ui/matchmaking/game_catalog_screen.dart), doc-комментарии в live-тестах (`/// flutter test test/phase…`).
- [x] **Сгенерированные stubs** — после финальной правки `protos/`: `make buf-generate` + `make buf-generate-dart` (или `make buf-dart-check`); сейчас в `lib/gen/**` и части `*/pb/**` ещё старые `// Phase N:` из proto.
- [x] **Проверка CI-имен** — `rg 'phase\d+_|compose_phase|TestComposePhase'` по репо (исключая `migrations/`, `CallPhase`, `*.pbxproj`); убедиться, что [e2e-features.yml](../.github/ci/e2e-features.yml) и все README со скриптами ссылаются только на новые пути.
- [x] **Локальная верификация** — `make compose-e2e-smoke`, `make build-all` (в сессии миграции `go test` gateway падал на TLS к proxy.golang.org, не на код).

**Промпт-якорь:** `Phase→features tail cleanup from docs/TODO.md Batch 12`.

#### Batch 12 — follow-up (после merge / перед CI)

- [ ] **Закоммитить diff Batch 12** — ~120 файлов: `docs/**`, `src/backend/*/pb/**/*.pb.go`, `src/frontend/lib/gen/**`, deploy/staging, правки `src/`. Без коммита job **`flutter`** упадёт на **`make buf-dart-check`** (drift `lib/gen` vs proto-комментарии).
- [ ] **`buf-ci` / line endings `protos/`** — `make build-all` локально упал на `buf format -d --exit-code` (CRLF на Windows vs LF в Docker/Linux CI). Проверить: `buf format -w protos/`; при необходимости `*.proto text eol=lf` в `.gitattributes` и re-normalize. Иначе job **`protobuf`** / `buf-ci` может краснеть после push с Windows.
- [ ] **Sync Go `pb/` после `make buf-generate`** — таргета в Makefile нет; копирование `gen/go/voice/**` → `src/backend/*/pb/voice/**` вручную (в сессии — 92 файла). Добавить `scripts/dev/sync-pb-from-gen.sh` + упоминание в [REPOSITORIES.md](REPOSITORIES.md) / Makefile, чтобы не забыть при следующем proto-change.
- [ ] **Локальная верификация (ещё не зелёная)** — `make compose-e2e-smoke` не гоняли; `go test` gateway на хосте — TLS к `proxy.golang.org` (среда, не код). Перед merge: CI или Linux/WSL + compose smoke.
- [ ] **`app stackN` / `Phase N` в `src/**` (вне gen)** — `rg 'app stack\d'` ~100+ (комментарии тестов, UI, service README): [integration_test/README.md](../src/frontend/integration_test/README.md), [matchmaking/README.md](../src/backend/matchmaking/README.md), [role/README.md](../src/backend/role/README.md), [admin/README.md](../src/admin/README.md), [ping-bot/README.md](../scripts/dev/ping-bot/README.md), [deploy/prod/README.md](../deploy/prod/README.md), `pubspec.yaml`, `firebase-messaging-sw.js`, dart clients (`notifications_client`, `roles_client`, …). Дочистить → ссылки на `docs/features/*.md` (как в Batch 12 для доков).
- [ ] **Имена golang-migrate файлов** — `000002_phase14_sanctions`, `000004_phase13_profiles_verification`, `000002_phase13_verification_type` (ссылки в тестах). Переименование опционально; не блокер CI, но шумит в `rg phase\d+_`.

**Промпт-якорь:** `Batch 12 follow-up from docs/TODO.md`.



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

| **Common** | Batch 11 | CI/CD, staging/prod deploy |

| **Low** | Batch 7 | Stories post-MVP |

| **Low** | Batch 8 | Bots CI (`-short` + nightly) — done |

| **Low** | Batch 9 | Flutter analyze — done |

| **Low** | Batch 10 | Runtime timeouts — done |

| **Low** | Batch 10b | Magic numbers → env/ConfigMap |

| **Common** | Batch 12 | Phase→features: хвосты миграции (доки, gen, rg) — done; follow-up: commit, buf-ci CRLF, src app stack |

