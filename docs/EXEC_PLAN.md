# Exec plan — приёмка вертикального среза (Auth ↔ User primary profile)

Связано с [PLAN.md](PLAN.md) и [DEPLOYMENT.md](DEPLOYMENT.md).

## Цель

Воспроизводимая проверка стыка **Auth (Java, `auth_db`, Redis) ↔ User gRPC (`user_db`)** и выдачи **access JWT с валидными `user_id` и `profile_id`**, где **`profile_id`** возвращён `UserService.EnsurePrimaryProfile`. Auth не получает credentials к `user_db`; нет окна «токен без профиля».

**Скоуп по [PLAN.md](PLAN.md):** обязательное ядро — Auth + данные в `profiles`. Проверка **через API Gateway** (те же пути `/api/v1/auth/*`, прокси на Auth + `GATEWAY_JWKS_URL`) — **опциональное расширение** того же сценария, когда Gateway уже направлен на живой Auth.

## Предпосылки

- PostgreSQL с БД `auth_db`; для live vertical также User Service со своей БД `user_db` (схема `profiles` — [migrations/user_db](../src/backend/migrations/user_db/) и [user-service.md](microservices/user-service.md)). Локальный full-app Compose поднимает оба сервиса и передаёт Auth адрес User gRPC.
- Redis (blacklist refresh/logout).
- Для режима `auth.persistence=jdbc`: PKCS#8 RSA PEM — `AUTH_JWT_PRIVATE_KEY_PEM` или `AUTH_JWT_PRIVATE_KEY_LOCATION` (в CI/smoke используется тестовый ключ из репозитория, см. ниже).

## Настройка стыка Auth ↔ User

В `application.yml` / env:

- `auth.persistence=jdbc`
- `spring.datasource.*` / `SPRING_DATASOURCE_*` — только Auth-owned `auth_db`.
- `auth.user-grpc.addr` / `USER_GRPC_ADDR` — internal gRPC адрес User Service.

Второй datasource и credentials к User-owned БД не поддерживаются: профильные данные доступны Auth только через User RPC.

## Воспроизводимые команды приёмки

Используйте **один** primary-вариант для PR; остальные — паритет CI или ручная отладка.

### 1. Primary (рекомендуется для PR): Maven + Testcontainers

Требуется **Docker** (сокет доступен JVM, как в CI).

Из каталога `src/backend/auth`:

```text
mvn -B test
```

**Что считается пройденным Auth-контрактом:** зелёные JDBC/Redis тесты и gRPC contract tests — register/login/refresh вызывают `EnsurePrimaryProfile`, используют возвращённый `profile_id`, а User unavailable или непригодный профиль не приводит к выдаче новой сессии. Юнит-тесты на `memory`-профиле не заменяют этот сценарий.

### 2. Паритет CI: образ Auth + compose + smoke-скрипт

Как в job `backend-auth` ([ci.yml](../.github/workflows/ci.yml)): собрать образ, затем скрипт поднимает `postgres` + `redis`, стартует контейнер Auth без доступа к `user_db` и проверяет `/health`, JWKS по REST и `GetJWKS` по gRPC. Это container smoke Auth, а не live-проверка профиля.

Из **корня** репозитория (нужны **bash** и **curl**, на Windows — Git Bash или WSL):

```text
docker build -t voice-auth:ci -f src/backend/auth/Dockerfile src/backend/auth
bash scripts/ci/auth-container-smoke.sh
```

Переопределения: `AUTH_IMAGE`, `AUTH_HTTP_PORT` (по умолчанию **18080**), `JWT_KEY_PATH`. Скрипт по завершении делает `docker compose down` и удаляет контейнер Auth — для ручных HTTP-шагов поднимите стек отдельно (см. п. 3) или опирайтесь на п. 1.

### 3. Ручной live vertical через full-app Compose

Full-app Compose связывает Auth с User через `USER_GRPC_ADDR=user:9090`; Auth получает только credentials к `auth_db`, User — к `user_db`. Из корня репозитория:

```text
docker compose --profile app up -d --build postgres redis nats user auth gateway
docker compose ps user auth gateway
```

Базовый URL — Gateway `http://127.0.0.1:18080`; для диагностики смотрите `docker compose logs auth user gateway`.

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
4. В User-owned БД `user_db` (диагностическая проверка, не доступ Auth):
   `SELECT id, account_id FROM profiles WHERE account_id = '<account_id>' AND is_primary = true` — **`id` = `profile_id`** из ответа.

Повтор для **`login`** и **`refresh`**: тот же первичный **`profile_id`** для аккаунта.

## Чеклист для описания PR

Вставьте в PR (или приложите вывод команд):

- [ ] Выполнен primary-прогон: `mvn -B test` в `src/backend/auth` при доступном Docker **или** указано, почему использован только smoke-скрипт / ручной прогон.
- [ ] Критерии из раздела «Критерии приёмки» выполнены (при ручном прогоне — кратко: статус-коды + факт совпадения `profile_id` с БД).
- [ ] При изменении контрактов: `buf lint` / `buf format -d --exit-code` (если затронуты `protos/`).

## Примечание по эволюции

User Service — единственный владелец `user_db`. Auth потребляет существующие internal RPC
`EnsurePrimaryProfile`, `ResolvePrimaryProfileIDs`, `SwitchProfile`, `SetVerification`,
`ClearVerification` и `MarkAccountRegular`; прямой datasource к `user_db` не допускается.
