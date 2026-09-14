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
| `GET` | `/api/v1/stories/{id}/reactions` | `GetStoryReactions` | только автор |
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
  rpc GetStoryReactions(GetStoryReactionsRequest) returns (GetStoryReactionsResponse); // только автор

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

story_media_deletion_outbox
├── operation_id (= story_id; immutable idempotency key)
├── story_id (no FK: story is logically deleted before dispatch)
├── media_file_id
├── available_at, attempt_count, last_error_at, last_error
├── lease_token, lease_until
├── created_at, updated_at
└── ready/lease claim index
```

## Background Jobs

- **Expiry worker**: каждую минуту — пометить expired stories (TTL 24h)
- **Archive cleanup**: bounded DB-first transaction locks expired unhighlighted Stories with `FOR UPDATE SKIP LOCKED`, inserts one media-deletion outbox record per media Story, then logically deletes the Stories. Text-only Stories do not create an outbox row. Highlight membership mutations lock the same Story rows, so a committed Highlight protects the Story/media, while a purge that commits first makes Add return `NotFound`.
- **Media deletion dispatcher**: at startup and at a short interval claims ready/expired outbox leases with a token, calls File Service after the Story commit, and completes only its own lease. Failures and ambiguous File outcomes back off and retry; File deletion is idempotent by immutable file ID.
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

- **File Service** — хранение медиа сторис и protected `ValidateStoryMedia`
- **User Service** — настройки приватности (кто видит сторис)
- **Social Service** — список друзей для фильтрации видимости
- **Matchmaking Service** — (через NATS, **deferred**) автоматическая заявка "ищу пати" из `story.lfp_created`
- **Notification Service** — (через NATS) уведомления об упоминаниях
- **Subscription Service** — проверка Premium (анонимный просмотр)

## Story media admission

`CreateStory` has a closed type set: `photo`, `video`, `text`. Photo/video
require one valid media UUID and must obtain File's protected empty-success
attestation; text forbids media and does not call File. Story must create a new
outgoing Phase-0 context with only its service credential and request ID, and
must not forward Gateway/client identity metadata or use `GetFileMetadata`.
Downstream credential, transport, deadline and dependency failures map to a
non-sensitive `Unavailable`; File's `InvalidArgument`, `NotFound`,
`PermissionDenied` and `FailedPrecondition` remain public operation codes.

Until the product canon selects an LFP media kind and video policy, media-less
LFP remains supported and any non-empty LFP `media_file_id` fails closed with
`FailedPrecondition` before File, persistence or an event. Exact Story-ID
claim/reference binding remains separate File-reference work.

### Protected media rollout

Story loads exactly two distinct PKCS#8 RSA keys from
`STORY_PRINCIPAL_SIGNING_KEYS_DIR`; `STORY_PRINCIPAL_ACTIVE_KID` selects the
signer. Its HTTP server exposes sorted public keys at `GET /.well-known/jwks.json`.
`FILE_PRINCIPAL_GRPC_ADDR`, `FILE_PRINCIPAL_TLS_CA_FILE` and
`FILE_PRINCIPAL_TLS_SERVER_NAME` are all required when this path is configured.
Partial configuration is a startup error; no configuration keeps media admission
unavailable. Text and media-less LFP remain usable.

Deployment owners provision Secret `voice-story-principal-keys` with exactly two
`<kid>.pem` keys; `voice-file-principal-tls` with `tls.crt`/`tls.key` and SAN
`voice-file-principal.<namespace>.svc`; `voice-principal-trust` with `ca.crt`
trusting both File and Story HTTPS JWKS certificates; `voice-story-jwks-tls` for
the HTTPS ingress; and `voice-file-replay` with Redis `password` (empty only for
an intentionally unauthenticated isolated Redis). Never put private keys into
ConfigMaps or Git.

Substitute namespace, ingress class, JWKS host, replay Redis address and active
key ID in the three [deployment templates](../../deploy/templates/story-media-principal-network.yaml).
Apply the network resources, then apply
[File patch](../../deploy/templates/story-media-file-patch.yaml) and
[Story patch](../../deploy/templates/story-media-story-patch.yaml) with
`kubectl patch deployment voice-file --type strategic --patch-file <file>` and
the analogous `voice-story` command in the selected namespace. Deploy File
first; enable Story signing and its HTTPS JWKS route before accepting media.
No plaintext fallback exists. Service metrics expose gRPC deny codes without
credentials or media/profile identifiers.

Before enabling production traffic, verify TLS trust, the two public JWKS keys,
successful clean image admission, forbidden foreign/chat-scoped media, ordinary
listener denial, and fail-closed Redis/JWKS outage. The namespace policy permits
9091 only from Story; audit other additive NetworkPolicies for broader grants.
Rotation publishes the next public key before selecting it and keeps the old
key for at least 30 seconds after its last signature. Roll pods when changing
mounted signing keys: the signer loads them at startup. Rollback may disable
media admission by removing the complete Story signing/client configuration;
never restore raw metadata validation.
