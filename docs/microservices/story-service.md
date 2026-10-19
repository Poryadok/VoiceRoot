# Story Service

## Private qualified LFP consent (accepted implementation contract)

The qualified pair JOIN/INVITE source is defined by BE-011 c23
`BE011-LFP-CONSENT-SOURCE-v1`. The private wire is
[`lfp_consent.proto`](../../protos/voice/story/v1/lfp_consent.proto), with a strict
canonical codec in `src/backend/pkg/lfpconsent`. Protocol1 UUIDs are nonzero
16-byte values, digests exactly32 bytes, and all material fields required.
Consent SHA256 uses `voice.story.lfp-consent-receipt.v1` plus one NUL byte and
the exact deterministic `LfpConsentMaterial` bytes. Unknown, duplicate,
reordered, overlong, truncated or semantically invalid material denies.

Story must capture account/profile/positive epoch from the exact verified
delegated Gateway responder action, persist immutable offer incarnation,
revision, criteria/offer digest and exact operation consent before delivery.
No raw profile header or public event creates this authority. The private
`LfpConsentRegistryService` is implemented by Matchmaking and accepts only
protected exact `service:story` requests. Register and readback bind the same
registration operation, consent and hash; the original MM registration ID,
generation and captured-search-intent hash must never change on retry.
`LfpConsentRegistration.registration_sha256` hashes deterministic fields1–7,
omitting field8, under `voice.mm.lfp-consent-registration.v1` plus one NUL.
Explicit NONE is part of MM's exact captured witness, never an absent digest.

The private `LfpConsentOfferService` is implemented by Story and accepts only
protected exact `service:matchmaking`. It validates the exact saved receipt and
current Story offer, including visibility, expiry/retraction and unchanged
incarnation/revision/digests. Success echoes the complete exact binding and
expiry; it is a bounded fresh observation, not a permanent permit. Outage is
unavailable, never allow. MM still owns current Auth-floor/all-account checks,
atomic acceptance and all member/session replacement.

Public `story.lfp_response` remains profile-only discovery. Historical requests
cannot be promoted by a current owner lookup or guessed epoch. Pair INVITE
preserves responder leadership, pair JOIN author leadership; unsupported group
contexts fail explicitly without dropping peers. Natural Story expiry after
consumption does not invalidate MM's already accepted immutable source.
Wire/codec presence alone does not install this protected runtime or journal;
the [producer execution plan](../testing/be011-lfp-consent-producer-exec-plan.md)
tracks their implementation and verification. Existing activation gates remain.

The component journal in `internal/lfpconsentstore` uses Story migration000005.
Each explicit authenticated author publication creates an immutable revision;
observing an existing Story never adopts it automatically. Response-operation
replay returns the original consent and epoch, and registration readback retains
MM's original captured intent. Canonical bytes must match every relational
selector and immutable offer before any readback can qualify.

Accepted restore-cohort contract (producer c7) requires immutable environment,
installation, external incarnation and admitted Story database binding alongside
the unchanged v1 material. `pkg/storylfpadmission.Barrier` is the shared local
ABI: the bound journal obtains admission before SQL and releases after commit or
rollback. Its local shared admission-head lock supplements that external permit.
Neither database marker nor constructing an Origin establishes admission.
Only confirmed transaction completion releases its permit. An ambiguous BEGIN,
COMMIT or ROLLBACK retains restrictive admission: pgx's closed transaction or
asynchronous connection close is not database-session quiescence. No local timeout
or reopen releases this uncertainty; the actual controller must retire the old
runtime and its database sessions before admitting replacement execution.
Producer c14 additionally requires immutable explicit HUMAN guest/regular actor
provenance, captured by the protected Gateway verifier and persisted alongside
consent/offer/cohort. The additive `lfp_actor.proto` ActorBound wrappers and
`pkg/lfpconsent/ACTOR_PROTOCOL.md` pin the sidecar/envelope/ACK domains while
retaining original v1 bytes. Bare v1 registration/validation endpoints or old ACKs
cannot qualify this path; no fallback or current-caller enrichment is allowed.
Fresh service validation must load the saved original viewer type and pass it
explicitly to current privacy rules. It cannot obtain regular by defaulting a
service context, ID or epoch floor. Guest and regular LFP paths are supported;
existing guest DM/Chat/Space restrictions remain separate. Additive migration
000006 stores immutable author/responder sidecars and qualified ACK atomically
with their original records. Legacy rows cannot be enriched. Its immutable
publication snapshot retains exact PostgreSQL criteria text and audience under
the existing offer state hash; fresh privacy must evaluate that snapshot rather
than an unrelated mutable row during A->B->A changes. The ActorBound Validate
component checks the exact original actor/current offer, fresh privacy through
an explicit saved viewer and final SQL state. Its required response field6
canonical_criteria_json contains exact immutable PostgreSQL criteria text whose
SHA256 is already committed by the v1 receipt. MM must check bytes/hash/shape;
neither public listings nor JSON map re-encoding can substitute for it.
HumanVerifier captures exact delegated Gateway actor/type/operation/body and
rechecks expiry after live dependencies. Actual protected action/registry root
composition remains an implementation prerequisite. Codec/Store inputs are
data, not proof that a protected Gateway verification occurred.
`BE011-STORY-LFP-RESTORE-CONTROLLER` owns the still-absent deployment controller,
full replica/transaction drain, external state and credential/route custody;
consumers are Story291 and MM145. Unknown authority leaves qualification closed.
The journal is a component, with no production authority or protected transport
installed. Personal journal data remains subject to account erasure/retention;
that composition and runtime activation remain open.

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

### Feed author dependency failure

Production feed requests resolve the viewer's author set through Social before
querying Story rows. A failed author lookup returns a non-sensitive `Unavailable`
without a global scan or a partial successful feed, including when Social returns
partial author IDs alongside its error. Recovery resumes author-scoped cursor
pagination and the existing per-Story privacy checks. An error querying the
resolved author set is returned without retrying against all active Stories.

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
