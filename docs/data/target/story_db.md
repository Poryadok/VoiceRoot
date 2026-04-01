# `story_db` — целевая схема

**Сервис:** Story ([story-service.md](../../microservices/story-service.md)). **Шаг порядка:** 16.

`author_profile_id`, `viewer_profile_id`, `reactor_profile_id` → User; `media_file_id`, `cover_file_id` → File (**без FK**).

---

## `stories`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `author_profile_id` | `UUID` | NOT NULL |
| `type` | `TEXT` | `photo` \| `video` \| `text` |
| `media_file_id` | `UUID` | NULL |
| `text_content` | `TEXT` | NULL |
| `text_style` | `JSONB` | NULL |
| `game_tag` | `TEXT` | NULL |
| `is_looking_for_party` | `BOOLEAN` | NOT NULL, DEFAULT false |
| `lfp_criteria` | `JSONB` | NULL |
| `mention_profile_ids` | `JSONB` | NULL |
| `view_count` | `INT` | NOT NULL, DEFAULT 0 |
| `visibility` | `TEXT` | `everyone` \| `friends` \| `custom` |
| `expires_at` | `TIMESTAMPTZ` | NOT NULL |
| `archived_until` | `TIMESTAMPTZ` | NOT NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |
| `deleted_at` | `TIMESTAMPTZ` | NULL |

**Индексы:** `(author_profile_id, created_at DESC)`; `(expires_at)`; `(archived_until)` для cleanup.

---

## `story_views`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `story_id` | `UUID` | NOT NULL, FK → `stories(id)` ON DELETE CASCADE |
| `viewer_profile_id` | `UUID` | NOT NULL |
| `is_anonymous` | `BOOLEAN` | NOT NULL, DEFAULT false |
| `viewed_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `PRIMARY KEY (story_id, viewer_profile_id)`; `(viewer_profile_id, viewed_at DESC)`.

---

## `story_reactions`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `story_id` | `UUID` | NOT NULL, FK → `stories(id)` ON DELETE CASCADE |
| `reactor_profile_id` | `UUID` | NOT NULL |
| `emoji` | `TEXT` | NOT NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `UNIQUE (story_id, reactor_profile_id)`; `(story_id)`.

---

## `highlights`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `profile_id` | `UUID` | NOT NULL |
| `name` | `TEXT` | NOT NULL |
| `cover_file_id` | `UUID` | NULL |
| `sort_order` | `INT` | NOT NULL, DEFAULT 0 |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `(profile_id, sort_order)`.

---

## `highlight_stories`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `highlight_id` | `UUID` | NOT NULL, FK → `highlights(id)` ON DELETE CASCADE |
| `story_id` | `UUID` | NOT NULL, FK → `stories(id)` ON DELETE CASCADE |
| `sort_order` | `INT` | NOT NULL, DEFAULT 0 |
| `added_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `PRIMARY KEY (highlight_id, story_id)`; `(story_id)`.
