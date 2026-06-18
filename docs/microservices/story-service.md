# Story Service

## Обзор

Сторис: 24-часовой контент, архив, хайлайты, интеграция с матчмейкингом ("ищу пати").

**Язык**: Go
**БД**: PostgreSQL `story_db`
**Хранилище**: Cloudflare R2 (через File Service)

## Ответственность

- Создание сторис (фото, видео, текст)
- TTL 24 часа (автоудаление)
- Архив (30 дней)
- Хайлайты (постоянные коллекции из архива)
- Game tag на сторис
- "Ищу пати" фича → авто-генерация заявки в матчмейкинг
- Emoji-реакции на сторис
- Приватные ответы на сторис (→ DM)
- Упоминания в сторис (@username)
- Счётчик просмотров
- Анонимный просмотр (Premium)
- Видимость по настройкам приватности (everyone / friends / nobody)

## REST (через Gateway)

Публичные маршруты: `/api/v1/stories/**` → Story Service gRPC ([api-gateway.md](api-gateway.md)).

| Метод | Путь | gRPC | Примечание |
|-------|------|------|------------|
| `POST` | `/api/v1/stories` | `CreateStory` | `type`, `text_content`, `media_file_id`, `mention_profile_ids`, `visibility` |
| `GET` | `/api/v1/stories/feed` | `GetStoryFeed` | `cursor`, `limit` |
| `GET` | `/api/v1/stories/archive` | `GetArchive` | только свой архив; `profile_id` опционален |
| `GET` | `/api/v1/stories/profiles/{profile_id}` | `GetProfileStories` | активные стори профиля |
| `POST` | `/api/v1/stories/looking-for-party` | `CreateLookingForParty` | `criteria_json`, опц. `media_file_id` |
| `GET` | `/api/v1/stories/{id}` | `GetStory` | — |
| `DELETE` | `/api/v1/stories/{id}` | `DeleteStory` | `204` |
| `POST` | `/api/v1/stories/{id}/views` | `MarkViewed` | `anonymous` (Premium) |
| `GET` | `/api/v1/stories/{id}/viewers` | `GetViewers` | только автор |
| `POST` | `/api/v1/stories/{id}/reactions` | `ReactToStory` | `emoji` |
| `POST` | `/api/v1/stories/{id}/reply` | `ReplyToStory` | приватный ответ → DM (`chat_id`, `message_id`) |
| `GET` | `/api/v1/stories/highlights` | `GetHighlights` | `profile_id`; фильтр по `visibility` хайлайта |
| `POST` | `/api/v1/stories/highlights` | `CreateHighlight` | `name`, `visibility` |
| `PATCH` | `/api/v1/stories/highlights/{id}` | `UpdateHighlight` | `name`, `visibility` |
| `DELETE` | `/api/v1/stories/highlights/{id}` | `DeleteHighlight` | `204` |
| `POST` | `/api/v1/stories/highlights/{id}/stories` | `AddToHighlight` | `story_id` |
| `DELETE` | `/api/v1/stories/highlights/{id}/stories/{story_id}` | `RemoveFromHighlight` | `204` |

## API (gRPC)

```protobuf
service StoryService {
  // CRUD
  rpc CreateStory(CreateStoryRequest) returns (Story);
  rpc DeleteStory(DeleteStoryRequest) returns (Empty);
  rpc GetStory(GetStoryRequest) returns (Story);

  // Лента
  rpc GetStoryFeed(GetStoryFeedRequest) returns (GetStoryFeedResponse); // stories + next_cursor в одном сообщении
  rpc GetProfileStories(GetProfileStoriesRequest) returns (StoryList);

  // Просмотры
  rpc MarkViewed(MarkViewedRequest) returns (Empty);
  rpc GetViewers(GetViewersRequest) returns (ViewerList);

  // Реакции
  rpc ReactToStory(ReactToStoryRequest) returns (Empty);

  // Архив
  rpc GetArchive(GetArchiveRequest) returns (StoryList);

  // Хайлайты
  rpc CreateHighlight(CreateHighlightRequest) returns (Highlight);
  rpc UpdateHighlight(UpdateHighlightRequest) returns (Highlight);
  rpc DeleteHighlight(DeleteHighlightRequest) returns (Empty);
  rpc AddToHighlight(AddToHighlightRequest) returns (Empty);
  rpc RemoveFromHighlight(RemoveFromHighlightRequest) returns (Empty);
  rpc GetHighlights(GetHighlightsRequest) returns (HighlightList);

  // "Ищу пати"
  rpc CreateLookingForParty(CreateLFPRequest) returns (Story);
}
```

## Модель данных

```
stories
├── id (UUID)
├── author_profile_id
├── type (photo | video | text)
├── media_file_id (FK → file_db, nullable)
├── text_content (nullable)
├── text_style (jsonb — font, color, background)
├── game_tag (string, nullable)
├── is_looking_for_party (bool)
├── lfp_criteria (jsonb — game, mode, role, region; nullable)
├── mention_profile_ids (jsonb)
├── view_count (int, denormalized)
├── visibility (everyone | friends | custom)
├── expires_at (created_at + 24h)
├── archived_until (expires_at + 30d)
├── created_at
└── deleted_at (nullable)

story_views
├── story_id (FK)
├── viewer_profile_id
├── is_anonymous (bool — Premium)
├── viewed_at
└── UNIQUE(story_id, viewer_profile_id)

story_reactions
├── story_id (FK)
├── reactor_profile_id
├── emoji (string)
├── created_at
└── UNIQUE(story_id, reactor_profile_id)

highlights
├── id (UUID)
├── profile_id (FK)
├── name
├── cover_file_id (FK, nullable)
├── sort_order (int)
├── created_at
└── updated_at

highlight_stories
├── highlight_id (FK)
├── story_id (FK)
├── sort_order (int)
├── added_at
└── UNIQUE(highlight_id, story_id)
```

## Background Jobs

- **Expiry worker**: каждую минуту — пометить expired stories (TTL 24h)
- **Archive cleanup**: ежедневно — удалить stories старше 30 дней из архива
- **LFP matcher**: при создании "ищу пати" Story Service публикует `story.lfp_created` в JetStream; **потребитель Matchmaking Service (авто-заявка из LFP-стори) отложен** — Matchmaking пока не подписан на этот subject

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`story.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие                   | Данные                              |
|---------------------------|-------------------------------------|
| `story.created`           | story_id, author_id, type, game_tag |
| `story.viewed`            | story_id, viewer_id                 |
| `story.reacted`           | story_id, reactor_id, emoji         |
| `story.expired`           | story_id                            |
| `story.highlight_created` | highlight_id, profile_id            |
| `story.lfp_created`       | story_id, author_id, criteria       |

## Зависимости

- **File Service** — хранение медиа сторис
- **User Service** — настройки приватности (кто видит сторис)
- **Social Service** — список друзей для фильтрации видимости
- **Matchmaking Service** — (через NATS, **deferred**) автоматическая заявка "ищу пати" из `story.lfp_created`
- **Notification Service** — (через NATS) уведомления об упоминаниях
- **Subscription Service** — проверка Premium (анонимный просмотр)


