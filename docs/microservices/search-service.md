# Search Service

## Обзор

Полнотекстовый поиск по сообщениям, пользователям, пространствам. Стратегия роста: PostgreSQL → Meilisearch → Elasticsearch.

**Язык**: Go
**БД**: PostgreSQL `search_db` (v1) / Meilisearch (v2) / Elasticsearch (v3)

## Ответственность

- In-chat search (поиск внутри конкретного чата/канала)
- Global search (контакты, чаты DM/группы, пространства, сообщения с highlighting)
- Instant search (debounce 300ms на клиенте)
- Пагинация (20 элементов)
- Разделение результатов: контакты, пространства, сообщения
- Federated search (параллельные запросы к нодам, graceful degradation)
- Индексация новых/изменённых сообщений
- Не индексирует E2E зашифрованные сообщения
- Не ищет внутри файлов

## API (gRPC)

```protobuf
service SearchService {
  rpc SearchInChat(SearchInChatRequest) returns (SearchResults);
  rpc SearchGlobal(SearchGlobalRequest) returns (GlobalSearchResults);
  rpc SearchUsers(SearchUsersRequest) returns (UserSearchResults);
  rpc SearchSpaces(SearchSpacesRequest) returns (SpaceSearchResults);
  rpc ReindexChat(ReindexChatRequest) returns (Empty); // admin
}
```

## Стратегия масштабирования

Полная **пороговая матрица** (когда именно v1→v2→v3, Meili vs ES, правила двойной записи): [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) — разделы «Полнотекстовый поиск» и «Пороговая матрица».

| Этап | Условие перехода (кратко) | Технология |
|------|---------------------------|------------|
| v1   | Старт                     | PostgreSQL tsvector + GIN |
| v2   | См. матрицу в ARCHITECTURE | Meilisearch |
| v3   | Только при требованиях вне возможностей Meili; иначе отложено | Elasticsearch |

### v1: PostgreSQL (`search_db`)

Проекции в отдельной БД сервиса (не колонки в `messaging_db.messages`): `message_search_documents`, `profile_search_documents`, `chat_search_documents`, `space_search_documents` — см. [data/target/search_db.md](../data/target/search_db.md). Расширение `pg_trgm` для fuzzy/prefix по мере необходимости.

### v2: Meilisearch

- Consumer из NATS `message.events` → индексация в Meilisearch
- Typo-tolerance, faceted search, instant results
- Абстрагированный интерфейс позволяет подмену без изменения API

## Индексация

```
NATS message.sent/edited/deleted ──► Search Service ──► Update index
```

- Новое сообщение → add to index
- Редактирование → update index
- Удаление → remove from index
- E2E сообщения → skip

## Публикуемые события (→ NATS)

| Событие                | Данные                            |
|------------------------|-----------------------------------|
| `search.query`         | profile_id, query, scope, results_count |
| `search.result_clicked`| profile_id, result_type, result_id|
| `search.zero_results`  | profile_id, query, scope          |

## Зависимости

- **Messaging Service** — (через NATS) получение новых сообщений для индексации
- **Role Service** — проверка прав на чтение канала при поиске
- **Space Service** — валидация доступа
- **Federation Service** — маршрутизация поиска к нодам
