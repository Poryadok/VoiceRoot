---
name: voice-project-full-verification
description: >-
  Verifies the whole Voice monorepo end-to-end: same layers as CI (buf, compose,
  Go services, Auth Maven, Docker images) plus host Flutter CI, optional buf
  breaking, then deep-dives via module skills. Use before merge or release,
  полная проверка репозитория, как CI локально, sign-off всего проекта,
  прогонить всё как в GitHub Actions.
---

# Проверка всего проекта (Voice монорепо)

Оркестрационный скилл: сначала **общий контур как в CI**, затем **углубление по затронутым модулям** через готовые скиллы тестирования/оценки. Не дублирует их чеклисты — при детальном аудите сервиса **прочитать** соответствующий `SKILL.md` и пройти его.

## Когда применять

- Перед merge крупного PR, релизный **sign-off**, «прогнать всё как в CI» локально.
- После рефакторинга инфраструктуры (`docker-compose.yml`, `Makefile`, `protos/`, workflows).
- Когда пользователь явно просит проверить **весь проект**, а не один сервис.

## Входные данные

1. **Ветка и база** для `buf breaking`: на PR — база как в CI (`github.base_ref`); локально — `make build-all-breaking` (нужен ref `master` / `origin/master`).
2. **Что менялось** (дифф или список путей) — чтобы не тратить время на лишние глубокие обходы, если полный прогон уже зелёный.
3. **Доступность инструментов**: Docker (для `make build-all`), Flutter SDK на хосте (для `make flutter-ci`), при необходимости buf на хосте.

В PowerShell цепочки: **`;`**, не `&&`.

---

## Уровень 1 — общие проверки (репозиторий целиком)

Соответствие **`docs/TESTING.md`** (таблица «Репозиторий целиком», CI в `.github/workflows/ci.yml`).

### 1.1 Docker-конвейер бэкенда (без Flutter, без buf breaking)

Из **корня** репозитория:

| Шаг | Команда / смысл |
|-----|------------------|
| Compose | входит в цель ниже |
| Полный backend + образы + buf lint/format | **`make build-all`** → `compose-config-ci`, `buf-ci`, `go test` в `src/backend/pkg` и всех Go-сервисах из `Makefile`, `auth-test-ci` (Maven), сборка Docker-образов `voice-*:local` |

Требует **Docker**. Зафиксировать OK/FAIL по каждой фазе (compose, buf, какой сервис упал).

### 1.2 Совместимость protobuf с базой (как на PR в CI)

| Шаг | Команда |
|-----|---------|
| Breaking | **`make build-all-breaking`** (= `build-all` + `buf-breaking-ci` против `master`) — если нет локального `master`, зафиксировать в отчёте и использовать CI или `git fetch` |

Обязательно, если менялись **`protos/`** или генерируемые контракты.

### 1.3 Flutter (на хосте, не в Docker)

| Шаг | Команда |
|-----|---------|
| Как job `flutter` в CI | **`make flutter-ci`** из корня → `flutter pub get`, `flutter analyze`, `flutter test` в `src/frontend/` |

Версия Flutter — как в **`.github/workflows/ci.yml`** (`flutter-version`), согласованность с `pubspec.yaml` (`environment.sdk`).

### 1.4 Документация (если менялись `docs/`)

- [ ] При наличии workflow **`.github/workflows/docs-link-check.yml`** — прогнать его сценарий локально или доверить CI; при изменении только доков без кода — не пропускать проверку ссылок, если она включена в процессе команды.

### 1.5 Что Level 1 **не** закрывает сам по себе

- **Gateway `go test -race`** с CGO — по `docs/TESTING.md`; при фокусе на gateway добавить точечный прогон (не всегда в `make build-all`).
- **Интеграционные тесты Flutter с бэкендом** — `integration_test` + стенд; см. скилл **`flutter-web-client-testing`**.
- **golangci-lint** по всему монорепо — в CI «по мере появления» (`docs/TESTING.md`); при политике команды — отдельная команда из корня/сервиса.

---

## Уровень 2 — модули через специализированные скиллы

После (или параллельно с) Level 1 открыть **Read** на нужный файл и выполнить релевантные разделы.

| Зона изменений | Скилл (путь) | Заметка |
|----------------|--------------|---------|
| Любой Go-сервис `src/backend/<service>/`, общий `src/backend/pkg/`, особенно детальный аудит | [`go-microservice-task-evaluation`](../go-microservice-task-evaluation/SKILL.md) | Для «полного проекта» — минимум: сервисы из диффа; для релиза — по решению ответственного пройти критичные (gateway, messaging, realtime и т.д.) |
| Auth `src/backend/auth/` | [`java-microservice-task-evaluation`](../java-microservice-task-evaluation/SKILL.md) | Maven/Java/Spring |
| Flutter клиент `src/frontend/` | [`flutter-web-client-testing`](../flutter-web-client-testing/SKILL.md) | Web, виджеты, analyze, при необходимости `build web` / chrome / integration |

Если пользователь требует **строгий TDD-канон** разработки — это не замена; см. **`tdd-code-workflow`** (`.agent/workflows/tdd-code-workflow/SKILL.md`).

---

## Уровень 3 — сквозная логика продукта (лёгкая сверка)

Без расширения `docs/`: только флаги несоответствия известным правилам репозитория.

- [ ] **Два слоя сообщений**: Realtime vs догрузка REST — не перепутаны при изменениях клиента/Gateway/Messaging (`docs/ARCHITECTURE_REQUIREMENTS.md`).
- [ ] **Auth на Java** — не переносится на Go без явного решения в задаче/доках (см. workspace rules / `docs/MICROSERVICES.md`).
- [ ] **План vs код** — при оценке «готово продуктово» сверка с `docs/PLAN.md` (заглушки, переименования сервисов).

---

## Порядок работы агента

1. Зафиксировать **дифф / список путей** и цель (PR / релиз / sanity).
2. Выполнить **Level 1** (`make build-all` ; `make flutter-ci` ; при нужде `make build-all-breaking`) и записать результаты.
3. По диффу выбрать **Level 2**: прочитать соответствующие `SKILL.md` и пройти чеклисты для затронутых модулей (или для критичного набора при полном аудите).
4. Кратко пройти **Level 3**, если затронуты границы сервисов или публичный API.
5. Выдать **сводный отчёт** (шаблон ниже).

---

## Шаблон сводного отчёта

```markdown
## Полная проверка Voice — <ветка / цель>

**Вердикт:** OK / OK с замечаниями / Не OK

### Level 1 (общий CI-контур)
- `make build-all`: OK/FAIL (какой шаг)
- `make build-all-breaking`: OK/FAIL/N/A
- `make flutter-ci`: OK/FAIL

### Level 2 (модульные скиллы)
- Go (`go-microservice-task-evaluation`): … сервисы … → краткий итог
- Java (`java-microservice-task-evaluation`): N/A / …
- Flutter (`flutter-web-client-testing`): N/A / …

### Level 3 (сквозные правила)
- Замечания: …

### Риски и пробелы
- …
```

---

## Источники истины

| Тема | Файл |
|------|------|
| Локальные команды, CI-состав | `docs/TESTING.md` |
| План и фазы | `docs/PLAN.md` |
| Сервисы и границы | `docs/MICROSERVICES.md` |
| Архитектурные инварианты | `docs/ARCHITECTURE_REQUIREMENTS.md` |

При нехватке требований — `docs/TODO.md` или вопрос человеку; не дополнять продуктовое поведение от себя.
