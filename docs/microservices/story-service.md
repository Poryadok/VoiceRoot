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

## API (gRPC)

```protobuf
service StoryService {
  // CRUD
  rpc CreateStory(CreateStoryRequest) returns (Story);
  rpc DeleteStory(DeleteStoryRequest) returns (Empty);
  rpc GetStory(GetStoryRequest) returns (Story);

  // Лента
  rpc GetStoryFeed(GetStoryFeedRequest) returns (StoryFeedResponse);
  rpc GetUserStories(GetUserStoriesRequest) returns (StoryList);

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
- **LFP matcher**: при создании "ищу пати" → отправить event в Matchmaking Service

## Публикуемые события (→ NATS)

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
- **Matchmaking Service** — (через NATS) автоматическая заявка "ищу пати"
- **Notification Service** — (через NATS) уведомления об упоминаниях
- **Subscription Service** — проверка Premium (анонимный просмотр)


