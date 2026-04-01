# `federation_db` — целевая схема

**Сервис:** Federation ([federation-service.md](../../microservices/federation-service.md)). **Шаг порядка:** 15.

`user_id` в `fallback_tokens` — аккаунт на master (**логически** `accounts.id`).

---

## `federation_nodes`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `name` | `TEXT` | NOT NULL |
| `host` | `TEXT` | NOT NULL |
| `port` | `INT` | NOT NULL |
| `description` | `TEXT` | NULL |
| `status` | `TEXT` | `pending` \| `active` \| `suspended` \| `defederated` |
| `auth_token_hash` | `TEXT` | NOT NULL |
| `tls_cert_fingerprint` | `TEXT` | NULL |
| `last_heartbeat_at` | `TIMESTAMPTZ` | NULL |
| `last_sync_at` | `TIMESTAMPTZ` | NULL |
| `registered_at` | `TIMESTAMPTZ` | NOT NULL |
| `approved_at` | `TIMESTAMPTZ` | NULL |
| `approved_by_profile_id` | `UUID` | NULL |
| `defederated_at` | `TIMESTAMPTZ` | NULL |

**Индексы:** `(status)`; `UNIQUE (host, port)` при одной ноде на endpoint.

---

## `federation_events`

Журнал sync master ↔ node (опционально персистить не всё — минимум для отладки и DLQ).

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `node_id` | `UUID` | NOT NULL, FK → `federation_nodes(id)` ON DELETE CASCADE |
| `direction` | `TEXT` | `inbound` \| `outbound` |
| `event_type` | `TEXT` | NOT NULL |
| `payload` | `JSONB` | NULL |
| `status` | `TEXT` | `pending` \| `delivered` \| `failed` |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |
| `delivered_at` | `TIMESTAMPTZ` | NULL |

**Индексы:** `(node_id, created_at DESC)`; `(status, created_at)`.

---

## `fallback_tokens`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `user_id` | `UUID` | NOT NULL — account_id на master |
| `node_id` | `UUID` | NOT NULL, FK → `federation_nodes(id)` ON DELETE CASCADE |
| `token_hash` | `TEXT` | NOT NULL |
| `roles` | `JSONB` | NULL |
| `expires_at` | `TIMESTAMPTZ` | NOT NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `UNIQUE (token_hash)`; `(user_id, node_id)`; `(expires_at)`.
