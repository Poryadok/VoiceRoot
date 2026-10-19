# Story Service

Go service for Story creation, feeds, profile stories, views, reactions, private
replies, archive, Highlights and looking-for-party (LFP) stories. PostgreSQL
`story_db` owns Story data; File Service owns media storage. Gateway exposes
`/api/v1/stories/**` through gRPC `StoryService`.

Product behavior and the full route/rollout contract live in
[Stories](../../../docs/features/stories.md) and
[Story Service](../../../docs/microservices/story-service.md).
[PLAN](../../../docs/PLAN.md) tracks product readiness; the API inventory here
does not imply complete Stories acceptance.

## Startup and configuration

The service uses Go 1.26. Run from this module with `go run .` after applying
the [Story migrations](../migrations/story_db) through the repository migration
workflow. Startup connects to PostgreSQL but does not apply migrations.
The [Dockerfile](Dockerfile) requires `src/backend` as its build context.

| Variable | Current behavior |
| --- | --- |
| `DATABASE_URL` | Enables the PostgreSQL-backed gRPC API and background workers. A configured database connection failure stops startup. If absent, only HTTP is started. |
| `LISTEN_ADDR` | HTTP address; default `:8080`. |
| `STORY_GRPC_LISTEN` | gRPC address; default `:9090`. |
| `NATS_URL` | Enables JetStream publishing; startup validates the centrally provisioned `story_events` stream. Absent configuration selects a no-op publisher. |
| `USER_GRPC_ADDR` | User privacy settings; creation and viewer-aware operations depend on this client. |
| `SOCIAL_GRPC_ADDR` | Friendship checks and feed-author lookup; without a client the author lister returns only the viewer. |
| `SPACE_GRPC_ADDR` | Space co-membership checks for privacy audiences. |
| `CHAT_GRPC_ADDR`, `MESSAGING_GRPC_ADDR` | DM creation and message sending for private replies. |
| `SUBSCRIPTION_GRPC_ADDR` | Active personal Premium check for anonymous views. |
| `FILE_GRPC_ADDR` | Ordinary File deletion client used by archive-media cleanup; separate from protected media admission below. |
| `STORY_TTL_DEV` | Positive Go duration overriding the default 24-hour Story TTL for disposable development checks; leave unset for the product lifetime. |

`GET /health` returns service/status JSON. It does not query the database or
upstreams and remains available without the gRPC API; HTTP success alone is
not proof of Story readiness or media/event delivery.

## Media and lifecycle

`CreateStory` accepts `photo`, `video` and `text`. Photo/video require File's
protected `ValidateStoryMedia` attestation; text forbids media. Media-less LFP
is supported, while LFP media is rejected until its policy is defined.

Protected media admission requires the complete group
`STORY_PRINCIPAL_SIGNING_KEYS_DIR`, `STORY_PRINCIPAL_ACTIVE_KID`,
`FILE_PRINCIPAL_GRPC_ADDR`, `FILE_PRINCIPAL_TLS_CA_FILE` and
`FILE_PRINCIPAL_TLS_SERVER_NAME`. The key directory contains exactly two
distinct PKCS#8 RSA keys. Partial configuration stops startup; leaving all five
unset leaves media admission unavailable. When configured, HTTP exposes public
keys at `GET /.well-known/jwks.json`. Follow the service contract's protected
rollout, trust, rotation and negative-check instructions before enabling media.

Stories normally leave the feed after 24 hours and retain an author archive
for 30 further days. The expiry worker runs every minute. Archive cleanup
preserves highlighted Stories and stages logical deletion plus durable media
outbox work in one database transaction before calling File. The worker runs
at startup and retries on a minute interval; absent File wiring leaves pending
media work for later delivery. This mechanism does not establish exact
Story/File reference ownership or prove the complete cleanup chain.

Domain events use protobuf `StoryStreamEvent` on `story.*` subjects in the
logical `story.events` domain (physical JetStream stream `story_events`).
The service validates existing stream configuration instead of provisioning
it. Publishing `story.lfp_created` does not establish Matchmaking automatic
application: that consumer remains separate work.

## Verification

From this module, `go test -short ./...` runs the short suite; `go test ./...`
also includes PostgreSQL/Testcontainers checks and needs a usable Docker runner.
Use the supported Linux runner on Windows, following
[TESTING](../../../docs/TESTING.md). Gateway/Flutter live Story smoke and
the expiry/archive/File cleanup chain are separate integration evidence;
developer tests do not replace deployment or acceptance checks.
