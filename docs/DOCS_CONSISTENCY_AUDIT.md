# Аудит согласованности документации

Чеклист перед мержем PR, затрагивающего архитектуру, API или схемы данных. Обновляйте дату последнего прохода внизу.

## Обязательно

- [ ] Изменения в границах сервисов согласованы с [MICROSERVICES.md](MICROSERVICES.md) и карточкой `docs/microservices/<service>.md`.
- [ ] Публичные маршруты REST/WebSocket отражены в [microservices/api-gateway.md](microservices/api-gateway.md) (или явно помечены как internal-only в карточке сервиса).
- [ ] Идентификаторы и ссылки между БД — [DATA_MODEL.md](DATA_MODEL.md); инвентарь хранилищ — [DATA_STORES.md](DATA_STORES.md).
- [ ] События NATS: publisher/subscriber не противоречат таблице Event Bus в [MICROSERVICES.md](MICROSERVICES.md) и сводной матрице [CONTRACT_MATRIX.md](CONTRACT_MATRIX.md); breaking change в `.proto` — по [REPOSITORIES.md](REPOSITORIES.md).
- [ ] Маршруты Gateway ↔ сервисы: префиксы `/api/v1/...` согласованы с [microservices/api-gateway.md](microservices/api-gateway.md) и таблицей в [CONTRACT_MATRIX.md](CONTRACT_MATRIX.md).
- [ ] SLO/Tier: при изменении критичности пути — [OPERATIONS.md](OPERATIONS.md).

## При изменении схем БД

- [ ] Скоуп волны — [DATA_SCOPE_V1.md](DATA_SCOPE_V1.md) (или фаза в [PLAN.md](PLAN.md)).
- [ ] Порядок миграций expand/contract — [OPERATIONS.md](OPERATIONS.md).

---

**Последний проход:** 2026-04-16 — полный аудит целостности: JetStream `social.events` / `role.events` в [CONTRACT_MATRIX.md](CONTRACT_MATRIX.md) и [MICROSERVICES.md](MICROSERVICES.md); выравнивание карточек сервисов (потоки NATS, идемпотентность Messaging/Subscription); фичи [stories.md](features/stories.md) / [search.md](features/search.md) / [reports.md](features/reports.md); rate limits Bot API и версии клиента в [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md); [TODO.md](TODO.md) мелкие пункты сняты.
