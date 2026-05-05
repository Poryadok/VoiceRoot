# Exec plan — приёмка вертикального среза (после Фазы 0)

Связано с [PLAN.md](PLAN.md) («Первый вертикальный срез») и [DEPLOYMENT.md](DEPLOYMENT.md).

## Цель

Воспроизводимая проверка: **Auth выдаёт access JWT с валидными `user_id` и `profile_id`**, `profile_id` соответствует строке в **`user_db.profiles`** (никакого «окна» без профиля в токене при включённом провижининге User DB).

## Предпосылки

- PostgreSQL с БД `auth_db` и `user_db` (схема `profiles` — [migrations/user_db](../src/backend/migrations/user_db/) и [user-service.md](microservices/user-service.md)).
- Redis (blacklist refresh/logout).
- Переменные Auth см. [auth README](../src/backend/auth/README.md).

## Включение провижининга профиля в Auth

В `application.yml` / env:

- `auth.persistence=jdbc`
- `auth.user-db.jdbc-url` — JDBC URL к **`user_db`** (не к `auth_db`). Если свойство пустое при `jdbc`, приложение **не стартует**.
- `auth.user-db.username` / `auth.user-db.password` — учётные данные (по умолчанию совпадают с `spring.datasource.*`, см. `application.yml`).

## Команды регрессии (локально / PR)

| Область | Команда |
|---------|---------|
| Auth (Java) | из `src/backend/auth`: `mvn -B test` |
| Контракты proto | из корня: `buf lint` и `buf format -d --exit-code` |
| Доки (ссылки) | workflow `docs-link-check` или ручной обход TOC по [TESTING.md](TESTING.md) |

## Критерии приёмки (smoke после `register`)

1. `POST /api/v1/auth/register` → **200**, в JSON есть `access_token`, `account_id`, **`profile_id`** (UUID).
2. `POST /api/v1/auth/validate` с `Authorization: Bearer <access_token>` → **200**, `user_id` = `account_id`, **`profile_id`** совпадает с шагом 1.
3. Декод JWT (например [jwt.io](https://jwt.io) с публичным ключом из `GET /api/v1/auth/.well-known/jwks.json`): claims **`user_id`**, **`profile_id`** присутствуют и совпадают с п. 1–2.
4. В БД `user_db`: `SELECT id, account_id FROM profiles WHERE account_id = '<account_id>' AND is_primary = true` — **`id` = `profile_id`** из ответа.

Повтор для `login` и `refresh`: тот же первичный `profile_id` для аккаунта.

## Примечание по эволюции

Пока User Service (Go) не выделен в отдельный процесс с gRPC, Auth может писать первичный профиль в `user_db` через второй datasource (см. [primary-profile-bootstrap.md](microservices/primary-profile-bootstrap.md)). После появления **EnsurePrimaryProfile** в User Service вызов заменяется на gRPC; критерии приёмки по данным в `profiles` сохраняются.
