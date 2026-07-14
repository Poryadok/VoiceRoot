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



- [ ] **Секреты staging k8s** — `voice-app-secrets` по [`deploy/staging/secret.example.yaml`](../deploy/staging/secret.example.yaml): JWT, Postgres URLs, R2 (`FILE_R2_*`, `USER_R2_*`), FCM/APNs для Notification, **Analytics** (`CLICKHOUSE_DSN`, `ANALYTICS_ID_HASH_KEY`) ([`DEPLOYMENT.md`](DEPLOYMENT.md)).

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



- [ ] **E2E encryption live** — gateway `compose_e2e_optout` / `compose_e2e_key_backup`; Flutter `encryption_*_e2e_live_test`.

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

- [ ] **Convert-guest: recovery для аккаунтов после бага transport-пароля** — аккаунты, сконвертированные до фикса (2026-07), остались с неизвестным паролем; нужен self-service reset password или support-runbook.
- [ ] **Convert-guest: док auth-service.md** — явно описать, что `password` в `ConvertGuest` = новый пароль regular-аккаунта (JWT гостя достаточен), не проверка transport-пароля.
- [ ] **Convert-guest live в compose-e2e** — `TestComposeConvertGuest_live` (новый пароль + login) в CI workflow; сейчас только opt-in локально.
- [ ] **Convert-guest: localized errors в GuestConvertSheet** — client validation и API (`validation_failed`, `invalid_credentials`, `rate_limited`) через `authErrorMessage`; `convertGuest` в контроллере — `resolveAuthErrorKey` по `errorCode`, как в `_authenticate`.
- [ ] **Convert-guest: negative Auth integration tests** — duplicate email, password <8, non-guest token, missing email/phone; дополнить `ConvertGuestIntegrationTest`.
- [ ] **Convert-guest: NATS `user.guest_converted`** — довести `GuestConvertNatsEventIntegrationTest`: REST convert + assert publish (сейчас stub).
- [ ] **Guest save-account reminder: server last-shown** — в спеке «локальный или серверный timestamp»; сейчас только `SharedPreferences` (кросс-устройство не синхронизируется).



- [ ] **Guest audience (Flutter settings UX)** — backend enforcement для `show_game_status` / `show_mm_rating` / `show_stories` через `IncludeGuests`; в Flutter нет отдельного multiselect «Гостевые аккаунты» на каждое поле (только общий `include_guests` в `PrivacyAudiencePicker`).



**Промпт-якорь:** `Guest accounts UX from docs/TODO.md Common Batch 6`.



### Batch 11 — CI/CD и deploy automation



Аудит 2026-07 и **2026-07-12** (selective build/deploy): [`ci.yml`](../.github/workflows/ci.yml), [`staging-deploy.yml`](../.github/workflows/staging-deploy.yml), [`compose-e2e-live.yml`](../.github/workflows/compose-e2e-live.yml), [`DEPLOYMENT.md`](DEPLOYMENT.md). Developer Portal **собирается в CI** (job `developer-portal`, push в GHCR на `master`); ручная сборка на staging — из‑за того, что **автодеплой не доезжал** (последний успешный Staging deploy ~2026-07-02; недавние push отменяли CI / deploy skipped). Детали аудита 2026-07-12 — подсекция ниже.



#### Path-filter CI refactor (merged 2026-07-07)



Рефакторинг: job `changes`, [`.github/ci/path-filters.yml`](../.github/ci/path-filters.yml), [`scripts/ci/resolve-go-matrix.sh`](../scripts/ci/resolve-go-matrix.sh), тиры 1/2/3, `local-ci-parity` только nightly. В `master` уже есть **`aa76cd4`** (compose migrate через `host.docker.internal`).



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

- [ ] **DNS staging FQDNs (ops)** — Cloudflare **A** для `app`, `admin`, `livekit` (плюс уже `voice` / `developers`) на IP ingress-ноды; для `livekit` — **DNS only** (grey cloud). Firewall: **30881/TCP**, **30882/UDP** на ноде. GitHub Variables: `VOICE_WEB_INGRESS_HOST`, `VOICE_ADMIN_INGRESS_HOST`, `VOICE_LIVEKIT_INGRESS_HOST`, `VOICE_APPLY_OBSERVABILITY=true`, `STAGING_SMOKE_ENABLED=true`, secret `GRAFANA_ADMIN_PASSWORD`, `LIVEKIT_API_KEY`/`SECRET` в `STAGING_APP_SECRETS_YAML`.



#### CI/CD audit 2026-07-12 (реализация + критика плана)

Полный разбор: path-filter на PR работает, на `master` и staging deploy — почти нет. Любой code-push → `staging-images-push` всех 19 Go-сервисов + auth/web/admin/portal → `workflow_run` → `render-and-apply.sh` + `rollout-app-tier.sh` (restart всего стека). Пункт «staging-deploy vs path filters» выше помечен [x], но реализована только `verify-staging-images.sh` (все образы на SHA), не selective build/deploy.

**Диагноз (зафиксировать в голове):**

- Контракт auto-deploy: [`verify-staging-images.sh`](../scripts/staging/verify-staging-images.sh) требует **все** образы стека с одним тегом → вынуждает [`staging-images-push`](../.github/workflows/ci.yml) пушить всё на каждый code-push в `master`, игнорируя job `changes`.
- Один `__IMAGE_TAG__` в [`deploy/staging/`](../deploy/staging/) — нет per-service selective deploy.
- [`path-filters.yml`](../.github/ci/path-filters.yml): `global` включает `deploy/**`; `code` = `**` минус `docs/**` → правки README/`.cursor/**` триггерят tier 2 на master.
- Jobs `backend-auth`, `web`, `admin`, `developer-portal`: условие `push && master` в `if` — сборка на master даже без изменений путей.
- Tier 2 (compose-e2e, platform Flutter) не required на PR → регрессии ловятся после merge.
- Single-node staging → `rollout-app-tier.sh` scale 0→1 / full restart — operational debt в pipeline.
- Два источника правды: [`scripts/ci/staging-go-services.txt`](../scripts/ci/staging-go-services.txt) vs [`deploy/staging/services.yaml`](../deploy/staging/services.yaml).

**P0 — selective build/deploy (высокий эффект):**

- [x] **Selective image push на master** — `build_go_services` / `staging-images-push` + `staging-images-promote`.
- [x] **Selective staging deploy** — `DEPLOY_MODE`, `deploy-changed.sh`, `rollout-subset.sh`.
- [x] **Сузить path-filter `code`** — `repo_meta`; `deploy/**` убран из `global`.
- [x] **Убрать `push && master` bypass** — `run_auth` / `run_web` / `run_admin` / `run_developer_portal`.

**P1 — качество gate и связность workflows:**

- [x] **Integration tests на PR для changed services** — `backend-go-integration-pr`.
- [x] **Meta-job `ci-gate`** — `verify-required-jobs.sh`.
- [x] **`workflow_call` вместо `workflow_run`** — `deploy-staging` в `ci.yml`.
- [x] **Дедуп Flutter на master** — `flutter-windows` только build; `run_flutter_tier2`.
- [ ] **Дедуп frontend Docker build** — отложено.

**P2 — архитектура deploy:**

- [x] **Stack manifest / lockfile** — `stack.lock.yaml` artifact + `staging-image-catalog.json`.
- [x] **Разделить infra deploy и app deploy** — `apply-infra.sh`, `apply-app-manifests.sh`.
- [x] **Убрать mutable `:latest`** — только SHA в CI.
- [ ] **Helm/Kustomize + GitOps** — отложено.
- [x] **Prod deploy: полный стек** — `deploy/prod/` + `prod-deploy.yml`.

**P3 — документация и план:**

- [x] **TESTING.md tier 2** — selective build/promote, `ci-gate`.
- [x] **DEPLOYMENT.md** — stack lockfile, deploy modes.
- [ ] **Пересмотреть цель continuous full-stack deploy** — частично (selective deploy); GitOps позже.
- [x] **Синхронизация staging-go-services.txt** — `generate-staging-go-services.sh`.

**Промпт-якорь:** `CI/CD selective build deploy from docs/TODO.md Batch 11 audit 2026-07-12`.



#### Post-commit audit 6498c89 (2026-07-12) — хвосты после `ci improvements`

Коммит **`6498c89`**: selective build/promote, `ci-gate`, `workflow_call` deploy, prod stack, `staging-image-catalog.json`. Закрыто в follow-up (ci-gate/web, stack.lock deploy, catalog verify, path-filters, promote BASE_SHA, user/space subset rollout, docs). Ниже — оставшийся техдолг и ops.

**Осознанный техдолг / trade-off (зафиксировать, не забыть):**

- [ ] **Promote bootstrap** — первый push в пустой GHCR, force-push, squash-merge → `BASE_TAG` без образов, CI падает на promote (нужен bootstrap full build или `STAGING_FORCE_FULL_ROLLOUT=true`).
- [ ] **`apply-app-manifests` всегда scale auth 0→1** — даже selective deploy; downtime Auth на каждый app apply ([`apply-app-manifests.sh`](../scripts/staging/apply-app-manifests.sh)).
- [ ] **Tier 2 не блокирует PR** — `compose-e2e`, platform Flutter smokes только master / `full`; регрессии после merge ([`branch-protection-checklist.md`](../.github/ci/branch-protection-checklist.md)).
- [ ] **Двойная сборка Flutter web на master** — tier 1 `flutter` (analyze+test) + job `web` (`flutter build web` + Docker); дедуп только для `flutter-windows` ([`ci.yml`](../.github/workflows/ci.yml)).
- [ ] **Дедуп frontend Docker build** — отложено (admin/developer-portal: npm build + docker build).
- [ ] **Helm/Kustomize + GitOps** — отложено; ordered rollout остаётся в bash на runner.
- [ ] **Prod reuse staging ops scripts** — [`render-and-apply-prod.sh`](../scripts/prod/render-and-apply-prod.sh) → `rollout-app-tier.sh`, `deploy-changed.sh`, `apply-observability.sh`, `ensure-app-secrets.sh` (алиасы `PROD_*` → `STAGING_*`).
- [ ] **Prod placeholders** — [`deploy/prod/domains.defaults`](../deploy/prod/domains.defaults) `*.voice.example.com`; secrets checklist только в README, не в ops TODO Critical.

**Ops / настройка GitHub (человек):**

- [x] **Branch protection: включить `ci-gate`** — Settings → master → required check **`ci-gate`** ([`branch-protection-checklist.md`](../.github/ci/branch-protection-checklist.md)); без этого skipped jobs формально не блокируют merge.
- [ ] **Sanity после selective CI** — один `workflow_dispatch` CI → `full`; первый master push с selective promote — проверить GHCR bootstrap; при необходимости `STAGING_FORCE_FULL_ROLLOUT=true` + manual deploy `deploy_mode=full`.
- [ ] **DNS staging FQDNs** — см. пункт выше в Batch 11 (ещё открыт).

**Промпт-якорь:** `Post-commit CI audit 6498c89 from docs/TODO.md Batch 11`.



#### Post-commit audit c3598f3 (2026-07-12) — хвосты после `still working on CI`

Коммит **`c3598f3`**: promote без ожидания frontend jobs, `ci-gate`+`web`, path-filter `deploy/**`→`compose`, `rollout-user-space-tier.sh`, stack-lock drift check, prod smoke, `VOICE_IMAGE_TAG` required, `verify-*-images` через catalog, `compose-e2e` без `run_go`, fix `jetstream_test.go`.

**Баги / пробелы (исправить):**

- [ ] **path-filters: `scripts/staging/**`, `scripts/prod/**`** — не в `global` / `compose` / `staging_infra` ([`path-filters.yml`](../.github/ci/path-filters.yml)); PR только с deploy-скриптами → `code=true`, но tier-1 jobs skipped, **`ci-gate` проходит без проверок**; push в `master` → promote all + deploy с пустым `CHANGED_SERVICES` (rollout фактически no-op).
- [ ] **`staging-stack-lock` не требует success `staging-images-push` / `staging-images-promote`** — `if: always()` + `changes.success`; при partial failed promote lock artifact и deploy всё равно стартуют (verify на deploy ловит missing, но run красный поздно).
- [ ] **`deploy-staging` не гейтит failed image jobs** — `if` проверяет только `staging-stack-lock.result == 'success'`; failed `backend-auth` / `web` / promote не блокируют workflow_call явно (частично спасает `needs:` + verify).
- [ ] **`rollout-user-space-tier`: JSON patch `add` `SPACE_GRPC_ADDR`** — повторный rollout может упасть, если env уже в pod template ([`rollout-user-space-tier.sh`](../scripts/staging/rollout-user-space-tier.sh)); idempotent patch или `set env` + restart.
- [ ] **Migrate Jobs: skip после первого success** — новые SQL в `src/backend/migrations/**` не применятся без ручного `kubectl delete job voice-migrate-*` ([`apply-migrate-jobs.sh`](../scripts/staging/apply-migrate-jobs.sh)); стратегия version/bump или force re-run.
- [ ] **Drift check только `staging-go-services.txt`** — в `staging-stack-lock` нет проверки [`staging-image-catalog.json`](../scripts/ci/staging-image-catalog.json) vs `deploy/staging/` / CI jobs.
- [ ] **Док-дрифт `compose-e2e` триггера** — [`TESTING.md`](TESTING.md) tier 2: «backend/frontend/compose»; в [`ci.yml`](../.github/workflows/ci.yml) убран `run_go` — Go-only push на `master` **не** гоняет `compose-e2e` (только `compose` / `frontend` / `global`).

**Осознанный техдолг / trade-off (зафиксировать):**

- [ ] **`compose-e2e` без Go-only триггера** — ускорение master CI; cross-service регрессии только nightly / `full` / compose-path changes (связано с Tier 2 не блокирует PR).
- [ ] **`staging-stack-lock` параллельно с auth/web/admin/portal** — lock пишется до push frontend-образов; auto-deploy ждёт эти jobs + verify — ок для happy path, не для отладки artifact mid-pipeline.
- [ ] **`rollout-user-space-tier` downtime `voice-space`** — scale 0→1 на каждый user/space deploy; альтернатива — полный `rollout-app-tier` (осознанно убрали в `c3598f3`).
- [ ] **`VOICE_IMAGE_TAG` required** — убран fallback `:latest` в [`render-and-apply.sh`](../scripts/staging/render-and-apply.sh) / prod; локальный apply без TAG падает (документировать в DEPLOYMENT или env example).
- [ ] **Prod smoke = alias staging** — [`smoke-prod.sh`](../scripts/prod/smoke-prod.sh) → [`smoke-staging.sh`](../scripts/staging/smoke-staging.sh), `STAGING_STAFF_TOKEN` из `PROD_STAFF_TOKEN`; нет отдельных prod acceptance checks.
- [ ] **Prod deploy без selective / stack.lock** — [`prod-deploy.yml`](../.github/workflows/prod-deploy.yml): нет `changed_services`, `needs_user_space_rollout`, artifact lock; `verify-prod-images` требует **все** образы catalog на TAG; `images-only` → `deploy-changed.sh` без `CHANGED_SERVICES` = no-op.
- [ ] **Prod `full` mode всегда `rollout-app-tier.sh`** — нет user/space subset rollout как на staging; single-node Recreate strategy остаётся.
- [ ] **S2S deps one-hop в `resolve-go-matrix.sh`** — e.g. `file` change не тянет `story` (story→file); для CI tests ок, для promote/build — только прямой path + gateway ([`resolve-go-matrix.sh`](../scripts/ci/resolve-go-matrix.sh)).
- [ ] **`e2e-manifest.sh` / smoke runtime** — awk-парсер YAML хрупкий; 16+ gateway + 15 flutter smoke на master — риск >15 min / flake ([`.github/ci/batch11-audit.md`](../.github/ci/batch11-audit.md)).
- [x] **Flutter `phase*` live test filenames** — rename на feature names завершён (`encryption_*_e2e_live_test`).
- [ ] **`.github/ci/batch11-audit.md` устарел** — статусы 2026-07-07, не отражает selective CI 2026-07-12; обновить или удалить после сверки с этой секцией.

**Ops / настройка (человек):**

- [ ] **Первый selective promote после `c3598f3`** — проверить GHCR bootstrap (см. Promote bootstrap выше); `workflow_dispatch` CI → `full` + при необходимости `STAGING_FORCE_FULL_ROLLOUT=true`.
- [ ] **`PROD_SMOKE_ENABLED` / `PROD_STAFF_TOKEN`** — GitHub Variables/Secrets для prod smoke (аналог staging Batch 13).

**Промпт-якорь:** `Post-commit CI audit c3598f3 from docs/TODO.md Batch 11`.



**Промпт-якорь:** `CI/CD deploy automation from docs/TODO.md Common Batch 11`.



### Batch 13 — Product Analytics (compose / CI / staging)

Реализация по [analytics.md](features/analytics.md); сервис — [analytics-service.md](microservices/analytics-service.md). Код и инфра Batch 13 в рабочем дереве; остаётся ops-секрет `STAGING_STAFF_TOKEN`.

- [x] **Синхронизация `analytics/go.sum`** — `go.sum` в репо, синхронен с `go.mod` (коммит `9983494`); локальный `go mod verify` на Windows может падать по TLS — см. [TESTING.md](TESTING.md) § «Локальные грабли».
- [x] **path-filters: ClickHouse init** — `docker/clickhouse/**` в `compose` ([`.github/ci/path-filters.yml`](../.github/ci/path-filters.yml)); добавлен фильтр `admin` для admin CI.
- [x] **Prometheus scrape: analytics** — `analytics:8080` в job `voice-go-services` ([`deploy/observability/local/prometheus.yml`](../deploy/observability/local/prometheus.yml)).
- [x] **Compose: Grafana + ClickHouse** — `clickhouse` / `clickhouse-init` на профилях `app` и `observability`; Product/Engagement дашборды работают с `--profile observability` без полного app-стека ([`docker-compose.yml`](../docker-compose.yml), [`deploy/observability/local/README.md`](../deploy/observability/local/README.md)).
- [x] **Grafana dashboards smoke** — job `grafana-analytics-smoke` + [`scripts/ci/grafana-analytics-dashboard-smoke.sh`](../scripts/ci/grafana-analytics-dashboard-smoke.sh) (UID + ClickHouse `rawSql` через `/api/ds/query`).
- [x] **Staging: ClickHouse DDL** — `render-and-apply.sh` вызывает [`apply-clickhouse-init.sh`](../scripts/staging/apply-clickhouse-init.sh) + Job из [`deploy/templates/clickhouse-init-job.yaml`](../deploy/templates/clickhouse-init-job.yaml) до migrate/rollout.
- [x] **Staging smoke: analytics** — `smoke-staging.sh`: dashboard + export при `STAGING_STAFF_TOKEN`; secret пробрасывается из [`staging-deploy.yml`](../.github/workflows/staging-deploy.yml). Bootstrap/patch secrets: `CLICKHOUSE_DSN` в [`ensure-app-secrets.sh`](../scripts/staging/ensure-app-secrets.sh) + [`patch-app-secrets-database-urls.sh`](../scripts/staging/patch-app-secrets-database-urls.sh).
- [x] **CI: ClickHouse integration tier 2** — `analytics-clickhouse-integration` на push `master` при `svc_analytics` или `compose` (не только schedule/dispatch); на PR по-прежнему tier 1 unit-тесты analytics.
- [x] **Admin Vitest** — job `admin` в CI (`npm ci` + vitest + build); [`src/admin/package-lock.json`](../src/admin/package-lock.json) для воспроизводимости.
- [x] **Analytics Dockerfile: go.sum в cache layer** — `COPY analytics/go.mod analytics/go.sum` перед `go mod download` ([`src/backend/analytics/Dockerfile`](../src/backend/analytics/Dockerfile)).

- [x] **Ops: `STAGING_STAFF_TOKEN`** — GitHub Actions repository secret + `GATEWAY_STATIC_TOKENS_JSON` в `voice-app-secrets` (patch [`patch-gateway-staff-token.sh`](../scripts/staging/patch-gateway-staff-token.sh) на deploy).

**Промпт-якорь:** `Analytics compose/CI/staging gaps from docs/TODO.md Batch 13`.



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



### Batch 12 — Phase→features (2026-07)

Миграция завершена: доки, тесты/имена CI, `lib/gen`/`pb`, `src/**` (вне gen/pb/migrations), `sync-pb-from-gen.sh`, `*.proto eol=lf` в `.gitattributes`. Инструмент: [`apply-phase-text-replacements.ps1`](../scripts/dev/apply-phase-text-replacements.ps1).

- [ ] **Windows sign-off** — скилл `voice-project-full-verification`: `compose-config-ci`, `buf-ci`, `flutter-ci` — OK; `backend-test-ci-short` — после `c3598f3` fix [`jetstream_test.go`](../src/backend/messaging/internal/messageevents/jetstream_test.go) (Flush + EnsureStream) **перепроверить на Windows/Docker**; compose smoke E2E не гонялся. См. [TESTING.md](TESTING.md) § «Локальные грабли».

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

| **Common** | Batch 13 | Product Analytics: compose/CI/staging хвосты |

| **Low** | Batch 7 | Stories post-MVP |

| **Low** | Batch 8 | Bots CI (`-short` + nightly) — done |

| **Low** | Batch 9 | Flutter analyze — done |

| **Low** | Batch 10 | Runtime timeouts — done |

| **Low** | Batch 10b | Magic numbers → env/ConfigMap |

| **Common** | Batch 12 | Phase→features — done; хвост: Windows sign-off |

