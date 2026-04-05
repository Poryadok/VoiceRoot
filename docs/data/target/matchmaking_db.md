# `matchmaking_db` — целевая схема

**Сервис:** Matchmaking ([matchmaking-service.md](../../microservices/matchmaking-service.md)). **Шаг порядка:** 12.

`profile_id` → User; `chat_id` / `voice_room_id` в матчах — внешние (**без FK**). Очереди и locks — **Redis**.

---

## `games`

| Колонка                 | Тип           | Описание                                |
|-------------------------|---------------|-----------------------------------------|
| `id`                    | `UUID`        | PK                                      |
| `name`                  | `TEXT`        | NOT NULL                                |
| `icon_url`              | `TEXT`        | NULL                                    |
| `external_id`           | `TEXT`        | NULL                                    |
| `config`                | `JSONB`       | NOT NULL — режимы, роли, ранги, регионы |
| `status`                | `TEXT`        | `active` \ `archived`                   |
| `created_by_profile_id` | `UUID`        | NULL                                    |
| `created_at`            | `TIMESTAMPTZ` | NOT NULL                                |
| `updated_at`            | `TIMESTAMPTZ` | NOT NULL                                |

**Индексы:** `(status, name)`; `(external_id)` UNIQUE WHERE NOT NULL.

---

## `parties`

| Колонка              | Тип           | Описание                   |
|----------------------|---------------|----------------------------|
| `id`                 | `UUID`        | PK                         |
| `leader_profile_id`  | `UUID`        | NOT NULL                   |
| `member_profile_ids` | `JSONB`       | NOT NULL                   |
| `game_id`            | `UUID`        | NOT NULL, FK → `games(id)` |
| `mode`               | `TEXT`        | NOT NULL                   |
| `criteria`           | `JSONB`       | NOT NULL                   |
| `created_at`         | `TIMESTAMPTZ` | NOT NULL                   |
| `disbanded_at`       | `TIMESTAMPTZ` | NULL                       |

**Индексы:** `(leader_profile_id)`; `(game_id)`.

---

## `matches`

| Колонка         | Тип           | Описание                             |
|-----------------|---------------|--------------------------------------|
| `id`            | `UUID`        | PK                                   |
| `game_id`       | `UUID`        | NOT NULL, FK → `games(id)`           |
| `mode`          | `TEXT`        | NOT NULL                             |
| `region`        | `TEXT`        | NOT NULL                             |
| `participants`  | `JSONB`       | NOT NULL                             |
| `voice_room_id` | `TEXT`        | NULL                                 |
| `chat_id`       | `UUID`        | NULL                                 |
| `status`        | `TEXT`        | `active` \ `completed` \ `abandoned` |
| `created_at`    | `TIMESTAMPTZ` | NOT NULL                             |
| `completed_at`  | `TIMESTAMPTZ` | NULL                                 |

**Индексы:** `(game_id, created_at DESC)`; `(status, created_at)`.

---

## `search_sessions`

| Колонка      | Тип           | Описание                                          |
|--------------|---------------|---------------------------------------------------|
| `id`         | `UUID`        | PK                                                |
| `profile_id` | `UUID`        | NOT NULL                                          |
| `party_id`   | `UUID`        | NULL, FK → `parties(id)` ON DELETE SET NULL       |
| `game_id`    | `UUID`        | NOT NULL, FK → `games(id)`                        |
| `mode`       | `TEXT`        | NOT NULL                                          |
| `criteria`   | `JSONB`       | NOT NULL                                          |
| `status`     | `TEXT`        | `searching` \ `matched` \ `timeout` \ `cancelled` |
| `timeout_at` | `TIMESTAMPTZ` | NOT NULL                                          |
| `matched_at` | `TIMESTAMPTZ` | NULL                                              |
| `match_id`   | `UUID`        | NULL, FK → `matches(id)` ON DELETE SET NULL       |
| `created_at` | `TIMESTAMPTZ` | NOT NULL                                          |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL                                          |

**Индексы:** `(profile_id, status)`; `(timeout_at)` для воркера таймаутов.

---

## `match_ratings`

| Колонка            | Тип           | Описание                                       |
|--------------------|---------------|------------------------------------------------|
| `match_id`         | `UUID`        | NOT NULL, FK → `matches(id)` ON DELETE CASCADE |
| `rater_profile_id` | `UUID`        | NOT NULL                                       |
| `rated_profile_id` | `UUID`        | NOT NULL                                       |
| `score`            | `SMALLINT`    | NOT NULL, 1–5                                  |
| `created_at`       | `TIMESTAMPTZ` | NOT NULL                                       |

**Индексы:** `PRIMARY KEY (match_id, rater_profile_id, rated_profile_id)`.

---

## `player_ratings`

Агрегаты по паре (профиль, игра).

| Колонка                  | Тип                | Описание                                     |
|--------------------------|--------------------|----------------------------------------------|
| `profile_id`             | `UUID`             | NOT NULL                                     |
| `game_id`                | `UUID`             | NOT NULL, FK → `games(id)` ON DELETE CASCADE |
| `average_rating`         | `DOUBLE PRECISION` | NOT NULL                                     |
| `total_matches`          | `INT`              | NOT NULL, DEFAULT 0                          |
| `total_ratings_received` | `INT`              | NOT NULL, DEFAULT 0                          |
| `created_at`             | `TIMESTAMPTZ`      | NOT NULL                                     |
| `updated_at`             | `TIMESTAMPTZ`      | NOT NULL                                     |

**Индексы:** `PRIMARY KEY (profile_id, game_id)`.

---

## `mm_bans`

| Колонка                | Тип           | Описание                |
|------------------------|---------------|-------------------------|
| `id`                   | `UUID`        | PK                      |
| `profile_id`           | `UUID`        | NOT NULL                |
| `reason`               | `TEXT`        | NOT NULL                |
| `banned_by_profile_id` | `UUID`        | NOT NULL                |
| `expires_at`           | `TIMESTAMPTZ` | NULL — NULL = permanent |
| `created_at`           | `TIMESTAMPTZ` | NOT NULL                |
| `revoked_at`           | `TIMESTAMPTZ` | NULL                    |

**Индексы:** `(profile_id) WHERE revoked_at IS NULL`; `(expires_at)`.


