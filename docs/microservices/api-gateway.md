# API Gateway

## Обзор

Единая точка входа для всех клиентских запросов. Маршрутизирует REST и WebSocket трафик к внутренним сервисам, применяет сквозные политики безопасности и rate limiting.

**Язык**: Go
**Фреймворк**: chi / echo (custom) или Kong

## Ответственность

- Маршрутизация HTTP/REST запросов к соответствующим сервисам (REST → gRPC transcoding)
- Проксирование WebSocket-соединений к Realtime Service
- JWT-валидация (проверка access token, извлечение claims)
- Rate limiting по правилам из конфигурации
- CORS, request/response logging
- Версионирование API (`/api/v1/...`)
- Проверка версии клиента (endpoint `/api/v1/version`, ответ 426 при force_update; Gateway — canonical owner этой политики)
- TLS termination
- Request ID генерация и propagation (для трейсинга)

## Rate Limiting

Правила (из ARCHITECTURE_REQUIREMENTS.md):

| Endpoint группа       | Лимит         | Окно   |
|-----------------------|---------------|--------|
| Auth (login/register) | 5 запросов    | 15 мин |
| OTP                   | 3 запроса     | 10 мин |
| Messages (send)       | 5 сообщений   | 5 сек  |
| File upload           | 10 загрузок   | 1 час  |
| Space creation        | 5 пространств | 1 день |
| Bot API               | 5000 запросов | 1 мин  |

Реализация: Redis sliding window counter. Ключ — `ratelimit:{user_id}:{endpoint_group}`.

## Маршрутизация

```
/api/v1/auth/**          → Auth Service
/api/v1/users/**         → User Service
/api/v1/friends/**       → Social Service
/api/v1/chats/**         → Chat Service
/api/v1/messages/**      → Messaging Service
/api/v1/spaces/**        → Space Service
/api/v1/roles/**         → Role Service
/api/v1/voice/**         → Voice Service
/api/v1/files/**         → File Service
/api/v1/notifications/** → Notification Service
/api/v1/search/**        → Search Service
/api/v1/matchmaking/**   → Matchmaking Service
/api/v1/moderation/**    → Moderation Service
/api/v1/subscription/**  → Subscription Service
/api/v1/bots/**          → Bot Service
/api/v1/stories/**       → Story Service
/api/v1/version          → Локальный конфиг (version check)
/ws                      → Realtime Service (WebSocket upgrade)
```

## Канонический формат клиентских API-доков

`api-gateway.md` фиксирует публичные namespace/route-группы. Детальная предметная семантика описывается в документах целевых сервисов (`docs/microservices/*`).

Для каждого публичного endpoint документация должна содержать:
- HTTP method + route + auth requirement
- request/response schema (обязательные поля и типы)
- error model (status code + `error_code`)
- pagination/курсоры (где нужно)
- idempotency/повтор запроса (где нужно)

## Аутентификация

1. Клиент отправляет `Authorization: Bearer <access_token>`
2. Gateway валидирует JWT (проверка подписи, expiry)
3. Извлекает claims: `user_id`, `profile_id`, `roles`, `subscription_tier`
4. Передаёт claims в gRPC metadata downstream сервисам
5. Публичные endpoints (login, register, version) — без JWT

## Зависимости

- **Redis** — rate limiting (sliding window), чтение JWT blacklist. Зона ответственности с **Auth Service**: [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) (раздел «Redis: API Gateway и Auth Service»).
- **Auth Service** — JWT public key (ротация через JWKS endpoint)
- **Version config store** — таблица `client_versions` (или эквивалентный конфиг-стор) для `/api/v1/version`

## Метрики (→ Analytics)

- `gateway.request.count` — по endpoint, method, status code
- `gateway.request.latency` — p50/p95/p99
- `gateway.ratelimit.hit` — заблокированные запросы
- `gateway.ws.connections` — текущие WebSocket соединения

## Масштабирование

Stateless, масштабируется горизонтально. За внешним Load Balancer (L4/L7).


