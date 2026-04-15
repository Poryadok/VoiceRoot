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

**Последний проход:** 2026-04-16 — второй проход (integrity audit): сверка [MICROSERVICES.md](MICROSERVICES.md) ↔ [CONTRACT_MATRIX.md](CONTRACT_MATRIX.md) ↔ [microservices/api-gateway.md](microservices/api-gateway.md) (HTTP-префиксы, `/ws`, Federation вне Gateway); таблица JetStream в матрице и Event Bus — совпадают; в блок «Владение данными» добавлена строка **Realtime Service** (Redis / без PostgreSQL) для согласованности с [DATA_STORES.md](DATA_STORES.md); полный прогон `markdown-link-check` по `docs/` с [.markdown-link-check.json](../.markdown-link-check.json) (игнор `https://` и `#`-якорей); CI: [.github/workflows/docs-link-check.yml](../.github/workflows/docs-link-check.yml); [TESTING.md](TESTING.md) обновлён. Ранее в тот же день: `social.events` / `role.events`, карточки NATS/идемпотентность, stories/search/reports, Bot rate limits и версии клиента в [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md), [TODO.md](TODO.md).
