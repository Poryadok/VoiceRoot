# Пробелы и открытые вопросы (документация)

Итерация после появления каркаса репозитория (Compose, `src/`, buf, миграции v1). Закрывайте пункты PR-ами; крупные темы дублируйте в issue-трекере, если используется.

## Инфраструктура и процесс

- [x] **Деплой на staging из CI (проверено end-to-end)** — push образа **gateway** в GHCR ([`ci.yml`](../.github/workflows/ci.yml)); деплой в `voice-staging` — [`staging-deploy.yml`](../.github/workflows/staging-deploy.yml); публичный доступ: Ingress, FQDN **`voice.tastytest.online`**, Cloudflare Flexible — [DEPLOYMENT.md](DEPLOYMENT.md), [STAGING_SERVER.md](STAGING_SERVER.md).
- [ ] **Сборка Docker-образов в CI** — сейчас job для **gateway**; добавить job’ы, когда появятся `Dockerfile` у остальных сервисов; кэш слоёв по согласованию команды.
- [x] **`make build-all` в корне** — через Docker: compose config, buf, тесты gateway, образ `voice-gateway:local` ([Makefile](../Makefile), [TESTING.md](TESTING.md)). Расширить под `flutter analyze`, другие Go-сервисы, миграции — по мере появления кода в `src/`.

## Контракты и генерация

- [ ] **Первый PR с переносом `s2s.proto` в `voice/s2s/v1/`** — `buf breaking` против старого `master` может падать до merge этого PR; после обновления базы проверка должна стать зелёной.
- [ ] **Расширение `protos/`** — публичные gRPC для Auth, Gateway, messaging и т.д.; политика **генерации** Go/Java (коммит vs генерация при сборке) — зафиксировать в [REPOSITORIES.md](REPOSITORIES.md) и в скриптах сборки.
- [ ] **BSR / удалённый registry** — сейчас только локальный модуль `protos`; при введении Buf Schema Registry обновить CI и [REPOSITORIES.md](REPOSITORIES.md).

## Данные и миграции

### Auth — закрытые прод-блокеры (остаток процесса)

- `mvn -B test` и сборка Docker-образа Auth проходят; REST+gRPC handlers и доменная логика есть.
- **Сделано:** JDBC-репозитории + Flyway `db/migration/V1__auth_schema.sql` при `auth.persistence=jdbc` (по умолчанию); Redis blacklist (`RedisTokenBlacklist`); JWKS из PKCS#8 PEM/файла (`JwtService.fromPkcs8PrivateKeyPem`); `Dockerfile` **EXPOSE 8080 9090**; golang-migrate [000002](src/backend/migrations/auth_db/000002_refresh_tokens_access_jti.up.sql) для колонки `access_jti` у существующих БД от `000001`; Testcontainers-тест `AuthJdbcRedisIntegrationTest` (на runner с Docker; без Docker — skipped).

- [ ] **Auth: smoke REST+gRPC в CI после сборки образа** — отдельный job или шаг (health + минимальный gRPC in-process уже покрыт unit/integration; при необходимости e2e против запущенного контейнера с Postgres+Redis).
- [x] **Auth: PostgreSQL-репозитории вместо in-memory** — JDBC при `auth.persistence=jdbc`; in-memory — профиль `test` (`auth.persistence=memory`).
- [x] **Auth: Redis blacklist вместо in-memory** — при `auth.persistence=jdbc`; in-memory blacklist в `test`.
- [x] **Auth: стабильные JWT-ключи** — PKCS#8 PEM или `auth.jwt.private-key-location` (classpath:/file:); `forTests` только для `memory`.
- [ ] **Двойное ведение Auth DDL** — SQL в [src/backend/migrations/auth_db/](../src/backend/migrations/auth_db/) и Flyway в модуле Auth; зафиксировать единый порядок применения в [README миграций](../src/backend/migrations/README.md) / Auth README.
- [ ] **UUIDv7 для `messages.id`** — в спеке приложение генерирует UUIDv7; при необходимости добавить расширение БД/генератор и уточнить в [messaging-service.md](microservices/messaging-service.md).

## Локальная разработка

- [ ] **Скрипт «поднять стенд + применить все миграции»** — опционально обёртка в `Makefile`/`scripts/` поверх [src/backend/migrations/README.md](../src/backend/migrations/README.md).
- [ ] **NATS / JetStream в Compose** — не входит в минимальный v1 стенд; добавить, когда Realtime/Messaging начнут публиковать события ([CONTRACT_MATRIX.md](CONTRACT_MATRIX.md)).

## Документация

- [x] **Auth README и runtime-порты** — см. [src/backend/auth/README.md](../src/backend/auth/README.md): REST `8080`, gRPC `9090`, переменные окружения/свойства, Postgres+Redis, Testcontainers; синхронизировано с `Dockerfile` (`EXPOSE`).
- [ ] Пройти [DOCS_CONSISTENCY_AUDIT.md](DOCS_CONSISTENCY_AUDIT.md) после первого значимого изменения контрактов или фаз.
