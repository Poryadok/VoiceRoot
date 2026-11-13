# TODO — CI

[← Индекс](../TODO.md)

CI/CD + выкат: GitHub Actions, promote/deploy, k8s secrets, observability на кластере, manifests.

## Critical

### Blockers

Стабильные ID для отложенных пунктов (агент без secrets/DNS/кластера):

1. Staging k8s secrets (`voice-app-secrets`)
2. Auth secrets (TOTP/NATS/Resend)
3. Billing secrets (Paddle/CloudPayments)
4. FCM/APNs secret values (**Вы**) — env names + `services.yaml` mounts fixed (#62); нужны human values в `voice-app-secrets`
5. Observability alert channel secret (Telegram/email для Alertmanager)
6. DNS staging FQDNs + firewall + GH variables (секция DNS & cluster prep **Вы**)

### CI runner drift mitigation (**Вы**)

GitHub moves the `ubuntu-latest` alias to Ubuntu 26.04: the migration window is 2026-10-19..2026-11-19 ([actions/runner-images#14748](https://github.com/actions/runner-images/issues/14748)). The docker-fixture class (compose db-init startup, Testcontainers) breaks on 26.04. Interim mitigation: PR [#1149](https://github.com/Poryadok/VoiceRoot/pull/1149) pins 13 ci.yml docker-class jobs + the t31 session-events E2E job to the alive `ubuntu-24.04` alias and re-audits the pinned representation: `WORKFLOW_SHA256` in `scripts/ci/local_ci_matrix_inventory.py`, the ci.yml entry of `SAFETY_SURFACE` in `scripts/ci/local_ci_routing.py` (master carried a stale `3556b913` contradiction against the reviewed `fb0168f1`), and the a1-attachment / a1-flutter-profile-handoff / compose-a1 reachability contracts now accept `ubuntu-latest` or pinned `ubuntu-24.04`. Remaining reds after the pin are proof debt (native50 transition, A1 multi-account compose proof, T055/T106/T107 Flutter A1 profile handoff, file attachment restart proof, Role/Space receipt proofs, Space native installer proof, the space Go integration test shard, Role native membership authority and invite decisions, the t31 HTTPS claim-inbox E2E receiver-DB proof — it fails on pinned runners too, the `Owned NATS enrollment proof` workflow that fails on every PR head with `ValueError: target8 Go image identity mismatch`, f13 historical replay `NATIVE-P3-SOURCE-v6` ledger gap), not the runner class; they are tracked as reopened TT cards. After 2026-11-19 the alias flips again: revisit the pinned representation then (do not leave `ubuntu-24.04` silently permanent).

### Secrets & alerts (**Вы**)

- [ ] **Секреты staging k8s** — `voice-app-secrets` по [`deploy/staging/secret.example.yaml`](../../deploy/staging/secret.example.yaml): JWT, Postgres URLs, R2 (`FILE_R2_*`, `USER_R2_*`), FCM/APNs для Notification, **Analytics** (`CLICKHOUSE_DSN`, `ANALYTICS_ID_HASH_KEY`) ([`DEPLOYMENT.md`](../DEPLOYMENT.md)).
- [ ] **Auth secrets** — `AUTH_TOTP_ENCRYPTION_KEY` (без `DEFAULT_DEV_KEY`); `AUTH_NATS_URL` → `NatsSubscriptionTierStore` иначе JWT `subscription_tier=free`; `RESEND_API_KEY` иначе `NoopMailSender`.
- [ ] **Billing secrets** — Paddle live (не `checkout.paddle.test`); CloudPayments когда RPC появится. Не шипить test checkout URL.
- [ ] **Observability: канал алертов** — Secret уведомлений (Telegram bot или email) для Alertmanager; без него P1-алерты уходят в null receiver ([`deploy/observability/README.md`](../../deploy/observability/README.md)).

### [A1] Staging clean install and acceptance

Latest deployment and live-browser evidence is recorded in [`a1-staging-acceptance-2026-09-27.md`](a1-staging-acceptance-2026-09-27.md). PR [#586](https://github.com/Poryadok/VoiceRoot/pull/586) merged as `f9c4abf9d208bec6af95a02b8545b9fc3a4213a7`; master CI [run 36767611478](https://github.com/Poryadok/VoiceRoot/actions/runs/36767611478) and full data-preserving staging deploy with smoke [run 36779116303](https://github.com/Poryadok/VoiceRoot/actions/runs/36779116303) succeeded on that SHA. The browser now reloads into the existing `@xronos2#6340` inbox, and read-state persistence was confirmed for one existing group and one DM. A1 remains open: the profile setup prompt still returns after Skip and reload, and the broader live messaging, attachment, privacy, archive/folder, recovery, and error-state gates remain in the linked evidence ledger. Earlier PR [#547](https://github.com/Poryadok/VoiceRoot/pull/547) and isolated Compose `a1-e2e` prove their respective scripted flows, not the entire live Web acceptance path.

- [ ] **[A1] Complete staging acceptance for live messaging** — User confirmed Quick Access holds 15 entries and adding a 16th replaces the selected existing item; GUID labels are accepted as UI debt for the next redesign. PRs #584 and #586 are deployed, and one existing group plus one DM retained cleared unread state after reload. The latest live browser reload loads the existing `@xronos2#6340` inbox, so the earlier empty-list/reconnect symptom did not recur in this check; the profile setup prompt does recur after Skip and reload, and a local client retry-state fix is pending deployment. Continue the remaining two-client channel/thread, reconnect/history, attachment persistence/download/hash and avatar upload, block/privacy, archive/folders, email reset, account recovery, and error/offline/profile/chat-switching gates in the linked evidence ledger. Do not start A2 before the complete A1 DoD passes.

### Observability staging

Проверки на живом кластере после `apply-observability.sh`; спека — [observability.md](../features/observability.md).

- [ ] **Loki: все поды** (**Вы**) — приложение + infra пишут в Loki (`kubectl get pods -n voice-observability` — Running). — **отложено: блокер #1,#5,#6** (нужен живой staging + secrets/алерты)
- [ ] **Трассировка `request_id`** (**Вы**) — E2E: `scripts/staging/smoke-request-id.sh` (DM → Loki chain Gateway → gRPC → NATS → `ws_fanout`); runbook [`TESTING.md`](../TESTING.md) § Debug by request_id. — **отложено: блокер #1,#5,#6** (нужен живой staging + secrets/алерты)
- [ ] **Grafana smoke** (**Вы**) — Overview: targets UP; дашборды Overview / Tier-0 / Infra / Logs открываются. — **отложено: блокер #1,#5,#6** (нужен живой staging + secrets/алерты)
- [ ] **Prometheus scrape** (**Вы**) — `gateway_http_requests_total` растёт при трафике на staging.  — **отложено: блокер #1,#5,#6** (нужен живой staging + secrets/алерты)
- [ ] **P1 алерты** (**Вы**) — правила активны; тестовый firing → сообщение в канал (не null receiver). — **отложено: блокер #1,#5,#6** (нужен живой staging + secrets/алерты)

### DNS & cluster prep (**Вы**)

- [ ] **DNS staging FQDNs** — Cloudflare **A** для `app`, `admin`, `livekit` (плюс уже `voice` / `developers`) на IP ingress-ноды; для `livekit` — **DNS only** (grey cloud). Firewall: **30881/TCP**, **30882/UDP** на ноде. GitHub Variables: `VOICE_WEB_INGRESS_HOST`, `VOICE_ADMIN_INGRESS_HOST`, `VOICE_LIVEKIT_INGRESS_HOST`, `VOICE_APPLY_OBSERVABILITY=true`, `STAGING_SMOKE_ENABLED=true`, secret `GRAFANA_ADMIN_PASSWORD`, `LIVEKIT_API_KEY`/`SECRET` в `STAGING_APP_SECRETS_YAML`.

## High

### Prod ingress & universal links

- [ ] **Prod universal links** — реальные AASA + `assetlinks.json` на `voice.gg` (сейчас Gateway — dev placeholders).
- [ ] **Well-known на prod** — Gateway отдаёт валидные `/.well-known/apple-app-site-association` и `assetlinks.json` для целевого домена.

### Deploy workflow

- [ ] **Messaging → Moderation staging smoke** — ConfigMap wiring is shipped: `MODERATION_GRPC_ADDR=voice-moderation:9090` is present in both staging/prod shared ConfigMaps and Messaging consumes them through `envFrom`. Still capture live staging evidence that the Messaging send path enforces Moderation shadow-ban and spam-mute checks; the existing smoke covers Gateway → Moderation admin calls, not this Messaging S2S path. DoD: a staging smoke proves both enforcement cases after deployment.
- [ ] **Sanity selective CI** — `workflow_dispatch` CI → `full`; первый master push с selective promote — проверить GHCR bootstrap; при необходимости `STAGING_FORCE_FULL_ROLLOUT=true` + manual deploy `deploy_mode=full`. — **отложено: нужен live GHCR/staging**
- [ ] **`PROD_SMOKE_ENABLED` / `PROD_STAFF_TOKEN`** — GitHub Variables/Secrets для prod smoke.

### Pipeline & promote

- [x] **Tier 2 не блокирует PR** — `compose-e2e`, platform Flutter smokes только master / `full`; регрессии после merge ([`branch-protection-checklist.md`](../../.github/ci/branch-protection-checklist.md)). Док drift закрыт: `compose-e2e` без `run_go`, триггеры admin/developer-portal (parallel/admin-ci-polish).
- [x] **`compose-e2e` без Go-only триггера** — master CI: cross-service smoke только при compose/frontend/global/admin/developer-portal; Go-only push не гоняет compose-e2e (parallel/admin-ci-polish). Full/nightly — schedule / `workflow_dispatch`.

### CI gate diagnosis — PR !1130 / !1131 (2026-11-11)

- [ ] **`nats-owned-enrollment-proof` target8 Go identity mismatch is a live-infra blocker, not repo drift** — Every recent PR head fails `owned-enrollment` at [`staging_story_target8_helper.py`](../../scripts/ci/staging_story_target8_helper.py) `authenticate_image`: after `docker image inspect docker.io/library/golang@sha256:8ac98ca534ac3f51e1f420a1dd2c15e74c75cfa0f23f3ad27eb5d7236c349a0c` (pulling the same platform-specific content-addressed reference when absent), the helper requires the inspect output to be exactly one image whose `Id` equals the pinned `GO_CONFIG = sha256:c5fe2bbc8331458475b45686d82e0c28b1161addfe33f1ebac0a715a29587701`, and it raises `target8 Go image identity mismatch` when the `Id` differs. Live Docker Hub OCI manifests for `library/golang@sha256:8ac98ca…` were re-verified through the registry manifests API on 2026-11-11: the pinned parent digest resolves, its `linux/amd64` child manifest is `sha256:66f9a494af2b76ecb3eab75ff47166df61caec6d200c59d556b77327493d83a8`, that child's config blob digest equals the pinned `GO_CONFIG`, and its env still carries `GOLANG_VERSION=1.26.8`. So the repo pins are current and correct; the failing equality is between the runner-local Docker image `Id` and the OCI config-blob digest, observable only inside a live hosted enrollment run. There were zero successful runs of `nats-owned-enrollment-proof.yml` in the recent 1000-run window (since at least 2026-11-02; the workflow triggers only on `pull_request`/`workflow_dispatch`, never on master pushes, so master stays green while every PR head fails this gate). Re-pinning `GO_CONFIG` (or switching the helper to OCI manifest verification) must be taken from live `docker image inspect` output captured by an authorized enrollment execution — offline re-pinning is unsafe and was not done here; do not pin the child manifest digest `66f9a494…`, which is not the value the helper compares. This PR records the diagnosis and does not change the helper or the pins.
- [ ] **PR !1131 head is missing `scripts/ci/p2_seeded_workflow_cleanup_test.py`** — both enrollment jobs run that cleanup self-test before any host work; on the PR !1131 head the job exited 2 with `python: can't open file ... No such file or directory` because that branch removed the script (it is present on master and on the PR !1130 head). The restore belongs inside the PR !1131 branch; no base branch may re-add feature-branch code.
- Informational, no base action: on the PR !1131 head `ci-gate` failed only as the aggregator (`affected checks did not succeed: compose-e2e, flutter-unit-tests (0)`). `compose-e2e` stopped in [`e2e_manifest.py`](../../scripts/ci/e2e_manifest.py) `affected_flutter` with `invalid E2E manifest selection (ValueError)`: the head added `src/frontend/test/livekit_initial_media_test.dart` (matches the changed-live-suite rule) while the E2E catalog under `.github/ci` does not list it, so the catalog registration must land in that branch. `flutter-unit-tests (0)` reported the shard-0 golden failure artifact (`flutter-golden-failures-0`) consistent with the new outgoing-call overlay v3 tests/goldens added by that head. Both are Tier-2 platform smokes kept to master/full by the shipped policy above.


## Common

### Manifests & rollout

- [ ] **Prod placeholders** — [`deploy/prod/domains.defaults`](../../deploy/prod/domains.defaults) `*.voice.example.com`; secrets checklist только в README, не в ops TODO Critical.
- [ ] **`rollout-user-space-tier` downtime `voice-space`** — scale 0→1 на каждый user/space deploy; альтернатива — полный `rollout-app-tier`.
- [x] **`VOICE_IMAGE_TAG` required** — fallback `:latest` убран в apply-скриптах; локальный apply без TAG падает; документировано в [`DEPLOYMENT.md`](../DEPLOYMENT.md), [`deploy/staging/env.example`](../../deploy/staging/env.example), [`deploy/prod/env.example`](../../deploy/prod/env.example); аудит [`voice-image-tag-audit.md`](../../.github/ci/voice-image-tag-audit.md) (batch 33a).
- [ ] **Production smoke runtime proof** — execute the [production authentication probes](../../deploy/prod/README.md#production-smoke) and shared checks against the authorized immutable production version. Offline wrapper tests prove fail-closed status/transport handling only; valid-session ACL, media and release acceptance remain separate.
- [ ] **Production User/Space subset runtime proof** — `full` honors explicit `needs_user_space_rollout`, with full-tier precedence/default preserved. Validate readiness and applicable images in an authorized protected production rollout; isolated dispatch checks are not deployment evidence. Single-node `Recreate` remains. — [runbook](../../deploy/prod/README.md#ci)

### Pipeline bugs

- [x] **`.github/ci/batch11-audit.md` удалён** — устаревший аудит 2026-07-07; актуальные риски перенесены в эту секцию / [`branch-protection-checklist.md`](../../.github/ci/branch-protection-checklist.md).

### Tech debt

- [x] **Promote bootstrap** — пустой GHCR / squash-merge / force-push: `find_promote_base_sha` больше не abort'ит `changes` (`set -e`); missing manifests → rebuild. Ручной `full` / `STAGING_FORCE_FULL_ROLLOUT=true` остаётся опцией.
- [x] **Дедуп frontend Docker build** — Admin/Developer Portal compile once in Docker for PR verification and selected master image publication; other admitted jobs retain the host build. Lint/test remain mandatory; standalone Docker builds and Vite arguments are unchanged. Exact-head hosted CI/image proof remains required before delivery.
- [ ] **Prod placeholders** — [`deploy/prod/domains.defaults`](../../deploy/prod/domains.defaults) `*.voice.example.com`; secrets checklist только в README, не в ops TODO Critical.
- [ ] **`staging-stack-lock` параллельно с auth/web/admin/portal** — lock пишется до push frontend-образов; auto-deploy ждёт эти jobs + verify — ок для happy path, не для отладки artifact mid-pipeline.
- [ ] **`rollout-user-space-tier` downtime `voice-space`** — scale 0→1 на каждый user/space deploy; альтернатива — полный `rollout-app-tier`.
- [x] **`VOICE_IMAGE_TAG` required** — см. Common / Manifests (batch 33a).
- [x] **S2S deps one-hop в `resolve-go-matrix.sh`** — direct `chat`→`file` edge selected; chat-only regression asserts the one-hop closure, without transitive `story`. Promote/build remains only direct paths + gateway ([`resolve-go-matrix.sh`](../../scripts/ci/resolve-go-matrix.sh)).


## Low

### Runtime config (magic numbers)

- [ ] **BOT_WEBHOOK_* retry/backoff** — HTTP interaction/autocomplete delivery already reads `BOT_WEBHOOK_DELIVERY_ATTEMPTS` and `BOT_WEBHOOK_RETRY_BACKOFF_MS`; retain exact-version configuration regression proof and integration review before closing. Durable outbox lease budgets are separate.
- [ ] **NATS_* connect/reconnect** — Realtime: connect/reconnect timeouts в конфиг.
- [ ] **pgxpool deployment capacity proof** — active Go service pools now consume `POSTGRES_MAX_CONNS` (default 10); Compose has per-service overrides and both ConfigMaps publish the default. Real deployed pool/replica capacity remains to be observed; see [OPERATIONS.md](../OPERATIONS.md#runtime-configuration-env).
- [ ] **BOT_RATE_LIMIT_* / Gateway rate-limit JSON** — staging defaults в ConfigMap (сейчас только dev bypass).

### Platform sign-off

- [ ] **Windows sign-off** — скилл `voice-project-full-verification`: `compose-config-ci`, `buf-ci`, `flutter-ci` — OK; `backend-test-ci-short` — после `c3598f3` fix [`jetstream_test.go`](../../src/backend/messaging/internal/messageevents/jetstream_test.go) (Flush + EnsureStream) **перепроверить на Windows/Docker**; compose smoke E2E не гонялся. См. [TESTING.md](../TESTING.md) § «Локальные грабли».

### Deferred / polish

- [ ] **Helm/Kustomize + GitOps** — отложено; [ADR 004](../adr/004-helm-gitops-deferred.md); ordered rollout остаётся в bash на runner.
- [ ] **Пересмотреть цель continuous full-stack deploy** — selective deploy есть; GitOps позже.
- [ ] **Qualify actual smoke duration/flake on Linux** — the shared typed YAML parser rejects malformed/partial selections and preserves catalog order; smoke/full stop before suites on parser failure. Actual Compose/Flutter runtime and flake qualification remain open; do not reduce catalog coverage to claim a timing improvement ([`e2e-manifest.sh`](../../scripts/ci/e2e-manifest.sh), [`e2e-features.yml`](../../.github/ci/e2e-features.yml)).


## CI drift diagnosis — Space/Role ban-catalog reds, run 43389461187 (2026-11-13)

Candidate `ca731506`; five red jobs = четыре классы, одна и тот же корень: floating Docker Hub Postgres tags drifted upstream. `postgres:16-bookworm` 16.15→16.16, `postgres:18-alpine` 18.6→18.7; new point releases backported per-attribute versioning, so `to_jsonb(pg_attribute)` now reports `attoptions: ["pg_attver=<N>"]` on all 51 ban-table columns (old builds: `attoptions: null`). `accountBanStructureSQL` hashes that raw JSON; pins `{16: d5a5fd0e…, 18: 271034e2…}` in [`account_ban_catalog.go`](../../src/backend/space/internal/store/account_ban_catalog.go) match only exact pre-drift builds → `checkAccountBanCatalog` fails → `complete account ban authority unavailable` / `native50 transition failed`. Guard pins (`links 8e7ffadc…`, `commit_guard 3de8a360…`) unaffected. Repro on the Windows host via containers: 16.15/18.6 chains reach `complete`/probe `check <nil>`; 16.16/18.7 fail at structure sha (`163587c8…`, `66bdef5f…`); drift delta 68286→69204 (16-series), 73927→74845 (18-series), `pg_attver` 0→51.

- **Class 1** — backend-go-role matrix step (`go test -tags role_space_native -run '^TestMemberAdmissionRole25ProtectedReceipt$'`): fixture = `native_space_integration_test.go` → `NewSourceMigrationFixture` default `PostgresImage` (floating 16-bookworm).
- **Class 2** — `role-space-receipt-integration` (Positive+Role25 receipt pair): same fixture site, same signature.
- **Class 3** — `space-native-installer-integration`: `services.postgres` image `postgres:16-bookworm` floating; provision chain `created→…→migrated50` then `native50 transition failed` on 16.16, `complete` on 16.15.
- **Class 4** — go-integration matrix space `go test ./...` (push+PR jobs): grpcsvc/store fixtures via `startSpacePostgresForTest`→`StartPostgres`→`PostgresImage` (drifted 16.16) и fixtures with literal `postgres:18-alpine` (drifted 18.7) — `TestBotInstallationScope*`, `TestSpaceModeration_*`, `TestSpaceDeletionProof*`, `TestRecoverLifecycleProof*`, `TestNative51*`, `TestVoiceCreateNative51*`.

**Fix (environment only, fail-closed, precedent = digest-pinned images across workflows):** default `PostgresImage` in `src/backend/pkg/integrationtest/postgres.go` → `postgres:16-bookworm@sha256:5264710be20f…` (16.15, Hub-addressable, verified); four `postgres:18-alpine` literals → `postgres:18-alpine@sha256:9ba53c7d41ea…` (18.6); ci.yml `space-native-installer-integration` service → `postgres@sha256:5264710b…`. Product pins/checks untouched. Default pin is the harness env-selection site — it restores the exact build all currently-green fixtures already ran on; no green job class changes behavior.

**Safety-surface re-audit (same change):** editing `.github/workflows/ci.yml` changes the reviewed CI-surface hashes, so the pins are re-pinned to the new ci.yml bytes (`ed793837…`, precedent `f301989`/`84aa3833`): `WORKFLOW_SHA256` in [`local_ci_matrix_inventory.py`](../../scripts/ci/local_ci_matrix_inventory.py) and the `ci.yml` entry of `SAFETY_SURFACE` in [`local_ci_routing.py`](../../scripts/ci/local_ci_routing.py). The Makefile is untouched, so the shard recipe counts (`policy 80 / runtime 23 / staging 34 / proofs 15`) and the 40 Windows-only policy tests are unchanged; the guard still refuses any future un-audited ci.yml drift. No `*_required.json` product inventory was edited.

**Out-of-scope pre-existing drift (same family, different workflow):** `owned-enrollment` in [`nats-owned-enrollment-proof.yml`](../../.github/workflows/nats-owned-enrollment-proof.yml) fails `target8 Go image identity mismatch` in [`staging_story_target8_helper.py`](../../scripts/ci/staging_story_target8_helper.py) — the pinned `golang@sha256:8ac98ca5…` build no longer authenticates (`GO_IMAGE` digest + `GOLANG_VERSION=1.26.8` env + config id). It is red on every PR run since at least 2026-11-12 (12+ runs: `codex/uiv3-chatroom-residual`, `codex/uiv3-subscription-suite`, `codex/ci-history-audit-ledger`, `codex/ci-docker-runner-pin`, `codex/ci-infra-parity-2026-11-13`, `codex/voice-audit-v6-docs-note`), independent of this PR's diff (that job never reads ci.yml and archives pinned story sources). Left untouched: outside the Space/Role scope; its re-pin would be its own diagnosis.

**Residual:** docs/PLAN.md and docs/TODO.md track these reds as native45/50/51 receipt WIP («stay red and tracked») — stale against current code: on pinned reviewed builds the same chains reach `complete`/`check <nil>`, the reds were drift, not unfinished product work; product status NOT relabeled here. `space_user_capture_ban_integration_test.go` keeps floating `postgres:16-alpine` (green, not part of the four classes — does not hit the structure check); future drift there needs its own diagnosis, not a silent relabel. Classes 1/2/4 green-verified only on hosted CI (testcontainers cannot run from the Windows host: no docker.sock); class 3 verified locally against 16.15.


**Промпт-якорь:** `CI/CD + deploy from docs/todo/ci.md` + приоритет/подсекция.
