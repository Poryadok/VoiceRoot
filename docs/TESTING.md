# Тестирование

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

### Go (сервисы, Gateway, Realtime и др.)

- Фреймворк: стандартный пакет **`testing`**, табличные тесты где уместно.
- Утверждения: **`github.com/stretchr/testify`** (`require` / `assert`) — единый стиль по репозиторию для читаемости.
- Интеграционные тесты с PostgreSQL/Redis: **testcontainers-go** (обёртки в [`src/backend/pkg/integrationtest/`](../src/backend/pkg/integrationtest/postgres.go)). Запускать **`go test` на хосте** при работающем Docker daemon (сокет доступен процессу теста). Не запускать Go-тесты внутри `docker run` без проброса сокета — testcontainers не поднимутся. На Windows Ryuk отключён в `integrationtest.ConfigureDockerTesting()`; после прогона — **`make testcontainers-prune`** (входит в `make build-all` / `backend-test-ci`).
- **`-short` vs полный прогон:** в PR/push CI job **`backend-go`** запускает `go test -short ./...` — тесты с `testing.Short()` (testcontainers, долгие HTTP/webhook) пропускаются. Полный `go test ./...` по матрице сервисов — nightly job **`backend-go-integration`** (cron + `workflow_dispatch`) и локально при необходимости: `make backend-test-ci` или `cd src/backend/<service> && go test ./...`. Паритет с PR CI на хосте: **`make backend-test-ci-short`**.
- HTTP: `httptest` для хендлеров Gateway без поднятия сети.
- gRPC: in-process server или клиент в тесте — как в уже существующем сервисе с тестами.

### API Gateway

- Локально: из [`src/backend/gateway/`](../src/backend/gateway/main.go) запускать `go test ./...`; для race-проверки на хосте с CGO — `CGO_ENABLED=1 go test -race ./...`.
- Обязательные contract tests: REST namespaces, `/ws`, JWT/JWKS, Redis blacklist, rate limit groups, trusted proxy `X-Forwarded-For`, CORS preflight, request id/claims propagation, `/api/v1/version`, `/metrics`.
- Production env для auth: `GATEWAY_AUTH_MODE=static` допускается только для dev/tests; рабочий режим задаётся через `GATEWAY_JWKS_URL`, `GATEWAY_JWT_ISSUER`, `GATEWAY_JWT_AUDIENCE`.
- Redis-проверки: `GATEWAY_REDIS_ADDR` включает sliding-window rate limiter и blacklist чтение; blacklist key prefix по умолчанию `jwt:blacklist:`, override — `GATEWAY_JWT_BLACKLIST_PREFIX`.
- Edge smoke после деплоя: `/health`, `/metrics`, `/api/v1/version?platform=<known>&version=<semver>`, один public REST route, один protected REST route с JWT и `/ws` upgrade через Realtime upstream.

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
| Flutter (как в CI, на хосте с SDK) | из корня: **`make flutter-ci`** — `flutter pub get`, `flutter analyze`, `flutter test` в `src/frontend/` (в т.ч. якорный `test/e2e_readiness_test.dart`). Каталог [`integration_test/`](../src/frontend/integration_test/README.md) — под будущие device/e2e сценарии, см. README там и скилл `flutter-web-client-testing` |
| Go-сервис   | `cd src/backend/<service> && CGO_ENABLED=0 go test ./...`; общий прогон — **`make golangci-ci`** из корня или `golangci-lint run ./...` в каталоге модуля; для Gateway дополнительно `CGO_ENABLED=1 go test -race ./...` (цель **`gateway-test-race-ci`**, входит в `build-all`) |
| Auth (Java) | `cd src/backend/auth && mvn -B test` (как **`make auth-test-ci`** / CI); образ и smoke — Docker, см. CI ниже |
| Developer Portal | `cd src/developer-portal && npm ci && npm test && npm run build` (как job **`developer-portal`** в CI) |

Дополнительно: **`make build-all-breaking`** — то же + `buf breaking` против локальной ветки `master` (на PR в CI база другая — см. ниже). Хостовый buf: `make buf-lint`, `make buf-format`, `make buf-breaking`. После ручного прогона интеграционных тестов без `build-all`: **`make testcontainers-prune`** (удаляет только контейнеры с label `org.testcontainers`, не трогает `voice-*` compose).

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
| Flutter `e2e_key_backup_live_test` → `Binding has not yet been initialized` или HTTP 400 на probe | `TestWidgetsFlutterBinding` подменяет сеть; `putKeyBackup` идёт в `FlutterSecureStorage` | Не вызывать `TestWidgetsFlutterBinding` в API live-тесте; передать `backupStorage: InMemorySecureSignalStorage()` в [`VoiceE2eClient`](../src/frontend/lib/backend/e2e_client.dart) |
| Voice Flutter: второй тест в файле таймаутит WS | LiveKit/voice cleanup между сценариями | Явный `dispose` WS, пауза ~2s между тестами, таймаут `waitForOp` 20s — [`voice_call_signaling_e2e_live_test.dart`](../src/frontend/test/voice_call_signaling_e2e_live_test.dart) |
| Gateway smoke «Voice» не бежит | В манифесте имя теста не совпадает с кодом | Код: `TestComposeVoiceCall1to1_live` ([`compose_voice_call_live_test.go`](../src/backend/gateway/compose_voice_call_live_test.go)); манифест [`.github/ci/e2e-features.yml`](../.github/ci/e2e-features.yml) |
| Analytics gateway live skip | Разные env-флаги | Go: `VOICE_RUN_LIVE_COMPOSE=true`; Flutter live: `VOICE_RUN_LIVE_INTEGRATION=true` |
| После правок chat/gateway в коде — E2E всё ещё красные | Compose крутит старые образы | `docker compose --profile app up -d --build chat gateway` (и другие затронутые сервисы) |

Полный sign-off на Windows без WSL: **`make compose-config-ci`**, **`make buf-ci`**, backend в Docker (таблица выше), **`make flutter-ci`** на хосте, compose smoke (gateway Docker + Flutter). Скилл: [`.cursor/skills/voice-project-full-verification/SKILL.md`](../.cursor/skills/voice-project-full-verification/SKILL.md).

---

## CI (GitHub Actions)

Файлы workflow лежат в репозитории; **они начинают выполняться только после** публикации репозитория на GitHub и включения Actions (ветки, secrets для GHCR/staging — [DEPLOYMENT.md](DEPLOYMENT.md)). Локально перед PR — минимум по затронутому коду (таблица выше); полный sign-off: **`make build-all`** + **`make flutter-ci`** (как nightly job **`local-ci-parity`**).

### Тиры CI

Правила путей: [`.github/ci/path-filters.yml`](../.github/ci/path-filters.yml) (job **`changes`**, [`dorny/paths-filter`](https://github.com/dorny/paths-filter)). Глобальные пути (`Makefile`, `scripts/ci/**`, `protos/**`, `src/backend/pkg/**`, compose и т.д.) расширяют blast radius. PR только с `docs/**` — job **`ci-skip-gate`** (tier 1 пропускается; ссылки — **`docs-link-check`**).

| Tier | Когда | Что |
|------|--------|-----|
| **1 — fast** | каждый PR; push в `master` | path-filtered: protobuf, compose-config, `flutter` (analyze+test), golangci и `backend-go` matrix **только затронутые** сервисы (`go test -short`), auth/devportal по путям. **Docker build verify** (`push: false`) на PR для затронутых образов; push в GHCR — tier 2 (`master`). |
| **2 — platform / E2E** | push в `master` (и `workflow_dispatch` → `full`) | `flutter-android-smoke`, `flutter-windows`, `flutter-ios`, `flutter-web-integration`; Docker build+push Go/auth/devportal; **`compose-e2e`** smoke (все фичи) при изменениях backend/frontend/compose. |
| **3 — parity** | cron 02:00 UTC; `workflow_dispatch` → `tier3-only` или `full` | **`local-ci-parity`** (`make build-all` + `make flutter-ci`), **`backend-go-integration`** (полный `go test` без `-short`), **`compose-e2e`** на schedule. |

Ручной запуск CI: **Actions → CI → Run workflow** — профиль `auto` (как PR по diff), `tier3-only` (ночной набор), `full` (все тиры).

Состав tier 1 (детали) в [.github/workflows/ci.yml](../.github/workflows/ci.yml):

1. **Protobuf** (если `protos/` или global): `buf lint`, `buf format`, на PR — `buf breaking` относительно базовой ветки.
2. **Compose** (если compose/deploy/global): `docker compose config`; NATS JetStream — [`scripts/ci/compose-nats-jetstream-check.sh`](../scripts/ci/compose-nats-jetstream-check.sh).
3. **Backend Go matrix** (затронутые сервисы): `go test -short ./...`; для **gateway** — `go test -race`; **Docker build+push в GHCR** только tier 2 (push в `master`).
4. **golangci** — только затронутые модули (`pkg` + сервисы из matrix).
5. **Auth** — Maven test на tier 1; Docker smoke + push — tier 2 (`master`).
6. **Flutter** — tier 1: `buf-dart-check`, analyze, test; tier 2: APK / Windows / iOS / Chrome deep-link smoke.
7. **Developer Portal** — `npm ci`, test, build; Docker push — tier 2 (`master`).
8. Проверка ссылок в `docs/` — [`.github/workflows/docs-link-check.yml`](../.github/workflows/docs-link-check.yml).

**Деплой на staging** вынесен в отдельный workflow [.github/workflows/staging-deploy.yml](../.github/workflows/staging-deploy.yml): триггер `workflow_dispatch` (ручной запуск с тегом образа) и, при переменной `STAGING_DEPLOY_ENABLED=true`, автозапуск после успешного `CI` на push в `master`. Перед apply проверяется наличие образа `gateway:<tag>` в GHCR. Секреты, GHCR и namespace — [DEPLOYMENT.md](DEPLOYMENT.md). Branch protection — [`.github/ci/branch-protection-checklist.md`](../.github/ci/branch-protection-checklist.md).

### Compose E2E (локально и CI)

- **`make compose-migrate-all`** — golang-migrate для Go-owned БД; контейнер migrate подключается к Postgres через `host.docker.internal` (compose publish на хост). При конфликте порта **5432** на хосте задайте `POSTGRES_PORT` в `.env` и `VOICE_MIGRATE_PG_HOST` (например `host.docker.internal`) в окружении перед migrate.
- **Smoke (tier 2 / master push):** `scripts/ci/compose-e2e-smoke.sh` — один тест на фичу продукта.
- **Full:** `make compose-e2e-live` или workflow [compose-e2e-live.yml](../.github/workflows/compose-e2e-live.yml).
- Манифест фич: [`.github/ci/e2e-features.yml`](../.github/ci/e2e-features.yml).

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
| E2E encryption | `TestComposeE2EKeyBackup_live` | `e2e_key_backup_live_test` |
| Analytics (staff) | `TestComposeAnalytics_live`, `TestComposeAnalyticsExport_live` | — (admin web only) |

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
