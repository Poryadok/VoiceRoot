# Жизненный цикл первичного профиля (Auth ↔ User)

Источники: [DATA_MODEL.md](../DATA_MODEL.md) (JWT `user_id` / `profile_id`), [user-service.md](user-service.md), [auth-service.md](auth-service.md).

## Роли

| Сервис | БД | Что владеет |
|--------|-----|-------------|
| Auth | `auth_db` | `accounts`, refresh, JWT issuance |
| User | `user_db` | `profiles` (канонический `profile_id`), онбординг |

## Момент создания `profiles`

- **Регистрация** и **логин** (первая сессия) и **refresh** (новая пара токенов) должны выдавать access JWT, в котором уже есть **`profile_id`**, совпадающий с **`profiles.id`** для этого аккаунта.
- Первичная строка `profiles` создаётся **до** подписи access token (нет окна, когда клиент получил токен без профиля в БД User).

## Поведение v1 (Фаза 1)

1. После успешного создания `accounts` (register) или при выдаче сессии (login/refresh) Auth запрашивает **идемпотентное** «обеспечить первичный профиль» для `account_id`.
2. Если строка с `is_primary = true` уже есть — возвращается её `id`.
3. Если нет — вставляется новая строка `profiles` с уникальной парой `(username, discriminator)` и человекочитаемым `display_name` (по умолчанию из email/телефона), создаётся строка `onboarding_state` для этого `profile_id`.
4. В JWT кладутся `user_id` (= `accounts.id`) и `profile_id` (= `profiles.id`).

## Мульти-профиль (будущее)

Переключение активного профиля (`SwitchProfile`) меняет только **выдачу следующего** access token (claim `profile_id`); первичный профиль остаётся в `profiles` с `is_primary = true`. Детали продукта — [multi-profile.md](../features/multi-profile.md) (Фаза 13+).

## Техническая отметка (bootstrap в коде)

До выделения User gRPC в проде допускается второй JDBC datasource в Auth только на `user_db` для шага «ensure primary profile»; целевое состояние — **вызов User Service** ([EXEC_PLAN.md](../EXEC_PLAN.md)).
