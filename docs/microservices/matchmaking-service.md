# Matchmaking Service

## Обзор

Матчмейкинг для геймеров: каталог игр, очереди поиска, алгоритм подбора, рейтинги. Ключевая отличительная фича Voice.

**Язык**: Go
**БД**: PostgreSQL `matchmaking_db`, Redis (очереди)

## Ответственность

- Каталог игр (JSON-документы с режимами, ролями, рангами)
- Глобальный матчмейкинг (v1 — без верификации)
- Space-level матчмейкинг (future — кастомные правила пространства)
- Соло и групповые заявки (party)
- Точные критерии: игра, режим, роль, ранг, регион
- Регион обязателен (нет cross-region)
- Таймауты: 15 мин (default) / 30 мин (expanded)
- Создание матч-отряда при матче
- Рейтинг после завершения (1-5 звёзд)
- Бан из ММ (не затрагивает мессенджер)
- Интеграция со Stories ("ищу пати")
- Нет одновременных поисков (1 активный на профиль)

## API (gRPC)

```protobuf
service MatchmakingService {
  // Каталог игр
  rpc ListGames(ListGamesRequest) returns (GameList);
  rpc GetGame(GetGameRequest) returns (Game);
  rpc CreateGame(CreateGameRequest) returns (Game); // admin/moderator
  rpc UpdateGame(UpdateGameRequest) returns (Game);
  rpc SearchGames(SearchGamesRequest) returns (GameList);

  // Поиск
  rpc StartSearch(StartSearchRequest) returns (SearchSession);
  rpc CancelSearch(CancelSearchRequest) returns (Empty);
  rpc GetSearchStatus(GetSearchStatusRequest) returns (SearchSession);

  // Матч
  rpc GetMatch(GetMatchRequest) returns (Match);
  rpc GetMatchHistory(GetMatchHistoryRequest) returns (MatchList);

  // Рейтинг
  rpc RateMatch(RateMatchRequest) returns (Empty);
  rpc GetPlayerRating(GetPlayerRatingRequest) returns (PlayerRating);

  // MM Ban
  rpc BanFromMM(BanFromMMRequest) returns (Empty); // moderation
  rpc UnbanFromMM(UnbanFromMMRequest) returns (Empty);
  rpc GetMMBanStatus(GetMMBanStatusRequest) returns (MMBanStatus);
}
```

## Модель данных

```
games
├── id (UUID)
├── name
├── icon_url
├── external_id (nullable — для авто-дедупликации)
├── config (jsonb) — {
│     modes: [{ name, slots, roles: [{name, required}], ranks: [{name, value}] }],
│     regions: ["eu", "na", "cis", ...]
│   }
├── status (active | archived)
├── created_by (profile_id)
├── created_at
└── updated_at

search_sessions
├── id (UUID)
├── profile_id
├── party_id (nullable — для групповых)
├── game_id (FK)
├── mode
├── criteria (jsonb — role, rank, region)
├── status (searching | matched | timeout | cancelled)
├── timeout_at
├── matched_at (nullable)
├── match_id (nullable, FK)
├── created_at
└── updated_at

parties
├── id (UUID)
├── leader_profile_id
├── member_profile_ids (jsonb)
├── game_id (FK)
├── mode
├── criteria (jsonb)
├── created_at
└── disbanded_at (nullable)

matches
├── id (UUID)
├── game_id (FK)
├── mode
├── region
├── participants (jsonb — [{profile_id, role, rank}])
├── voice_room_id (nullable — voice room матча / матч-отряда)
├── chat_id (nullable — temp chat)
├── status (active | completed | abandoned)
├── created_at
└── completed_at (nullable)

match_ratings
├── match_id (FK)
├── rater_profile_id
├── rated_profile_id
├── score (1-5)
├── created_at
└── UNIQUE(match_id, rater_profile_id, rated_profile_id)

player_ratings
├── profile_id (FK)
├── game_id (FK)
├── average_rating (float)
├── total_matches (int)
├── total_ratings_received (int)
├── created_at
└── updated_at

mm_bans
├── profile_id (FK)
├── reason
├── banned_by (profile_id)
├── expires_at (nullable — null = permanent)
├── created_at
└── revoked_at (nullable)
```

## Алгоритм матчинга

```
Redis Queue per (game_id, mode, region):
  sorted by created_at (FIFO)

Matcher Worker (горизонтально масштабируемый):
1. Poll очередь
2. Для каждой заявки:
   a. Найти совместимые заявки (exact match по criteria)
   b. Если набралось slots — создать Match
   c. Создать voice room матч-отряда (Voice Service)
   d. Создать temp chat (Chat Service)
   e. Уведомить участников (NATS → Notification)
3. Проверить таймауты (15/30 мин)
```

## Публикуемые события (→ NATS)

| Событие               | Данные                                     |
|-----------------------|--------------------------------------------|
| `mm.search_started`   | session_id, profile_id, game, mode, region |
| `mm.search_cancelled` | session_id, profile_id                     |
| `mm.search_timeout`   | session_id, profile_id                     |
| `mm.match_found`      | match_id, participants, game, room_id      |
| `mm.match_completed`  | match_id, duration                         |
| `mm.rating_submitted` | match_id, rater_id, rated_id, score        |
| `mm.player_banned`    | profile_id, reason                         |

## Зависимости

- **Redis** — очереди поиска, active session lock
- **Voice Service** — создание voice room для матч-отряда при матче
- **Chat Service** — создание временного чата при матче
- **Notification Service** — (через NATS) уведомление о найденном матче
- **Story Service** — (через NATS) "ищу пати" → автоматическая заявка
- **Moderation Service** — проверка ММ-банов


