# Пробелы и открытые вопросы (документация)

Итерация после появления каркаса репозитория (Compose, `src/`, buf, миграции v1). Закрывайте пункты PR-ами; крупные темы дублируйте в issue-трекере, если используется.

## Инфраструктура и процесс

- [x] **Деплой на staging из CI** — в репозитории: push образа **gateway** в GHCR при push в `master` ([`ci.yml`](../.github/workflows/ci.yml)); деплой в `voice-staging` — [`staging-deploy.yml`](../.github/workflows/staging-deploy.yml) (`kubectl` + [`deploy/staging/`](../deploy/staging/)). В GitHub вручную: Environment **staging**, secret **`STAGING_KUBECONFIG`** (kubeconfig в base64), variable **`STAGING_DEPLOY_ENABLED`**=`true` для автодеплоя после зелёного `CI` на `master`; иначе только **`workflow_dispatch`**. Детали — [DEPLOYMENT.md](DEPLOYMENT.md), состав CI — [TESTING.md](TESTING.md).
- [ ] **Сборка Docker-образов в CI** — добавить job’ы, когда появятся `Dockerfile` у сервисов; кэш слоёв по согласованию команды.
- [ ] **Единая точка входа `make` / скрипты** — расширить корневый [Makefile](../Makefile) (или аналог) под `go test`, `flutter analyze`, миграции, когда код появится в `src/`.

## Контракты и генерация

- [ ] **Первый PR с переносом `s2s.proto` в `voice/s2s/v1/`** — `buf breaking` против старого `master` может падать до merge этого PR; после обновления базы проверка должна стать зелёной.
- [ ] **Расширение `protos/`** — публичные gRPC для Auth, Gateway, messaging и т.д.; политика **генерации** Go/Java (коммит vs генерация при сборке) — зафиксировать в [REPOSITORIES.md](REPOSITORIES.md) и в скриптах сборки.
- [ ] **BSR / удалённый registry** — сейчас только локальный модуль `protos`; при введении Buf Schema Registry обновить CI и [REPOSITORIES.md](REPOSITORIES.md).

## Данные и миграции

- [ ] **Двойное ведение Auth DDL** — сейчас SQL в [src/backend/migrations/auth_db/](../src/backend/migrations/auth_db/); после появления Java-модуля Auth перенести source of truth в Flyway рядом с сервисом и убрать дублирование или связать явно в README миграций.
- [ ] **UUIDv7 для `messages.id`** — в спеке приложение генерирует UUIDv7; при необходимости добавить расширение БД/генератор и уточнить в [messaging-service.md](microservices/messaging-service.md).

## Локальная разработка

- [ ] **Скрипт «поднять стенд + применить все миграции»** — опционально обёртка в `Makefile`/`scripts/` поверх [src/backend/migrations/README.md](../src/backend/migrations/README.md).
- [ ] **NATS / JetStream в Compose** — не входит в минимальный v1 стенд; добавить, когда Realtime/Messaging начнут публиковать события ([CONTRACT_MATRIX.md](CONTRACT_MATRIX.md)).

## Документация

- [ ] Пройти [DOCS_CONSISTENCY_AUDIT.md](DOCS_CONSISTENCY_AUDIT.md) после первого значимого изменения контрактов или фаз.
