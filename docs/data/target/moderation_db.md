# `moderation_db` — целевая схема

**Сервис:** Moderation ([moderation-service.md](../../microservices/moderation-service.md)). **Шаг порядка:** 13.

Цели жалоб и санкций — UUID в других сервисах (**без FK**).

---

## `reports`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `reporter_profile_id` | `UUID` | NOT NULL |
| `target_type` | `TEXT` | `user` \| `message` \| `space` \| `channel` \| `bot` |
| `target_id` | `UUID` | NOT NULL |
| `category` | `TEXT` | `spam` \| `harassment` \| `offensive` \| `fake` \| `cheating` \| `other` |
| `description` | `TEXT` | NULL |
| `evidence` | `JSONB` | NULL |
| `status` | `TEXT` | `pending` \| `reviewing` \| `resolved` \| `dismissed` |
| `assigned_to_profile_id` | `UUID` | NULL |
| `resolved_at` | `TIMESTAMPTZ` | NULL |
| `resolution` | `JSONB` | NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `(status, created_at)`; `(target_type, target_id)`; `(reporter_profile_id)`.

---

## `sanctions`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `target_account_id` | `UUID` | NOT NULL |
| `type` | `TEXT` | `warning` \| `temp_ban` \| `perm_ban` \| `shadow_ban` \| `mm_ban` |
| `reason` | `TEXT` | NOT NULL |
| `report_id` | `UUID` | NULL, FK → `reports(id)` ON DELETE SET NULL |
| `issued_by_profile_id` | `UUID` | NOT NULL |
| `expires_at` | `TIMESTAMPTZ` | NULL |
| `revoked_at` | `TIMESTAMPTZ` | NULL |
| `revoked_by_profile_id` | `UUID` | NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `(target_account_id, revoked_at)`; `(type, created_at DESC)`.

---

## `appeals`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `sanction_id` | `UUID` | NOT NULL, FK → `sanctions(id)` ON DELETE CASCADE |
| `appellant_account_id` | `UUID` | NOT NULL |
| `reason` | `TEXT` | NOT NULL |
| `status` | `TEXT` | `pending` \| `approved` \| `denied` |
| `reviewed_by_profile_id` | `UUID` | NULL |
| `reviewed_at` | `TIMESTAMPTZ` | NULL |
| `review_notes` | `TEXT` | NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `UNIQUE (sanction_id)`; `(status, created_at)`.

---

## `auto_mod_log`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `target_profile_id` | `UUID` | NOT NULL |
| `trigger` | `TEXT` | `spam_pattern` \| `report_threshold` |
| `action` | `TEXT` | `mute` \| `shadow_ban` |
| `details` | `JSONB` | NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |
| `reverted_at` | `TIMESTAMPTZ` | NULL |

**Индексы:** `(target_profile_id, created_at DESC)`.
