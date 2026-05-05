# Exec plan — приёмка вертикального среза (после Фазы 0)

Связано с [PLAN.md](PLAN.md) («Первый вертикальный срез») и [DEPLOYMENT.md](DEPLOYMENT.md).

## Цель

Воспроизводимая проверка стыка **Auth (Java, JDBC) ↔ PostgreSQL (`auth_db` + `user_db`) ↔ Redis** и выдачи **access JWT с валидными `user_id` и `profile_id`**, где **`profile_id`** — строка первичного профиля в **`user_db.profiles`** (нет окна «токен без профиля» при включённом провижининге User DB).

**Скоуп по [PLAN.md](PLAN.md):** обязательное ядро — Auth + данные в `profiles`. Проверка **через API Gateway** (те же пути `/api/v1/auth/*`, прокси на Auth + `GATEWAY_JWKS_URL`) — **опциональное расширение** того же сценария, когда Gateway уже направлен на живой Auth.

## Предпосылки

- PostgreSQL с БД `auth_db` и `user_db` (схема `profiles` — [migrations/user_db](../src/backend/migrations/user_db/) и [user-service.md](microservices/user-service.md)). Локально: `docker compose up -d` применяет [docker/postgres](../docker/postgres/) (создание БД + DDL `profiles` для `user_db`).
- Redis (blacklist refresh/logout).
- Для режима `auth.persistence=jdbc`: PKCS#8 RSA PEM — `AUTH_JWT_PRIVATE_KEY_PEM` или `AUTH_JWT_PRIVATE_KEY_LOCATION` (в CI/smoke используется тестовый ключ из репозитория, см. ниже).

## Включение провижининга профиля в Auth

В `application.yml` / env:

- `auth.persistence=jdbc`
- `auth.user-db.jdbc-url` — JDBC URL к **`user_db`** (не к `auth_db`). Если свойство пустое при `jdbc`, приложение **не стартует**.
- `auth.user-db.username` / `auth.user-db.password` — учётные данные (по умолчанию совпадают с `spring.datasource.*`, см. `application.yml`).

## Воспроизводимые команды приёмки

Используйте **один** primary-вариант для PR; остальные — паритет CI или ручная отладка.

### 1. Primary (рекомендуется для PR): Maven + Testcontainers

Требуется **Docker** (сокет доступен JVM, как в CI).

Из каталога `src/backend/auth`:

```text
mvn -B test
```

**Что считается пройденным вертикальным срезом:** зелёный `AuthJdbcRedisIntegrationTest` — регистрация, строка в `profiles`, совпадение `profile_id` с БД, `validate`, `login`/`refresh` с тем же первичным профилем, JWKS/claims согласованы с токеном. Юнит-тесты на `memory`-профиле не заменяют этот интеграционный сценарий.

### 2. Паритет CI: образ Auth + compose + smoke-скрипт

Как в job `backend-auth` ([ci.yml](../.github/workflows/ci.yml)): собрать образ, затем скрипт поднимает `postgres` + `redis`, при необходимости накатывает DDL `user_db`, стартует контейнер Auth и проверяет `/health`, JWKS по REST и `GetJWKS` по gRPC.

Из **корня** репозитория (нужны **bash** и **curl**, на Windows — Git Bash или WSL):

```text
docker build -t voice-auth:ci -f src/backend/auth/Dockerfile src/backend/auth
bash scripts/ci/auth-container-smoke.sh
```

Переопределения: `AUTH_IMAGE`, `AUTH_HTTP_PORT` (по умолчанию **18080**), `JWT_KEY_PATH`. Скрипт по завершении делает `docker compose down` и удаляет контейнер Auth — для ручных HTTP-шагов поднимите стек отдельно (см. п. 3) или опирайтесь на п. 1.

### 3. Ручной HTTP + SQL (отладка / демо)

Базовый URL Auth: **`BASE`** = `http://127.0.0.1:8080` (или порт, проброшенный из `docker run`, например **18080** из smoke).

Из **корня** репозитория: собрать образ, поднять Postgres и Redis, определить имя Docker-сети compose (как в [auth-container-smoke.sh](../scripts/ci/auth-container-smoke.sh)) и запустить Auth с пробросом **8080** на хост.

```text
docker build -t voice-auth:local -f src/backend/auth/Dockerfile src/backend/auth
docker compose up -d postgres redis
POSTGRES_CID=$(docker compose ps -q postgres)
NETWORK=$(docker inspect "$POSTGRES_CID" --format '{{range $k, $v := .NetworkSettings.Networks}}{{$k}}{{"\n"}}{{end}}' | head -1)
JWT_KEY="$PWD/src/backend/auth/src/test/resources/jwt-test-private.pem"
docker rm -f auth-local 2>/dev/null || true
docker run -d --name auth-local --network "$NETWORK" \
  -e SPRING_DATASOURCE_URL=jdbc:postgresql://postgres:5432/auth_db \
  -e SPRING_DATASOURCE_USERNAME=voice \
  -e SPRING_DATASOURCE_PASSWORD=voice \
  -e AUTH_USER_DB_JDBC_URL=jdbc:postgresql://postgres:5432/user_db \
  -e AUTH_USER_DB_USERNAME=voice \
  -e AUTH_USER_DB_PASSWORD=voice \
  -e SPRING_DATA_REDIS_HOST=redis \
  -e SPRING_DATA_REDIS_PORT=6379 \
  -e AUTH_JWT_PRIVATE_KEY_LOCATION=file:/run/jwt.pem \
  -v "$JWT_KEY:/run/jwt.pem:ro" \
  -p 8080:8080 \
  voice-auth:local
```

На Windows выполняйте блок в **Git Bash** или WSL (нужны `head`, подстановка `$(...)`). Альтернатива без shell: скопировать флаги из `docker run` в [auth-container-smoke.sh](../scripts/ci/auth-container-smoke.sh) и добавить `-p 8080:8080`, подставив сеть и абсолютный путь к `jwt-test-private.pem`.

Проверка таблицы через compose:

```text
docker compose exec -T postgres psql -U voice -d user_db -c "SELECT id, account_id FROM profiles WHERE is_primary = true ORDER BY created_at DESC LIMIT 5;"
```

### 4. Монорепо / контракты (дополнительно к п. 1–2)

| Область | Команда |
|---------|---------|
| Контракты proto | из корня: `buf lint` и `buf format -d --exit-code` |
| Весь backend как в CI | из корня: `make build-all` ([Makefile](../Makefile), [TESTING.md](TESTING.md)) |
| Доки (ссылки) | workflow `docs-link-check` или ручной обход по [TESTING.md](TESTING.md) |

### 5. Опционально: те же пути через API Gateway

Убедитесь, что Gateway проксирует namespace `auth` на базовый URL Auth (без лишнего path), и задайте валидатор JWT:

- `GATEWAY_REST_UPSTREAMS_JSON` — например `{"auth":"http://host.docker.internal:8080"}` (host зависит от ОС/compose).
- `GATEWAY_JWKS_URL` — `http://<auth-host>:<port>/api/v1/auth/.well-known/jwks.json`
- `GATEWAY_JWT_ISSUER` = `voice-auth`, `GATEWAY_JWT_AUDIENCE` = `voice-client` (как в [application.yml](../src/backend/auth/src/main/resources/application.yml) Auth).

Критерии ниже выполняются с **`BASE` = URL Gateway** и теми же путями `/api/v1/auth/...`, если ответы совпадают с прямым вызовом Auth.

## Критерии приёмки (smoke после `register`)

1. `POST /api/v1/auth/register` с телом JSON (минимум email + password), например  
   `{"email":"slice-check@example.com","password":"Correct horse battery staple","device_info_json":"{}"}`  
   → **200**, в теле есть `access_token`, `account_id`, **`profile_id`** (UUID).
2. `POST /api/v1/auth/validate` с заголовком `Authorization: Bearer <access_token>` → **200**, `user_id` = `account_id`, **`profile_id`** совпадает с шагом 1.
3. Декод JWT (например [jwt.io](https://jwt.io) с публичным ключом из `GET /api/v1/auth/.well-known/jwks.json`): claims **`user_id`**, **`profile_id`** присутствуют и совпадают с п. 1–2.
4. В БД `user_db`:  
   `SELECT id, account_id FROM profiles WHERE account_id = '<account_id>' AND is_primary = true` — **`id` = `profile_id`** из ответа.

Повтор для **`login`** и **`refresh`**: тот же первичный **`profile_id`** для аккаунта.

## Чеклист для описания PR

Вставьте в PR (или приложите вывод команд):

- [ ] Выполнен primary-прогон: `mvn -B test` в `src/backend/auth` при доступном Docker **или** указано, почему использован только smoke-скрипт / ручной прогон.
- [ ] Критерии из раздела «Критерии приёмки» выполнены (при ручном прогоне — кратко: статус-коды + факт совпадения `profile_id` с БД).
- [ ] При изменении контрактов: `buf lint` / `buf format -d --exit-code` (если затронуты `protos/`).

## Примечание по эволюции

Пока User Service (Go) не выделен в отдельный процесс с gRPC, Auth может писать первичный профиль в `user_db` через второй datasource (см. [primary-profile-bootstrap.md](microservices/primary-profile-bootstrap.md)). После появления **EnsurePrimaryProfile** в User Service вызов заменяется на gRPC; критерии приёмки по данным в `profiles` сохраняются.
