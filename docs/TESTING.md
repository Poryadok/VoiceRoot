# Тестирование

Договорённости по слоям тестов, стеку и CI. Детали релиза и отката — [OPERATIONS.md](OPERATIONS.md); локальный подъём сервисов — [PLAN.md](PLAN.md) (Docker Compose), [DEPLOYMENT.md](DEPLOYMENT.md).

---

## Цели

- Регрессии ловятся **до** merge в `master`: линтеры и тесты в CI обязательны после включения branch protection на `master`.
- Критичные пути (авторизация, выдача JWT, маршруты Gateway, сохранение сообщений) покрываются **интеграционными** тестами по мере появления реализации.
- Контракты gRPC/REST: совместимость protobuf/JSON и тесты на стороне consumer. **Pact** и аналогичные инструменты **не** внедряем до отдельного решения в документации.

---

## По языкам и компонентам

### Go (сервисы, Gateway, Realtime и др.)

- Фреймворк: стандартный пакет **`testing`**, табличные тесты где уместно.
- Утверждения: **`github.com/stretchr/testify`** (`require` / `assert`) — единый стиль по репозиторию для читаемости.
- Интеграционные тесты с PostgreSQL/Redis: **testcontainers-go** или зависимости из `docker compose` в CI — способ выбрать при добавлении первого такого набора и описать в корне репозитория (README или `docs/`).
- HTTP: `httptest` для хендлеров Gateway без поднятия сети.
- gRPC: in-process server или клиент в тесте — как в уже существующем сервисе с тестами.

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
| Go-сервис   | `go test ./...`, `golangci-lint run` (или эквивалент из Makefile) |
| Auth (Java) | `./mvnw test` или `./gradlew test` — по сборке проекта            |
| Flutter     | `flutter analyze`, `flutter test`                                 |

Предпочтительно единая точка входа: `Makefile` или скрипты в корне репозитория (когда появятся).

---

## CI (GitHub Actions)

Состав workflow для PR в `master` (фаза 0 в [PLAN.md](PLAN.md)):

1. Линтеры по затронутым языкам.
2. Юнит- и быстрые интеграционные тесты.
3. Сборка Docker-образов для изменённых сервисов (с кэшем слоёв).
4. Деплой на **staging** — [DEPLOYMENT.md](DEPLOYMENT.md).

---

## Покрытие и качество

- Минимальный процент покрытия кода **не** задаём.
- Новый нетривиальный код сопровождается тестами на основную ветку поведения и известные edge cases.
- Падение теста в `master` — приоритет исправления наравне с регрессией в проде.

---

## Связанные документы

- [CONTRIBUTING.md](CONTRIBUTING.md) — ветки и PR
- [DEPLOYMENT.md](DEPLOYMENT.md) — где гоняются тесты в CI и staging
- [OPERATIONS.md](OPERATIONS.md) — canary, rollback


