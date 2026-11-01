# Тестирование

See [Release evidence and recovery lessons](RELEASE_EVIDENCE.md) for executed
scenario coverage, discriminating negative tests, acceptance-reader testing and
source-bound visual/runtime evidence.

Договорённости по слоям тестов, стеку и CI. Детали релиза и отката — [OPERATIONS.md](OPERATIONS.md); локальный подъём сервисов — [PLAN.md](PLAN.md) (Docker Compose), [DEPLOYMENT.md](DEPLOYMENT.md).

---

## Цели

- Регрессии ловятся **до** merge в `master`: линтеры и тесты в CI обязательны после включения branch protection на `master`.
- Критичные пути (авторизация, выдача JWT, маршруты Gateway, сохранение сообщений) покрываются **интеграционными** тестами по мере появления реализации.
- Контракты gRPC/REST: совместимость protobuf/JSON и тесты на стороне consumer. **Pact** и аналогичные инструменты **не** внедряем до отдельного решения в документации.

---

## Порядок разработки (TDD)

### Область применения

Норма ниже относится к **новой нетривиальной поведенческой логике** (согласовано с разделом «Покрытие и качество» ниже).

### Явный запрос скилла `tdd-code-workflow`

Если в задаче явно сказано следовать **TDD Code Workflow** / скиллу `tdd-code-workflow` (см. канон [`.agent/workflows/tdd-code-workflow/SKILL.md`](../.agent/workflows/tdd-code-workflow/SKILL.md)), агент обязан идти **строго по этому документу**: план до правок прод-кода, тесты из доков первыми, делегирование ревью/тестов где доступно (Task, отдельные сессии), красный–зелёный–рефакторинг, финальный чеклист из скилла — без «ускоренного» одного прохода вместо воркфлоу.

**Исключения:**

- Чистый рефакторинг без изменения наблюдаемого поведения.
- Механические правки (форматирование, переименования без смысловых изменений).
- Короткие спайки и прототипы — по явной договорённости в задаче.
- Если инфраструктура для нужного типа тестов ещё не заведена — после первого набора описать способ запуска в репозитории (как для интеграционных тестов в разделе про Go ниже).

### Цикл работ

1. Зафиксировать ожидаемое поведение по релевантным документам в `docs/` (например фича и контракт сервиса из [FEATURES.md](FEATURES.md) / `docs/features/`, [MICROSERVICES.md](MICROSERVICES.md), [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md); при смене схемы — миграции и [DATA_STORES.md](DATA_STORES.md), [DATA_MODEL.md](DATA_MODEL.md)).
2. Написать или обновить тесты так, чтобы они проверяли это поведение (допустим первый шаг — падающий тест).
3. Реализовать минимальный код до **зелёных** тестов.
4. Рефакторинг при неизменно зелёных тестах.
5. Прогнать команды из раздела «Что запускать локально перед PR» для затронутых компонентов и линтеры соответствующего языка.

### Красный тест или правки ожиданий в тестах

Приоритет источников истины:

1. Проверить: соответствует ли ожидание в тесте документации в `docs/` и модели данных ([DATA_MODEL.md](DATA_MODEL.md), контракты в `docs/microservices/`).
2. Если поведение кода расходится с **документацией** — править **реализацию**; при необходимости обновить `docs/` в том же PR ([CONTRIBUTING.md](CONTRIBUTING.md): контракты и поведение не должны расходиться с докой).
3. Менять **ожидания в тесте** только если изменилась спецификация (или задача явно меняет требование) и это отражено в `docs/`; иначе тест остаётся регрессией на документированное поведение.

### Генерация кода (ИИ и проч.)

Не добавлять продуктовое поведение «от себя». Если в доках не хватает оснований — зафиксировать пробел в [TODO.md](TODO.md) или запросить решение у человека.

---

## По языкам и компонентам

Изменения маршрутизации CI проверяются `python scripts/ci/fast-gate-routing_test.py`,
`python .github/ci/selected-acceptance-gate_test.py` и
`bash scripts/ci/resolve-go-matrix_test.sh` (Python с PyYAML и `jq` для shell).
Матрица тестов Go-потребителей и матрица Docker-сборок различаются: образы нужны
для прямых изменений и реальных исходных зависимостей Dockerfile, общие build
inputs расширяют проверку. Правила PR/master и сохранённые обязательства —
[CI gate policy](../.github/ci/gate-policy.md#pr-scope-and-image-verification).

### Help: реальный browser handoff

The `flutter-help-proof` CI job builds `test/fixtures/settings_help_web_launcher_smoke.dart`
and runs `node scripts/ci/settings-help-browser-handoff.mjs
src/frontend/build/help-launcher-web <evidence-dir>` from the root (Node.js 24,
`CHROME_BIN=google-chrome`, and `GITHUB_SHA` identifying the checkout).
For a separate run, first run `flutter pub get` and the following from `src/frontend`:
`flutter build web --release --no-pub --no-web-resources-cdn --target
test/fixtures/settings_help_web_launcher_smoke.dart --output build/help-launcher-web`.
The fixture uses the real `HelpSheet` and `url_launcher`; mock HTTP isolates
unrelated backend requests. Trusted pointer clicks navigate Settings → Help →
README / Issues in both orientations, checking `Page.windowOpen.userGesture`,
the exact new target URL, exactly one new tab, and Help remaining error-free.
The `settings-help-browser-<run_id>` artifact contains the head SHA
(`VOICE_SOURCE_SHA` when set), checkout SHA, event and target JSON, Chrome
version, and six PNGs. It proves browser handoff; external GitHub content and
binding to the deployed SHA require separate checks.

### Web cold startup

Promotable web runtime configuration: `flutter test
test/web_gateway_config_test.dart test/gateway_config_test.dart` covers explicit
origin binding, unsafe/missing configuration and compile-time client compatibility.
`python3 src/frontend/docker/runtime_config_deployment_test.py` (PyYAML from
`scripts/ci/requirements-e2e.txt`) checks the canonical ConfigMap/read-only mount,
DEV rendering and single image producer. It is included in `ci-script-tests-runtime`.

For the actual web bootstrap boundary, build the release Dockerfile once, extract
`/usr/share/nginx/html` from that image, and run
`node src/frontend/docker/runtime_config_browser_test.mjs <extracted-web-root>`
in Linux with Node 24 and Chrome (`CHROME_BIN` optional). This uses the unchanged
compiled assets under two owned loopback app/API origins. A disposable saved
session forces the real refresh client: correct configs must send only to their
selected API after config delivery; missing, invalid and wrong-origin configs
must produce no API calls or first frame. External network is blocked; bundled
CanvasKit bytes satisfy the renderer request. This does not qualify a live
environment, its endpoint tuple, or a coherent backend image bundle.

`node --test scripts/ci/web-startup.test.mjs` checks the early HTML loader,
RU/EN, bootstrap/engine/runApp errors, stalled startup, and loader removal only
on `flutter-first-frame`. The `flutter-startup-proof` job builds the Help fixture and
`node scripts/ci/web-startup-browser.mjs src/frontend/build/help-launcher-web <output>`
checks the real Flutter engine: the HTTP fixture holds main.dart.js, the current
Voice icon and status appear before Flutter starts, and the first frame removes
the loader. A separate bootstrap 503 and trusted pointer click test reload/retry.
The check uses fresh pages, a disabled browser cache, and desktop and mobile viewports.
To check a full production build, pass `src/frontend/build/web` after
`flutter build web --release --no-pub --no-web-resources-cdn`.

Node.js 24, Chrome/Chromium (`CHROME_BIN`) и полный `VOICE_SOURCE_SHA` обязательны.
Artifact содержит browser version, viewport, PNG и `voice-startup:*` performance
marks (HTML, entrypoint, engine, run-app, first-frame). Это локальные timings с
искусственной задержкой, не production latency и не доказательство причины
пользовательского медленного старта. Реальные device/network и deployed asset
delivery проверяются отдельно. Loader использует существующий `voice_sky.png`,
system font и inline critical CSS; 90s — только recovery UI deadline, запрос
может завершиться позднее. Ошибки не раскрывают URL или raw exception.

### Go (сервисы, Gateway, Realtime и др.)

- Фреймворк: стандартный пакет **`testing`**, табличные тесты где уместно.
- Утверждения: **`github.com/stretchr/testify`** (`require` / `assert`) — единый стиль по репозиторию для читаемости.
- PostgreSQL/Redis integration tests use **testcontainers-go** (helpers in [`src/backend/pkg/integrationtest/`](../src/backend/pkg/integrationtest/postgres.go)). Run `go test` on a host with a reachable Docker daemon. Ryuk is disabled on Windows by `integrationtest.ConfigureDockerTesting()`. Global Testcontainers pruning is unsafe on shared hosts and is disabled; run-owned cleanup and isolation are still being implemented.
- `scripts/ci/local_heavy_slots.py` is a locking primitive under development. Its default capacity is two; callers can select 1–64 slots on first initialization. All cooperating callers must use the same trusted host-local directory, capacity, and implementation version; conflicting or damaged configuration fails closed. Capacity is fixed for that directory and is not automatically reconfigured. A crash or exception retains the slot for owner-aware recovery. It is not connected to general test execution and does not make parallel Docker runs safe by itself.
- `ScheduledRun.start_host` and `recover_host` use one OS-wide slot directory: Windows common application data or `/var/tmp` on Linux. They do not fall back to a per-checkout directory if access fails. This location coordinates only callers that use the host scheduler; it does not yet supervise general Testcontainers or Compose commands.
- `ScheduledRun.recover` can recover a slot bound to an exact `OwnedRun` journal after its owner exits. It checks the slot binding, journal lock, daemon and every owned Docker identity, then cleans those resources before releasing the slot. Uncertain operations and mismatched identities remain unavailable for manual reconciliation. This covers only resources explicitly created through `OwnedRun`; it does not supervise or recover arbitrary Testcontainers, Compose, or child processes.
- `scripts/ci/local_compose_audit.py` checks a rendered Compose model for known shared ports, resource names, mounts, networks and build tags. It expects a run-specific scratch directory and reads the model from standard input. It is not yet connected to a runner and does not establish Docker resource ownership or safe cleanup.
- `scripts/ci/local_docker_owned.py` journals creation and cleanup of explicitly created containers, networks and volumes on an internal network without host ports. The opt-in `VOICE_LOCAL_DOCKER_TEST=1 python scripts/ci/local_docker_owned_integration_test.py` checks that cleaning one such run preserves two other owned sets. `VOICE_LOCAL_DOCKER_TEST=1 python scripts/ci/local_docker_owned_cancel_test.py` checks cleanup after a controlled failure and recovery after terminating a fixture owner. Existing Testcontainers and Compose attribution, recovery of uncertain Docker operations, host access and runner integration remain unfinished.
- **`-short` vs полный прогон:** в PR/push CI job **`backend-go`** запускает `go test -short ./...` — тесты с `testing.Short()` (testcontainers, долгие HTTP/webhook) пропускаются. Полный `go test ./...` по матрице сервисов — nightly job **`backend-go-integration`** (cron + `workflow_dispatch`) и локально при необходимости: `make backend-test-ci` или `cd src/backend/<service> && go test ./...`. Паритет с PR CI на хосте: **`make backend-test-ci-short`**.

PR integration routing includes affected services, existing RPC and build-source
consumers, and documented event subscribers. Shared package, proto, Auth and
global wiring changes conservatively select every Go service integration. A
successful matrix job alone is not proof that a particular opt-in or
Docker-dependent test ran; inspect its execution evidence and skips.

For a selected PR Go integration matrix, the evidence job rechecks all
downloaded member diagnostics against `scripts/ci/go_integration_required.json`
and the candidate/run/attempt. Its source audit requires a clean checkout at
the exact candidate SHA and rejects untracked Go or workspace inputs. The
initial list covers observed untagged
integration cases plus an Analytics adapter sentinel; it does not inventory all
native, tagged, opt-in, subtest or cross-service tests. Dedicated checks below
remain required for their separate cases.
The source audit requires every `*_integration_test.go` entrypoint to appear
in that baseline or in `scripts/ci/go_integration_exceptions.json`. An
`unresolved` entry records a coverage gap and is never evidence that it ran;
the audit does not discover native tests under other filename conventions.
The baseline also names the Space Bot installation add, remove and tombstone
proofs, including both add/remove subtests. Their fixture uses the qualified
native 45, 50 and 51 migration transitions; this does not replace the separate
tagged Space catalog and account-ban suite.

The affected Space native-installer job also provisions a separate fresh
current-schema database with the source-bound installer and runs the tagged
account-ban missing-capture guard through a signed native creator and the
authenticated ban path, the native creator proof, and the account-ban catalog
drift proof. The receipt requires all three cases and the eight catalog drift
subtests, and rejects skips. These checks do not cover the other tagged cases
that require populated historical 34, qualified 50, or separately qualified 51
fixtures.

The affected Story PR proof runs `go test -json -count=1 -p 1` for
`./internal/lfpconsentstore` and `./internal/lfpconsenttransport` against an
owned PostgreSQL service. It requires the named store and protected transport
cases in `scripts/ci/story_lfp_required.json` and rejects every skipped test
in those packages. Locally, set `LFP_TEST_DATABASE_DSN` only to an owned,
disposable PostgreSQL database; the fixtures create and drop schemas there.
An exit code of zero without that DSN is not evidence for those cases.

The affected User PR proof runs `./internal/identityassociation` against an
owned PostgreSQL service using `USER_ASSOCIATION_TEST_DSN`. It requires the
named package inventory in `scripts/ci/user_association_required.json` and
rejects skips. Its fixtures create and drop databases; point the DSN only at an
owned disposable PostgreSQL server. The job also runs the protected capture
transport against the same owned PostgreSQL and an owned Redis service. Its
named subtests are listed in `scripts/ci/user_capture_required.json`; skips
are rejected in that focused run. The User admission-eligibility transport
test also runs with the owned PostgreSQL and Redis services; its named case is
listed in `scripts/ci/user_admission_required.json`.
The same PostgreSQL service runs the native primary and secondary profile-birth
capture tests listed in `scripts/ci/user_birth_required.json`, with skips rejected.

The affected Notification PR proof runs `./internal/chatquota` with
`BE195_REDIS_ADDR` pointing to an owned Redis service, requires the named cases
in `scripts/ci/notification_quota_required.json`, and rejects skips. Use only a
disposable Redis server for the local real-Redis concurrency test.

The affected Role PR proof runs `TestRole13` in `./internal/store` against an
owned PostgreSQL server via `ROLE_MIGRATION_TEST_DSN`. The fixtures create and
drop disposable databases. The named rollback and concurrency cases in
`scripts/ci/role_migration_required.json` must run without skips.
The same affected job also prepares an owned Role version 20 catalog from the
ordered migration files and runs `TestBootstrapActualRole20PG` with owned Redis;
that historical-catalog proof does not replace current-schema migration checks.
The affected Bot PR proof uses an owned PostgreSQL server through
`BOT_MEMBERSHIP_TEST_DATABASE_URL` and requires both native membership cases in
`scripts/ci/bot_membership_required.json` without skips.
The affected Voice PR proof runs the real Redis ledger atomicity case with an
owned Redis server via `VOICE_R18_REDIS_TEST_ADDR` and rejects skips. The
source-disabled later-slice crash harness seams remain separate and are not
claimed as passing by this proof.
The affected Messaging PR proof runs the immutable thread-list node and
rollback test against owned PostgreSQL via `THREADLIST_TEST_DATABASE_URL` and
rejects skips.
The affected Space PR proof enables the `integration` tag and uses an owned
PostgreSQL server configured to run a real, cost-delayed autovacuum worker.
It requires `TestMaintenanceWindowWithRealAutovacuum` without skips and checks
that the actual worker remains present while the maintenance window is valid.
- BE-076 message views: from `src/backend/messaging`, run
  `go test ./internal/store ./internal/grpcsvc -run TestMessageViews -count=1`
  and `go test ./testfixture -run TestMessagingCompactionUsesActualUserDeletionAuthority -count=1`.
  The latter builds an isolated User fixture subprocess to avoid duplicate
  protobuf registration across service modules, then uses actual protected
  mTLS/JWKS authority, a shared network miniredis replay store and two disposable
  PostgreSQL databases. It covers authority outage, retry and explicit
  soft-deletion compaction; frozen/active/missing profiles retain views. It skips
  under `-short`; no DEV/STAGING or external providers are used.
- HTTP: `httptest` для хендлеров Gateway без поднятия сети.
- Messaging response-loss proof: from `src/backend/messaging`, run
  `go test ./internal/grpcsvc -run '^TestMessagingSendMessage_RetryAfterLostResponseDoesNotDuplicate$' -count=1 -v`.
  The real Messaging service and disposable PostgreSQL commit one 4000-codepoint
  send before a test proxy replaces its first response with `Unavailable`.
  An explicit same-key, same-payload retry must return the committed ID; a
  changed payload must conflict. SQL verifies one message and one unchanged
  outbox event, and recipient history contains exactly that message. This is a
  controlled gRPC response-loss check, not evidence of a live HTTP/edge cause or
  an edit/friend-invitation retry contract. Use the Linux/container route below
  when host test execution is restricted.
- Space retained-installer regression: on an owned fresh private PostgreSQL 18
  server, set `NATIVE_INSTALLER_TEST_ADMIN_URL`, `NATIVE_INSTALLER_TEST_MIGRATE`
  (the pinned migrate executable) and `NATIVE_INSTALLER_TEST_BINARY` (the
  production `cmd/native-migrate` binary built from the same checkout with its
  normal source/migrator identities). With `psql` on PATH, run from
  `src/backend/space`: `go test -tags=integration ./internal/nativeinstaller
  -run '^TestProductionFreshChainAndRetainedVerify$' -count=1 -v`.
  The test provisions an absent `space_db`, retains actual completed JSONL,
  applies canonical runtime-role SQL, checks unchanged seal and all 24 account-ban
  DML grants, and verifies through the production CLI. A real non-owner serving
  login creates a native Space and reads membership authority. Dirty/unknown
  markers, unauthorized grants, elevated runtime identity and catalog tampering
  must HOLD; restored state verifies without migration or resealing. The test
  drops only the exact database it created; discard the private server afterward.
  This bounded source regression does not prove full Compose/Gateway startup.
  The separate `TestCommittedPhaseFailureNeverAutoResumes` also requires
  `NATIVE_INSTALLER_TEST_FAILURE_HELPER`: build
  `./internal/nativeinstaller/testdata/evidencefailure` with the same source
  revision, backend Go source digest and migrate executable digest as the
  production binary. The [CI job](../.github/workflows/ci.yml) shows the exact
  build flags. Run it only against a fresh owned private server; its nine
  subtests deliberately interrupt evidence writes and verify that the production
  installer refuses to resume each retained database.
- Space maintenance-window regression: on an owned disposable PostgreSQL server
  with absent `space_db`, configure `autovacuum_naptime=1s`,
  `autovacuum_vacuum_cost_delay=20ms` and `autovacuum_vacuum_cost_limit=1`.
  Set `NATIVE_INSTALLER_WINDOW_TEST_ADMIN_URL` to its administrative `postgres`
  database and run from `src/backend/space`: `go test -tags=integration
  ./internal/nativeinstaller -run '^TestMaintenanceWindowWithRealAutovacuum$'
  -count=1 -v`. The actual production checker must accept a real autovacuum worker
  while rejecting foreign writers, idle unnamed clients, changed PID/role/OID
  and lost owner locks. The test creates/drops only its previously absent
  `space_db`; discard the private server afterward. This checks the window guard,
  not the full migration chain or a hosted failure's backend inventory.
  Required CI executes this same test in the `backend-go` Space matrix entry
  with an owned digest-pinned PostgreSQL 16 container and unconditional cleanup.
- gRPC: in-process server или клиент в тесте — как в уже существующем сервисе с тестами.
- Voice R22.3: `go test ./internal/roomlifecycle -run '^Test(VoiceDBRedisDivergenceMigration|PostgresLifecycleStore)_D1[^0-9].*$' -count=1`,
  затем `go test ./internal/roomlifecycle -count=1`, `go test -race
  ./internal/roomlifecycle -count=1`, `go test ./... -count=1`, `go vet ./...`
  и repository-pinned `golangci-lint run ./...`. Migration/classifier gate
  требует real PostgreSQL 16 с zero skips; Redis 7 понадобится для deferred
  D2/D3 bridge tests, не для D1-only classifier transaction.

### Voice dormant native/P3 composition

`go test -race -run '^TestProduction' .` из `src/backend/voice` выполняет
production startup factory для обычного режима без Space/NATS и для Space с
существующими compatibility consumers. Проверяются compatibility delivery,
pending native roster facts без lease/publish/delivered marker и EJECT_ONLY P3.
По умолчанию используются Testcontainers. Для собственного изолированного Linux
runner можно явно передать `VOICE_COMPOSITION_TEST_DATABASE_URL` (PostgreSQL с
правом создавать disposable databases) и `VOICE_COMPOSITION_TEST_NATS_URL`.
Voice fixtures создают отдельную БД на тест и применяют те же миграции/SQL;
серверы должны принадлежать проверяющему. Этот opt-in также доступен focused
ordinarycall/spacemedia/spacelifecycle PostgreSQL fixtures. Проверка отдельного
child process сохранённых bytes/pending state выполняется при explicit DSN.
NATS fixture создаёт только существующие compatibility streams/consumers;
она не доказывает installed ACL, A3 activation, provider retirement или rollout.
Startup fixture использует уже committed native факты; arbitration между live
admission writer и recovery проверяется и исправляется отдельно.

### Space tree integration proof

Realtime P3 has a scoped Linux-only external-PostgreSQL suite tagged
`realtimep3integration`. Set `REALTIME_P3_TEST_DATABASE_URL` to an owned isolated
PostgreSQL16 database and run Realtime `go test -race -tags=realtimep3integration
-run 'Test(ProtectedRealtime|ActualHub|Committed|FreezeReceipt|ExactManifest|Restart|Unknown|Multipage|ReceiptCompaction)' . ./internal/lifecycle`.
The same tag exposes native Chat `TestRealtimeNativeLifecycle` and Space
`TestRealtimeParticipantEleven` store cases; these create and drop uniquely named
databases on that owned server to apply complete native migration catalogs.
The required suite fails when its DSN is absent rather than skipping. Containers
need no host ports or Docker socket for this suite. Ordinary legacy short-test
skips remain distinct from these runtime cases; never report them as PG proof.

The shared local Space/Realtime JWKS exporter has a separate no-network Linux
check. From the repository root, build `docker/social-principal` as
`voice-jwks-bootstrap-test`, then run its Python fixture with the source mounted
read-only (no host ports):

```sh
docker run --rm --network none --entrypoint python3 \
  -e LOCAL_JWKS_BOOTSTRAP_TEST=1 \
  -v "$PWD/docker/social-principal:/fixture:ro" voice-jwks-bootstrap-test \
  /fixture/export_jwks_test.py
```

It checks both issuers reject weak or mathematically unusable private pairs
before any public generation is written and on reuse of matching public docs.
Valid distinct pairs and actual nondefault RSA exponents remain accepted. This
fixture does not qualify deployed keys, network trust or lifecycle readiness.


Для узкого exact-SHA доказательства запускается workflow
[space-tree-proof.yml](../.github/workflows/space-tree-proof.yml). Он выполняет
четыре PostgreSQL-backed кейса: update/delete категории, update/delete voice room
с удалением tree node, удаление text node и обогащение text-chat узлов именем и
типом из Chat Service. Hosted Ubuntu запускает существующую
testcontainers-Go fixture без `-short`; job сверяет SHA checkout и завершится
ошибкой, если любой названный тест не выдал `PASS` либо был skipped. Подробный
verbose log хранится как workflow artifact.

После merge workflow можно вручную запустить с точным `source_sha` через его
копию в default branch. Этот узкий proof не заменяет обычный CI сервиса и не
закрывает более широкие критерии A2.

### Bot durable inbound message delivery

From `src/backend/bot`, run `go test ./internal/consumer -run
'^TestMessageEventsJetStreamDurableDelivery$' -count=1 -timeout=3m -v` with
Docker available. The component fixture owns disposable PostgreSQL 16 and
NATS JetStream containers and provisions the documented
`message_events/bot_message_events` push durable before starting the production
consumer. It checks offline backlog, database-failure NAK/redelivery, signed
webhook retry after consumer restart, polling failure replay followed by an
actual gRPC receive, source replay deduplication and unauthorized-chat denial.
`-short` skips this container test. The HTTP receiver is a local fixture;
deployed webhook delivery, service credentials/ACLs and production activation
remain separate checks.

### API Gateway

- Локально: из [`src/backend/gateway/`](../src/backend/gateway/main.go) запускать `go test ./...`; для race-проверки на хосте с CGO — `CGO_ENABLED=1 go test -race ./...`.
- Обязательные contract tests: REST namespaces, `/ws`, JWT/JWKS, Redis blacklist, rate limit groups, trusted proxy `X-Forwarded-For`, CORS preflight, request id/claims propagation, `/api/v1/version`, `/metrics`.
- Production env для auth: `GATEWAY_AUTH_MODE=static` допускается только для dev/tests; рабочий режим задаётся через `GATEWAY_JWKS_URL`, `GATEWAY_JWT_ISSUER`, `GATEWAY_JWT_AUDIENCE`.
- Redis-проверки: `GATEWAY_REDIS_ADDR` включает sliding-window rate limiter и blacklist чтение; blacklist key prefix по умолчанию `jwt:blacklist:`, override — `GATEWAY_JWT_BLACKLIST_PREFIX`.
- Edge smoke после деплоя: `/health`, `/metrics`, `/api/v1/version?platform=<known>&version=<semver>`, один public REST route, один protected REST route с JWT и `/ws` upgrade через Realtime upstream.

### Public staging audio media check

`TestStagingVoiceCallBidirectionalAudio_live` is a separate opt-in Linux harness
for an authorized operator. It uses the existing synthetic PCM/RTP peers and
requires ten nonempty advancing audio RTP packets in each direction. Signaling
alone cannot pass it. It does not run in the ordinary staging smoke.

Prepare two dedicated, verified regular accounts with an existing DM, current
access tokens, and no active calls. Reserve both accounts exclusively for the
check. Place this JSON outside the checkout in an owner-only (`chmod 600`) file;
substitute actual IDs/tokens locally and never attach the file or raw credentials
to evidence:

```json
{
  "schema": "voice-staging-media-v1",
  "api_origin": "https://api-staging.example.test",
  "livekit_origin": "wss://livekit-staging.example.test",
  "chat_id": "existing-dm-uuid",
  "caller": {"account_id": "caller-account-uuid", "profile_id": "caller-profile-uuid", "access_token": "private-caller-token"},
  "callee": {"account_id": "callee-account-uuid", "profile_id": "callee-profile-uuid", "access_token": "private-callee-token"}
}
```

Use the actual configured staging origins. The API origin must exactly match
`VOICE_STAGING_URL`; the returned Voice token URLs must both match the fixture's
WSS origin. TLS verification stays enabled, redirects are rejected, and there
is no localhost fallback. Before creating a call the harness checks `/users/me`
against both account/profile pairs and refuses existing active calls. The file
is locked for the run. The harness does not register accounts, send OTP, reset
Redis rate limits, or change DM privacy settings.

From `src/backend/gateway`, on Linux with the repository Go version and
`libopus-dev`, `libopusfile-dev`, `libsoxr-dev`, and `pkg-config` installed:

```sh
VOICE_RUN_STAGING_MEDIA=true \
VOICE_STAGING_URL=https://api-staging.example.test \
VOICE_STAGING_MEDIA_FIXTURE=/absolute/private/media-fixture.json \
CGO_ENABLED=1 go test -tags=live -run '^TestStagingVoiceCallBidirectionalAudio_live$' -count=1 -parallel=1 -timeout=3m -v .
```

The harness registers cleanup as soon as call creation returns its room ID;
after RTP/ICE/token failures it still ends that owned call and checks both
accounts have no active call. A process kill or uncertain HTTP call-creation
result can prevent this cleanup: inspect the dedicated accounts' active-call
state through the normal API before retrying. Do not blindly repeat a mutation
or terminate an unrelated call. Record the tested source SHA, actual deployed
commit/image, environment, both-direction RTP result and cleanup result.
Preparing this harness is not a real staging media PASS or final acceptance.

Portable fixture/preflight tests need no live credentials or native libraries:

```sh
go test staging_voice_media_fixture_test.go staging_voice_media_fixture_validation_test.go -count=1
```

The `Gateway media test build` workflow compiles the Gateway test binary on
Linux with `CGO_ENABLED=1`, the native Opus dependencies and `-tags=live` for
Gateway/pkg pull requests. Its `go test -c` step executes no tests or package
initializers, needs no staging credentials, and closes only the native build
gate. Actual media/cleanup evidence still requires the authorized live run.

### Java (Auth Service, Spring Boot)

- Юнит и slice-тесты: **JUnit 5**, **Spring Boot Test** (`@WebMvcTest`, `@DataJpaTest` и т.д. по слою).
- Контейнер БД в CI: **Testcontainers** для PostgreSQL, если тесту нужна реальная схема.

The BE170 test-only `AuthBotSessionFloorGrpcTest.main` runs the existing protected
Auth floor fixture for private Go-to-Java composition. Compile its test class,
then launch it with an output directory and bounded lifetime (1..600 seconds).
Default inputs retain the original fixed clock, generated test signer and
account. Set `VOICE_AUTH_BOT_FLOOR_INTEROP_CLOCK_MODE=system` to use the same
live UTC clock for principal verification, JWKS caching and readiness. In this
mode the readiness `clock` is an observed startup instant; the shared Go issuer
must keep using its live clock, rather than freezing that instant.

For one caller-owned test Bot signer shared with Role/Space/Chat, supply **all
three** `VOICE_AUTH_BOT_FLOOR_INTEROP_SIGNER_KEY_FILE` (RSA PKCS8),
`VOICE_AUTH_BOT_FLOOR_INTEROP_SIGNER_KEY_ID` and
`VOICE_AUTH_BOT_FLOOR_INTEROP_JWKS_FILE`. The latter must be the complete approved
current/next public snapshot fetched by the fixture owner through its existing
protected JWKS transport. The private/public pair and kid must match it; the
snapshot bytes are reused unchanged. Partial, blank, invalid or mismatched
inputs fail before a listener starts. The optional
`VOICE_AUTH_BOT_FLOOR_INTEROP_ACCOUNT_ID` selects a canonical nonzero UUID only
for the existing mock account/floor facts and readiness. It performs no identity
write. The ready record preserves prior fields and adds `clock_mode`,
`signer_mode`, and the copied public `test_jwks` path. Never print private-key
contents. These inputs do not configure production Auth or grant new authority;
run only in an owned private Linux fixture without outer ports or schema work.

### Flutter (клиент)

- Виджеты и логика: **`flutter test`**.
- Presigned object-storage uploads/downloads: `flutter test
  test/presigned_upload_test.dart` checks the native Compose-to-loopback
  connection with its signed Host. Separately run `flutter test --platform
  chrome test/presigned_upload_web_test.dart` to check that browsers preserve
  the exact signed endpoint and send no Gateway bearer or custom Host. Web
  deployments must issue a browser-reachable storage URL with the required
  storage CORS policy; browsers cannot spoof the signed Host after a rewrite.
- Сценарии с бэкендом: **`integration_test`**, когда API для сценария доступен (staging или локальный compose).
- Статический анализ: **`flutter analyze`** — в CI на каждый PR; для a11y см. [features/accessibility.md](features/accessibility.md).

### Продуктовые / ручные

- Доступность и критические сценарии релиза — по чеклистам в `docs/features/` (например [accessibility.md](features/accessibility.md)); ручная регрессия перед релизом — по решению ответственного за выкат.

---

## Локальная отладка логов (Tier-0)

После `docker compose --profile app up` сервисы пишут **JSON-строки** в stdout с полями `service`, `request_id`, `event` (`http_access`, `grpc_call`, `nats_publish`, `nats_consume`, `ws_connect`, `ws_fanout`, …), а также `chat_id` / `message_id` / `event_id` на критичном пути Messaging → NATS → Realtime.

Собрать логи в один файл для grep и для Cursor-агента:

```bash
make compose-logs-collect
```

Результат (в `.gitignore`):

- `.local/compose.log` — сырой вывод `docker compose logs` (с префиксом контейнера).
- `.local/dev.ndjson` — только JSON-строки приложений (для `jq`, парсеров и агента).

Примеры поиска:

```bash
# одна цепочка по correlation id (из ответа Gateway или Flutter X-Request-Id)
rg "request_id.*<id>" .local/dev.ndjson

# async-связка без общего request_id
rg "chat_id.*<uuid>" .local/dev.ndjson
```

Локальный compose для `app` profile задаёт `LOG_LEVEL=debug`. Web Flutter WS не шлёт custom headers — `request_id` на upgrade генерирует Gateway.

---

## Debug by `request_id` on staging

После деплоя observability-стека ([deploy/observability/README.md](../deploy/observability/README.md)) и приложения в `voice-staging` — сквозная отладка одного пользовательского действия по correlation id. Спека: [features/observability.md](features/observability.md).

### Prerequisites

1. Observability в кластере: `kubectl get pods -n voice-observability` — все `Running` (см. `scripts/staging/apply-observability.sh`).
2. Grafana (ClusterIP): `kubectl port-forward -n voice-observability svc/grafana 3000:80` → http://localhost:3000 (admin из Secret `grafana-admin`).
3. Два тестовых аккаунта на staging, Realtime/WS доступен (клиент или curl + wscat).

### E2E сценарий: отправка DM

The script `scripts/staging/smoke-request-id.sh` is the DEV variant described
below; its email lifecycle uses the DEV-only verification mail fixture and it
refuses other namespaces. This section's staging procedure remains a manual
check against already verified staging test accounts.

1. **Войти** (REST login через Gateway) и сохранить access token.
2. **Открыть WS** к Realtime (`/ws` через Gateway) с тем же JWT — иначе `ws_fanout` не появится у получателя.
3. **Отправить DM** — `POST /api/v1/messages` (или эквивалентный маршрут send message) с телом `chat_id` + текст.
4. **Скопировать `request_id`** из заголовка ответа **`X-Request-Id`** (клиент Flutter тоже может слать свой id; на WS upgrade id выдаёт Gateway).
5. **Проверить цепочку в Loki** — по возрастанию `time` ожидаются события:

| Порядок | `event` | Типичный `service` |
|---------|---------|-------------------|
| 1 | `http_access` | `gateway` |
| 2 | `grpc_call` | `messaging` (или `chat`) |
| 3 | `nats_publish` | `messaging` |
| 4 | `nats_consume` | `realtime` |
| 5 | `ws_fanout` | `realtime` |

На пути также допустимы `grpc_call` в других Tier-0 сервисах и поля `chat_id`, `message_id`, `event_id` в JSON.

### DEV account and request-id smoke

The automated smoke creates two disposable `voice-qa.test` accounts, checks
their session-bound `EMAIL_PENDING` state, retrieves each delivered OTP from the
documented `voice-dev-mail` loopback endpoint, and verifies using the normal
authenticated Auth route. It then exercises normal password login and refresh,
creates a DM, sends and reads a message, confirms the receiver's WebSocket
subscription and matching `message_create` frame, and checks the
Gateway-to-Realtime Loki chain. No database or administrative account access is
used. It requires the DEV and observability namespaces, Gateway ingress, and
`curl`, `kubectl`, `python3`, Node.js 24, `npm`, and `tar` in the Linux execution
environment. The script fetches `ws` 8.21.0 from the official npm registry into
its private temporary directory and verifies the published SHA-512 integrity
before use (`Vsp28b7DRcimFQvrqu2Wek3z1iYxDCWqHYB8Qsnk/S4RfaCQzPGPyBNuVjJV3cd6UiKtUtp6sNM77gWvzcCH+g==`):

```bash
VOICE_STAGING_URL=https://api-dev.comrade.click \
VOICE_STAGING_NAMESPACE=voice-dev \
VOICE_STAGING_WS_URL=wss://api-dev.comrade.click/ws \
scripts/staging/smoke-request-id.sh
```

The DEV mail fixture access is documented in [`scripts/dev/README.md`](../scripts/dev/README.md).
For repeat runs, `VOICE_SMOKE_SESSION_FILE` may point to an absolute path
outside the checkout. The script stores the verified account pair and rotated
refresh tokens there as an owner-only mode-0600 file; subsequent runs use the
normal Auth refresh endpoint instead of registering or logging in again. The
fixture is bound to the canonical `https://api-dev.comrade.click` origin and
`voice-dev` namespace; a fixture from an older unbound schema or another
environment is rejected before any refresh request. Keep that file private
and do not commit it. Refresh and access credentials are kept in mode-0600
private files while issuing requests; the WebSocket client reads its bearer
token from a private file rather than a process argument.

For ad-hoc disposable QA fixtures, preserve the origin/environment binding,
owner-only permissions, symlink rejection and duplicate-key rejection too.
Persist the intended account identifiers and recoverable credentials privately
before the first registration/auth request, then save each normal session stage
atomically. This allows recovery after a parser/assertion failure without losing
the fixture or repeatedly creating accounts against a shared-IP login limit.
Reuse the documented normal refresh path; this advice does not authorize account
mutation outside the selected environment or putting credentials in arguments/logs.

### LogQL (Explore или Grafana)

Promtail ставит label **`namespace`** из pod metadata; **`request_id`** — только в JSON тела строки (не label). Запрос:

```logql
{namespace="voice-staging"} | json | request_id="<paste-id-here>"
```

Узкий фильтр по типу события:

```logql
{namespace="voice-staging"} | json | request_id="<id>" | event="ws_fanout"
```

Локальный паритет (без Loki): `rg "request_id.*<id>" .local/dev.ndjson` после `make compose-logs-collect` — см. раздел выше.

### Grafana dashboard

Дашборд **Voice Logs — Request ID** (`uid`: `logs-request-id`) в папке **Voice**:

- URL после port-forward: http://localhost:3000/d/logs-request-id/voice-logs-request-id
- Вставить `request_id` в переменную панели → логи + timeline по `event`.

Связанные дашборды: **Voice Overview** (RPS, 5xx), **Tier-0 Paths** (SLO). Provisioning: `deploy/observability/grafana/`.

### Если цепочка обрывается

| Симптом | Куда смотреть |
|---------|----------------|
| Нет `http_access` | Gateway pod logs / ingress; проверить `X-Request-Id` в ответе |
| Нет `grpc_call` | Messaging/Chat pod, Prometheus `grpc_server_handled_total` |
| Нет `nats_publish` / `nats_consume` | NATS JetStream, Infra dashboard, `nats_jetstream_stream_messages_pending` |
| Нет `ws_fanout` | Realtime WS подписка на `chat_id`, `realtime_ws_connections_active` |

Полный smoke-чеклист после деплоя observability — [deploy/observability/README.md](../deploy/observability/README.md#smoke-after-deploy).

---

## Что запускать локально перед PR

Минимум для затронутого кода:

| Изменения в | Локально                                                          |
|-------------|-------------------------------------------------------------------|
| Репозиторий целиком (sign-off / tier 3) | **`make build-all`** + **`make flutter-ci`** — как nightly **`local-ci-parity`**; на каждый коммит не обязательно |
| Flutter (как в CI, на хосте с SDK) | из корня: **`make flutter-ci`** — tokens/contrast/a11y-web-axe, `flutter pub get`, `flutter analyze`, `flutter test` в `src/frontend/` (в т.ч. якорный `test/e2e_readiness_test.dart`). Device-driver host matrix: `flutter test integration_test/device_driver_smoke_test.dart -d flutter-tester` (CI job **`flutter-device-driver`**). Android emulator (skips without device): `flutter test integration_test/android_emulator_deeplink_test.dart -d emulator-5554`. См. [`integration_test/README.md`](../src/frontend/integration_test/README.md) и скилл `flutter-web-client-testing` |
| Go-сервис   | `cd src/backend/<service> && CGO_ENABLED=0 go test ./...`; общий прогон — **`make golangci-ci`** из корня или `golangci-lint run ./...` в каталоге модуля; для Gateway дополнительно `CGO_ENABLED=1 go test -race ./...` (цель **`gateway-test-race-ci`**, входит в `build-all`) |
| Voice R22.3 D1 | Из `src/backend/voice`: `go test ./internal/roomlifecycle -run '^Test(VoiceDBRedisDivergenceMigration|PostgresLifecycleStore)_D1[^0-9].*$' -count=1`; Testcontainers владеет изолированной PostgreSQL DB и cleanup, shared Compose data не используется. |
| Auth (Java) | `cd src/backend/auth && mvn -B test` (как **`make auth-test-ci`** / CI); образ и smoke — Docker, см. CI ниже |
| Developer Portal | `cd src/developer-portal && npm ci && npm test && npm run build` (как job **`developer-portal`** в CI) |
| Admin / Portal runtime config | Run each app's `npm test`, `npm run lint`, `npm run build`; from root run `node src/admin/browser/runtime-config.mjs src/admin/dist admin` and repeat with `src/developer-portal/dist portal`. Run `python3 src/admin/browser/runtime-config-manifest-test.py`, `python3 scripts/prod/frontend-runtime-render_test.py`, and `python3 scripts/staging/check-frontend-runtime-config_test.py`. The Chromium test checks one built asset identity across two origin bindings, missing/invalid/wrong-origin zero-API behavior and Retry. |

Additionally, `make build-all-breaking` runs `buf breaking` against local `master`; PR CI uses its configured base. Host buf commands are `make buf-lint`, `make buf-format`, and `make buf-breaking`. `make testcontainers-prune` deliberately refuses unscoped cleanup. Run `make -j4 ci-script-tests-pr` for current path-filter, promotion-matrix and gate inputs; `make ci-script-tests` includes the historical audit. Exact current-merge verification is separate, as described below.

The hosted Flutter job runs the AppIcon, draft storage, linked OAuth and
presigned-upload browser suites in Chrome. It validates the JSON event stream
against all four files and rejects skipped tests. The draft storage suite is
browser-only; a VM run cannot prove it. The local isolated Flutter runner is
still diagnostic and does not issue a complete preflight receipt. It checks
captured source files and rejects added files or links outside its declared
Flutter output paths after each command. Those output paths, the SDK and
external package provenance remain unbound inputs; a successful diagnostic
therefore cannot authorize a PR handoff.
The full discovered inventory is divided into bounded VM and Chrome commands;
each batch has its own event stream, and every required suite must appear in its
assigned platform's results. This avoids Windows batch-command length limits.
On Windows each diagnostic command runs in an owned Job Object with a bounded
execution wait and verified descendant cleanup. Linux execution is refused until
equivalent containment is available. Process creation, filesystem operations,
temporary output growth, and external SDK provenance are not yet bounded or
fully verified, so this remains diagnostic evidence only.
When tests skip, the diagnostic records each skipped identity and its matching
sections in the captured E2E catalog; an empty section list remains unmapped.
Catalog membership is not execution evidence and does not waive a skip.
When the accessibility capture suite is selected, the runner enables its
capture switch and requires both H/V PNGs inside the isolated snapshot; their
hashes are recorded as diagnostic artifacts.

`python3 scripts/ci/local_preflight.py --base <full-base-sha> --head
<full-head-sha> --head-ref <branch> --output <new-directory-outside-checkout>
--flutter <flutter> --go <go-executable>`
uses the shared local plan, runs selected Go service short-test diagnostics and
the full Flutter diagnostic when selected. The Go executable must match the
version of the digest-pinned Go image, which must already be present locally;
the shared planner also requires `jq`. Go runs use host-wide slots, isolated
source and Docker resources, limit Go runtime and package parallelism to two,
and report individual skipped tests. The shared
pkg non-short job, integrations, lint and other unimplemented checks remain
`not_run`. The report remains incomplete and the command exits nonzero even
when all implemented diagnostics succeed: it cannot authorize a PR handoff.

When `ci-script-tests` is selected, `--ci-staging-assets <json-file>` supplies
the local inputs for experimental owned staging, proofs and policy shard executions. The JSON
object must contain exactly the absolute paths `helper`, `jq`, `postgres_image`,
`go_image`, `python_image`, `nats_image`, `nats_box_image`, `alpine_image`,
`packages`, `wheel`, `preservation_go_cache`, `staging_go_proxy`, and `kubectl` for the planner-selected policy shard. The runtime aggregate additionally requires `phase0_jdk` and `phase0_compose`; the preflight projects the exact assets needed by each shard before starting any worker. The worker
requires its pinned byte hashes and image IDs, a clean checkout matching the
planned commit and tree, and a full base SHA. Pinned client, Go, Node and Docker
CLI images must already be present. Missing or mismatched inputs fail closed.
The caller must maintain trusted control of the supplied assets, Docker daemon
and private socket. The worker uses an owned internal network and a private
daemon; uncertain operations retain the journal and slot for reconciliation.
Its serial, offline Go execution and adapted preservation fixture differ from
hosted CI. The preservation prebuild alone uses `GOGC=30` and a 1200 MiB Go
soft memory target; this does not bound total container memory or establish why
a previous compiler process was killed. Startup, filesystem work and disk growth are not independently
bounded. The maintained staging path has not yet passed a complete real run.
The policy runner checks 69 recipes and the hosted compiled-source test, with
source-pinned, individually identified Linux platform skips; any other skip
fails closed. The policy client requires the pinned Linux/amd64 kubectl v1.34.0
binary (SHA-256 `cfda68cba5848bc3b6c6135ae2f20ba2c78de20059f68789c090166d6abc3e2c`)
for the real loopback transport test. The hosted runner's kubectl version is
not pinned by the workflow and has not been established. It uses the owned Python 3.12 client, which differs from the
hosted step's default system Python. The policy path has not yet passed a
complete real run. Without the runtime image inputs below, these runs omit runtime and leave `ci-script-tests`
`not_run` and preflight incomplete even if the staging receipt is produced.

Supplying `--ci-runtime-images <json-file>` as well requests the runtime shard
after staging, proofs and policy. The staging-assets JSON must then also contain
absolute paths `phase0_jdk` and `phase0_compose`: the Linux/amd64 Eclipse Temurin
25.0.2+10 JDK archive (SHA-256 `987387933b64b9833846dee373b640440d3e1fd48a04804ec01a6dbf718e8ab8`)
and Docker Compose v2.39.4 binary (SHA-256 `7af95166a730b87e172d4fc9aefea8725d3c6c7327d59149267b452114ddb7d4`).
The worker checks these bytes and transferred copies before use. The runtime-images JSON object has exactly `postgres`, `docker_cli`
and `python` records; each record contains an absolute `archive` path, its
`sha256`, and an `image_id`. PostgreSQL must match the fixture's fixed image ID;
the caller authenticates the Docker 27 CLI and Python bookworm image identities
and maintains custody of all archives. The owned private daemon loads and
checks the supplied images. Phase0 retains the hosted non-root suite and verifies
its sole source-pinned root-only skip separately in an owned network-none client
with a read-only source volume. The worker binds both outputs to the exact test
inventory; no unaccounted skip is accepted. The pinned story-media script is adapted to use the
preloaded PostgreSQL image, an offline build and an owned private socket and
read-only source mirror. These adaptations and image provenance are not hosted
CI parity. The runtime path has not yet passed a complete real run, and
`ci-script-tests` remains `not_run`.

When the planner selects `backend-auth`, an optional private Auth diagnostic
can run one selected Testcontainers case by supplying all of `--auth-helper`,
`--auth-helper-sha256`, `--auth-postgres-archive`,
`--auth-postgres-archive-sha256`, `--auth-postgres-image-id`,
`--auth-maven-image-id`, `--auth-maven-cache`, and `--auth-maven-cache-sha256`.
Paths must be absolute and image identities and SHA-256 values exact. The
caller must maintain exclusive control of the Docker daemon, network and socket;
the diagnostic does not authenticate helper, images, tools or Maven cache, or
establish crash and concurrency acceptance. It reports `incomplete` and leaves
the full `backend-auth` check `not_run`; it is not a preflight or PR receipt.

An alternative full Auth diagnostic requires `--auth-full-helper`,
`--auth-full-helper-sha256`, `--auth-full-client-image-id`,
`--auth-full-maven-cache`, `--auth-full-maven-cache-sha256`,
`--auth-full-images`, and `--auth-full-images-sha256`. The image manifest maps
each of `postgres:16-alpine`, `redis:7-alpine`, `redis:7.4-alpine`,
`nats:2.11-alpine`, and `migrate/migrate:v4.18.1` to an absolute `archive`,
archive `sha256`, and `image_id`. Duplicate image IDs are not supported.
The client image must provide Java, Maven, make, Bash and the required shell
utilities. It executes `make auth-test-ci` offline against a private daemon,
checks the captured Surefire reports, and records non-required skips. The
caller must authenticate inputs and maintain exclusive daemon, socket and shared
workspace custody. This diagnostic remains `incomplete`, keeps `backend-auth`
`not_run`, and does not establish crash recovery or full preflight acceptance.

When the plan selects `compose-config` or `compose-e2e`, the optional fixed
private Compose fixture requires all five explicit inputs: `--compose-helper`,
`--compose-helper-sha256`, `--compose-postgres-archive`,
`--compose-postgres-archive-sha256`, and `--compose-postgres-image-id`.
It runs a no-network fixture, retains the audited rendered model, and reports
`incomplete`; the selected Compose checks remain `not_run`. It does not run
the Voice stack. The caller must control the daemon, socket and evidence files;
image, helper and tool provenance, concurrent failures and crash recovery are
not established by this diagnostic. Its journal check relies on the trusted
producer and is not an independent Docker or slot observation.

The standalone Windows-only `python -B scripts/ci/local_nats_hosted_diagnostic.py
<request.json>` runs the two named hosted NATS scripts in an owned private
daemon. The JSON object has exactly `root`, `source`, `source_copy`, `deps`,
`deps_archive`, `output`, `helper`, `helper_hash`, `jq`, `jq_hash`, and `images`.
`source` is a captured source identity and `source_copy` its verified private
materialization; `deps` and `deps_archive` are the matching private Go dependency
acquisition and archive. `images` is an ordered array of `[archive path,
archive SHA-256, image ID, tag]` for `nats:2.12-alpine`,
`natsio/nats-box:0.18.0`, and `alpine:3.22`. Helper, jq and image paths must be
absolute; hashes and IDs must match the supplied files and images. The caller
authenticates these inputs and controls the daemon, private socket, network and
files throughout. Each script must produce its own expected marker without its
skip marker, and source and inner resources are rechecked after each run. The
worker and broker use isolated Python imports (`-I -S`); only an explicit set
of project modules is available to the worker. The caller must trust the
parent Python interpreter and its ambient import environment. The Windows
diagnostic broker provides a bounded wait. A timeout or
worker failure can leave a journal and slot needing explicit recovery; the
broker does not prove generic Docker-daemon cancellation. Process startup,
filesystem operations and temporary disk growth have separate bounds or none.
The report is always `incomplete`: the remaining shards, registry tag drift, Go
ziphash provenance, filesystem and disk bounds, crash and foreign-sentinel
acceptance remain unproven. This diagnostic cannot authorize a PR handoff.

The staging preservation script retains its hosted `/var/tmp` behavior by
default. Its optional `--owned-temp-root <absolute-private-directory>` creates
the fixture under a canonical, non-symlink, mode-0700 directory owned by the
non-root caller. The caller must retain exclusive custody and arrange the same
private path in its isolated Docker daemon; the option alone does not establish
source identity, daemon isolation, or safe cleanup after a crash.

When the plan selects `protobuf`, the local diagnostic runs Buf lint, format,
breaking against the captured immutable base, verifies Go generated-code drift
with a private Git index, checks the Auth proto copy, compiles Auth with a private Maven
repository, and runs the R23 contract suite including its Java generated
consumer on an isolated source snapshot. It records tool versions, binary hashes,
command logs and test counts, and requires no skipped R23 tests. It requires
Go 1.26.10, Buf 1.50.0,
JDK 25, Maven 3.9, and the CI-pinned Go protobuf generators on `PATH`. This
diagnostic is still incomplete:
dependency provenance and other consumer compilation are not yet complete
preflight evidence.

The separate `flutter-music-proof` job retains the Music capture and playback
artifacts. Its selector includes frontend sources, shared test support, assets,
dependency files, design inputs, protobufs, and global or unknown CI changes.
Only the four independent browser suite files named above are excluded when
they are the sole changes; mixed changes still select Music. The required-jobs
gate rejects a missing, skipped, cancelled, or failing selected Music proof.
Help handoff and web startup run as separate required proof jobs. They use the
same conservative selection as Music, including shared frontend, design and
protocol changes; only the four independent browser suite files are excluded
when they are the sole changes. Each job builds the Help launcher fixture and
retains the original evidence artifact name. A hosted same-candidate comparison
passed both checks; two startup loading screenshots differed between runs.

A manual CI dispatch with `profile=full` forces the code scope even when the
last diff contains only documentation. A code push to `master` also selects the
full validation jobs while retaining the independently resolved staging image
build matrix. `full-validation-aggregation` rejects a non-master or mismatched
checkout, non-successful required jobs, incomplete Go service matrices, and
unresolved Go integration source inventory. Image promotion and staging deploy
depend on that aggregation in the same workflow run. The gate currently fails
because the integration inventory remains unresolved, so these promotions and
deployments remain blocked until the missing proofs are implemented and pass.
The source audit is limited to `*_integration_test.go`; other test files and
generated or external inputs still require source-binding review.

For PRs, the A1 multi-account proof follows its Auth, User, Chat, Messaging,
Social, Gateway, Realtime, shared-package and protocol inputs. The Flutter
profile-handoff proof follows all four manifest tests, shared frontend source
and test support, package inputs, and the same relevant backend boundaries.
Global and unknown CI changes select both proofs. An unrelated standalone
frontend test does not select either A1 journey by itself; master full
validation remains separate.

The main Go CI jobs and dedicated Go integration proofs resolve and test with
`-mod=readonly`, then reject changes to backend `go.mod` or `go.sum` files,
including ignored newly created sums.
The Gateway integration matrix also runs the Linux `live`-tagged leave diagnostic
parent and requires its 204 and 503 subtests without skips. The child probe is
an explicitly mapped subprocess helper: its 503 path intentionally fails inside
the subprocess and the parent verifies that failure and redacted diagnostic.
The Space native-installer job checks the real 45 and 50 transitions and the
trusted-50 catalog boundary. A source-bound test helper interrupts evidence
writing immediately after each migration; tests then perform the actual
transition and require the catalog and creator cases without skips. The job
removes only the database it just created in its disposable PostgreSQL service
before provisioning each next fixture. Interrupted receipts are test input,
not deployment evidence.
`make backend-test-ci` uses the same read-only resolution; the explicit
`make go-mod-tidy-all` target remains available for intentional maintenance.
Other proof jobs and generated or external inputs still require source-binding
review before the full run can qualify promotion.

### Локальные грабли (Windows, compose E2E)

Типичные сбои при `make build-all` / `make compose-e2e-smoke` на **Windows** (PowerShell). В цепочках команд используй **`;`**, не `&&`.

| Симптом | Причина | Обход / фикс |
|---------|---------|--------------|
| `go test` / `make build-all` → `tls: protocol version not supported` к `proxy.golang.org` | Среда Windows (прокси/TLS), не код | Go-тесты и `go mod tidy` — в контейнере: `docker run --rm -v /var/run/docker.sock:/var/run/docker.sock -v "<repo>:/workspace" -w /workspace golang:1.26-bookworm bash -c "make backend-test-ci-short"` (полный — `backend-test-ci`). Live gateway E2E — отдельный `docker run` с `--add-host=host.docker.internal:host-gateway` и `VOICE_API_BASE_URL=http://host.docker.internal:18080` |
| `make compose-e2e-smoke` падает на шаге gateway | Тот же TLS на хостовом `go test` | Gateway smoke в Docker (см. выше); Flutter smoke — `flutter test` на хосте с `--dart-define=VOICE_RUN_LIVE_INTEGRATION=true` |
| `buf format -d --exit-code` / CRLF в `protos/` | Line endings Windows vs Linux CI | `buf format -w protos/`; при необходимости `*.proto text eol=lf` в `.gitattributes` |
| PATCH `/api/v1/users/me/privacy` → 400 `preset is invalid` | `UpdatePrivacySettings` требует валидный `preset` (`personal` / `gaming` / `work`) при любом PATCH | В E2E-хелперах слать полный gaming preset, не только одно поле; см. `setComposePrivacyAllowDmEveryone` в [`compose_live_helpers_test.go`](../src/backend/gateway/compose_live_helpers_test.go), `allowOpenGamingPrivacy` в [`live_gateway_harness.dart`](../src/frontend/test/support/live_gateway_harness.dart) |
| AddMembers / install bot → 403 `invite blocked by recipient privacy settings` | Игровой preset: `allow_chat_space_invites` = друзья+ДД; бот-актор на том же аккаунте не друг | Chat: bypass для профилей **одного account_id** ([`privacy_audience.go`](../src/backend/chat/internal/grpcsvc/privacy_audience.go)); в тестах — `allowOpenGamingPrivacy` у invitee перед инвайтом |
| `docker build` subscription/moderation/… → `analytics/pb` / `pkg/analyticsevents` | `pkg` тянет `voice.app/voice/analytics`; в Dockerfile не было `COPY analytics/pb` | В Dockerfile сервиса с `../pkg`: копировать `analytics/pb/voice/analytics` на этапах mod download и build; `go mod tidy` в модуле |
| Flutter `encryption_key_backup_e2e_live_test` → `Binding has not yet been initialized` или HTTP 400 на probe | `TestWidgetsFlutterBinding` подменяет сеть; `putKeyBackup` идёт в `FlutterSecureStorage` | Не вызывать `TestWidgetsFlutterBinding` в API live-тесте; передать `backupStorage: InMemorySecureSignalStorage()` в [`VoiceE2eClient`](../src/frontend/lib/backend/e2e_client.dart) |
| Voice Flutter: второй тест в файле таймаутит WS | LiveKit/voice cleanup между сценариями | Явный `dispose` WS, пауза ~2s между тестами, таймаут `waitForOp` 20s — [`voice_call_signaling_e2e_live_test.dart`](../src/frontend/test/voice_call_signaling_e2e_live_test.dart) |
| Gateway smoke «Voice» не бежит | В манифесте имя теста не совпадает с кодом | Код: `TestComposeVoiceCall1to1_live` ([`compose_voice_call_live_test.go`](../src/backend/gateway/compose_voice_call_live_test.go)); манифест [`.github/ci/e2e-features.yml`](../.github/ci/e2e-features.yml) |
| Analytics gateway live skip | Разные env-флаги | Go: `VOICE_RUN_LIVE_COMPOSE=true`; Flutter live: `VOICE_RUN_LIVE_INTEGRATION=true` |
| После правок chat/gateway в коде — E2E всё ещё красные | Compose крутит старые образы | `docker compose --profile app up -d --build chat gateway` (и другие затронутые сервисы) |

Полный sign-off на Windows без WSL: **`make compose-config-ci`**, **`make buf-ci`**, backend в Docker (таблица выше), **`make flutter-ci`** на хосте, compose smoke (gateway Docker + Flutter). Скилл: [`.cursor/skills/voice-project-full-verification/SKILL.md`](../.cursor/skills/voice-project-full-verification/SKILL.md).

Проверка PostgreSQL healthcheck требует запущенный Docker daemon и образы `postgres:16-alpine` и `ghcr.io/jqlang/jq:1.7`: `docker pull postgres:16-alpine; docker pull ghcr.io/jqlang/jq:1.7; bash scripts/ci/compose-postgres-tcp-readiness_test.sh`. Скрипт создаёт и удаляет собственные изолированные ресурсы.

---

## CI (GitHub Actions)

The `codex/contact-requests-panel-acceptance` PR uses the existing Flutter job to
run `test/contact_requests_capture_test.dart` and
`test/message_requests_shell_test.dart` with
`VOICE_CONTACT_REQUESTS_CAPTURE_DIR` and retain `contact-requests-hv-<run_id>`.
Eighteen PNGs show the actual `SocialPanel` pending tab: populated, busy,
empty, loading, load error, Russian populated, and incoming profile lookup
503/404/loading states in H (1280×800 with
a 360px panel) and V (390×844). Capture mode loads the shipped token catalog,
Noto Sans and Material Icons. Nine additional PNGs show the real authenticated
`VoiceApp` request workspace in H/V, its V drawer, and H/V mutation busy/error/
final-return states. Those tests exercise the production folder selection and
request controller paths, keyboard retry, and actor/navigation fencing.
Friendship Pending (§4.3) remains distinct from stranger-DM requests
(§1.1b/1.3a); full original DoD and deployment acceptance require the current
head's actual artifacts and independent review.

Файлы workflow лежат в репозитории; **они начинают выполняться только после** публикации репозитория на GitHub и включения Actions (ветки, secrets для GHCR/staging — [DEPLOYMENT.md](DEPLOYMENT.md)). Локально перед PR — минимум по затронутому коду (таблица выше); полный sign-off: **`make build-all`** + **`make flutter-ci`** (как nightly job **`local-ci-parity`**).

### Node.js в workflow и фронтовых пакетах

На GitHub-hosted runners **Node.js 20 deprecated** (см. [changelog](https://github.blog/changelog/2025-09-19-deprecation-of-node-20-on-github-actions-runners/)): actions с `runs.using: node20` дают warning и принудительно выполняются на Node 24.

**Не использовать Node.js 20** нигде в репозитории — ни в `actions/setup-node`, ни в Docker (`node:20`), ни в `engines` в `package.json`, ни в major-версиях GitHub Actions на node20 runtime (например `actions/upload-artifact@v4`).

| Слой | Канон |
|------|--------|
| `actions/setup-node` | `node-version: '24'` |
| Admin / Developer Portal `package.json` | `"engines": { "node": ">=24.0.0" }` |
| Docker build (admin, portal) | `FROM node:24-alpine` |
| Артефакты CI | `actions/upload-artifact@v6`, `actions/download-artifact@v7` (node24) |

Self-hosted runner на staging: версия runner **≥ 2.327.1** для node24 actions ([setup-github-runner.sh](../scripts/staging/setup-github-runner.sh)).

### Тиры CI

Правила путей: [`.github/ci/path-filters.yml`](../.github/ci/path-filters.yml) (job **`changes`**, [`dorny/paths-filter`](https://github.com/dorny/paths-filter)). Глобальные пути (`Makefile`, `scripts/ci/**`, `protos/**`, `src/backend/pkg/**`, compose и т.д.) расширяют blast radius. PR только с `docs/**` — job **`ci-skip-gate`** (tier 1 пропускается; ссылки — **`docs-link-check`**).

| Tier | Когда | Что |
|------|--------|-----|
| **1 — fast** | каждый PR; push в `master` | path-filtered: protobuf, compose-config, `flutter` (analyze+test), golangci и `backend-go` matrix **только затронутые** сервисы (`go test -short`), **`backend-go-integration-pr`** (полный `go test` по matrix на PR), job **`ci-gate`**. Docker build verify на PR; push в GHCR — только изменённые образы на `master` + **promote** остальных с `github.event.before`. |
| **2 — platform / E2E** | push в `master` (и `workflow_dispatch` → `full`) | selective **build** + **promote** + artifact `stack.lock.yaml`; job **`deploy-staging`** (`workflow_call`) при `STAGING_DEPLOY_ENABLED=true`; Flutter platform smokes (`run_flutter_tier2`); **`compose-e2e`** при изменениях compose, frontend, global, admin или developer-portal (не при Go-only backend diff). |
| **3 — parity** | cron 02:00 UTC; `workflow_dispatch` → `tier3-only` или `full` | **`local-ci-parity`** (`make build-all` + `make flutter-ci`), **`backend-go-integration`** (полный `go test` без `-short`), **`compose-e2e`** на schedule. |

Ручной запуск CI: **Actions → CI → Run workflow** — профиль `auto` (как PR по diff), `tier3-only` (ночной набор), `full` (все тиры).

### Быстрый PR gate и исторический аудит

`ci-script-tests` — четыре обязательных matrix job: `policy`, `runtime`,
`staging`, `proofs`, с `fail-fast: false`. Они сохраняют все прежние команды;
F05–F12 проверяются в `runtime`, а полный F13 replay вынесен в
`f13-historical-replay`. Отдельный `current-diff-contract` обязателен на каждом
PR, включая docs-only: проверяет точные event base/head, два parent commit,
полное дерево чистого merge, неизменность существующих SQL-миграций
(blob/type/mode, включая удаление и rename) и source-disabled Voice scope
текущего diff. Это ограниченный regression guard, не доказательство всей
исторической цепочки или семантической безопасности произвольного Go-кода.
Auth/ACL, protobuf, migration/runtime tests и выбранные E2E остаются в gate.

`chat-auth-java-integration` runs for affected Auth, Chat, shared Go and
protobuf inputs. It compiles the candidate Auth fixture from a clean checkout,
checks the Maven test report and classpath, then runs the actual Go-to-Java
interoperability case against fresh PostgreSQL and Redis with no skips. Its
candidate mode verifies the full checkout identity and preserves the existing
fixed-pin mode for historical component proofs. The receipt binds the result
to the CI candidate and attempt; the case uses a synthetic ingress peer and
does not claim a public end-to-end flow.

`flutter` сохраняет analyze, codegen, a11y, browser proofs и captures.
Полный unit/widget discovery выполняется в четырёх `flutter-unit-tests`
через встроенные `--total-shards=4 --shard-index=0..3`; внутри каждого runner
остаётся `--concurrency=1`. Gate требует успех всей matrix. Ни один результат
валидатора не переиспользуется из кеша: новый base/head, diff или версия
валидатора всегда проверяются заново. Toolchain/dependency caches не заменяют
выполнение тестов.

Compose E2E собирает текущие images через прежний `docker compose up --build`
и выполняет полный smoke. Host Go module downloads и compiled objects
кешируются по явным `go.sum` Gateway, Notification и shared pkg; каждый
`go test` сохраняет `-count=1`. Для отдельной network-free проверки:
`python3 scripts/ci/compose_go_cache_integration.py` — повторные тесты должны
исполняться с тёплым compilation cache, а изменение source вызывать падение.
Первый новый cache key может быть cold seed. Подробнее:
[CI gate policy](../.github/ci/gate-policy.md).

Локальный эквивалент быстрых CI-script checks (Linux/Git Bash, committed
checkout; дополнительно запускаются component checks из этого документа):

```bash
python3 scripts/ci/current-diff-contract.py --base "$(git merge-base HEAD origin/master)"
make -j4 ci-script-tests-pr
```

Для точного GitHub synthetic merge передайте также `--pr-head <event-head>`
и `--target <event-merge-sha>`, используя `--base <event-base>`.
Полный прогон — `make ci-script-tests`; отдельно replay —
`VOICE_CONTRACT_PROFILE=full bash scripts/ci/voice-db-runtime-provisioning-contract_test.sh`.

The full `voice-history-audit` runs nightly at 02:00 UTC and on a manual full
CI dispatch. It is not a dependency of `ci-gate` or staging validation.
Production deployment checks the exact image-tag commit against the latest
successful scheduled audit and its intervening history. The baseline is reused
only when the source is an ancestor and every intervening change is known to
be unrelated; a new merge, unknown inputs, a missing baseline, or an unverifiable history
cause a fresh full audit. The currently recognized unrelated scope is limited
to static `src/frontend/assets/` files. The deployment operator may explicitly
select `skip_voice_history_audit` and provide a single-line reason; the reason
is recorded in the workflow summary. A fresh audit retains the source SHA and
diagnostic log as an artifact for 14 days, including on failure. This history
audit does not replace the current-diff contract or the required PR checks.

Методика, baseline и повторяемый сбор очереди, execution, attempts, mean/p95 —
[CI gate policy](../.github/ci/gate-policy.md). Цель около 10 минут требует
hosted замеров; локальный PASS или длительность одного агрегатора её не доказывает.

Отдельный Linux Compose gate T31 запускается workflow
[`.github/workflows/t31-session-events-e2e.yml`](../.github/workflows/t31-session-events-e2e.yml)
через `workflow_dispatch` на нужном ref или автоматически для затронутых
путей в PR. Он собирает Phase0 fixture, Gateway и GIS из checkout, создаёт
локальные Auth аккаунты через verification-stub и проверяет HTTPS claim,
независимый receiver inbox/effect, lease reclaim и повтор ACK. Для этого gate
не требуются Windows Docker runtime или реальные provider credentials.

Состав tier 1 (детали) в [.github/workflows/ci.yml](../.github/workflows/ci.yml):

1. **Protobuf** (если `protos/` или global): `buf lint`, `buf format`, на PR — `buf breaking` относительно immutable SHA базы из события PR. Checkout должен совпадать с SHA synthetic merge из события, а его два parent — с event base/head; несовпадение или отсутствие объекта останавливает проверку до Buf. В лог выводятся checkout/head/base и tree identities. Продвижение удалённой базовой ветки после создания события не меняет baseline. Регрессии с настоящими Git/Buf (продвижение remote, реальное удаление декларации и несовпадение базы) запускаются командой `python3 scripts/ci/buf-breaking-pr_test.py` и в job `protobuf` после установки Buf.
2. **Compose** (если compose/deploy/global): `docker compose config`; NATS JetStream — [`scripts/ci/compose-nats-jetstream-check.sh`](../scripts/ci/compose-nats-jetstream-check.sh).
3. **Backend Go matrix** (затронутые сервисы): `go test -short ./...`; для **gateway** — `go test -race`; **Docker build+push в GHCR** только tier 2 (push в `master`).
4. **golangci** — только затронутые модули (`pkg` + сервисы из matrix).
5. **Auth** — Maven test на tier 1; Docker smoke + push — tier 2 (`master`).
6. **Flutter** — tier 1: `buf-dart-check`, analyze, test; tier 2: APK / Windows / iOS / Chrome deep-link smoke.
7. **Admin** — tier 1: `npm ci`, **ESLint**, vitest, `tsc`/build; Docker push — tier 2 (`master`). Vitest покрывает OAuth/login, moderation, analytics pages, game catalog config editor, App shell routing smoke (`src/admin/src/test/`). Отдельный [Admin browser](../.github/workflows/admin-browser.yml) на изменениях Admin запускает настоящий Chromium поверх собранного SPA: OAuth S256/invalid state, staff queue/assignment, denied resolution с сохранением ввода, sanction Cancel/Confirm, retry resolve, appeal и logout/direct reload. HTTP fixture проверяет запросы и bearer, но не доказывает живые Auth/staff permissions или backend enforcement. Локальный запуск и exact-source JSON/PNG artifacts: [Admin README](../src/admin/README.md).
8. **Developer Portal** — `npm ci`, test, build; Docker push — tier 2 (`master`). Compose smoke selects `TestComposeDeveloperPortal_live`: real Voice authorization-code/S256 PKCE, callback state, single-use code and authenticated Bot registration/list/detail with cleanup, plus redirect/password/verifier/replay/unauthenticated denials. It uses the actual Auth/User/Bot services and delivered local email OTP; unavailable opted-in portal fails. This is HTTP protocol/persistence proof, while browser callback execution and staging OAuth remain separate. Exact command and local prerequisites: [Developer Portal README](../src/developer-portal/README.md#tests).
8. Проверка ссылок в `docs/` — [`.github/workflows/docs-link-check.yml`](../.github/workflows/docs-link-check.yml).

**Деплой на staging** — job **`deploy-staging`** в [`ci.yml`](../.github/workflows/ci.yml) вызывает [`staging-deploy.yml`](../.github/workflows/staging-deploy.yml) (`workflow_call`) после `staging-stack-lock`; ручной запуск — **Actions → Staging deploy** с обязательным git SHA. См. [DEPLOYMENT.md](DEPLOYMENT.md). Branch protection: job **`ci-gate`** + [`.github/ci/branch-protection-checklist.md`](../.github/ci/branch-protection-checklist.md).

### Compose E2E (локально и CI)

- Compose bootstrap creates and migrates Realtime's owned `realtime_db` before
  its fail-closed startup. Dirty or nonempty untracked Realtime catalogs fail
  without `migrate force`; reconcile their migration history explicitly.
  `python3 docker/postgres/compose-realtime-migrations_test.py` exercises fresh,
  existing-volume, repeated, dirty and untracked paths through the shell pipeline
  with stateful command fixtures; actual Compose startup remains a separate check.

The E2E catalog CLI (`bash scripts/ci/e2e-manifest.sh MANIFEST SECTION`)
requires Python 3 and `python3 -m pip install -r scripts/ci/requirements-e2e.txt`
(PyYAML 6.0.3). CI installs that dependency in every catalog-consuming job.
Selections preserve YAML list order, decode quoted/flow values, and reject
duplicate keys/items, empty or invalid test lists before emitting names.
`full_gateway_run` is read by the same parser as a nonempty single-line scalar.
Smoke/full runners stop before suite execution on parser failure; no test-list
fallback is inferred. Check with `python3 scripts/ci/e2e_manifest_test.py` and
`bash scripts/ci/e2e-manifest_test.sh`. These offline checks do not establish
live smoke duration, provider coverage or product acceptance.

- **`make compose-migrate-all`** — golang-migrate для Go-owned БД; контейнер migrate подключается к Postgres через `host.docker.internal` (compose publish на хост). При конфликте порта **5432** на хосте задайте `POSTGRES_PORT` в `.env` и `VOICE_MIGRATE_PG_HOST` (например `host.docker.internal`) в окружении перед migrate.
- **Smoke (tier 2 / master push):** `scripts/ci/compose-e2e-smoke.sh` — один тест на фичу продукта.
- **Full:** `make compose-e2e-live` или workflow [compose-e2e-live.yml](../.github/workflows/compose-e2e-live.yml).
- The live MatchFound Compose scenario layers `docker-compose.voice-space-media.yml` over the base stack. On Linux, generate its fixture with `PHASE0_MATCHSQUAD_FIXTURE_UID=65532` so the image's existing nonroot Chat and Matchmaking processes can read only their owner-only provider material; mounts remain read-only. The fixture supplies a disposable Matchmaking signer/JWKS, dedicated mTLS material for Chat and Voice provisioning, and the paired Gateway-to-Voice delegated-member listener required by Voice's fail-closed startup contract. CI proof wrappers restore only these fixture paths to the runner before removing their temporary directory. Production still needs its separately managed Matchmaking signing keys, published JWKS, trust configuration, and certificates.
- **A1 isolated Flutter proof:** `make compose-a1-flutter-profile-handoff` runs ordered T-055 profile handoff, T-106 soft-delete, T-107 folders/Quick Access persistence, and pending-email logout → fresh login → verification against its own Compose project and ports.
- On PR/master runs, `ci-gate` requires literal success from each selected acceptance job, including the A1 proofs, Flutter key-backup matrix/platform jobs, and analytics integration/smoke jobs. Their existing path/event selectors remain authoritative; intentionally unselected jobs may skip. CI-only changes still check selected jobs even when the `code` filter is false. Offline selector/result coverage runs with `make verify-required-jobs-test`.
- T-055 fixture readiness observes the selected fresh DM's Chat `last_message_at` projection before recording the handoff. Messaging preview visibility alone does not prove that Chat's asynchronous ordering projection has settled. The measured two-page/item/cursor/hello-order assertions remain unchanged.
- Манифест фич: [`.github/ci/e2e-features.yml`](../.github/ci/e2e-features.yml).
- Flutter live helper `registerUser` завершает публичный email verification flow через compose-only Resend fixture: `otp/send` → `GET /emails/latest?to=<unique-email>` → `otp/verify`. Compose публикует fixture только на loopback; изолированный runner передаёт его URL через `VOICE_AUTH_MAIL_STUB_URL`. Staging/production сохраняют стандартный endpoint `https://api.resend.com/emails`.

- BE-107 transport acceptance uses the real `ResendMailSender` over HTTP to the
  disposable fixture, with a unique recipient per case and the actual injected
  Auth Clock. Capture is submission evidence only; the client reads OTPs from
  the delivered-only inbox after explicit settlement. Both OTP types cover
  delivery, synchronous rejection, accepted-then-bounced non-delivery, ordinary
  resend recovery, and fresh independent codes just before, at and after the
  ten-minute expiry boundary. Rejected/expired/bounced flows preserve identity,
  password and session state; successful reset revokes refresh sessions.
  Neither NoopMailSender nor the test profile's CapturingMailSender can satisfy
  this transport suite. Test controls stay local to the disposable fixture;
  there is no production clock endpoint or Auth bounce callback.
  Run `node --test docker/verification-stub/server.test.mjs`, then from
  `src/backend/auth` run `mvn -B -Dtest=MailTransportAcceptanceTest test`
  with JDK 25 and Node.js 24 on PATH. The ordinary Auth CI job runs both.
- A1 session-bound verification proof must register/convert once, obtain the
  initial code from the mail fixture, use the restricted bearer for resend/
  verify/status (without an email body field), and assert replacement-session
  persistence before regular routing or bounded `PROMOTION_PENDING` recovery.
- R22.3 D1 tests являются source-disabled storage tests, не public Voice/media
  E2E и не активируют handlers, coordinator или Redis bridge.
- In the full `voice-history-audit`, F13 analyzes every non-first parent of an ordinary integration merge as a separate source contribution, including octopus merges. Each source needs one unambiguous ancestor base; guarded source tree entries and ownership must survive the merge and final result. An octopus may not invent guarded result paths. The outer master release merge still requires the exact push-before first parent and two-parent shape. Ordinary PRs use the bounded current-diff contract above; historical provenance remains a separate release/nightly obligation. Neither check authorizes R22 runtime activation.
- Native/P3 source qualification uses a separate fixed `NATIVE-P3-SOURCE-v1`
  record and later verifier carrier; it does not replace Game/R1/R2 or the
  Voice013 guarded-correction authority. The complete repaired projection,
  dependency context, literal A1–A4/B1–B4 evidence matrix and source reviews must
  qualify together. An `OPEN` clause rejects source authority even after a
  source-closure review. The current record is an input checkpoint with that
  matrix explicitly open; its existence is not a green F13 result.
- The separately typed native master consumer validates the actual PR merge or
  push event, event-before/base, exact two-parent `[base, source]` result and
  complete source/result mode/type/OID/deletion parity. Its content baseline is
  distinct from the actual master event base. Real producer rows retain source
  ownership, nested bases, content history, runtime flags and intermediate
  guarded taint; only an exact reviewed pre-H restoration can discharge that
  intermediate obligation. Post-H content, missing/forged rows and incomplete
  closure do not qualify. Synthetic fixture state cannot change the selected
  production record, producer map or history ledger. Focused tests run through
  `make ci-script-tests`; no ACTIVE A3/v6/MM/provider deployment is implied or
  required for this dormant/restrictive source unit.

### Space Voice leave failure diagnostics

The real public leave helper adds a locally generated `X-Request-Id` and emits
`VOICE503_DIAGNOSTIC` only before a non-204 failure. Its strict HTTP 204 assertion,
received-RTP assertions and SDK-close/deferred-leave order remain unchanged.
The diagnostic reads at most 4,097 bytes and emits only closed public error/message
inventory and the validated local request ID; arbitrary bodies, headers, JWTs,
DSNs and reader errors are never logged.

From `src/backend/gateway`, run `go test -run
'^TestComposeSpaceLeaveDiagnostic(AllowsOnlyPublicInventory|RejectsUnsafeInput|BoundsBodyRead)$' .`
for the ordinary privacy checks. The offline real-helper probe is
`CGO_ENABLED=1 go test -tags=live -run
'^TestComposeSpaceLeaveDiagnosticPreservesStrictStatus$' .` on Linux with the
existing LiveKit native build dependencies. It uses a private loopback HTTP
fixture and subprocess to verify HTTP 204 success, strict HTTP 503 failure, request correlation
and redaction; it does not run a service stack or reproduce a historical CI cause.

### E2E по фичам

Краткий каталог smoke-тестов (полный список — в манифесте выше).

| Фича | Gateway (`TestCompose*`) | Flutter live |
|------|--------------------------|--------------|
| Auth | `TestComposeAuthLifecycle_live` | `auth_logout_e2e_live_test` |
| Friends / DM | `TestComposeFriends_live`, `TestComposeDMRealtime_live` | `friends_e2e_live_test`, `dm_two_users_e2e_live_test` |
| Voice 1:1 | `TestComposeVoiceCall1to1_live` | `voice_call_signaling_e2e_live_test` |
| Groups | `TestComposeGroups_live` | `groups_e2e_live_test` |
| Spaces | `TestComposeSpaces_live` | `spaces_creation_e2e_live_test` |
| Matchmaking | `TestComposeMatchmakingSearch_live` | `matchmaking_e2e_live_test` |
| Search | `TestComposeSearch_live` | `search_e2e_live_test` |
| Moderation | `TestComposeModeration_live` | `moderation_e2e_live_test` |
| Billing | `TestComposeBilling_live` | `billing_e2e_live_test` |
| Stories | `TestComposeStories_live` | `stories_e2e_live_test` |
| Bots | `TestComposeBotsSlash_live` | `bots_slash_e2e_live_test` |
| Deep links | `TestComposeDeepLinks_live` | `deeplink_invite_e2e_live_test` |
| E2E encryption | `TestComposeE2EKeyBackup_live` | `encryption_key_backup_e2e_live_test` |
| Analytics (staff) | `TestComposeAnalytics_live`, `TestComposeAnalyticsExport_live` | — (admin web only) |

For staging accounts that already share a DM, Flutter also has the isolated
`existing_accounts_dm_ws_live_test.dart` check. It requires the separate
`VOICE_RUN_EXISTING_ACCOUNT_DM_WS=true` flag and
`VOICE_LIVE_SENDER_EMAIL`, `VOICE_LIVE_SENDER_PASSWORD`,
`VOICE_LIVE_RECEIVER_EMAIL`, and `VOICE_LIVE_RECEIVER_PASSWORD` environment
variables (plus optional per-account `*_TOTP_CODE` values). It logs into the
existing accounts, confirms the same DM is in both `main` inboxes, and sends a
unique message while the receiver is subscribed over WebSocket. It does not
register accounts, change privacy settings, or fetch history to satisfy the
delivery assertion. The generic `VOICE_RUN_LIVE_INTEGRATION` flag does not
enable this check.

For a dedicated pair of existing staging test accounts that are not friends
and have no existing request record between them, run
`existing_accounts_friend_request_ws_live_test.dart` with
`VOICE_RUN_EXISTING_ACCOUNT_FRIEND_REQUEST_WS=true` and
`VOICE_LIVE_REQUESTER_EMAIL/PASSWORD` plus
`VOICE_LIVE_RECIPIENT_EMAIL/PASSWORD` (optional per-account
`*_TOTP_CODE`). It connects the recipient to Realtime before sending one
friend request, requires the matching `notification` WebSocket event, and
checks the recipient's REST request snapshot without reloading. The recipient
declines the request in teardown so it does not remain pending; Social retains
the request as `declined` for the requester, so this is a one-shot check for
that pair. Do not unfriend established accounts to prepare it. This test does
not create accounts or change privacy settings and is not enabled by the
generic `VOICE_RUN_LIVE_INTEGRATION` flag.

Opt-in: `VOICE_RUN_LIVE_COMPOSE=true` в `src/backend/gateway` для analytics live tests; требуется compose app stack с ClickHouse + staff token.

---

## Покрытие и качество

- Минимальный процент покрытия кода **не** задаём.
- Новый нетривиальный код сопровождается тестами на основную ветку поведения и известные edge cases; для новой нетривиальной логики порядок работы — раздел «Порядок разработки (TDD)» выше.
- Падение теста в `master` — приоритет исправления наравне с регрессией в проде.

---

## Связанные документы

- [CONTRIBUTING.md](CONTRIBUTING.md) — ветки и PR
- [DEPLOYMENT.md](DEPLOYMENT.md) — где гоняются тесты в CI и staging
- [OPERATIONS.md](OPERATIONS.md) — canary, rollback

### User catalog actor snapshot (BE085)

Run `cd src/backend/user; go test ./internal/grpcsvc -run TestCatalogActor -count=1`
for the exact ownership/deleted_at/nonmutation PostgreSQL suite and the real
mTLS/JWKS/request-binding/shared-miniredis transport checks. Run
`cd src/backend/pkg; go test ./socialprincipal -run TestCatalogActor -count=1`
for the signed caller matrix and complete listener configuration checks.
The User `-short ./...` suite keeps the protected Messaging snapshot regressions.
PostgreSQL tests require Docker and are distinct from short tests; miniredis is
explicitly a test dependency, not deployed Redis acceptance. Execution details:
[catalog actor ExecPlan](testing/catalog-actor-states-exec-plan.md).
