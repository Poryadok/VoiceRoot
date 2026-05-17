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
- Интеграционные тесты с PostgreSQL/Redis: **testcontainers-go** или зависимости из `docker compose` в CI — способ выбрать при добавлении первого такого набора и описать в корне репозитория (README или `docs/`).
- HTTP: `httptest` для хендлеров Gateway без поднятия сети.
- gRPC: in-process server или клиент в тесте — как в уже существующем сервисе с тестами.

### API Gateway

- Локально: из [`src/backend/gateway/`](../src/backend/gateway/) запускать `go test ./...`; для race-проверки на хосте с CGO — `CGO_ENABLED=1 go test -race ./...`.
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

## Что запускать локально перед PR

Минимум для затронутого кода:

| Изменения в | Локально                                                          |
|-------------|-------------------------------------------------------------------|
| Репозиторий целиком (как в CI, через Docker) | из корня: **`make build-all`** — compose config, buf (lint + format check), тесты всех Go-сервисов и `pkg`, **golangci-lint** по каждому модулю (конфиг [`.golangci.yml`](../.golangci.yml), линтер ставится через `go install` внутри образа Go 1.26), **`go test -race` только для Gateway**, Maven test Auth, сборка образов `voice-<service>:local` ([Makefile](../Makefile)). **Flutter** в этот Docker-конвейер не входит — см. строку ниже |
| Flutter (как в CI, на хосте с SDK) | из корня: **`make flutter-ci`** — `flutter pub get`, `flutter analyze`, `flutter test` в `src/frontend/` (в т.ч. якорный `test/e2e_readiness_test.dart`). Каталог [`integration_test/`](../src/frontend/integration_test/) — под будущие device/e2e сценарии, см. README там и скилл `flutter-web-client-testing` |
| Go-сервис   | `go test ./...`; общий прогон линтера — **`make golangci-ci`** из корня или `golangci-lint run ./...` из каталога модуля с конфигом из корня репозитория; для Gateway дополнительно (уже в `make build-all` через Docker) `CGO_ENABLED=1 go test -race ./...` |
| Auth (Java) | `./mvnw test` или `./gradlew test` — по сборке проекта            |

Дополнительно: **`make build-all-breaking`** — то же + `buf breaking` против локальной ветки `master` (на PR в CI база другая — см. ниже). Хостовый buf: `make buf-lint`, `make buf-format`, `make buf-breaking`.

---

## CI (GitHub Actions)

Файлы workflow лежат в репозитории; **они начинают выполняться только после** публикации репозитория на GitHub и включения Actions (ветки, secrets для GHCR/staging — [DEPLOYMENT.md](DEPLOYMENT.md)). До этого локальная проверка в духе job’ов CI — **`make build-all`**; для Flutter — дополнительно **`make flutter-ci`** (на хосте с установленным SDK).

Состав для PR в `master` (фаза 0 в [PLAN.md](PLAN.md)), как задумано в [.github/workflows/ci.yml](../.github/workflows/ci.yml):

1. **Protobuf**: `buf lint`, `buf format`, на PR — `buf breaking` относительно базовой ветки.
2. **Compose**: `docker compose config` (валидация файла); затем проверка, что в конфиге есть сервис **`nats`** с JetStream (флаг **`-js`** в `command`) — [`scripts/ci/compose-nats-jetstream-check.sh`](../scripts/ci/compose-nats-jetstream-check.sh) и фильтр [`scripts/ci/compose-nats-jetstream.jq`](../scripts/ci/compose-nats-jetstream.jq) (нужен `jq` на runner). Локально без `jq` на хосте: то же через образ `ghcr.io/jqlang/jq:1.7`, см. цель **`compose-config-ci`** в [Makefile](../Makefile).
3. **Backend Go matrix**: `go test ./...` и Docker build для каждого Go-сервиса в `src/backend/<service>/`; для **gateway** дополнительно `CGO_ENABLED=1 go test -race ./...`; **push в GHCR** только при **push** в `master` (теги `:latest` и `:<git_sha>`). Для PR — только сборка без push.
4. **golangci** (отдельный job): `go install golangci-lint` (v2, см. workflow) и прогон по всем модулям `src/backend/pkg` и `src/backend/<service>/` с [`.golangci.yml`](../.golangci.yml) в корне.
5. **Auth (Java)**: Maven test; Docker build с загрузкой образа в локальный engine (`voice-auth:ci`); **smoke запущенного контейнера** против Postgres+Redis из [`docker-compose.yml`](../docker-compose.yml) — [`GET /health`](../src/backend/auth/src/main/java/voice/backend/auth/HealthController.java), JWKS по REST, gRPC `GetJWKS` (скрипт [`scripts/ci/auth-container-smoke.sh`](../scripts/ci/auth-container-smoke.sh)); push образа в GHCR только при push в `master` после успешного smoke.
6. **Flutter** ([`src/frontend/`](../src/frontend/)): `flutter pub get`, `flutter analyze`, `flutter test` — job `flutter` в [.github/workflows/ci.yml](../.github/workflows/ci.yml). Локально не входит в **`make build-all`** (Docker-конвейер); использовать **`make flutter-ci`** или команды из таблицы выше. Расширенные сценарии с реальным API и `integration_test` + драйвер — по мере появления (см. [`src/frontend/integration_test/README.md`](../src/frontend/integration_test/README.md), скилл `flutter-web-client-testing`).
7. Проверка относительных ссылок в `docs/` при изменениях в документации — `.github/workflows/docs-link-check.yml`, конфиг `.markdown-link-check.json` в корне.

**Деплой на staging** вынесен в отдельный workflow [.github/workflows/staging-deploy.yml](../.github/workflows/staging-deploy.yml): триггер `workflow_dispatch` (ручной запуск с тегом образа) и, при переменной `STAGING_DEPLOY_ENABLED=true`, автозапуск после успешного `CI` на push в `master`. Секреты, GHCR и namespace — [DEPLOYMENT.md](DEPLOYMENT.md).

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
