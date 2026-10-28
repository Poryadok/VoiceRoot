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
rechecks expiry after live dependencies. Protected ActionServer and the new
ActorBound RegistryProducer preserve exact original replay, verify full
qualified ACK and require durable local capture before public success. They do
not install a listener or actual authority. Additive Gateway consent routes use
a dedicated mTLS client and real JWKS/blacklist/strict-floor human delegation;
legacy profile-only discovery never supplies qualified evidence. Actual
protected root/controller/DB-session/peer and retention composition remain
implementation prerequisites. Codec/Store inputs are
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
- **Media deletion dispatcher**: at startup and at a short interval claims ready/expired outbox leases with a token, calls protected `ReleaseStoryMedia` after the Story commit, and completes only its own lease. The immutable operation ID is the Story ID; release names `(media_file_id, STORY, story_id, NULL, NULL)`. Failures and ambiguous File outcomes back off and retry. File owns zero-reference detection and physical GC; this worker never calls broad `DeleteFile`. Release remains pending until File has a verified global backfill/activation authority, which is not installed in production yet.
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
- **Space Service** — viewer-scoped Space/member pages for feed author candidates
- **Matchmaking Service** — (через NATS, **deferred**) автоматическая заявка "ищу пати" из `story.lfp_created`
- **Notification Service** — (через NATS) уведомления об упоминаниях
- **Subscription Service** — проверка Premium (анонимный просмотр)

### Feed author dependency failure

Production feed requests resolve self, friends and co-members of the viewer's
current live Spaces before querying Story rows. Space candidates come from all
viewer-scoped `ListMySpaces` and current-member `ListMembers` pages; no public
Space catalog or global author scan is used. Co-membership only adds candidates:
selected-Space audiences and other privacy rules still apply to each Story.
A failed Social or Space lookup/page returns a non-sensitive `Unavailable`
without a global scan or partial successful feed, including when a dependency
returns partial results alongside its error. Recovery resumes author-scoped cursor
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
`FailedPrecondition` before File, persistence or an event. Read-only validation
does not acquire a reference; the exact photo/video lifecycle is defined below.

### Exact media reference lifecycle

Photo/video publication additionally calls protected `ClaimStoryMedia`, with an
immutable Story ID, File ID, author and expected media type. File repeats the
admission predicate while holding its blob and File locks and durably acquires
`(file_id, STORY, story_id, NULL, NULL)` before publication. Claim operation IDs
are UUIDv5 of `voice.story.media.acquire.v1:<story UUID>` under the URL namespace;
release operation IDs equal the Story UUID, so acquire and release never share
an idempotency key. Each transport attempt gets a fresh signed credential and
request ID; inbound Gateway/client authority is discarded.

Migration `000007` adds a Story-owned `story_media_reference_intents` table.
Creation saves the exact Story/File pair before File, then locks that intent
through acquire and Story insertion. The Story and removal of the intent commit
together. Failed or ambiguous creation leaves the intent for bounded recovery
after one minute. Recovery locks the intent and releases its exact key; File
durably retires even a not-yet-acquired key, so a delayed acquire cannot revive
it after verified activation. Recovery retries failed/ambiguous releases without
dropping intent ownership. Without File's global-ledger authority it remains
durably pending; it cannot substitute an empty ledger or a local boolean.

Author `DeleteStory` atomically queues the same release and hides the Story.
The same author can replay success; other authors and unknown IDs receive
`NotFound`. A minimal Story-owned deletion receipt preserves replay for 30 days
after successful deletion even if archive cleanup physically removes the Story;
the worker removes expired receipts in bounded batches. Text creates no media
intent or release. Highlights retain the original Story reference through
expiry/archive; only explicit author deletion or eligible archive cleanup
releases it.

The production dispatcher uses the configured signed File client and remains
pending when that protected dependency is absent. Historical unclaimed media
is never converted into broad File deletion: releasing an absent exact reference
retires that key but does not initiate blob GC. Deterministic owner backfills and
the global File-reference activation gates remain prerequisites for a deployment
cutover. No production `StoryReferenceReleaseAuthority` provider exists, so exact
release/physical-GC activation is an explicit unmet BE-096 criterion. The global
File-reference cutover owner must supply and verify that authority before this
consumer can complete; this code change is not full lifecycle completion,
rollout or acceptance evidence.

### Protected media rollout

Story loads exactly two distinct PKCS#8 RSA keys from
`STORY_PRINCIPAL_SIGNING_KEYS_DIR`; `STORY_PRINCIPAL_ACTIVE_KID` selects the
signer. Its HTTP server exposes sorted public keys at `GET /.well-known/jwks.json`.
`FILE_PRINCIPAL_GRPC_ADDR`, `FILE_PRINCIPAL_TLS_CA_FILE`,
`FILE_PRINCIPAL_TLS_SERVER_NAME`, `FILE_PRINCIPAL_CLIENT_CERT_FILE` and
`FILE_PRINCIPAL_CLIENT_KEY_FILE` are all required when this path is configured.
The Story client presents a certificate with client-auth usage signed by the
existing CA in File's `FILE_PRINCIPAL_CLIENT_CA_FILE`; an absent, unreadable or
mismatched certificate/key fails startup. TLS trust and the signed request-bound
`service:story` credential are both required; neither replaces the other.
Partial configuration is a startup error; no configuration keeps media admission
unavailable. Text and media-less LFP remain usable.

For direct service-to-service HTTPS JWKS, set all three of
`STORY_PRINCIPAL_JWKS_HTTPS_LISTEN`, `STORY_PRINCIPAL_JWKS_TLS_CERT_FILE` and
`STORY_PRINCIPAL_JWKS_TLS_KEY_FILE`. The dedicated TLS listener serves the same
public key set at `/.well-known/jwks.json`; partial settings, an invalid or
currently unusable server certificate, or a failed bind stop startup. Without
these settings, the existing HTTP endpoint can be served through an independently
configured HTTPS ingress. The File consumer always verifies the TLS chain and
server name; no plaintext fallback is introduced.

Deployment owners provision Secret `voice-story-principal-keys` with exactly two
`<kid>.pem` keys; `voice-file-principal-grpc-tls` with `tls.crt`/`tls.key` and SAN
`voice-file-principal.<namespace>.svc`; `voice-file-principal-client-ca` with
`ca.crt` for client certificates accepted by the File principal listener; and
`voice-principal-trust` with `ca.crt`
trusting the File server and both Story and Messaging HTTPS JWKS certificates;
`voice-file-jwks-client-tls` with the client certificate and key accepted by the
configured JWKS endpoints; `voice-story-file-client-tls`
with `tls.crt`/`tls.key` issued by that existing trust authority for Story's
client-auth connection to File, issued by a CA in `voice-file-principal-client-ca`;
`voice-story-jwks-tls` for
the HTTPS ingress; and `voice-file-replay` with Redis `password` (empty only for
an intentionally unauthenticated isolated Redis). The File principal gRPC TLS
Secret is separate from any existing File JWKS TLS Secret; preserve existing
trust and provision the listener identity without rotating unrelated consumers.
Never put private keys into ConfigMaps or Git.

Substitute namespace, ingress class, Story JWKS host, exact trusted Messaging JWKS
URL, replay Redis address and active
key ID in the three [deployment templates](../../deploy/templates/story-media-principal-network.yaml).
For the ingress route, apply the network resources, then apply
[File patch](../../deploy/templates/story-media-file-patch.yaml) and
[Story patch](../../deploy/templates/story-media-story-patch.yaml) with
`kubectl patch deployment voice-file --type strategic --patch-file <file>` and
the analogous `voice-story` command in the selected namespace. Deploy File
first; enable Story signing and its HTTPS JWKS route before accepting media.
No plaintext fallback exists. Service metrics expose gRPC deny codes without
credentials or media/profile identifiers.

File's required Messaging issuer must already publish its actual current/next
keys through the protected Messaging JWKS listener. The optional
[Messaging patch](../../deploy/templates/story-media-messaging-patch.yaml)
uses regular-file Secret `subPath` mounts because the Messaging signer rejects
symlinks. Set its signing Secret name to an immutable, versioned reviewed key
set. These mounts do not receive Secret updates in a running pod: rotation
requires a controlled pod rollout and a verified overlap of published keys.
The patch enables the signer and JWKS only, not the File/Search purge owner
connections. Issue the JWKS server and client certificates under their reviewed
trust domains before applying it; the separate
[Messaging JWKS Service](../../deploy/templates/story-media-messaging-service.yaml)
provides port 8443. Template wiring alone is not a TLS proof.

Alternatively to the ingress, the [direct Story HTTPS patch](../../deploy/templates/story-media-story-jwks-patch.yaml)
and [Service](../../deploy/templates/story-media-story-jwks-service.yaml) expose
the dedicated listener inside the namespace. For this direct route, select only
the File Service and NetworkPolicy from `story-media-principal-network.yaml`;
do not apply its Ingress. Both Story and Messaging JWKS URLs
must match the DNS SANs of their respective server certificates; an endpoint is
usable only after its TLS chain and, for Messaging, File's client certificate
are verified. Existing NetworkPolicies may allow broader in-namespace traffic;
the Services themselves are not an access restriction.

Before enabling production traffic, verify TLS trust, the two public JWKS keys,
successful clean image admission, forbidden foreign/chat-scoped media, ordinary
listener denial, and fail-closed Redis/JWKS outage. The namespace policy permits
9091 only from Story; audit other additive NetworkPolicies for broader grants.
Rotation publishes the next public key before selecting it and keeps the old
key for at least 30 seconds after its last signature. Roll pods when changing
mounted signing keys: the signer loads them at startup. Rollback may disable
media admission by removing the complete Story signing/client configuration;
never restore raw metadata validation.

### Private LFP account-erasure qualification

Accepted `StoryLfpErasure-v1` (producer c40, artifact SHA256
`9061a5d658ae93fabd16e20c14e462a746dbd889a0dfaad2359cf408366ab8c0`)
requires a distinct authenticated terminal account-lifecycle decision after the
30-day recovery window. `user.account_deleted`, inactivity, epoch floors and
archive/media purge do not authorize journal erasure. Every consent, actor,
offer snapshot, operation and ACK/hash is personal data, with no exemption.

`internal/lfperasure` supplies closed typed boundaries only. Actual owner,
historical scope, complete current snapshot/seal and approved finite negative
memory/storage custody are absent. Restored qualified routes/outgoing retries
require reconciliation and current negative subject/object/operation/cohort
admission. Terminal-erased must differ from ordinary MM NotFound; absent history
cannot authorize fresh Register. Local SQL scrub, physical copies and installed
admission need separate proofs. See the [producer ExecPlan](../testing/be011-lfp-consent-producer-exec-plan.md).

### Private reference inventory export

`StoryReferenceInventoryService` is a separate File-only mTLS and signed-principal
listener. Begin materializes immutable, deterministically hashed pages of at most
1000 entries; Page and Changes bind the complete installation/restore/migration,
File/owner incarnation, component, writer version and snapshot identity. It never
uses viewer listings or forwarded Gateway identity. Seal remains fail-closed.

Migration `000008_reference_inventory` installs a transactional owner gate and
counter. Ownership mutations always lock that gate before Story rows and advance
the counter. Before the first committed snapshot no identifier journal/cache is
collected. Begin derives all retained, creation-pending, release-pending and
unresolved entries from authoritative rows at cut B, initializes tuple state and
enables capture atomically. Every later committed revision has a complete ordered
absolute-state journal assignment, including empty revisions. Rollback consumes no
revision; retries replay the original snapshot. Capture persists independently of
exporter configuration, and later snapshots do not erase earlier consumer ranges.
Historical invalid bindings and independent covers without a retained Story pin
are counted unresolved. No historical export invokes `ClaimStoryMedia`.

The optional `STORY_REFERENCE_INVENTORY_` configuration is complete-or-absent:
`GRPC_LISTEN`, `TLS_CERT_FILE`, `TLS_KEY_FILE`, `CLIENT_CA_FILE`,
`REPLAY_REDIS_ADDR`, `FILE_JWKS_URL`, `JWKS_CA_FILE`; optional
`REPLAY_REDIS_PASSWORD` and paired `JWKS_CLIENT_CERT_FILE`/`JWKS_CLIENT_KEY_FILE`.
Missing trust/replay or an ordinary listener cannot serve inventory. No public
admin endpoint or environment value creates a migration authority.

Incomplete/unacknowledged snapshots and journal evidence have no 30-day expiry;
P30 starts only on completion/ack and dependency-safe compaction is separate work.
This protective export is not a global writer seal or release activation. See the
[execution plan](../testing/be096-reference-inventory-exec-plan.md).
