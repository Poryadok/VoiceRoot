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

### Help: реальный browser handoff

Обычный Flutter job CI собирает `test/fixtures/settings_help_web_launcher_smoke.dart`
и запускает `node scripts/ci/settings-help-browser-handoff.mjs
src/frontend/build/help-launcher-web <evidence-dir>` из корня (Node.js 24,
`CHROME_BIN=google-chrome`, `GITHUB_SHA` — полный SHA проверяемого checkout).
Перед отдельным запуском из `src/frontend` нужны `flutter pub get` и
`flutter build web --release --no-pub --no-web-resources-cdn --target
test/fixtures/settings_help_web_launcher_smoke.dart --output build/help-launcher-web`.
Fixture использует настоящий `HelpSheet` и `url_launcher`; mock HTTP изолирует
только посторонние backend-запросы. В H/V доверенные pointer clicks проходят
Settings → Help → README / Issues; проверяются `Page.windowOpen.userGesture`,
точный URL нового target, ровно одна новая вкладка и сохранение Help без ошибки.
Artifact `settings-help-browser-<run_id>` содержит head SHA (`VOICE_SOURCE_SHA`,
если задан) и SHA checkout (`GITHUB_SHA`), JSON событий/targets,
версию Chrome и шесть PNG. Это доказательство передачи ссылки браузеру;
загрузка содержимого внешнего GitHub и привязка к deployed SHA проверяются отдельно.

### Go (сервисы, Gateway, Realtime и др.)

- Фреймворк: стандартный пакет **`testing`**, табличные тесты где уместно.
- Утверждения: **`github.com/stretchr/testify`** (`require` / `assert`) — единый стиль по репозиторию для читаемости.
- Интеграционные тесты с PostgreSQL/Redis: **testcontainers-go** (обёртки в [`src/backend/pkg/integrationtest/`](../src/backend/pkg/integrationtest/postgres.go)). Запускать **`go test` на хосте** при работающем Docker daemon (сокет доступен процессу теста). Не запускать Go-тесты внутри `docker run` без проброса сокета — testcontainers не поднимутся. На Windows Ryuk отключён в `integrationtest.ConfigureDockerTesting()`; после прогона — **`make testcontainers-prune`** (входит в `make build-all` / `backend-test-ci`).
- **`-short` vs полный прогон:** в PR/push CI job **`backend-go`** запускает `go test -short ./...` — тесты с `testing.Short()` (testcontainers, долгие HTTP/webhook) пропускаются. Полный `go test ./...` по матрице сервисов — nightly job **`backend-go-integration`** (cron + `workflow_dispatch`) и локально при необходимости: `make backend-test-ci` или `cd src/backend/<service> && go test ./...`. Паритет с PR CI на хосте: **`make backend-test-ci-short`**.
- HTTP: `httptest` для хендлеров Gateway без поднятия сети.
- gRPC: in-process server или клиент в тесте — как в уже существующем сервисе с тестами.
- Voice R22.3: `go test ./internal/roomlifecycle -run '^Test(VoiceDBRedisDivergenceMigration|PostgresLifecycleStore)_D1[^0-9].*$' -count=1`,
  затем `go test ./internal/roomlifecycle -count=1`, `go test -race
  ./internal/roomlifecycle -count=1`, `go test ./... -count=1`, `go vet ./...`
  и repository-pinned `golangci-lint run ./...`. Migration/classifier gate
  требует real PostgreSQL 16 с zero skips; Redis 7 понадобится для deferred
  D2/D3 bridge tests, не для D1-only classifier transaction.

### Space tree integration proof

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

### Flutter (клиент)

- Виджеты и логика: **`flutter test`**.
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

Дополнительно: **`make build-all-breaking`** — то же + `buf breaking` против локальной ветки `master` (на PR в CI база другая — см. ниже). Хостовый buf: `make buf-lint`, `make buf-format`, `make buf-breaking`. После ручного прогона интеграционных тестов без `build-all`: **`make testcontainers-prune`** (удаляет только контейнеры с label `org.testcontainers`, не трогает `voice-*` compose). Контракт path-filter / promote matrix / **`ci-gate`**: **`make ci-script-tests`** (parity с job `staging-stack-lock` drift-check — **`make image-catalog-drift-check`**).

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

Отдельный Linux Compose gate T31 запускается workflow
[`.github/workflows/t31-session-events-e2e.yml`](../.github/workflows/t31-session-events-e2e.yml)
через `workflow_dispatch` на нужном ref или автоматически для затронутых
путей в PR. Он собирает Phase0 fixture, Gateway и GIS из checkout, создаёт
локальные Auth аккаунты через verification-stub и проверяет HTTPS claim,
независимый receiver inbox/effect, lease reclaim и повтор ACK. Для этого gate
не требуются Windows Docker runtime или реальные provider credentials.

Состав tier 1 (детали) в [.github/workflows/ci.yml](../.github/workflows/ci.yml):

1. **Protobuf** (если `protos/` или global): `buf lint`, `buf format`, на PR — `buf breaking` относительно базовой ветки.
2. **Compose** (если compose/deploy/global): `docker compose config`; NATS JetStream — [`scripts/ci/compose-nats-jetstream-check.sh`](../scripts/ci/compose-nats-jetstream-check.sh).
3. **Backend Go matrix** (затронутые сервисы): `go test -short ./...`; для **gateway** — `go test -race`; **Docker build+push в GHCR** только tier 2 (push в `master`).
4. **golangci** — только затронутые модули (`pkg` + сервисы из matrix).
5. **Auth** — Maven test на tier 1; Docker smoke + push — tier 2 (`master`).
6. **Flutter** — tier 1: `buf-dart-check`, analyze, test; tier 2: APK / Windows / iOS / Chrome deep-link smoke.
7. **Admin** — tier 1: `npm ci`, **ESLint**, vitest, `tsc`/build; Docker push — tier 2 (`master`). Vitest покрывает OAuth/login, moderation, analytics pages, game catalog config editor, App shell routing smoke (`src/admin/src/test/`).
8. **Developer Portal** — `npm ci`, test, build; Docker push — tier 2 (`master`).
8. Проверка ссылок в `docs/` — [`.github/workflows/docs-link-check.yml`](../.github/workflows/docs-link-check.yml).

**Деплой на staging** — job **`deploy-staging`** в [`ci.yml`](../.github/workflows/ci.yml) вызывает [`staging-deploy.yml`](../.github/workflows/staging-deploy.yml) (`workflow_call`) после `staging-stack-lock`; ручной запуск — **Actions → Staging deploy** с обязательным git SHA. См. [DEPLOYMENT.md](DEPLOYMENT.md). Branch protection: job **`ci-gate`** + [`.github/ci/branch-protection-checklist.md`](../.github/ci/branch-protection-checklist.md).

### Compose E2E (локально и CI)

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
- F13 analyzes every non-first parent of an ordinary integration merge as a separate source contribution, including octopus merges. Each source needs one unambiguous ancestor base; guarded source tree entries and ownership must survive the merge and final result. An octopus may not invent guarded result paths. The outer master release merge still requires the exact push-before first parent and two-parent shape. This provenance support does not authorize R22 runtime activation.

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
