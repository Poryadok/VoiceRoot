# TODO — Backend

- [ ] **[Social Phase-0] Migrate remaining User profile/account lookups** —
  `GetProfile` and `ListProfileIDsForAccount` still require ordinary User 9090 access.
  The privacy-principal cutover protects only `GetPrivacySettings` and Space
  `AreCoMembers`; do not apply a blanket Social→User 9090 deny until these
  separate calls have an explicit signed-principal contract and regression
  coverage for friends, contacts and account-level blocks. Source:
  [social-service.md](../microservices/social-service.md#privacy-service-principals).

[← Индекс](../TODO.md)

Микросервисы, Gateway (backend), protos, NATS, compose live verification.

Аудит микросервисов (Go/Java), Gateway backend, protos/pkg, NATS. Источник: product audit 2026-07-14 + сверка фич 2026-08-17. Дубликаты roadmap — только ссылка. Снятые как сделанные (JoinSpace, GetMessage, shadow_ban insert, SearchGlobal ACL, pending_accept sweeper, social.events, file retention/SHA, DeleteAccount, MuteChat/ArchiveChat, notification persist, EnsurePrimaryProfile, DeleteProfile REST, Cancel/Resume local, contacts gRPC) — `tmp/feature-audit/synthesis.md`.

## Critical

### Subscription


- [ ] **[Subscription] Checkout is a stub — no Paddle Billing API; returns `checkout.paddle.test` URLs; no real purchase path** — `src/backend/subscription/internal/grpcsvc/subscription.go` (`CreateCheckoutSession`, `CreateSpaceCheckoutSession`)
- [ ] **[Subscription] CloudPayments not implemented — СНГ provider entirely missing** — `src/backend/subscription/internal/grpcsvc/subscription.go` (`HandleCloudPaymentsWebhook` → `Unimplemented`); no `internal/billing/cloudpayments.go`
- [ ] **[Subscription] JWT `subscription_tier` stuck at `free` without NATS — `AuthBeans` биндит `NatsSubscriptionTierStore` только если `auth.nats.url` / `AUTH_NATS_URL` задан, иначе `InMemorySubscriptionTierStore`. User читает JWT, не Subscription S2S; GIF/banner/лимиты профилей остаются `free`. File работает из-за Gateway override** — `AuthBeans.java`; `InMemorySubscriptionTierStore.java`; `src/backend/gateway/subscription_tier.go`; `src/backend/user/internal/grpcsvc/user.go`, `user_avatar.go`. **ops:** `AUTH_NATS_URL` на staging/prod — [ci.md](ci.md) § Critical.
- [x] **[Subscription] Space Pro purchase does not affect Space/Voice — webhook writes `subscription_db.space_subscriptions`; Space reads `space_db.space_subscriptions`; no sync/NATS consumer in prod (tests use `SeedSpaceProActive`)** — **done:** Subscription S2S `SyncSpaceProSubscription` + Space NATS `subscriptionconsume` upsert `space_db.space_subscriptions` (`subscription/internal/grpcsvc/subscription.go`, `space/internal/subscriptionconsume/`, compose `SPACE_GRPC_ADDR`). Compose live member-cap: `TestComposeSpaceProMemberCap_live` (#14).
- [x] **[Subscription] Voice Space Pro cap never applied in prod — `SpacePro` lookup not wired in `main.go`; room cap stays 32** — **done:** `voice/main.go` wires `SpacePro` via `SUBSCRIPTION_GRPC_ADDR`; `voice_room.go` raises cap to 128 when `HasSpacePro`. Compose sets `SUBSCRIPTION_GRPC_ADDR` on voice.

### Protos/Pkg


- [ ] **[Protos/Pkg] Split NATS wire format vs `jetstream_events.proto`** — `protos/voice/events/v1/jetstream_events.proto` defines protobuf envelopes, but publishers diverge:
### Space


- [ ] **[Space / BE-245] Owner lifecycle/product surfaces partial** — BE-116 now blocks the obsolete hard `DeleteSpace` at the database boundary so it cannot cascade away Space audit/outbox evidence. BE-245 must replace it with the password/2FA-confirmed 7-day hidden/frozen schedule, owner-only `RestoreSpace`, terminal same-transaction P3 aggregate purge and idempotent cross-service purge/attachment GC; the disabled legacy RPC's public gRPC status mapping remains undefined and must be frozen with that vertical. Emit scheduled/restored/deleted-at-purge events only from the documented lifecycle. Compensated `TransferOwnership` (T-011) and gRPC `GetAuditLog` shipped; Gateway/Flutter leave-as-owner/delete/restore/audit flows remain open. — `protos/voice/space/v1/space.proto`; `src/backend/space/internal/grpcsvc/`; `src/backend/gateway/`; `src/frontend/`; [spaces.md](../features/spaces.md).
- [ ] **[R23 Chat/Proto] Bind terminal purge prerequisite receipts before destructive Chat cleanup** — current `voice.chat.v1.PurgeSpaceRequest` carries only generic purge authority and cannot prove Messaging participant-3 completion or File acceptance of the exact `CHAT` producer release. Decide the deterministic/request-bound transport, Messaging/File issuance authority, caller and retry owner for the Chat-owned File release, exact cross-receipt binding, and restart/response-loss resume acceptance. Until that decision lands, Chat purge remains fail-closed and cannot delete rows or issue completion. — [chat-service.md](../microservices/chat-service.md#p3-terminal-purge-prerequisite-proof-seam-decision-required); `protos/voice/chat/v1/chat.proto`; `protos/voice/messaging/v1/messaging.proto`; `protos/voice/file/v1/file.proto`.
- [ ] **[A2 Gateway/Space] Implement frozen public lifecycle, invite, tree and audit contract** — [API Gateway A2 target](../microservices/api-gateway.md#a2-space-rest-contract-target) now defines routes, JSON, ACL/disclosure, replay and paging. Remaining: lifecycle Space proto/service operation fields and durable journal/outcomes, wiring the shipped Auth proof consume/receipt path into Space transfer, separate deletion-proof lifecycle, signed caller cutover, accepted audit writers and protected producer outboxes, gRPC filter mapping and complete filter/viewer/key-bound cursor envelope, Gateway transcoders and Flutter vertical with negative transport tests. Existing selected routes remain characterization only; expose target behavior only after the complete corresponding vertical is verified.
- [x] **[Space] Space Pro cache never synced — `space_db.space_subscriptions` comment says “synced from Subscription”; only test seed `UpsertSpaceSubscription` writes; Subscription writes `subscription_db` only** — **done:** NATS consumer `space/internal/subscriptionconsume` + S2S `SyncSpaceProSubscription` write entitlement cache; `SeedSpaceProActive` remains test helper only.
- [ ] **[Space] Replace single `entry_requirement` with versioned composable AND `entry_policy`** — migrate legacy value, implement phone → captcha → questions → manual approval pipeline, pending join requests, fail-closed verifier errors, and ensure invite is consumed only after all requirements/approval succeed. Guests fail policies requiring phone. — [spaces.md](../features/spaces.md), `src/backend/space/internal/grpcsvc/join.go`, `invites.go`.
- [x] **[Space] Social block на join fail-open — `ensureJoinNotBlocked` no-op если `Blocks`/`ProfileAccounts` nil** — **done:** fail-closed `FailedPrecondition` when Social/User S2S unwired; IT `join_block_degradation_test.go`.
- [x] **[Space] Tree pin — migration `is_pinned`/`pin_order` on `space_tree_nodes`, `PinTreeNode`/`UnpinTreeNode` RPC handlers, `ReorderSpaceTree` pin group, `space.tree_node_upserted` payload** — **done:** `000007_tree_pin` migration; store `PinTreeNode`/`UnpinTreeNode`; grpc handlers; `ReorderSpaceTree` pin-group validation; JetStream `SpaceTreeChanged` includes `is_pinned`/`pin_order`.

### Moderation


- [x] **[Moderation] Shadow-ban forward bypass** — **done (PR #132):** `ForwardMessage` / `insertForwardCommentary` apply `IsShadowBanned` + `ghost_only` / suppress `message.sent` like `SendMessage` (`messaging_grpc.go`).
- [x] **[Moderation] Sanction notifications: consumer stub** — **done (T-013):** `routeModerationNotification` resolves `target_account_id` → profile ids via User `ListProfileIDsForAccount` (`notification/internal/s2s/account_profiles.go`); `HandleSanctionApplied` routes `system` **push** with presence skipped (no Realtime system in-app yet); shadow_ban stays silent (`reports.md`).
- [x] **[Moderation] Appeals not exposed to users** — Gateway `POST /api/v1/moderation/appeals` (201 Created → `SubmitAppeal`); Flutter `VoiceModerationClient.submitAppeal` + settings appeal sheet (`docs/features/reports.md` § Апелляция). **Batch 27a**.

### Social


- [x] **[Social] REST contacts/favorites отсутствуют** — Gateway `GET/POST /api/v1/friends/contacts`, `GET/POST /api/v1/friends/favorites` → gRPC `ListContacts`/`AddContact`/`SetFavorite`/`ListFavorites` (`transcode_friends.go`, `transcode_friends_contacts_test.go`); **Batch 23a**.
- [x] **[Social] Friend invite block fail-open — `ensureFriendInvitationNotBlocked` no-op if `ProfileAccounts` nil (`USER_GRPC_ADDR` unset) or caller `x-voice-user-id` missing** — **done:** fail-closed `FailedPrecondition` when Blocks/ProfileAccounts nil; `Unauthenticated` when account metadata missing (`social_friends.go`); IT `friend_invite_block_degradation_test.go`. Compose already sets `USER_GRPC_ADDR` on social.

### User


- [x] **[User] OAuth verification goes through User Service** — Auth uses source-scoped
  `ApplyVerificationSourceState` with durable revisions/retry; direct `user_db` writes were removed.

### Matchmaking


- [ ] **[Matchmaking] Party snapshot из voice roster отсутствует** — `PartyStore` stub; `StartSearch` валидирует `partySize=1`. Нет сброса очереди при leave/join войса (`docs/features/matchmaking.md`). V1 валидирует обязательную self-reported роль по каталогу и допускает повторы; будущая balanced matchmaking функция должна отдельно определить квоты, уникальность и распределение ролей.

#### A3 MM ↔ Voice party and match-squad lifecycle contract (accepted target)

This is the frozen implementation contract for the A3 party/reset and ephemeral
match-squad gaps. It follows the Phase-0 protected-principal rules in
`ARCHITECTURE_REQUIREMENTS.md`; current `x-voice-internal-caller` and forwarded
profile metadata are migration inputs only and confer no authority.

**Authoritative party snapshot.** Add protected
`VoiceService.GetMatchmakingPartySnapshot`, allowed only to
`service:matchmaking`. The request contains `search_operation_id`,
`initiator_account_id`, `initiator_profile_id` and positive
`initiator_session_epoch`; these are subject data copied by Matchmaking only from
the verified delegated-user principal on `StartSearch`. It contains no room ID,
party ID or member list. The service credential is exact-RPC/request-hash bound;
raw identity metadata, duplicate credentials and a caller other than Matchmaking
are rejected before the handler. Party identity is profile-scoped: the initiating
device need not own the media connection, but the authenticated profile must be
the unique active Voice membership used for the lookup.

The response has `protocol_version=1`. Voice returns one of these exact results:

- `SOLO`: no active Voice membership, no `room_id`, `roster_version=0`, and the
  sole member is `initiator_profile_id`;
- `VOICE_ROSTER`: an active ordinary `call`, `group_voice` or `voice_room`, with
  server-owned `room_id`, positive monotonic `roster_version`, and unique profile
  IDs sorted by UUID bytes. The initiator occurs exactly once. Members are durable
  memberships in `JOINED` or the still-valid at-most-30-second `RECONNECTING`
  state. `JOINING`, `LEAVING`, `LEFT`, `EJECTED`, revoked/expired epoch and a
  non-`ACTIVE` room are excluded. A stale locator that still points at an
  inactive/revoked membership is `FAILED_PRECONDITION`, not `SOLO`.
  Mute/deafen/speak state does not change party
  eligibility. A room whose immutable purpose is `MATCH_SQUAD` is not a pre-match
  party and returns `FAILED_PRECONDITION`; multiple active memberships for the
  initiator are an invariant violation and fail closed.

The snapshot read and `roster_version` are taken under the same Voice transaction
boundary as membership mutations. For a concurrent join/leave, the caller sees
either version `N` before the mutation and later receives event `N+1`, or the
post-mutation members at `N+1`; a mixed roster/version is forbidden. Voice
unavailability makes `StartSearch` fail without a party, sessions or queue rows.
The public `StartSearch.party_id` is never roster authority and must be rejected
when supplied by an external user; LFP uses its existing protected MM-owned party
path. Matchmaking validates catalog party bounds, then atomically stores the
initiator, source kind, source room/version and SHA-256 of the deterministic
sorted member manifest before enqueueing one party. Every member is covered by
the one-active-search invariant, not only the initiator.

**Roster-change delivery and ordering.** Add
`VoiceRosterMembershipChanged` to `VoiceStreamEvent` on subject
`voice.roster_membership_changed`. The payload is exactly `protocol_version=1`,
`room_id`, positive `roster_version`, `change=JOINED|LEFT` and `profile_id`; the envelope supplies
UUID `event_id` and `occurred_at`. Voice increments the room version and writes
the outbox event in the same transaction that commits the membership mutation,
then publishes deterministic bytes with `Nats-Msg-Id=event_id`. A denied/no-op or
an exact replay emits no new version/event.

Matchmaking uses a durable pull consumer and a PostgreSQL inbox. It stores
`event_id`, payload hash and `(room_id,roster_version)` before explicit ACK.
Exact duplicate delivery is a no-op. Reuse of an event ID with different bytes,
or of a room/version with different membership-change bytes, is
`CONTRACT_MISMATCH`: cancel affected searching/pending parties fail closed and
page immediately; never guess an ordering or discard silently. Delivery may be
duplicated or out of order, so MM does not depend on arrival order: any valid
JOINED/LEFT with `event.roster_version > party.source_roster_version` cancels the
whole voice party exactly once, removes all its sessions from queues and emits
one cancellation per session with reason `VOICE_ROSTER_CHANGED`. In
`pending_accept`, that party is cancelled and unrelated parties return to
searching, matching the existing own-party decline rule. An event at or below the
captured version has no effect. A JOINED event also cancels an active `SOLO`
search owned by that profile. A join by a profile absent from the captured
manifest still cancels by source room ID.

Events are the prompt reset path, not the sole correctness fence. Immediately
before proposal reservation and again before squad provisioning after all accepts,
MM calls the same protected snapshot RPC for every candidate party, including
each `SOLO` initiator, and
requires the exact source room, version and member-manifest hash. Mismatch applies
the same cancellation; `UNAVAILABLE` leaves candidates unreserved/retryable and
never creates or activates a match. This closes consumer lag and
snapshot-to-inbox races.

**Temporary resource ownership and teardown.** Normal `CreateChat`/`StartCall`
resources are never inferred to be match resources from their name, participants
or a forwarded header. Provisioning must use protected, Matchmaking-only
`ChatService.CreateMatchSquadChat` and `VoiceService.CreateMatchSquadRoom`. Each
request carries one durable UUID `operation_id`, `match_id`, the sorted participant
manifest plus its hash; Voice additionally carries the Chat resource and Chat
creation receipt binding. Chat/Voice atomically persist immutable
`owner_kind=MATCH_SQUAD`, `owner_id=match_id`, creation operation, manifest hash
and resource ID, and return an immutable creation receipt. Same-operation exact
replay returns byte-identical stored receipt; changed bytes conflict. MM persists
the request bytes before the first call. `UNAVAILABLE`, `DEADLINE_EXCEEDED` and an
ambiguous lost response retry the same operation; if the second resource has a
terminal binding/validation rejection or the match is abandoned before activation,
MM compensates by tearing down every resource whose creation receipt it obtained.

The authenticated `CompleteMatch` leave is idempotent by
`(actor_profile_id,operation_id)` and may mark only that match participant left.
Non-final leaves create no teardown. The transaction that records the final
participant leave sets `matches.status=completed` and `completed_at` for rating
and history, creates exactly one durable `match_squad_teardowns` aggregate and two
participant rows (`CHAT`, `VOICE`) in `NOT_STARTED`; concurrent final leaves cannot
create another operation. The teardown state is independent of match history and
is `NOT_STARTED|IN_FLIGHT|COMPLETE|RETRYABLE_FAILURE|CONTRACT_MISMATCH`.
Matchmaking is the sole teardown
coordinator, while Chat and Voice remain the sole authorities for their local
resources.

MM calls protected `ChatService.TeardownMatchSquadChat` and
`VoiceService.TeardownMatchSquadRoom`, each allowed only to
`service:matchmaking`. The exact request is `protocol_version=1`, the stored
`teardown_operation_id`, `match_id`, exact resource ID, creation `receipt_id` and
participant-manifest hash. A receiver must match all fields to its immutable
creation binding before changing state. Therefore even a valid Matchmaking
principal cannot delete an ordinary group/chat/room or another match's resource.
Unknown resource/receipt is `NOT_FOUND`; a binding or same-operation body mismatch
is `FAILED_PRECONDITION` and is never converted to success.

Chat atomically terminally fences the chat, removes user membership/navigation
visibility and commits its durable `chat.deleted` outbox before returning a
receipt. Thereafter Chat list/get/member mutations and Messaging paths guarded by
Chat membership deny access; the receipt proves logical access teardown, not
physical erasure of Messaging-owned retained bytes. Voice first atomically closes
admission and grant reissue, then ejects participants, removes the Redis
projection and confirms LiveKit room deletion or authoritative absence; it issues
no completion receipt until these effects are confirmed. A timeout never
manufactures a receipt. Each receipt binds stable `receipt_id`, participant kind,
protocol/operation/match/resource/creation receipt/manifest/request hashes,
`status=COMPLETED` and database `completed_at`; exact replay returns its stored
bytes.

MM validates and stores each receipt before marking that participant `COMPLETE`.
Calls may run in parallel; only both exact receipts move the teardown aggregate to
`COMPLETE` and release the single durable `mm.match_completed` outbox event. One
success plus one outage remains pending and retries only incomplete work. Each
attempt uses the exact stored request bytes, a fresh 10-second deadline and
unbounded exponential retry from one second to a five-minute cap with bounded
jitter. Fifteen minutes without progress alerts; contract mismatch pages
immediately and is never auto-skipped. Full request/receipt bytes remain 30 days
after aggregate completion; compact terminal `(match_id,resource_id,receipt_id,
completed_at)` fences remain permanent. The match/history row is retained and no
longer exposes usable Chat/Voice IDs after aggregate completion.

**Required acceptance evidence.** Deterministic service/integration tests must
prove: protected-principal negative cases and no side effects; SOLO and each
eligible/ineligible roster state; atomic snapshot-versus-concurrent join/leave;
client member/room/party spoof rejection; whole-party active and pending-accept
reset; solo-on-join reset; duplicate/out-of-order/conflicting event handling;
synchronous revalidation under delayed/missing event delivery; one teardown under
concurrent final leaves; ordinary/cross-match resource delete denial; partial
provision compensation; Chat access deny and Voice grant/media deny after teardown;
dropped-response exact receipt recovery; one participant outage with restart-safe
retry; and exactly one `mm.match_completed` only after both validated receipts.
The A3 multi-client Compose proof is solo + voice party → roster mutation reset →
new search/match/accept → temporary message + test-media → all leave → old Chat
and Voice IDs denied while history remains.
- [x] **[Matchmaking] Platform MM ban fail-closed + S2S** — `StartSearch` / matcher fail-closed when `BanStore` nil (`platform_ban_degradation_test.go`, `worker_ban_degradation_test.go`, **#73**); Moderation `mm_ban` → `ApplyPlatformMMBan` / revoke (`sanctions.go`).
### Role


- [x] **[Role] Voice Service never wires Role Service — `Roles` is nil in prod; `VOICE_JOIN`, `VOICE_SPEAK`, `VOICE_MUTE_OTHERS`, etc. are not enforced on join/speak/mute. Only `EnsureScreenShare` exists and is unused without a Role client.** — **partial:** `voice/main.go` wires `Roles` via `ROLE_GRPC_ADDR`; `EnsureVoiceJoin` + `EnsureScreenShare` enforce join/share (`role_guard.go`, `voice_room.go`). T-062 closes the remaining state/commander guard: `VOICE_SPEAK` gates explicit unmute, enabled broadcast, and Space LiveKit token `canPublish`; `VOICE_MUTE_OTHERS` gates enabling commander, enabling broadcast, and grant/revoke floor. A denied speaker receives `canPublish=false`; missing or unavailable Role dependency fails closed for protected Space actions. Self-mute and non-Space calls remain available, and Owner bypass stays in Role Service. gRPC regressions cover deny, unavailable, allow, room override context, and decoded token grants. Compose/Flutter `VOICE_JOIN` deny live shipped (`TestComposeVoiceJoinDeny_live`, #14). Already-issued LiveKit JWTs remain effective until their configured TTL because the canon has no refresh/revocation contract; a target-participant mute RPC remains a separate gap.
- [x] **[Role] Chat send overrides are API-only — `TEXT_CHAT_SEND_MESSAGES` deny via `chat_overrides` is computed in Role Service but Messaging `SendMessage` never calls `CheckPermission` / `HasChatPermission` for send; E2E only probes `/api/v1/roles/check`.** — **done (send path):** `SendMessage` / `ForwardMessage` call `checkSpaceSendPermission` → `HasChatPermission(..., TEXT_CHAT_SEND_MESSAGES)` (`messaging_grpc.go`); Messaging IT `messaging_send_permission_integration_test.go`; compose/Flutter deny live `TestComposeRolesSendDeny_live` (#14). Other TEXT_CHAT_* bits still partial (see High Role bullets).

### Cross-cutting


- [ ] **[Cross-cutting] JWT `subscription_tier` never syncs from billing unless `AUTH_NATS_URL` — duplicate of Critical Subscription; User/Chat trust JWT. Staging/prod must bind NATS store + не оставлять InMemory.** — `AuthBeans.java`; [ci.md](ci.md)
- [x] **[Cross-cutting] Space Pro entitlement duplicated, not synced — webhook writes `subscription_db.space_subscriptions` (`subscription/internal/grpcsvc/subscription.go`); Space enforces caps from `space_db.space_subscriptions` (`space/internal/store/entitlement.go`). No S2S/event sync on `subscription.activated` / `space_pro`. Live Space Pro billing does not raise member cap.** — **done (sync):** webhook → S2S `SyncSpaceProSubscription` and/or NATS `subscription.space_pro_*` → Space entitlement cache. Compose live member-cap: `TestComposeSpaceProMemberCap_live` (#14).
- [ ] **[Cross-cutting] `subscription.events` downstream consumers incomplete — Subscription publishes the domain stream, including personal `plan_expired`, `downgrade`, and D1/D3/D7 `grace_reminder`; User/File do not consume the required personal downgrade/limit effects, Analytics subscribes but does not map expiry/downgrade, and Notification only recognizes valid grace reminders, logs consumption, and ACKs without push/email dispatch.** — `src/backend/subscription/internal/subscriptionevents/jetstream.go`; `src/backend/subscription/internal/sweeper/sweeper.go`; `src/backend/analytics/internal/adapters/domain.go`; `src/backend/notification/subscription_events_consumer.go`; `docs/CONTRACT_MATRIX.md`
- [x] **[Cross-cutting] Web JWT in WS query string** — web uses `POST /api/v1/realtime/ws-ticket` + `/ws?ticket=`; legacy `access_token` query retained for compat. — `docs/ARCHITECTURE_REQUIREMENTS.md`, Gateway, Flutter `RealtimeHub`
- [ ] **[Cross-cutting] Flutter shell parity (audit R2-A03–A05, H14)** — folders + Quick Access + archive RPCs/UI shipped (batches 15–21). **Remaining:** mobile drawer IA (stub), stacked chrome polish, R2-A04 defer — [client.md](client.md).

### Messaging


- [x] **[Messaging] Staging/prod `MODERATION_GRPC_ADDR` wiring** — **done:** compose uses `moderation:9090`; staging/prod configmaps use `voice-moderation:9090`, so Messaging wires `PlatformMod` outside local compose too (`docker-compose.yml`, `deploy/staging/configmap-app.yaml`, `deploy/prod/configmap-app.yaml`).
- [x] **[Messaging] `ForwardMessage` bypasses channel/thread send policy** — **done:** `ForwardMessage` calls `checkSpaceSendPermission`, `threadPolicyDeps().validateSend`, and sets `posted_as_chat` when channel forbids main-feed (`messaging_grpc.go`; `messaging_forward_integration_test.go`). `GetMessage` **есть** (`messaging_grpc.go` ~792).
- [x] **[Messaging] E2E forward policy gap** — **done:** `validateE2ESend` on forward target; E2E→plain `FailedPrecondition`, E2E→E2E preserves `is_e2e` (Messaging ITs).

### Search


- [x] **[Search] `SearchGlobal` `matched_chats` ACL** — `intersectAccessibleChats` + `AccessibleChatIDs`; profile hits — `AccountPairBlocked` в `filterProfileHits`. Remaining: User `SearchProfiles` path (`/api/v1/users/search`) still separate.

### Voice


- [x] **[Voice] Space voice rooms: fail-open если `SPACE_GRPC_ADDR` unset** — **done:** `ensureSpaceMember` fail-closed `FailedPrecondition` when `SpaceMembers` nil; `space_member_degradation_test.go`.
- [x] **[Voice] `LeaveCall` group** — aliases больше не убивают сессию: group → `RemoveParticipant`, end if empty. 1:1 Leave всё ещё `EndCall`.

### Auth


- [ ] **[A4 Auth/User/Chat/File/Search] Complete account erasure lifecycle** — password+2FA confirmation, 30-day restore, then idempotent PII/credential/profile-media erasure or pseudonymization; retain messages with non-public author tombstone and isolate minimal legal/anti-abuse records by production retention policy. Existing `DeleteAccount`/`RestoreAccount` and ListChats deleted-peer filter are partial. — [auth-and-contacts.md](../features/auth-and-contacts.md), [client.md](client.md).
- [x] **[Auth] Email signup and convert-guest verification gate** — pending identity preserves session/history and guest-level restrictions; successful email verification durably queues conversion recovery, which retries User `MarkAccountRegular`, Auth-local promotion to `regular`, and `user.guest_converted` publication. Negative User-failure coverage is in the Auth contract tests. — [auth-and-contacts.md](../features/auth-and-contacts.md), `src/backend/auth/`.


## High

### Subscription


- [ ] **[Subscription] Cancel / Resume только в `subscription_db` — не зовут Paddle/CloudPayments API; после реального checkout отмена не остановит биллинг у провайдера. Gateway REST cancel/resume **есть**** — `subscription.go` (`CancelSubscriptionByID`); `transcode_subscription.go`
- [x] **[Subscription] `subscription.events` producer — domain JetStream publisher is wired alongside analytics telemetry; personal lifecycle emits `plan_expired`, `downgrade`, and `grace_reminder` in addition to activation/payment events.** — `src/backend/subscription/internal/subscriptionevents/jetstream.go`; `src/backend/subscription/main.go`; `src/backend/subscription/internal/sweeper/sweeper.go`. **Remaining consumers:** Cross-cutting item below.
- [x] **[Subscription] Personal grace expiry — minute sweeper transitions due `grace_period` rows to `cancelled`; repeated runs do not reselect the transitioned row.** — `src/backend/subscription/main.go` (`runGraceSweeper`); `src/backend/subscription/internal/sweeper/sweeper.go`; `src/backend/subscription/internal/grpcsvc/lifecycle_integration_test.go`
- [ ] **[Subscription] Paddle webhook lifecycle incomplete — only `subscription.activated` + `subscription.payment_failed`; no renew, cancel, pause, or period-end handling** — `src/backend/subscription/internal/grpcsvc/subscription.go` (`HandlePaddleWebhook`)
- [ ] **[Subscription] Downgrade event has no downstream freeze/picker enforcement — personal grace/period expiry emits `subscription.downgrade`, but User/Flutter do not consume it to select two profiles, freeze excess profiles, or unfreeze on renewal.** — `src/backend/subscription/internal/sweeper/sweeper.go`; `src/backend/subscription/internal/grpcsvc/user_profile_downgrade.go`; `src/frontend/lib/ui/profile/profile_downgrade_picker_screen.dart`
- [ ] **[Subscription] Space Pro payment-failure grace lifecycle remains separate/open — personal sweeper D1/D3/D7 + grace expiry does not implement equivalent failed-payment grace/reminders/expiry for `space_subscriptions`.** — `src/backend/subscription/internal/sweeper/sweeper.go`; `src/backend/subscription/internal/store/sweeper.go`
- [ ] **[Subscription] Gateway: no CloudPayments webhook route** — `src/backend/gateway/transcode_subscription.go` (only `webhooks/paddle`)

### File


- [ ] **[File] SHA-256 deduplication missing** — no hash lookup, no reuse of existing R2 key; `file_references` table absent (spec model in `d:\Git\Voice\docs\microservices\file-service.md`; only `files` in `d:\Git\Voice\src\backend\migrations\file_db\000001_init.up.sql`). Acknowledged in `d:\Git\Voice\src\backend\file\README.md`.
- [x] **[File] NATS `file.downloaded` отсутствует** — File публикует best-effort `file.downloaded` после успешного `GetFileURL` presign; Messaging preview-refresh после conversion остаётся отдельной задачей.
- [ ] **[File] No async worker / `processing` status** — conversion runs inline in `ConfirmUpload`; `processing` never set (`d:\Git\Voice\src\backend\file\internal\grpcsvc\file_grpc.go`; `d:\Git\Voice\docs\microservices\file-service.md` pipeline).
- [x] **[File] Originals kept after image processing** — processed keys written, source `r2_key` not removed (`d:\Git\Voice\src\backend\file\internal\imgproc\webp.go`; contradicts `d:\Git\Voice\docs\features\file-storage.md`).
- [ ] **[File] `CheckQuota` ignores premium** — always returns `r2file.MaxFreeFileBytes` as limit (`d:\Git\Voice\src\backend\file\internal\grpcsvc\file_grpc.go` L449–454); README says subscription quotas beyond free tier are out of scope.
- [ ] **[File] ffmpeg GIF→MP4 / video 720p / PDF first-page thumb отсутствуют** — image WebP inline в `ConfirmUpload`; video as-is. README: dedup out of scope. `ListFiles` REST **есть**; cursor/`filter_chat` — см. Common File.
- [ ] **[File] Infected-file Notification fan-out contract is undefined** — Gateway now maps `scan_result=infected` to `412 file_infected` and scanner failure to `412 file_scan_failed`; Flutter discards the blocked attachment, prevents `SendMessage` and offers to pick another file. `FileScanResult` has no documented recipient/attachment authority for Notification, and `files.chat_id` is only upload context, so no consumer or Realtime fan-out is implemented until that contract is specified.

### Protos/Pkg


- [ ] **[Protos/Pkg] Analytics subscription mapping remains partial** — runner already subscribes to `social.events`, `role.events`, `file.events`, `subscription.events`, and `moderation.events`; only deferred `federation.events` is absent. The Subscription mapper handles plan-started/payment events, not expiry/downgrade. — `src/backend/analytics/internal/consumer/runner.go`; `src/backend/analytics/internal/adapters/domain.go`; `docs/MICROSERVICES.md`
- [ ] **[Protos/Pkg] Space event catalog residual drift** — `ChatStreamEvent` now has payloads for invite, member joined/left, updated and deleted events. Remaining gaps vs [space-service.md](../microservices/space-service.md): no `space.member_banned` payload; no voice-room created/deleted payloads; `SpaceUpdated` carries only `space_id`, not `changed_fields`.
- [ ] **[Protos/Pkg] Go `pb/` codegen sync asymmetry** — `scripts/dev/sync-pb-from-gen.sh` syncs 7 trees (`analytics`, `chat`, `file`, `messaging`, `role`, `user`, `voice`); 10+ packages hub under `src/backend/voice/pb/voice/` and `src/backend/user/pb/voice/`. No CI drift check (unlike `make buf-dart-check` for `src/frontend/lib/gen/`). Stale committed stubs possible after proto edits.
- [ ] **[Protos/Pkg] `pkg/` resilience gap** — `docs/MICROSERVICES.md` requires circuit breaker on all gRPC calls; `src/backend/pkg/grpcclient/` only provides `dial.go` (`DialTarget`) and `wait.go`. No breaker/retry/mTLS helpers.
- [ ] **[Protos/Pkg] `pkg/` auth metadata fragmentation** — Gateway contract in `src/backend/gateway/transcode_grpc.go` (`x-voice-user-id`, `x-voice-profile-id`, …), but 12+ per-service `internal/authctx/` copies (`src/backend/*/internal/authctx/`). Only partial shared helpers: `src/backend/pkg/guestguard/`, `src/backend/pkg/correlation/`, `src/backend/pkg/jwt/` (edge validation, not inbound gRPC claim parsing).

### Space


- [x] **[Space] Backend gRPC `GetAuditLog` baseline** — newest-first, space-scoped keyset paging (default 50/max 100), full entry mapping, `SPACE_VIEW_AUDIT_LOG` fail-closed permission check; additive actor/action/time proto fields and a signed unfiltered snapshot cursor are shipped. **Residual open:** gRPC filter execution and the complete filter/viewer/key-bound cursor envelope, Gateway REST, Flutter UI and missing audit writers — see items below. Gateway moderation `ExportAuditLog` is a separate surface.
- [x] **[Space] `UpdateSpace` writes visibility / entry_requirement / questions / mm_config** — `space.go`; Role `SPACE_MANAGE_SETTINGS` (`requireSpacePermission`). Tree CRUD — `requireSpaceTreeManage`, не owner-only.
- [x] **[Space] T-009 stale implementation claim: `mm_config` / `entry_questions` already load and round-trip** through `SpaceRow`, `spaceRowToProto`, `UpdateSpace`, `GetSpace`, `ListMySpaces` and `UpdateSpaceMmConfig`; regression coverage locks this in. **Residual open:** execution of questions/captcha/manual entry requirements remains the Critical item above.
- [ ] **[Space] Tree node Pro limit (500) not implemented — hardcoded `MaxTreeNodes = 50`, no entitlement check (unlike member cap)** — `src/backend/space/internal/store/tree.go`
- [ ] **[Space] Catalog indexing fragile — Search hydrator calls `GetSpace` (member-only); `SearchPublicSpaces` Unimplemented; ranking verified-first нет; нет `space.updated` re-index** — `src/backend/search/internal/deps/deps.go`, `chat_space_indexer.go`
- [ ] **[Space] NATS events incomplete and publish-failure contract undefined** — join/leave/kick publish `space.member_joined` / `space.member_left`; update/delete publish `space.updated` / `space.deleted`. Remaining: `space.member_banned` is absent; voice-room created/deleted publisher methods are no-ops; Search does not reindex `space.updated`. On a publication error, current mutations have inconsistent observability: invite/member paths persist the mutation and call no-op `logInviteEventFailure`, while tree and space paths persist it and log `nats_publish`. The canonical Space docs define the event/audit surface but do not specify atomicity with persistence, retry/outbox, durable delivery, recovery, audience, or ordering. **Unblock:** define that delivery/failure contract before adding a characterization that would freeze today's best-effort behavior. Sources: [space-service.md](../microservices/space-service.md), [spaces.md](../features/spaces.md), `src/backend/space/internal/grpcsvc/{invites,join,members,tree,space}.go`, `src/backend/space/internal/spaceevents/`.
- [ ] **[Space] Member timeout not enforced downstream — `IsProfileTimedOut` exists, unused outside Space** — `src/backend/space/internal/store/moderation.go`

### Moderation


- [x] **[Moderation] `mm_ban` S2S to Matchmaking** — `ApplySanction` type `mm_ban` → `Matchmaking.ApplyPlatformMMBan`; revoke → `RevokePlatformMMBan` (`sanctions.go`). Peer-scoped `BanFromMM` remains separate API.
- [ ] **[Moderation] Auto-moderation diverges from spec** — `CheckMessage` only detects ≥3 links; no repeated-message detection; no 1h timed mute (second spam hit → permanent block for that pattern only, no window); no “first 10 messages after mute” pass.
- [ ] **[Moderation] Report threshold audience is static env, not object audience** — `MODERATION_PLATFORM_AUDIENCE_SIZE` (default 1000) drives 1% calc; spec calls for relative threshold vs target’s audience.
- [ ] **[Moderation] Admin audit export пуст только если store пуст** — Gateway `writeModerationAuditExportJSON` мапит `ExportAuditLog` (`transcode_moderation_admin.go`). Не hardcoded `[]`.
- [x] **[Moderation] Sanction notification push delivery** — **done (T-013):** consumer resolves profiles, applies policy with **presence skipped**, sends `system` push copy so online recipients are not silent-dropped (`moderation_events_consumer.go`).
- [ ] **[Moderation/Notification] T-023 — sanction `system` in-app contract and Realtime fan-out** — current moderation consumer resolves `target_account_id` to profiles, intentionally skips presence, and sends push so online users are not silent-dropped. Still undefined/absent: Notification→Realtime transport, system-sanction payload, transport dedupe, final account→profiles semantics, and Flutter `system` presentation. This is a narrow temporary exception, not a presence rule for every system notification — [notification-service.md](../microservices/notification-service.md).

### Social


- [x] **[Social] Contacts/favorites REST** — Gateway `GET/POST /api/v1/friends/contacts`, `GET/POST /api/v1/friends/favorites` shipped (**Batch 23a**). **Remaining:** `SyncPhoneContacts` UX.
- [x] **[Social] Outgoing request status exposed** — `PendingFriendRequest.status` (`pending` | `declined`) in proto + `ListFriendRequests` mapping; Flutter outgoing requests tab shows declined label — **Batch 24b** (`friends.md`).

### User


- [ ] **[User] Premium animated GIF avatar is a dead path** — premium gate in `user_avatar.go` but `image/gif` rejected by `r2avatar/validate.go` (`TestValidateUploadParams_rejectsGifInPhase1`); conflicts with `docs/features/user-profile.md`. `GetSettings`/`UpdateSettings` **есть** (`user_settings.go`). `GetPrivacySettings` ownership check **есть** (non-S2S → `GetOwnedProfile`).
- [ ] **[User] `SetPrimaryProfile` отсутствует** — `is_primary` только bootstrap; phone search всегда primary.
- [x] **[User] NATS contract gaps** — **done:** `user.game_detected` and `user.settings_changed` publish their documented payloads; `user.profile_updated` carries `changed_fields`, `user.verified` carries `verification_type`, and `user.profile_switched` carries `old_profile_id` / `new_profile_id` while retaining legacy `profile_id` compatibility. — `protos/voice/events/v1/jetstream_events.proto`, `src/backend/user/internal/userevents/jetstream.go`, [user-service.md](../microservices/user-service.md).
- [ ] **[User] Durable `last_seen_at` (PostgreSQL)** — spec requires PG persistence for header; code retains interim Redis-only 30-day timestamp — [presence.md](../features/presence.md), [user-service.md](../microservices/user-service.md). `show_last_seen` proto/DDL and viewer-aware interim read filter are shipped.
- [x] **[User] `show_last_seen` privacy enforcement** — **done:** additive `PrivacySettings.show_last_seen`, `privacy_settings` DDL, preset defaults, and fail-closed viewer-aware `GetPresence`/`GetBulkPresence` filtering of interim timestamp. Durable PostgreSQL last-seen and WS fan-out remain separate — [user-service.md](../microservices/user-service.md).
- [ ] **[User] Homoglyph-normalized search not implemented** — anti-spoof on create only (`src/backend/user/internal/store/verification.go`); `SearchProfilesAfter` uses raw `ILIKE` (`src/backend/user/internal/store/profile_search.go`); spec requires normalized lookup (`docs/features/verification.md`).
- [ ] **[User] Premium custom status not gated** — `UpdatePresence` and `UpdateProfile` accept `custom_status` for all tiers (`src/backend/user/internal/grpcsvc/user_presence.go`, `src/backend/user/internal/grpcsvc/user.go`); spec: Premium only.

### Analytics


- [ ] **[Analytics] Health dashboard under-spec vs docs** — event type defaults to `api_request` (PR #125 / master); product still under-delivers latency/error/WS KPIs from `docs/features/analytics.md` / analytics-service.md (`query.go`).
- [ ] **[Analytics] Product dashboard under-spec** — `product` returns `dau` as `uniqExact(user_id_hashed)` over the whole range (default 30d), not daily DAU; missing MAU/WAU/onboarding completion per `docs/features/analytics.md` / `docs/microservices/analytics-service.md` (`d:\Git\Voice\src\backend\analytics\internal\store\query.go`).
- [ ] **[Analytics] Grafana vs REST registration mismatch** — Grafana panel counts `profile_created` (`d:\Git\Voice\deploy\observability\grafana\dashboards\voice-analytics-product.json`); REST `product` dashboard counts `user_registered` (`d:\Git\Voice\src\backend\analytics\internal\store\query.go`).
- [ ] **[Analytics] DoD ingest path untested** — Live tests only check RBAC 200/403 and export HTTP 200; no assertion that `message.sent` → ClickHouse row within 60s (`d:\Git\Voice\src\backend\gateway\compose_analytics_live_test.go`, `d:\Git\Voice\src\backend\gateway\compose_analytics_export_live_test.go` vs `docs/features/analytics.md` DoD §1).
- [ ] **[Analytics] Silent no-op without ClickHouse** — `CLICKHOUSE_DSN` unset → service starts, ingest buffers, nothing persisted (`d:\Git\Voice\src\backend\analytics\main.go`); k8s secret refs are `optional: true` (`d:\Git\Voice\deploy\staging\services.yaml`, `d:\Git\Voice\deploy\prod\services.yaml`).
- [ ] **[Analytics] Weak prod hash-key guard** — Missing `ANALYTICS_ID_HASH_KEY` falls back to dev default (`d:\Git\Voice\src\backend\analytics\main.go`).

### Matchmaking


- [x] **[Matchmaking] Decline semantics vs spec** — `handleMatchDecline` party-aware (declining party cancelled, others continue searching); cross-party IT in `grpcsvc/match_test.go` (PR #4). Compose live: `TestComposeMatchmakingCrossPartyDecline_live` (#14).
- [ ] **[Matchmaking] Match squad not ephemeral** — Squad creates a normal group chat + group voice (`squad/grpc_clients.go`). `CompleteMatch` only updates MM DB (`grpcsvc/rating.go`); no Chat/Voice teardown. Contradicts “auto-delete when all leave” (`docs/features/matchmaking.md`). Implement the protected creation binding, durable two-participant teardown ledger and exact receipts frozen in **A3 MM ↔ Voice party and match-squad lifecycle contract** above; a normal group/call must never become teardown-authorized by inference.
- [ ] **[Matchmaking] `UpdateGame` mutates catalog config for any caller** — Any authenticated user can change `config_json` (`grpcsvc/server.go`, `store/games.go`). Conflicts with user-game immutability (`docs/features/matchmaking.md`) and moderator-only catalog edits (`docs/features/game-catalog.md`).

### Role


- [ ] **[Role] Verification roles not implemented — auto roles on verification status (Steam, rank, etc.) per `docs/features/roles.md` / `docs/microservices/role-service.md`; no code in Role or User/Auth integration.** — `docs/features/roles.md` § «Верификационные роли»; `src/backend/role/` (absent)
- [ ] **[Role] Voice chat organizer role not implemented — no system/custom role, no permission bits, no Voice-side organizer powers (mute, floor, raise-hand).** — `docs/features/roles.md` § «Организатор войс-чата»; `src/backend/role/permissions/permissions.go`
- [ ] **[Role] Override targets not validated S2S — `SetChatOverride` / `SetVoiceRoomOverride` accept arbitrary UUIDs; doc dependency on Space/Chat validation is missing.** — `src/backend/role/internal/grpcsvc/roles.go`, `roles_manage.go`; `docs/microservices/role-service.md` § «Зависимости»
- [ ] **[Role] `MODERATION_MANAGE_REPORTS` unused — bit exists; Moderation service has no `CheckPermission` integration.** — `src/backend/role/permissions/permissions.go`; `src/backend/moderation/`
- [ ] **[Role] `SPACE_MANAGE_MATCHMAKING` unused — no Role checks in Matchmaking service.** — `src/backend/matchmaking/`
- [ ] **[Role] Many text-chat permission bits not enforced downstream — Messaging checks send + mentions/threads/pins; не все attach/embed/react/manage/slow-mode bits.** — `src/backend/messaging/internal/grpcsvc/messaging_grpc.go`, `threads_policy.go`
- [x] **[Role] `SPACE_MANAGE_SETTINGS`** — `UpdateSpace` → `requireSpacePermission(..., SpaceManageSettings)` (`space.go`).
- [ ] **[Role] Admin ≡ Owner on effective mask — `GetEffectiveMask` short-circuits Admin to `AllMask()`; Admin system role mask is also `all`. Doc algorithm step 5 («кроме Owner-specific») has no distinct owner bits, so Admin is functionally Owner for all 42 flags.** — `src/backend/role/internal/store/roles.go` (`GetEffectiveMask`), `permissions/permissions.go` (`SystemRoles`); `docs/microservices/role-service.md` § «Вычисление effective permissions»

### Bot


- [ ] **[Bot] Inbound chat message events → bot webhook/poll not implemented** — `docs/microservices/bot-service.md` describes `NATS: message in whitelisted chat → Bot Service`; code only **publishes** `bot.events` (`internal/botevents/jetstream.go`, wired in `main.go`), no consumer/subscriber anywhere under `src/backend/bot/`.
- [ ] **[Bot] Redis gRPC rate limiter fails open** — on Redis error, requests proceed unlimited (`internal/ratelimit/redis_limiter.go`); staging sets `BOT_REDIS_ADDR` in `deploy/staging/services.yaml`.
- [ ] **[Bot] `GetChatMessagesForBot` → `Unimplemented` если Messaging unset / history path** — privileged `TEXT_CHAT_READ_HISTORY`; без Messaging live — Unimplemented. Portal CSRF/manifest — [admin.md](admin.md).
- [ ] **[Bot] Token / webhook-secret rotation does not invalidate active sessions** — `RegenerateToken` / `RegenerateWebhookSecret` only update DB; no hub deferred-token purge per `docs/features/bots.md` §tokens.

### Cross-cutting


- [ ] **[Cross-cutting] Subscription lifecycle remains incomplete outside shipped personal sweeper core — `HandleCloudPaymentsWebhook` → `Unimplemented`; Cancel/Resume local-only (не провайдер); grace reminder events have no push/email dispatch; downgrade events have no User/Flutter freeze flow; Space Pro failed-payment grace is absent.** — `src/backend/subscription/internal/grpcsvc/subscription.go`; `src/backend/subscription/internal/sweeper/sweeper.go`; `src/backend/notification/subscription_events_consumer.go`
- [ ] **[Cross-cutting] `CheckLimit` unused outside Subscription — no runtime gRPC callers in Chat/Space/User/File for documented caps (profiles, space/chat counts, etc.). Enforcement is ad hoc: File via Gateway live `GetSubscription`, User via JWT tier, Chat has no subscription client.** — `src/backend/subscription/internal/grpcsvc/subscription.go`, `src/backend/gateway/subscription_tier.go`, `src/backend/user/internal/grpcsvc/user.go`
- [ ] **[Cross-cutting] Resilience claims vs code — `MICROSERVICES.md` promises circuit breakers + NATS DLQ; no `gobreaker`/DLQ in `src/backend/`. Tier-0 degradation is partial (Gateway file tier fallback only).** — `docs/MICROSERVICES.md`, `src/backend/` (absence)
- [x] **[Cross-cutting] No E2E for Space Pro billing path — smoke/full cover personal premium + file limits (`compose_billing_live_test.go`, `billing_e2e_live_test.dart`); zero `space_pro` webhook → invite/member-cap tests.** — **done (member-cap live):** `TestComposeSpaceProMemberCap_live` (webhook join + cap, #14). Remaining product: Flutter Space Pro checkout / real Paddle (Critical Subscription; Common Flutter Space Pro).
- [ ] **[Cross-cutting] Tier-2 E2E not PR-gated** — `ws_resume`, `message_delivery`, `in_app_notifications` run on master/full (`e2e-features.yml`); regressions can land before nightly. Policy: [ci.md](ci.md) § High Pipeline.
- [x] **[Cross-cutting] Device `integration_test` driver suite — partial:** host matrix `device_driver_smoke_test.dart` + CI `flutter-device-driver`; Gateway well-known Team ID/SHA/package env-driven; Android emulator driver skips without device. Still open: NT-05, prod App Links/AASA, CallKit/PushKit/LiveKit media. — `src/frontend/integration_test/README.md`, `docs/TESTING.md`
- [x] **[Cross-cutting] Federation: staging/CI vs local compose** — `federation` added to `docker-compose.yml` app profile (health/metrics scaffold); still omitted from `GATEWAY_GRPC_UPSTREAMS_JSON` by design (S2S-only).
- [ ] **[Cross-cutting] Admin vs PLAN — `PLAN.md` lists Admin as “зарезервировано”; `src/admin/` ships moderation queue + product analytics pages with CI job. Cross-cutting staff product surface undocumented in PLAN.** — `docs/PLAN.md`, `src/admin/`

### Messaging


- [x] **[Messaging] `message.forwarded` NATS event** — `PublishMessageForwarded` on forward path (`messaging_grpc.go`; `messageevents/jetstream.go`).
- [ ] **[Gateway/Messaging] Raise `MessagesSend` rate limit to 100 messages / 5 sec** — local app stack hits rate limit after ~5 quick messages; desired product behavior: normal chat should allow up to **100 сообщений / 5 сек** while keeping anti-bot protection. Update docs currently saying `5 сообщений / 5 сек` ([ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md), [text-chat.md](../features/text-chat.md)) together with `defaultRateLimitRules` / `GATEWAY_RATE_LIMIT_RULES_JSON` tests when implementing.
- [x] **[Messaging] `ForwardMessageRequest.commentary` ignored** — **done:** commentary inserts a separate message via `insertForwardCommentary` (`messaging_grpc.go`; Messaging forward ITs).
- [x] **[Messaging] “Copy as new message” / forward without attribution** — **done:** `ForwardMessageRequest.without_attribution` → regular message, no Forwarded-from; skips `allow_forward` deny (FW-03).
- [x] **[Messaging] Forward-author privacy block not enforced** — spec says user can forbid forwarding their messages; Messaging `ForwardMessage` checks User `allow_forward` via S2S (`PermissionDenied`).
- [ ] **[Messaging] Group/channel per-message view counts remain future** — `text-chat.md` requires a deduplicated per-message view counter; this is separate from the shipped per-member `MarkRead`/`GetReadState`/`GetBulkReadState`/`GetChatListMetadata` unread/read metadata contract and is not an A1 gate.
- [ ] **[Messaging] `content_type`: article, location, video_note, music** — **partial (parallel track):** `messages.content_type` column + `SendMessage`/`Message.content_type` proto; location/article send without `file_id`; `video_note`/`music` payload validation still open — [messaging-service.md](../microservices/messaging-service.md) — **P0**
- [ ] **[Messaging / C-002, P-008] schedule lifecycle (`schedule_message`, `send_when_online`)** — **foundation shipped:** additive `delivery_schedule`/scheduled-RPC contracts, `scheduled_messages` migration and `MessageSent.was_scheduled = 9` / optional `scheduled_at = 10`; populated schedule arms fail closed until handler ownership. Open: validation, shared immediate/scheduled idempotency ledger or advisory lock, owner lifecycle handlers, worker/presence, atomic message/outbox dispatch and producer usage. PR #310 owns `send_silent`. Composer and Notification consumption remain separate. — [messaging-service.md](../microservices/messaging-service.md) § Scheduled messages — **P0**
- [x] **[Messaging] `GetChatListMetadata` preview DTO** — **done (Batch 13 + parallel track):** `last_message_content_type` from durable `messages.content_type` with attachment inference fallback; `is_outgoing` + `delivery_state` shipped (Batch 12).
- [x] **[Messaging] Durable `last_message_delivery_state`** — `read_receipts.last_delivered_message_id`, consumer on `message.delivery_ack`, derivation in `GetChatListMetadata` (Batch 12).
- [ ] **[Messaging / C-002, P-008] scheduled RPC handlers** — wire declarations are shipped; implement chat-scoped owner-only pending list and caller-owned update/cancel/send-now transitions with race guards. — [messaging-service.md](../microservices/messaging-service.md) § Scheduled RPC shape — **P0**
- [ ] **[Messaging] File processed → preview refresh consumer** — NATS handler on `file.processed` to update list metadata / invalidate cache — [messaging-service.md](../microservices/messaging-service.md)
- [ ] **[Messaging/Subscription] Premium multi-reaction limit enforcement** — after subscription entitlement doc lands
- [ ] **[Messaging / C-002, P-008] scheduled `message.sent` producer** — event fields are shipped; dispatch must set `was_scheduled` and original timed `scheduled_at` only in the later worker/send-now slice.

### Search


- [x] **[Search] Reverse-direction / bidirectional block on SearchUsers/SearchGlobal** — `filterProfileHits` + `AccountPairBlocked`. Remaining: User `SearchProfiles` (`/api/v1/users/search`) и SQL `BlockedAccountIDs` pre-filter (outgoing) — post-filter закрывает.
- [ ] **[Search] JetStream `DeliverNew` → no historical backfill — consumers only index events after subscription; deploy/reset leaves `search_db` empty for past messages/profiles unless manual per-chat reindex.** — `src/backend/search/internal/indexer/consumer.go`
- [ ] **[Search] Chat/space projection staleness after create — indexer handles only `ChatCreated` / `SpaceCreated`; no handlers for group rename (`UpdateGroupChat`), space update (`UpdateSpace`), visibility change, or `SpaceTreeChanged`.** — `src/backend/search/internal/indexer/chat_space_indexer.go`; upstream: `src/backend/chat/internal/grpcsvc/group.go`, `src/backend/space/internal/grpcsvc/space.go`
- [ ] **[Search] `ReindexChat` not admin-gated — spec (`docs/microservices/search-service.md`) says admin; any authenticated profile with read access can trigger full chat backfill. No Gateway HTTP route.** — `src/backend/search/internal/grpcsvc/search.go` (`ReindexChat`); absent from `src/backend/gateway/transcode_search.go`

### Chat — navigation contracts (audit 2026-08-28) — **P0**

Канон: [navigation.md](../features/navigation.md), [chat-service.md](../microservices/chat-service.md), [GLOSSARY.md](../GLOSSARY.md).

- [x] **[Chat] Proto: folder membership + pin RPCs** — `AddChatToFolder`, `RemoveChatFromFolder`, `ReorderFolderChats`, `PinChatInFolder`, `UnpinChatInFolder`; `ListChatsRequest.folder_id`; `buf generate` (**Batch 19**).
- [x] **[Chat] Migration: `folders`** — `000008_folders.up.sql` per chat-service.md sketch; seed system folders (All/DM/Groups/Channels/Spaces) lazy init on `ListFolders` (**Batch 18**).
- [x] **[Chat] Migration: `folder_chats`** — `000009_folder_chats.up.sql` with `(profile_id, folder_id, chat_id, sort_order, is_pinned, pin_order)` (**Batch 18** DDL; membership/pin handlers **Batch 19**).
- [x] **[Chat] Handlers: folder membership + pin** — store + gRPC add/remove/reorder/pin/unpin; archived reject; system pin overlay vs custom explicit membership (**Batch 19**).
- [x] **[Chat] `ListChats` folder filter** — `folder_chats` join / system `filter_config_json`; sort pinned → sort_order → activity (**Batch 19**).
- [x] **[Chat] Folder CRUD handlers** — `ListFolders`/`CreateFolder` **done (Batch 18)**; `UpdateFolder`/`DeleteFolder` **done (Batch 20)**.
- [x] **[Chat] Proto: Quick Access RPCs** — `ListQuickAccess`, `AddQuickAccess`, `RemoveQuickAccess`, `ReorderQuickAccess` in `chat.proto` (**Batch 17**).
- [x] **[Chat] Migration: `quick_access_chats`** — `000010_quick_access_chats.up.sql` per chat-service.md sketch (**Batch 17**).
- [x] **[Chat] Handlers: Quick Access** — enforce limit 15; `AddQuickAccess` idempotent; integration test reorder (**Batch 17**: `quick_access.go`, store + gRPC tests).
- [x] **[Chat] Archive removes Quick Access** — `ArchiveChat(archived=true)` calls `RemoveQuickAccess` (**Batch 18**).
- [x] **[Chat] Incoming message keeps an archived chat archived** — removed obsolete DM `AutoUnarchiveDMRecipients`; `message.sent` preserves `is_archived=true` while retaining activity and declined-DM re-contact handling; main/archive inbox regressions cover the contract. Canon: [text-chat.md](../features/text-chat.md) § «Архивирование».
- [x] **[Notification] Archived-chat message suppression** — Notification reads Chat member archive metadata, suppresses push and notification-center routing fail-closed when recipient metadata is absent, while Chat retains unread/activity ownership. Canon: [notifications.md](../features/notifications.md) § «Архивированные чаты».
- [x] **[Chat] Gateway REST** — folder RPCs + `GET /chats?folder_id=` (**Batch 19**): `GET/POST /api/v1/chats/folders`, `PATCH/DELETE …/folders/{id}`, `POST/DELETE …/folders/{id}/chats`, `PUT …/chats/order`, `POST/DELETE …/chats/{chatId}/pin`; Quick Access REST — **done (Batch 17)**; `inbox=archive` on `GET /chats` — **done Batch 15**.

### Telegram-parity audit — open CODE (2026-08-28)

Источник: `tmp/telegram-ux-audit/AUDIT.md` (DOC closed). ID — для трассировки с audit tracker.

- [x] **[Chat] R3-A04 — Message requests bucketing** — **done (Batch 21a):** `EnsureDM` sets recipient `inbox_bucket` via Social friends/contacts S2S (`HasContact`); `message.sent` consumer promotes `declined` → `requests` on re-contact — `dm_inbox.go`, `dm_request_recontact.go`.
- [x] **[Messaging] R3-A05 — `PinMessage` permission gap** — standalone `group`/`channel` without `space_id`: deny pin for `member` role (owner/admin allowed); space chats still use Role `TEXT_CHAT_PIN_MESSAGES` — [messaging-service.md](../microservices/messaging-service.md).
- [x] **[Messaging] R3-A06 — `validateAttachments` blocks rich payloads** — **done (Batch 31a):** `content_type` branches for location/article (no File row) and file-backed rich types (`sticker`, `gif`, `music`, `video_note`) with payload shape + File metadata validation — `messaging_grpc.go`, tests — [messaging-service.md](../microservices/messaging-service.md).
- [x] **[Chat] R3-A12 — Standalone `channel` chats** — `CreateChat` without `space_id` creates standalone channel with creator as `chat_members` owner (`CreateChannelChat`); space channels unchanged — **Batch 26b**. **Batch 14:** membership `channel` rows appear in `ListChats` main inbox SQL.
- [x] **[Chat] R3-A14 — `CreateChat`/`UpdateChat` proto fields ignored** — `topic` persisted on create; `topic`/`threads_enabled`/`allow_user_main_feed` on `UpdateChat`; channels updatable — `group.go`, store `UpdateGroupChat` — **Batch 27b**.
- [x] **[Chat] R3-A15 — standalone group/channel guest admission** — `chat_db` migration `000012_allow_guests_fail_closed` hardens the default and backfills standalone rows; `UpdateChat` changes future standalone admission, and User guest-marker checks fail closed. Integration coverage includes group/channel opt-in, forward-only disable, migration backfill, and Space-chat rejection — `guest_admission_integration_test.go`, [chat-service.md](../microservices/chat-service.md) § Guest admission. The Space-owned scope remains open below.
- [x] **[Chat] R3-A16 — `ListChats` space merge bugs** — unified SQL pagination for space chats on page 2+ (`listChatsPageMainWithSpaces` UNION in store; gRPC passes `spaceIDs` on every page). Prior partial fixes: Batch 13 archived filter + hydration; Batch 16 pagination.
- [ ] **[Chat/Messaging/File] Stickers/GIF wire (R2-A32, R4-04)** — expand checklist: `chat_db` migrations `sticker_packs`/`stickers`/`profile_installed_packs`; Chat RPCs (`ListInstalledStickerPacks`, `InstallStickerPack`, `SearchGifs`, …); Gateway REST ([api-gateway.md](../microservices/api-gateway.md) § Stickers and GIF); ~~Messaging proto `STICKER`/`GIF` + send validation~~ **Messaging send validation done (Batch 31a)**; File `UPLOAD_INTENT_STICKER`/GIF transcode; `ListSharedMedia` `STICKERS` kind extension — **P0**
- [x] **[Messaging] Durable delivery consumer** — **done (Batch 12):** Realtime JetStream `message.delivery_ack` publish (Batch 11) + Messaging consumer → `last_delivered_message_id`; list ✓✓ via `GetChatListMetadata.last_message_delivery_state`.
- [x] **[Realtime] R3-A27 — @mention notification payload naming** — WS `mention` op uses `profile_id` (not `user_id`) in `dispatchMentionAdded` (Batch 11).
- [x] **[User] R3-A19 — Presence WS privacy filter (code)** — **done (PR #332):** Realtime resolves a User-filtered `GetPresence` snapshot for every friend and shared-chat WebSocket recipient (including Redis cross-instance chat fan-out). `show_online` controls the sparse online payload and `show_last_seen` controls its timestamp; invisible and User-policy failures emit no sensitive fields or drop the ephemeral update fail-closed. The per-recipient lookup has a shared 2-second deadline and a 16-request concurrency bound — [presence.md](../features/presence.md).
- [x] **[Notification] R3-A23/R4-A15 — `message_request` type in code** — **done (Batch 22a):** `TypeMessageRequest` in notification delivery; push/in-app route by recipient `inbox_bucket=requests` via Chat `ListMembers.inbox_bucket` S2S; Realtime WS fan-out emits `message_request` (not `new_message`) for requests inbox — [notification-service.md](../microservices/notification-service.md). **Client toggle:** [client.md](client.md) Batch 22b.

### Chat — other


- [ ] **[Chat/Messaging/File] Stickers + GIF** — **P0**, 0 code: `[Chat]` pack store + provider search RPC; `[Messaging]` `STICKER`/`GIF` send payload + composer contract; `[File]` animated asset processing — superseded single-line below
- [ ] **[Chat] Стикер-паки / GIF / voice-note first-class — 0 кода** — see `[Chat/Messaging/File] Stickers + GIF` above; voice-note via `[File]` upload category
- [ ] **[File] Upload intent/category: video vs video_note** — proto field + processing branch in `ConfirmUpload` — composer video-note flow — **P0**
- [x] **[Chat] `MuteChat` / `ArchiveChat`** — `mute_archive.go`.
- [x] **[Chat] Group `last_message_at` never updated from message stream** — **done:** `TouchLastMessageAt` updates `type IN ('dm','group','channel')` (`dm.go`); store IT `last_message_at_integration_test.go`.
- [x] **[Chat] Group last_message_at from message stream** — **done:** same as above.
- [x] **[Chat] `UpdateChat` ignores thread settings** — **done (Batch 27b):** `threads_enabled` / `allow_user_main_feed` persisted via `UpdateGroupChat`.
- [x] **[Chat] `UpdateChat` rejects channels** — **done (Batch 27b):** `UpdateChat` allows `group` and `channel`; topic/thread flags via Chat API.
- [ ] **[Chat] Subscription S2S not integrated** — doc dependency (`docs/microservices/chat-service.md`); limit hardcoded `GroupMemberLimit = 500` (`src/backend/chat/internal/store/group.go`). No subscription-tier differentiation.
- [ ] **[Chat] Group admin role unused** — schema allows `owner|admin|member`; only `owner` may `RemoveMember` / `UpdateChat` (`src/backend/chat/internal/grpcsvc/group.go`). No code assigns `admin`. Conflicts with `docs/features/text-chat.md` admin powers.

### Notification


- [ ] **[Notification] `friend_request` delivery зависит от Social NATS** — publisher + `social_events_consumer.go` есть; проверить wiring `NATS_URL` на notification в k8s. Тихие часы/settings **пишутся в БД** (`store/settings.go`) — клиентский dual-write: [client.md](client.md).
- [x] **[Notification] `send_silent` consumption** — `message.sent.send_silent` now maps to platform push silence/no-badge controls while preserving grouping and in-app/unread policy — [notification-service.md](../microservices/notification-service.md)
- [ ] **[Notification] `reply` delivery** — `reply` marked in the notification contract but Realtime maps thread replies to `new_message`; add producer/fan-out support and routing coverage — [notifications.md](../features/notifications.md), `src/backend/realtime/in_app_notification_fanout.go`
- [ ] **[Notification] `system` in-app / Gateway gaps (T-023)** — Moderation NATS consumer produces `system` push for sanctions and narrowly skips presence until an in-app path exists. Still missing/undefined: Notification→Realtime transport + payload + dedupe, final account→profiles semantics, Flutter presentation, other system producers, and Gateway REST exposure — `src/backend/notification/moderation_events_consumer.go`; `src/backend/notification/internal/grpcsvc/server.go`; `src/backend/gateway/transcode_notifications.go`; `src/backend/realtime/`

### Federation


- [ ] **[Federation] Hollow pod on every staging/prod deploy** — `voice-federation` is Tier-1 restart in `scripts/staging/rollout-app-tier.sh`; image built/pushed on every `master` push via `.github/workflows/ci.yml` (`staging-images-push`) and `scripts/ci/staging-image-catalog.json`. Burns CI/CD + cluster resources with no product surface.
- [x] **[Federation] `federation_db` documented but never provisioned** — `docs/DATA_STORES.md`, `docs/microservices/federation-service.md` now mark `federation_db` as planned/deferred; still absent from `docker/postgres/initdb.d/`, `scripts/dev/compose-migrate-all.sh`, `src/backend/migrations/`, `deploy/templates/` until implementation.
- [x] **[Federation] Prometheus scrape misconfigured** — federation scaffold now exposes GET `/metrics` via `pkg/promhttp`; k8s annotations unchanged.
- [ ] **[Federation] Spec ↔ proto drift (implementation trap)** — when work starts, docs and contracts disagree:
- [ ] **[Federation] `federation.events` contract is dead** — `docs/CONTRACT_MATRIX.md` lists Federation → Analytics/Role/Moderation; zero publishers/consumers in `src/backend/analytics/`, `src/backend/role/`, `src/backend/moderation/`, `src/backend/federation/`.

### Story


- [x] **[Story] `show_stories = Nobody` global privacy bypass** — CreateStory caps explicit visibility to `show_stories` floor; `canViewStory` denies when floor is Nobody. Path: `src/backend/story/internal/grpcsvc/story.go` (`capCreateStoryVisibility`, `canViewStory`).
- [ ] **[Story] No `media_file_id` ownership / story-context validation** — any UUID accepted; video duration checked only when File client is wired. Path: `src/backend/story/internal/grpcsvc/story.go` (`CreateStory`, `CreateLookingForParty`); File story context exists in `src/backend/file/internal/grpcsvc/file_grpc.go` but Story does not enforce it.
- [ ] **[Story] Feed degrades to global scan when Social fails** — `GetStoryFeed` falls back to `ListActiveStoriesPaginated` (all active rows) if `ListFeedAuthorIDs` errors; only post-filtered by `canViewStory`. Path: `src/backend/story/internal/grpcsvc/story.go` (`GetStoryFeed`); related: `src/backend/story/internal/privacy/friends.go`, `src/backend/gateway/compose_stories_degradation_live_test.go` (checks liveness only).
- [ ] **[Story] `DeleteStory` orphans R2 media** — soft-delete only; purge worker targets `expired_at IS NOT NULL`, so early-deleted stories never reach `RunArchivePurgeOnce` / `FileDeleter`. Paths: `src/backend/story/internal/store/store.go` (`DeleteStory`), `src/backend/story/internal/jobs/jobs.go`.
- [x] **[Story] Moderation cannot hide stories from feeds** — `HideStoryFromFeed` + `hidden_from_feed_at`; non-author feed/GetStory filtered. Residual: Moderation `ResolveReport` does not yet call Story hide RPC. Paths: `src/backend/story/`; `protos/voice/story/v1/story.proto`.

### Voice


- [ ] **[A2 contract / Voice] Implement the recorded self and moderator room-move contract** — **in progress (moderator move PR):** `MoveToVoiceRoom` remains the authenticated actor’s self-move; `MoveVoiceRoomParticipant` has an explicit target. For a moderator move, the actor needs `VOICE_MOVE_OTHERS` in the source, the target needs `VOICE_JOIN` in the destination, and the actor does not need destination join permission. Actor, target, source, and destination must share a Space; failure leaves both rosters unchanged. **Unblock:** align proto/Gateway/Voice/Role tests and atomic roster/session behavior with this recorded decision. Sources: `tmp/fleet/slave-driver/a2-20260907/CURRENT_STATE.md` § Recorded product and technical decisions; [voice-chat.md](../features/voice-chat.md); [role-service.md](../microservices/role-service.md).
- [ ] **[A2 contract / Role↔Voice] Commander, floor, and broadcast policy lacks a canonical permission mapping** — `VOICE_MUTE_OTHERS` and `VOICE_DEAFEN_OTHERS` already define moderator mute/deafen, so this gap is limited to commander, broadcast, and grant/revoke-floor behavior; those actions must not silently reuse a client/state flag. **Unblock:** Role product/contract owner records their action-to-permission mapping and target/session invariants before these endpoints or UI controls are added. Sources: [role-service.md](../microservices/role-service.md) § Голосовая комната; `tmp/fleet/slave-driver/a2-20260907/A2-P2-acl-design.md` ACL matrix.
- [ ] **[Voice] Один active voice session на профиль across devices** — спека [platforms.md](../features/platforms.md); steal/kick first не дожат.
- [x] **[Voice] S2S deps declared in spec but not wired in `main.go`** — **done (wire):** `voice/main.go` sets `Roles` (`ROLE_GRPC_ADDR`), `SpacePro` (`SUBSCRIPTION_GRPC_ADDR`), `SpaceMembers` (`SPACE_GRPC_ADDR`) when env present; compose sets all three. Remaining gaps: speak/mute role bits, roster NATS events (sibling Voice bullets).
- [ ] **[Voice] Missing NATS events vs `voice-service.md` / Analytics** — never published: `voice.call_started`, `voice.participant_joined`, `voice.participant_left`. Publisher surface stops at incoming/accepted/declined/missed/ended/state/screen-share. Analytics adapter expects `call_started`.
- [ ] **[Voice] Space voice join/leave publishes no roster events** — no `participant_joined` / `participant_left` / `voice.state_changed` on `JoinVoiceRoom` / `LeaveVoiceRoom`; Realtime consumer has no handlers for those subjects anyway.
- [ ] **[A2 Voice↔Realtime] Implement the frozen Phase-0 roster projection wire** — add Voice-authored `full_profile_ids` / `occupancy_profile_ids`, `roster_epoch`, `projection`, independent contiguous `projection_version`, UUID `event_id` and the exact full/occupancy payload variants; publish only after the authoritative mutation commits, fail closed on Space/Role audience uncertainty, and add Realtime delivery/snapshot-gap tests. Occupancy events contain only count and projection metadata; full-only state changes neither emit nor advance the occupancy projection. The existing participant compatibility stream is not this watcher stream. — [voice-service.md](../microservices/voice-service.md#phase-0-space-room-media-roster-и-lifecycle-target-не-реализовано), [realtime-service.md](../microservices/realtime-service.md#phase-0-space-room-roster-fan-out-target-не-реализовано).
- [ ] **[Voice] Staging LiveKit WebRTC likely broken without ops beyond WS smoke** — `deploy/staging/infra.yaml` sets `use_external_ip: true` but no `node_ip` (compose uses explicit `node_ip: 127.0.0.1` in `deploy/livekit/livekit.yaml`). Signaling is `wss://` via Ingress; RTC is NodePort **30881/TCP + 30882/UDP** on the node — not validated by `scripts/staging/smoke-staging.sh` (WS probe only).
- [ ] **[Voice] No LiveKit Server SDK room lifecycle** — docs say create/close rooms via SDK; implementation only mints JWT (`internal/livekit/token.go`). Rooms rely on implicit LiveKit auto-create; no explicit teardown.

### Auth


- [ ] **[Auth] Resend на staging/prod** — `ResendMailSender` есть; без `RESEND_API_KEY` → `NoopMailSender`. [ci.md](ci.md).
- [x] **[Auth] NATS `user.guest_converted` not wired in compose/staging** — **done (compose):** `AUTH_NATS_URL` + `depends_on: nats` in `docker-compose.yml`; convert publishes + `TestComposeConvertGuestNATS_live`. Staging env still worth verifying separately.
- [ ] **[Auth] Password change (logged-in) + revoke-all-refresh not implemented** — reset-via-OTP есть; нет change-password для сессии. UI reset — [client.md](client.md).

### Realtime


- [ ] **[Realtime] Subscription bootstrap lacks Space voice/tree scopes** — Chat `ListChats` bootstrap now pages all visible DM/group/channel chats and friend presence has its own `user.presence_changed` stream. Realtime still has no authoritative Space voice-room/tree watcher bootstrap; client chat `subscribe` must not be reused to infer that audience. — [realtime-service.md](../microservices/realtime-service.md#подписки), `src/backend/realtime/dm_chat_lister_grpc.go`, `src/backend/realtime/user_events_consumer.go`.
- [ ] **[Realtime] In-app `notification` targets WS-subscribed profiles, not chat membership** — `in_app_notification_fanout.go` uses `hub.profileIDsSubscribedToChat(chatID)` as the recipient set. Connected group members who have not subscribed to that chat miss `notification` (and may miss `message_create` too).
- [ ] **[Realtime] Redis connection registry is write-only** — `redis_registry.go` `Register`/`Unregister` are called from `ws.go` but never read for routing. Doc describes `{profile_id → [instance_id, conn_id]}` registry for multi-instance fanout (`realtime-service.md`); actual cross-instance path is Redis Pub/Sub + per-instance NATS durables only.

### Multi-Profile

- [x] **[Multi-Profile] Auth `switch-profile` uses User Service** — Auth calls the existing
  `User.SwitchProfile` contract before issuing the replacement JWT; direct profile SQL was removed.
- [ ] **[Multi-Profile] Premium profile limit unreliable** — `CreateProfile` gates on JWT `subscription_tier` (`user.go`); tier stuck at `free` until Auth↔Subscription wired (см. [Subscription] JWT tier).
### Auth / Social

- [ ] **Auth phone-hash S2S live** — `compose_phone_sync_live_test` на живом стеке (unit-тесты `ResolvePhoneHashes` / `auth_phone_hash.go` есть).

## Common

### Subscription


- [ ] **[Subscription] `GetLimits` far below spec — only `file_upload_bytes` + `profile_count`; missing retention, space-join cap, voice quality, cosmetic flags, space voice/tree/emoji limits** — `src/backend/subscription/internal/limits/limits.go`; `docs/microservices/subscription-service.md`; `docs/features/subscription.md`
- [ ] **[Subscription] `GetLimitsRequest.scope_space` ignored** — `protos/voice/subscription/v1/subscription.proto`; `src/backend/subscription/internal/grpcsvc/subscription.go` (`GetLimits`)
- [ ] **[Subscription] `CheckLimit` wrong scope — `space_member_count` uses `HasActiveSpaceProForPurchaser(account_id)` instead of space entitlement** — `src/backend/subscription/internal/grpcsvc/subscription.go`; `src/backend/subscription/internal/store/store.go`
- [ ] **[Subscription] Space tree node cap not Space-Pro-aware — hardcoded 50, spec 500 for Pro** — `src/backend/space/internal/store/tree.go` (`MaxTreeNodes = 50`)
- [ ] **[Subscription] `GetBillingHistory` stub — always empty list** — `src/backend/subscription/internal/grpcsvc/subscription.go`
- [ ] **[Subscription/Notification] Grace reminder delivery and duplicate safety remain incomplete — Subscription emits `subscription.grace_reminder` on D1/D3/D7 and sent-day bookkeeping suppresses ordinary sequential repeats, but publish-before-mark has no atomic claim or `Nats-Msg-Id`, so crashes/concurrent replicas can duplicate. Notification only validates recognized days, logs consumption, and ACKs; no push/email dispatch or client presentation.** — `src/backend/subscription/internal/sweeper/sweeper.go`; `src/backend/subscription/internal/store/grace_reminders.go`; `src/backend/subscription/internal/subscriptionevents/jetstream.go`; `src/backend/notification/subscription_events_consumer.go`
- [ ] **[Subscription] Activation ignores billing period / provider metadata — always `monthly`, synthetic `provider_subscription_id`** — `src/backend/subscription/internal/store/store.go` (`ActivatePremium`, `ActivateSpacePro`)
- [ ] **[Subscription] `billing_events.amount` / `currency` never written** — `src/backend/migrations/subscription_db/000001_init.up.sql`; `src/backend/subscription/internal/store/store.go` (`insertBillingEventTx`)
- [ ] **[Subscription] No Flutter Space Pro checkout / management** — `src/frontend/lib/backend/subscription_client.dart`; `src/frontend/lib/ui/settings/subscription_settings_screen.dart` (Premium only)

### File


- [ ] **[File] “WebP” conversion is JPEG bytes** — `encodeJPEG` with `.webp` key suffix and `Content-Type: image/jpeg` (`d:\Git\Voice\src\backend\file\internal\imgproc\webp.go`); spec requires WebP re-encode (`d:\Git\Voice\docs\features\file-storage.md`).
- [ ] **[File] Post-processing caps remain unimplemented for future intent-driven GIF/sticker/video/video_note/audio/document/article/location branches** — current non-E2E image→WebP processing enforces ≤5 MiB before derivative writes; `UploadIntent` and the remaining converters are not implemented. Canon: `docs/features/file-storage.md`; `docs/microservices/file-service.md`.
- [ ] **[File/Gateway/Flutter] Implement the accepted thumbnail URL variant (P-007)** — extend existing `GetFileURLRequest` with backward-compatible `FileURLVariant variant`; `UNSPECIFIED` keeps converted→original selection, `THUMBNAIL` presigns only `thumbnail_r2_key`, without fallback. Retain `FileAccessSelector`/ACL and `FILE_READ_SURFACE_URL`; missing thumbnail → `FailedPrecondition`, invalid variant → `InvalidArgument`; response remains URL + expiry (TTL 1h). Gateway serves `GET /api/v1/files/{id}/url?variant=thumbnail`; Flutter may set `previewUrl` only from this presigned response and must retain `previewUrl == null` until the variant is implemented. Refresh the same variant after URL `403`/`410`; never expose direct R2 URLs or keys. Add proto/Gateway/File/Flutter tests, including compatibility, ACL, missing/invalid variants, no fallback and refresh. Canon: [file-service.md](../microservices/file-service.md#thumbnail-url-variant-accepted-not-implemented), [api-gateway.md](../microservices/api-gateway.md), [file-storage.md](../features/file-storage.md).
- [ ] **[File] ClamAV E2E likely ineffective** — live test uses `eicar.com` + `text/plain` (`d:\Git\Voice\src\frontend\test\file_clamav_infected_e2e_live_test.dart`); `shouldScan` only matches `.exe`/`.zip`/`.bat` + zip/exe MIME (`d:\Git\Voice\src\backend\file\internal\grpcsvc\file_grpc.go` L588–596) — scan skipped, confirm may succeed.
- [ ] **[File] `ListFiles` chat filter unimplemented** — `filter_chat` → `FailedPrecondition` (`d:\Git\Voice\src\backend\file\internal\grpcsvc\file_grpc.go` L408–410).
- [x] **[File] Free-tier `expires_at` not set** — non-E2E free uploads have `expires_at = NULL`; retention cron has nothing to query (`d:\Git\Voice\src\backend\file\internal\grpcsvc\file_grpc.go` L163–167).
- [ ] **[File] Attachment lifecycle partial** — Messaging validates `ready` + chat link + scan (`d:\Git\Voice\src\backend\messaging\internal\grpcsvc\messaging_grpc.go` L312–359), but no `file_references`, no expiry placeholder UX (`d:\Git\Voice\docs\features\file-storage.md` “кучка костей”), no message preview refresh on `file.processed`.

### Protos/Pkg


- [x] **[Protos/Pkg] Auth proto duplication sync gate** — the protobuf CI gate compares canonical `protos/voice/auth/v1/auth.proto` with the Auth Maven copy `src/backend/auth/src/main/proto/voice/auth/v1/auth.proto`; a change to either path triggers the check. Fixture regression coverage verifies matching copies, comment-only differences, wire mismatches, missing-copy diagnostics, and the CI wiring.
- [ ] **[Protos/Pkg] Federation protos orphaned from service** — `protos/voice/s2s/v1/s2s.proto`, `federation_management.proto` codegen to Flutter (`src/frontend/lib/gen/voice/s2s/`) and Go hubs, but `src/backend/federation/go.mod` depends only on `voice/backend/pkg` (scaffold; deferred per `docs/PLAN.md`).
- [ ] **[Protos/Pkg] `common.proto` under-specified** — `protos/voice/common/v1/common.proto` has pagination only; no shared idempotency/actor/ref types despite `docs/ARCHITECTURE_REQUIREMENTS.md` idempotency key and `messaging.proto` inline idempotency contract.
- [ ] **[Protos/Pkg] Analytics taxonomy vs proto** — `docs/MICROSERVICES.md` analytics examples include `file_downloaded`, `space_left`, `voice_room_created`, `message_forward`, notification push metrics; no corresponding arms in `jetstream_events.proto` and no publishers found.

### Space


- [ ] **[Space] ChatLookup S2S hardening (Agent batch)** — set `x-voice-internal-caller=space` on Chat GetChat; Warn on lookup failures instead of silent skip; add unit/mock test for enrichment; optional batch GetChat to avoid N+1 (`chat_lookup.go`, `main.go`). Closed High wiring via PR #129.
- [ ] **[Space] Complete the accepted BE-116 audit ledger runtime** — the bounded Space ledger core, seven Space-local writers, durable Space outbox, 365-day retention worker, signed unfiltered snapshot cursor and full filter store query are shipped. The canonical action/target/details registry, complete cursor contract and protected Role/Chat `AppendAuditEvent` proto are defined in [space-service.md](../microservices/space-service.md#phase-0-space-audit-ledger-accepted-target-runtime-not-implemented). Remaining: the other accepted Space-local writers, idempotent protected ingestion, Role/Chat durable producer outboxes, gRPC filter mapping and complete cursor binding, plus positive/negative integration tests. Sources: `src/backend/space/internal/store/{invite,tree,audit}.go`; `src/backend/space/internal/grpcsvc/`; `src/backend/role/internal/grpcsvc/`; `src/backend/chat/internal/grpcsvc/`.
- [x] **[Space] `RevokeInvite` / `ListInvites` use `SpaceManageInvites`** — **done:** PR #225 replaced the owner-only gate with `requireSpacePermission(..., SpaceManageInvites)`; `Test(ListInvites|RevokeInvite)_RequiresManageInvitesPermission` verifies delegated access and denial without the permission.
- [x] **[Space] Invite/public join membership event** — both join paths call `finalizeMembership`, which publishes `space.member_joined` after a new membership is created (`invites.go`, `join.go`, `spaceevents/jetstream.go`).
- [x] **[Space] Kick/leave membership event** — `KickMember` and `LeaveSpace` publish `space.member_left` after removal (`members.go`, `join.go`, `spaceevents/jetstream.go`). Residual Space event gaps remain in the dedicated NATS item above.
- [ ] **[Space] No gateway REST for leave/join-public/delete/transfer/audit/templates** — `src/backend/gateway/transcode_spaces.go`, `transcode_spaces_members.go`
- [ ] **[Space] Flutter client gaps — `spaces_client.dart` has no leave/join-public/transfer/audit/delete** — `src/frontend/lib/backend/spaces_client.dart`
- [ ] **[Space] Test holes — no integration tests for unimplemented RPCs; tree update/delete/category update/voice update/delete/RemoveTreeNode thin coverage** — `src/backend/space/internal/grpcsvc/*_integration_test.go`
- [ ] **[Space] Stale README still says “scaffold / out of scope”** — `src/backend/space/README.md`

### Moderation


- [x] **[Moderation] Service README status** — describes the implemented core and keeps residual gaps explicit.
- [ ] **[Moderation] No report dedup / rate limiting** — unlimited reports per reporter/target; no abuse protection.
- [ ] **[Moderation] Report targets not validated** — no S2S checks that message/space/story/user exists (deps listed in `moderation-service.md` unused beyond profile→account lookup).
- [ ] **[Moderation] Admin API gaps** — no HTTP for `ReviewAppeal`, `RevokeSanction`, `GetReport` by ID; admin UI (`src/admin/src/api/moderation.ts`) only list/resolve/sanction/audit stub.
- [ ] **[Moderation] Compose E2E gap** — `TestComposeModeration_live` covers perm_ban + login block; comment mentions shadow ban but test does not exercise it.
- [ ] **[Moderation] Global moderator phone requirement not enforced** — staff role checked at Gateway; no verified-phone gate in Moderation.
- [ ] **[Moderation] Trust E2E scope ≠ moderation depth** — `TestComposeTrust_live` / `trust_e2e_live_test.dart` cover report 202 + privacy + 2FA only, not sanctions/appeals/automod.

### Social


- [ ] **[Social] No store-layer unit tests** — `src/backend/social/internal/store/friendships.go`, `blocks.go` only exercised via grpc integration tests; coverage artifact shows 0 hits on store paths (`src/backend/social/coverage`, `$prof`).
- [ ] **[Social] No `s2s` privacy tests** — `src/backend/social/internal/s2s/privacy.go` (`GRPCUserPrivacy`, `GRPCSpaceCoMembership`) untested; only `auth_phone_hash_test.go` in `s2s/`.
- [ ] **[Social] Test helper ≠ production wiring** — `src/backend/social/testsocial/bufconn_server.go` omits `Privacy`, `PhoneHashes`, `SpaceCoMembership` wired in `main.go`.
- [ ] **[Social] Flutter client surface incomplete** — `friends_client.dart` now has contacts/favorites (**Batch 23b**), `listBlocked`/`unblockAccount` + Blocked tab (**Batch 24a**), `syncPhoneContacts` stub + Contacts tab action (**Batch 25a**), QR add friend UI (**Batch 26a**). Gateway exposes phone sync (`transcode_friends.go`). **Deferred:** live camera QR scanner (paste link works).
- [ ] **[Social] No live/E2E for friend-request privacy denial** — `privacy_actions_e2e_live_test` / `compose_privacy_actions_live_test.go` exercise DM/calls/files, not `POST /api/v1/friends/invitations`.
- [ ] **[Social] Stale service README** — `src/backend/social/README.md` still claims health-only scaffold; contradicts implemented gRPC + migrations.

### User


- [ ] **BE-131 [User] Org DNS verification lifecycle** — pending DNS request needs TTL: exactly one active request; a new Start atomically expires the prior request; Check accepts only the current unexpired request; an old TXT never grants a badge; verifier unavailability does not consume the request. Source: `src/backend/user/internal/store/verification.go`.
- [x] **[User] `README.md` status** — describes the implemented RPC surface and keeps residual gaps explicit (`src/backend/user/README.md`).

### Analytics


- [ ] **[Analytics] Dashboard types missing** — Only `product`, `engagement`, `revenue`, `health`, `moderation` in query layer; no `search` / `voice` / `federation` despite spec tables (`d:\Git\Voice\src\backend\analytics\internal\store\query.go`, `docs/microservices/analytics-service.md`).
- [ ] **[Analytics] Funnel `onboarding` not implemented** — Only `registration` funnel (`d:\Git\Voice\src\backend\analytics\internal\store\query.go`); proto/admin reference registration only (`d:\Git\Voice\protos\voice\analytics\v1\analytics.proto`, `d:\Git\Voice\src\admin\src\pages\FunnelsPage.tsx`).
- [ ] **[Analytics] `GetMetrics` filters ignored** — Proto `filters` map never applied (`d:\Git\Voice\src\backend\analytics\internal\grpcsvc\query.go`, `d:\Git\Voice\protos\voice\analytics\v1\analytics.proto`).
- [ ] **[Analytics] MVs unused by query API** — `dau_mv` / `events_by_type_mv` created in DDL (`d:\Git\Voice\docker\clickhouse\init\001_events.sql`) and used in Grafana, but REST queries scan raw `voice.events` (`d:\Git\Voice\src\backend\analytics\internal\store\query.go`).
- [ ] **[Analytics] Thin test coverage** — No tests for `consumer`, `grpcsvc`, or `store/query`; integration tier only tests `InsertBatch` (`d:\Git\Voice\src\backend\analytics\internal\store\clickhouse_integration_test.go`). Existing unit tests: adapters, buffer, hash, health only.
- [ ] **[Analytics] Admin UI partial** — Product table + registration funnel + export only; no engagement/revenue/retention/search/voice pages (`d:\Git\Voice\src\admin\src\pages\`).
- [ ] **[Analytics] Grafana dashboards partial** — Only product, engagement, ingest (`d:\Git\Voice\deploy\observability\grafana\dashboards\`); missing revenue/health/moderation/search/voice panels from spec.
- [x] **[Analytics] `role_events` stream consumed** — **done:** the current runner registers `role_events` / `role.>` with `handleRoleMsg`; PR #238 retries only missing streams with exponential backoff and context cancellation, covered by helper-classification tests (`src/backend/analytics/internal/consumer/runner.go`, `runner_test.go`).
- [ ] **[Analytics] Engagement metrics shallow** — Voice “minutes”, MM sessions, active spaces, stories use coarse event counts or are absent (`d:\Git\Voice\src\backend\analytics\internal\store\query.go`, `d:\Git\Voice\src\backend\analytics\internal\adapters\domain.go`).
- [ ] **[Analytics] Export live test doesn’t verify audit log** — Comment claims audit path; test only checks HTTP 200 (`d:\Git\Voice\src\backend\gateway\compose_analytics_export_live_test.go` vs DoD §3 in `docs/features/analytics.md`).
- [ ] **[Analytics] Prod ClickHouse DDL manual** — Comment-only apply path vs staging Job automation (`d:\Git\Voice\deploy\prod\infra.yaml` vs `d:\Git\Voice\scripts\staging\apply-clickhouse-init.sh`).
- [ ] **[Analytics] Doc drift: Redis buffer** — `docs/PLAN.md` L84 and `docs/ARCHITECTURE_REQUIREMENTS.md` mention Redis buffer; implementation and `docs/DATA_STORES.md` say in-memory only (`d:\Git\Voice\src\backend\analytics\internal\buffer\accumulator.go`).
- [ ] **[Analytics] Stale service README** — Still describes scaffold/health-only (`d:\Git\Voice\src\backend\analytics\README.md`).

### Matchmaking


- [x] **[Matchmaking] `mm.player_banned` publication** — `BanFromMM` emits one protobuf `MatchmakingStreamEvent.player_banned` on `mm.player_banned` only when a peer ban is newly inserted; repeated idempotent bans do not republish (`grpcsvc/rating.go`, `store/bans.go`, `mmevents/publisher.go`; `TestBanFromMM_PublishesOnceAfterNewPeerBan`).
- [ ] **[Matchmaking] Popular-games ordering missing** — `ListGames` sorts by `created_at DESC` (`store/games.go`). Spec wants popularity by active queue depth (`docs/features/game-catalog.md`).
- [ ] **[Matchmaking] `CreateGame` lacks `icon_url` / `external_id`** — Columns exist (`migrations/matchmaking_db/000001_init.up.sql`, `store/games.go`) but `CreateGame` only persists name+config (`grpcsvc/server.go`).
- [ ] **[Matchmaking] Party / voice-derived MM absent** — `PartyStore` is a stub (`store/parties.go`); `StartSearch` always validates `partySize=1` (`grpcsvc/search.go`, `criteria/criteria.go`). Voice join/leave reset flow from spec not implementable yet.
- [ ] **[Matchmaking] Test gaps for prod-scale modes** — No matcher test for seeded 10-slot games or role-diversity matching (`matcher/worker_test.go` uses custom 2-slot Duo only).

### Role


- [ ] **[Role] `color`, `is_mentionable` in DB, not in API — columns in migration; absent from proto, store scans, REST responses.** — `src/backend/migrations/role_db/000001_init.up.sql`, `protos/voice/role/v1/role.proto`, `src/backend/role/internal/store/roles.go`, `grpcsvc/roles.go`
- [x] **[Role] No live E2E for voice room overrides — store/grpc tests exist; no compose/Flutter E2E for `VOICE_JOIN` deny (UI strings exist).** — **done (VOICE_JOIN deny):** `TestComposeVoiceJoinDeny_live` + Flutter VOICE_JOIN deny (#14).
- [ ] **[Role/Space] Implement the approved Owner-role transfer path.** The contract now reserves `Owner` mutation for the dedicated trusted, operation-bound Space transfer and requires direct/public member-role RPCs to reject it. Current compensated `Space.TransferOwnership` still calls public Role `AssignRole`/`RevokeRole` and has no Auth-proof or `operation_id` binding. Add the authenticated dedicated path, Role guards and contract tests together; do not expose the old handler through Gateway/Flutter. — [space-service.md](../microservices/space-service.md#ownership-transfer-contract-target-not-implemented); [spaces.md](../features/spaces.md#контракт-подтверждения-передачи-владения); `src/backend/space/internal/grpcsvc/{space.go,roles.go}`; `src/backend/role/internal/grpcsvc/roles.go`; `src/backend/role/internal/store/roles.go`
- [ ] **[Realtime/Role] Voice-room override events have no authoritative recipient index — Role publishes `role.voice_override_set` / `role.voice_override_removed` with `space_id`, `voice_room_id`, `role_id`, but Realtime only indexes chat subscriptions. Define and implement an authoritative voice-room subscription or participant routing contract before claiming WS invalidation for voice overrides; never broadcast to an inferred Space audience.** — `src/backend/realtime/{ws_hub.go,role_events_consumer.go}`, `docs/microservices/{role-service.md,realtime-service.md}`
- [x] **[Role] API `created_at` uses persisted DB value** — **done:** PR #239 maps `RoleRow.CreatedAt`; unit and integration coverage verify the serialized timestamp (`src/backend/role/internal/grpcsvc/roles.go`, `roles_created_at_test.go`, `roles_integration_test.go`).
- [ ] **[Role] Federation role sync — listed in role-service deps; Federation deferred, no SyncSnapshot path.** — `docs/microservices/role-service.md`; `src/backend/federation/`

### Bot


- [ ] **[Bot] `ListInstalledBots` mislabels chat types** — all whitelist refs hardcoded to `CHAT_TYPE_CHANNEL` (`internal/grpcsvc/interaction.go`).
- [ ] **[Bot] `DeleteBot` is soft-disable only** — `status = 'disabled'` (`internal/grpcsvc/bot.go`); `bot_space_installations` / `bot_chat_whitelist` rows remain.
- [ ] **[Bot] `UpdateBot` ignores proto fields** — only name/description updated; `avatar_url`, `scopes_json` from `UpdateBotRequest` ignored (`internal/grpcsvc/bot.go`).
- [ ] **[Bot] Archive chat not implemented** — `TEXT_CHAT_CREATE_IN_SPACE` docs say create/**archive** (`docs/features/bots.md`); only `CreateBotChat` exists (`internal/grpcsvc/bot_c.go`).
- [ ] **[Bot] Channel install skips `Chat.AddMembers`** — `InstallBotInSpace` `continue`s on channel refs (`internal/grpcsvc/interaction.go`); bot actor may not join channel chats.
- [ ] **[Bot] Autocomplete skips offline check** — `ExecuteSlashInteraction` gates on presence; `AutocompleteSlashOption` does not (`internal/grpcsvc/autocomplete.go` vs `interaction.go`).

### Cross-cutting


- [ ] **[Cross-cutting] Inconsistent entitlement resolution — Gateway live-calls Subscription only for File (`gateway/subscription_tier.go`); User/Chat trust JWT metadata. Premium UX fragmented after payment.** — `src/backend/gateway/subscription_tier.go`, `src/backend/user/internal/authctx/authctx.go`
- [ ] **[Cross-cutting] Compose infra version drift — local Postgres 16 / Redis 7 (`docker-compose.yml`) vs target Postgres 18 / Redis 8 (`docs/MICROSERVICES.md`). Staging/prod parity risk for migrations and Redis features.** — `docker-compose.yml`, `docs/MICROSERVICES.md`
- [ ] **[Cross-cutting] Partial-feature integration E2E missing — no cross-smoke for: premium → profile banner/GIF/3rd profile; premium → Story anonymous view; subscription grace → push/email; bot slash → in-app notification (only isolated feature tests).** — `.github/ci/e2e-features.yml`, `src/frontend/test/`
- [ ] **[Cross-cutting] `profiles_verification` / `encryption_dm` not in smoke — PLAN partial/shipped-opt-in; smoke has `encryption_key_backup` only, not DM encryption or verification flows.** — `.github/ci/e2e-features.yml`, `docs/PLAN.md`
- [ ] **[Cross-cutting] gRPC mTLS not wired — admitted in `docs/DEPLOYMENT.md`; `MICROSERVICES.md` security section still states mTLS between services. Staging relies on NetworkPolicy + `BOT_GRPC_GATEWAY_ONLY`.** — `docs/DEPLOYMENT.md`, `deploy/templates/network-policy-voice-bot.yaml`
- [ ] **[Cross-cutting] Distributed tracing absent** — v1 uses `request_id` in logs; deferred per [ADR 003](../adr/003-distributed-tracing-deferred.md). — `docs/features/observability.md`, `deploy/observability/`

### Messaging


- [ ] **[Messaging] `ListThreads` bounded successor** — legacy store aggregates `messages` with a limit-only query. Implement the Messaging-owned per-viewer versioned AVL read model, durable Chat membership outbox/inbox, journal/backfill/readiness, cursor state and live visibility revocation defined in `docs/microservices/messaging-service.md` and `docs/testing/listthreads-versioned-readmodel-exec-plan.md`; do not activate cursor pagination through candidate-limited SQL.
- [ ] **[Messaging] `message_attachments` target table not migrated** — spec DDL; implementation uses `messages.attachments` JSONB + indexes (`000008_shared_media_indexes`).
- [ ] **[Messaging] Test holes on forward / GetMessage** — forward tests cover DM/group attribution only; no channel forward, E2E forward, commentary, or `GetMessage` integration test.
- [ ] **[Messaging] NATS publish best-effort** — DB commit succeeds, JetStream failure only logged (`logPublishError`); no outbox/retry.

### Search


- [ ] **[Search] Meilisearch v2 not started — no client, abstraction, or compose/k8s wiring; v1 Postgres only (correct per threshold matrix, but no swap-ready interface).** — `src/backend/search/` (entire module); `docs/DATA_STORES.md`
- [ ] **[Search] Federated search not implemented — spec in `docs/features/search.md` / `docs/microservices/search-service.md`; federation deferred in `docs/PLAN.md`.** — — (no code)
- [ ] **[Search] Analytics telemetry incomplete — only partial `analytics.search.query` (`query_len`, `message_hits`); missing `search.zero_results`, `search.result_clicked`, `profile_id`, `scope`, `results_count` per `docs/microservices/search-service.md`.** — `src/backend/search/internal/grpcsvc/search.go`
- [ ] **[Search] Role Service documented, Chat used for ACL — `CanReadMessages` delegates to Chat `GetChat`, not Role Service.** — `src/backend/search/main.go` (`ChatReadAccess`); `docs/microservices/search-service.md`
- [ ] **[Search] No deletion tombstones — `user_account_deleted`, chat/space delete events not consumed; stale rows remain in projections.** — `src/backend/search/internal/indexer/profile_indexer.go`, `chat_space_indexer.go`, `message_indexer.go`
- [ ] **[Search] `ProfileSwitched` not indexed — new active profile may be missing from `profile_search_documents` until a separate create/update event.** — `src/backend/search/internal/indexer/profile_indexer.go`; `protos/voice/events/v1/jetstream_events.proto`
- [x] **[Search] Privacy audience on profile discovery** — SearchUsers/SearchGlobal filter by target `allow_friend_requests` via User S2S + Social/Space matcher; bidirectional blocks (`filterProfileHits`). Remaining: User `SearchProfiles` path (`/api/v1/users/search`) still separate.

### Chat


- [ ] **[Chat] NATS event surface incomplete vs doc** — published: `chat.created`, `chat.member_changed` (`src/backend/chat/internal/chatevents/jetstream.go`). Not published: `chat.updated`, `chat.deleted`, granular `member_added`/`removed`/`left` (`docs/microservices/chat-service.md` table).
- [ ] **[Chat] S2S enrichment fails open** — Messaging errors logged and zeroed (`src/backend/chat/internal/grpcsvc/list_chats.go:77-81`). Documented degradation, but no metric/alert on enrichment skip.
- [x] **[Chat] README status** — describes the implemented gRPC core and keeps residual gaps explicit (`src/backend/chat/README.md`).

### Notification


- [ ] **[Notification] Email (Resend) channel missing — spec lists auth-only email via Resend; zero code in Notification (Auth also has `otp_codes` DDL but no Resend sender).** — `docs/microservices/notification-service.md`, `src/backend/notification/` (no email package), `src/backend/auth/src/main/resources/db/migration/V1__auth_schema.sql`
- [ ] **[Notification] Redis rate limiting not implemented — spec mentions rate limiting; Redis used only for grouping.** — `docs/microservices/notification-service.md`, `src/backend/notification/internal/grouping/store.go`, `main.go`
- [ ] **[Notification] Analytics telemetry incomplete — only `analytics.notification.push_sent` on gRPC `SendNotification`; NATS-driven pushes don’t publish; no `push_delivered` / `push_clicked`.** — `src/backend/notification/internal/grpcsvc/server.go`, `docs/microservices/notification-service.md`
- [ ] **[Notification] `APNS_VOIP_TOPIC` in deploy unused — VoIP sender uses `APNS_BUNDLE_ID` as topic, not separate VoIP topic from secrets.** — `deploy/staging/secret.example.yaml`, `src/backend/notification/internal/apns/voip_sender.go`
- [ ] **[Notification] APNs E2E proves registration only, not delivery — unlike FCM compose test with `RecordSender` + debug endpoint.** — `src/frontend/test/apns_e2e_live_test.dart`, `src/frontend/test/fcm_delivery_e2e_live_test.dart`, `src/backend/notification/debug_http.go`
- [ ] **[Notification] No NATS/JetStream integration tests in Notification service — consumer wiring untested end-to-end at service boundary (Gateway has register-device live test only).** — `src/backend/notification/` (no `*_integration_test.go` for consumers), `src/backend/gateway/compose_notification_live_test.go`
- [ ] **[Notification] JetStream `DeliverNew()` on all consumers — restarts skip in-flight/backlog; at-least-once redelivery behavior not covered by explicit ack/nak handling.** — `src/backend/notification/*_events_consumer.go`

### Federation


- [x] **[Federation] Not in local compose stack** — `docker-compose.yml` now has `federation` (health/metrics scaffold); `GATEWAY_GRPC_UPSTREAMS_JSON` still omits it by design (S2S-only).
- [ ] **[Federation] No k8s migrate job** — unlike shipped services, no `federation_db` template in `deploy/templates/`; first real impl needs DB bootstrap path.
- [ ] **[Federation] No downstream product hooks** — federated spaces/auth/search/moderation described in `docs/features/federation.md`, `docs/features/search.md` §owners — no `federat*` code in `src/backend/space/`, `src/backend/search/`, `src/backend/auth/`, `src/backend/notification/`.
- [ ] **[Federation] No control-plane surface** — `FederationManagementService` in `protos/voice/s2s/v1/federation_management.proto` (mTLS, admin ops) — no server, no `src/admin/` UI.
- [x] **[Federation] K8s manifest lacks gRPC port** — `voice-federation` Deployment/Service now expose `:9090` with `FEDERATION_GRPC_LISTEN`; gRPC server still unimplemented in scaffold.
- [ ] **[Federation] Gateway upstream omission** — `deploy/staging/configmap-app.yaml` / `deploy/prod/configmap-app.yaml` omit federation from `GATEWAY_GRPC_UPSTREAMS_JSON` (correct for S2S-only; document when gRPC lands).

### Story


- [ ] **[Story] `visibility_audience` not writable via API** — DB column + read path exist; `CreateStoryRequest` has no audience JSON; `visibilityFromRequest("custom")` stores `privacy.Nobody()`. Blocks real space/custom per-story audience (Batch 7 covers Flutter picker; backend contract gap). Paths: `protos/voice/story/v1/story.proto`, `src/backend/story/internal/grpcsvc/audience.go`, `src/backend/migrations/story_db/000002_visibility_audience.up.sql`.
- [ ] **[Story] Highlights lack `visibility_audience` JSONB** — only coarse `visibility` TEXT; no space multiselect per [stories.md](../features/stories.md) §Highlights. Paths: `src/backend/migrations/story_db/000001_init.up.sql`, `src/backend/story/internal/grpcsvc/story.go` (`canViewHighlight`).
- [x] **[Story] `AddToHighlight` allows active stories** — fixed: store requires `expired_at` while locking the Story before it creates a Highlight membership. Path: `src/backend/story/internal/store/store.go` (`AddToHighlight`).
- [x] **[Story] Archive purge worker delayed first run** — closed at `d66d92b`: startup `run()` dispatches immediately; `main` passes `serviceCtx` through the worker to cancel in-flight File work; focused `TestStartArchivePurgeWorker_runsOnceOnStartup` and `TestStartArchivePurgeWorker_propagatesServiceCancellationToStartupDispatch` cover both paths. Separate `DeleteStory` orphan-media and compose expiry full-chain TODOs remain open.
- [ ] **[Story] `game_tag` unvalidated** — free string, no Matchmaking catalog lookup. Path: `src/backend/story/internal/grpcsvc/story.go`.
- [ ] **[Story] No compose/live E2E for LFP** — Gateway unit test only. Paths: `src/backend/gateway/transcode_stories_test.go`; no `compose_*lfp*` / Flutter LFP create flow in CI ([`.github/ci/e2e-features.yml`](../../.github/ci/e2e-features.yml)).
- [ ] **[Story] Stale service README** — still says “scaffold”. Path: `src/backend/story/README.md`.

### Voice


- [x] **[Voice] `GetVoiceStates` populates commander/floor fields** — `is_commander`, `hand_raised`, `has_floor`, `is_broadcasting` in store + GetVoiceStates + state events (П.11 / VC-07).
- [ ] **[Voice] E2E coverage gaps vs PLAN “shipped”** — present: DM signaling (`TestComposeVoiceCall1to1_live`), optional bidirectional audio (`compose_voice_call_media_live_test.go`), Flutter `group_voice` / `spaces_voice` / `screen_share` API tests. Missing: compose live test for **space** voice + screen share with Role guard; no staging **RTC/media** smoke; `group_voice` E2E never exercises `LeaveCall` multi-participant behavior.
- [ ] **[Voice] Stale service README** — still says “scaffold / out of scope” while PLAN marks voice shipped.

### Auth


- [ ] **[Auth] NATS event matrix mostly unimplemented** — `docs/microservices/auth-service.md` lists `user.registered`, `user.logged_in`, `user.logged_out`, `user.2fa_enabled`, `user.account_deleted`, `user.account_restored`; `AuthEventPublisher` only defines `user.guest_converted`. Files: `src/backend/auth/src/main/java/voice/backend/auth/events/AuthEventPublisher.java`, `src/backend/auth/src/main/java/voice/backend/auth/events/NatsAuthEventPublisher.java`.
- [ ] **[Auth] Disable 2FA not implemented** — No RPC/REST to turn off TOTP or invalidate backup codes after enrollment. Sessions list/revoke **есть** (`GET /api/v1/auth/sessions`, `TestComposeAuthSessions_live`); Flutter UI — [client.md](client.md).
- [ ] **[Auth] gRPC token context via `lastAccessToken` atomic** — `enable2FA`, `verify2FA`, `putE2EKeyBackup`, `getE2EKeyBackup`, `convertGuest` rely on in-process `lastAccessToken` when metadata missing; unsafe for concurrent direct gRPC. File: `src/backend/auth/src/main/java/voice/backend/auth/grpc/AuthGrpcService.java`.
- [ ] **[Auth] `auth-service.md` doc drift** — Missing/incorrect vs code: `SwitchActiveProfile`, `SetAccountStatus`, `ResolvePhoneHashes`, OAuth2 (developer-portal + admin), `backup_codes`, `last_online_at`, `linked_identities`; E2E migration cited as `V4` but Flyway uses `V4__e2e_key_backups.sql` + golang `000005`. File: `docs/microservices/auth-service.md`. *(Partial overlap with TODO.md “convert-guest doc auth-service.md” — that item is narrower.)*
- [ ] **[Auth] `src/backend/auth/README.md` migration section stale** — Still says Flyway “single migration V1”; repo has `V1`–`V5` and golang `000001`–`000006`. File: `src/backend/auth/README.md`.

### Realtime


- [ ] **[Realtime] Doc metrics vs implementation** — `realtime-service.md` lists `realtime.events.delivered`, `realtime.events.fanout_latency`, `realtime.reconnects`; `metrics.go` exposes only connections, connect counters, hello histogram, NATS lag. (`docs/features/observability.md` documents the implemented set — drift between service doc and observability spec.)
- [ ] **[Realtime] Phantom membership operations in service doc** — documented server ops `member_add` / `member_remove` are not emitted; `chat_events_consumer.go` canonically maps membership changes to `chat_update` with `change`. Remove the phantom ops or introduce them only with a documented client migration; the remaining implemented message/Voice operations are now listed.
- [ ] **[Client/Role] `role_update` is delivered but Flutter does not consume it — define the authoritative refetch target and implement client invalidation/refetch for `role.chat_override_set` / `role.chat_override_removed`; until then WS delivery does not change a rendered permission state.** — `src/backend/realtime/role_events_consumer.go`, `src/frontend/lib/`, `docs/microservices/realtime-service.md`
- [ ] **[Realtime] Six separate NATS connections per instance** — `main.go` opens one connection per consumer + lag poller (no shared `*nats.Conn`), increasing reconnect churn and FD usage at scale.
- [ ] **[Realtime] Test gaps for newer paths** — No subscription/fan-out integration test for `role_events_consumer.go`; mapping-only coverage exists for `role.chat_override_removed`. No integration tests for `matchmaking_events_consumer.go` or `user_presence_updater_grpc.go`. Cross-instance `message_delivered` is covered by `TestRedisDeliveryAckFanoutCrossInstance`; friend WS presence is covered by `TestComposePresenceDNDInvisible_live`.

### Multi-Profile

- [ ] **[Multi-Profile] No create/delete profile rate limits** — anti-abuse spec in `docs/features/multi-profile.md`; no throttling in `CreateProfile` / `DeleteProfile` (`src/backend/user/internal/grpcsvc/user.go`).
- [ ] **[Multi-Profile] Premium vanity `@username` (no `#1234`) not implemented** — all profiles get 4-digit discriminator (`src/backend/user/internal/store/profile.go`); monetization in `docs/features/multi-profile.md`.
- [ ] **[Multi-Profile] Additional phone per profile not implemented** — spec: доп. номер на профиль (не основной); only account `accounts.phone` → primary profile (`PhoneHashResolver`, `auth.proto` S2S).
- [ ] **[Multi-Profile] Transfer contact between profiles not implemented** — spec §контакты: перевести контакт в нужный профиль после phone-add; depends on Contacts RPCs ([Social] Contacts RPCs).
- [ ] **[Multi-Profile] Per-profile notification policy incomplete** — push tokens per `profile_id` (`notification/.../device_tokens.go`); `PermissivePolicyLoader` default-open; inactive-profile DND not enforced end-to-end (`multi-profile.md` §статусы и уведомления).

## Low

### Subscription


- [x] **[Subscription] README status** — describes the implemented core without overstating the product stub (`src/backend/subscription/README.md`).
- [ ] **[Subscription] Default webhook secret in prod path — `test-webhook-secret` if `PADDLE_WEBHOOK_SECRET` unset** — `src/backend/subscription/internal/billing/paddle.go`
- [ ] **[Subscription] Duplicate `DELETE` in `ActivatePremium`** — `src/backend/subscription/internal/store/store.go` (lines 95–99)
- [ ] **[Subscription] E2E / test gaps — no compose/live CloudPayments, provider-side cancel, personal grace→downstream notification/freeze, Space Pro failed-payment grace, or billing history scenario.** Personal grace expiry has service integration coverage; Space Pro webhook→join live is `TestComposeSpaceProMemberCap_live` (#14). — `src/frontend/test/billing_e2e_live_test.dart`; `src/backend/gateway/compose_billing_live_test.go`; `src/backend/subscription/internal/grpcsvc/lifecycle_integration_test.go`
- [ ] **[Subscription] Premium cosmetic enforcement gaps outside Subscription module — profile custom status has no Premium tier gate; anonymous view tracked separately in `docs/todo/backend.md`** — `src/backend/user/internal/grpcsvc/user.go`; `docs/todo/backend.md` (Anonymous view)
- [ ] **[Subscription] Doc/constant drift — free space join 100 vs 50; free voice 360p vs 480p in different docs; not unified in limits** — `docs/features/subscription.md`; `docs/microservices/subscription-service.md`; `src/backend/subscription/internal/testfixtures/limits.go`

### File


- [x] **[File] Proto lifecycle enum** — additive `FILE_LIFECYCLE_STATUS_EXPIRED = 6`; `expired` DB status maps to `EXPIRED`, while `deleted` remains `DELETED` (`protos/voice/file/v1/file.proto`, `src/backend/file/internal/grpcsvc/file_grpc.go`).
- [ ] **[File] No `sha256_hash` index/unique** — dedup would need schema work (`d:\Git\Voice\src\backend\migrations\file_db\000001_init.up.sql`).
- [ ] **[File] `story_id` column unused in access rules** — stored (`d:\Git\Voice\src\backend\migrations\file_db\000003_story_context.up.sql`) but `ensureFileAccess` only checks uploader or chat member (`d:\Git\Voice\src\backend\file\internal\grpcsvc\file_grpc.go` L538–554).

### Protos/Pkg


- [ ] **[Protos/Pkg] Package naming friction** — Voice service proto at `protos/voice/calls/v1/calls.proto` (`package voice.calls.v1`, `service VoiceService`); intentional per comment, but mismatches mental model “Voice Service → voice.proto”.
- [ ] **[Protos/Pkg] `buf` deps not pinned** — `protos/buf.yaml` notes future `buf.mod.yaml`/`buf.lock` for googleapis; today only `google/protobuf/timestamp.proto` imports; blocks formal `google.rpc.Status` error model extension mentioned in `docs/ARCHITECTURE_REQUIREMENTS.md`.
- [ ] **[Protos/Pkg] Realtime/WebSocket contract outside buf** — per `docs/REPOSITORIES.md`; WS `s`/`resume` payload lives in service docs/code only, not breaking-checked protobuf.
- [ ] **[Protos/Pkg] `pkg/analyticsevents`** — `src/backend/pkg/analyticsevents/publisher.go` uses `analyticsv1.AnalyticsEvent` protobuf for `analytics.*` telemetry (good), separate from domain `jetstream_events.proto` streams; two parallel event layers to keep in sync manually.
- [ ] **[Protos/Pkg] Reserved-field breaking hygiene present but narrow** — `reserved` only in `protos/voice/messaging/v1/messaging.proto`, `file.proto`, `notification.proto` (`chat_type`, `mute_until_rfc3339`); other domains lack reserved tags for removed fields.

### Space


- [ ] **[Space] `SearchPublicSpaces` on Space proto duplicates Search service catalog (`/api/v1/search/spaces`) — dead RPC on Space** — `protos/voice/space/v1/space.proto`, `src/backend/search/internal/grpcsvc/search.go`
- [ ] **[Space] Space templates (Gaming/Work/Social) — proto + `spaces.md`, zero implementation** — `protos/voice/space/v1/space.proto`, `docs/features/spaces.md`
- [ ] **[Space] Member `nickname` in schema, no update RPC** — `src/backend/migrations/space_db/000001_init.up.sql`, `protos/voice/space/v1/space.proto`
- [ ] **[Space] QR join — product doc only, no Space API** — `docs/features/spaces.md`
- [ ] **[Space] Space-level `mm_config` for matchmaking — column exists, unused** — `src/backend/migrations/space_db/000001_init.up.sql`
- [ ] **[Docs/Space/Chat/Role/Messaging] Freeze Space-chat guest access decision contract** — the Space admission slice now uses `000016_allow_guests_fail_closed` to default/backfill `spaces.allow_guests=false`; an owner or `SPACE_MANAGE_SETTINGS` administrator can opt in through `UpdateSpace.allow_guests`, and guest `JoinByInvite` atomically requires a live invite plus that opt-in. The authoritative Space-chat contract remains open: define the protected RPC/event projection for `is_guest && space.allow_guests && chat.allow_guests && space_membership && TEXT_CHAT_VIEW`, caller principals, request/response fields, fail-closed dependency behavior, disable semantics for existing guest members, and enforcement in Space tree, Chat list/get, Messaging send/history/read state, and Realtime delivery. Until accepted, the completed standalone Chat R3-A15 slice does not enable Space-chat guest access and Chat keeps its Space-chat mutation disabled. — [spaces.md](../features/spaces.md), [text-chat.md](../features/text-chat.md), [chat-service.md](../microservices/chat-service.md).

### Moderation


- [ ] **[Moderation] `GetAutoModStats` semantics weak** — counts `auto_mod_log` rows, not messages scanned; `CheckMessage` does not increment checked counter.
- [ ] **[Moderation] Spam mute action taxonomy mismatch** — logs `mute` / `mute_permanent` actions; docs/model use `mute` / `shadow_ban`; Messaging only blocks when pattern re-matches.
- [x] **[Moderation] Appeal review metadata in proto** — `reviewed_at` and nullable `review_notes` round-trip through Review/Get/List responses; pending appeals omit both fields. **T-035**.
- [ ] **[Moderation] Limited unit coverage** — `automod_unit_test.go` only link-flood + threshold math; no unit tests for sanctions/appeals handlers (integration tests only).
- [ ] **[Moderation] Federation moderation** — documented in `moderation-service.md`, not implemented (federation deferred per PLAN).

### User


- [ ] **[User] Guest audience in User service is implemented for presence** — `show_online` / `show_game_status` + `include_guests` tested (`src/backend/user/internal/grpcsvc/privacy_integration_test.go`); `show_mm_rating` / `show_stories` enforced in Matchmaking/Story, not User (by design per `docs/features/privacy.md` enforcement path). Flutter per-field `include_guests` — waves A–J (2026-07-15).

#### Multi-Profile — audit (2026-07-15)

Спека: [multi-profile.md](../features/multi-profile.md). PLAN: **partial** (User, Auth). Аудит кода + сверка с TODO — ниже только открытое.

**Связанные пункты в других секциях (не дублировать):** [Subscription] JWT `subscription_tier` stuck `free` (лимит 5 профилей); Downgrade lifecycle + `ProfileDowngradePickerScreen`; NATS `user.profile_switched` gaps; [Search] `ProfileSwitched` not indexed; [Cross-cutting] premium → 3rd profile E2E; [Social] contacts REST / phone-sync. `EnsurePrimaryProfile` **есть**.

**Уже в коде (не заводить повторно):** `CreateProfile` + preset + `accent_color` + privacy seed; `ListMyProfiles` / `GET /api/v1/users/profiles`; `POST /api/v1/auth/switch-profile`; soft-delete `DeleteProfile` (gRPC); `ApplyDowngradeProfiles` + `frozen_at`; desktop `ProfileSwitcher` + mobile `ProfileAvatarSwitcher`; `profile_context_controller` (WS reconnect, MM cancel, space exit); accent theme + migration; voice `voiceBindingProfileId` + conflict dialog; account-level blocks; friend/chat isolation live tests (`compose_profile_isolation_live_test`, `profiles_verification_e2e_live_test`).

### Analytics


- [ ] **[Analytics] Prometheus names vs spec** — Code: `analytics_ingest_*`, `analytics_clickhouse_insert_latency_seconds` (`d:\Git\Voice\src\backend\analytics\internal\metrics\metrics.go`); docs: `analytics.ingest.events_per_second`, `analytics.ingest.batch_size` (`docs/microservices/analytics-service.md`). No `events_per_second` or `batch_size` histogram.
- [ ] **[Analytics] gRPC ingest lag metric contract gap (blocks parity)** — Docs name `analytics.ingest.lag_seconds` (`docs/microservices/analytics-service.md`) and set a pipeline SLO from NATS event to ClickHouse availability (`docs/ARCHITECTURE_REQUIREMENTS.md`), but do not define this metric's measurement instant, timestamp authority, validity policy (missing, future, or skewed timestamp), histogram shape, or labels/cardinality. Current NATS code observes `time.Since(event.timestamp)` only when non-negative at buffer append; that implementation is not a documented gRPC contract. Product/observability decision needed before TDD parity work: define the metric semantics for direct ack-less gRPC ingest and whether it must remain comparable with durable NATS ingest, then cover `IngestEvent` and `IngestBatch` without changing their external API.
- [ ] **[Analytics] `DeliverNew` only** — New durable consumers skip backlog (`d:\Git\Voice\src\backend\analytics\internal\consumer\runner.go`); acceptable per spec but limits replay/backfill.
- [x] **[Analytics] Export CSV includes hashed IDs** — CSV exports include existing `user_id_hashed` / `profile_id_hashed` columns while JSON remains unchanged (`src/backend/analytics/internal/grpcsvc/query.go`).
- [ ] **[Analytics] User-level activity gap** — `message_sent` hashes `profile_id` only; `user_id_hashed` empty → DAU/retention undercount messengers (`d:\Git\Voice\src\backend\analytics\internal\adapters\domain.go`).
- [ ] **[Analytics] Gateway health telemetry off by default** — `GATEWAY_ANALYTICS_SAMPLE_RATE` default 0 (`d:\Git\Voice\src\backend\gateway\analytics_telemetry.go`); health dashboard needs explicit enablement.
- [ ] **[Analytics] CH schema doc naming** — Docs use `user_id`/`profile_id`; DDL uses `user_id_hashed`/`profile_id_hashed` (`docs/microservices/analytics-service.md` vs `d:\Git\Voice\docker\clickhouse\init\001_events.sql`).

### Role


- [x] **[Role] Server comment** — `RoleGRPC` now identifies the implemented service without stale red-phase wording. — `src/backend/role/internal/grpcsvc/server.go`
- [x] **[Role] README status** — describes the implemented role and permission surface. — `src/backend/role/README.md`
- [x] **[Role] `CreateRole` hierarchy validation — non-owner with `SPACE_MANAGE_ROLES` may create only below their top role; equal/higher denial leaves no role or event, Owner bypass is covered.** — `src/backend/role/internal/grpcsvc/roles.go`, `roles_manage_integration_test.go`
- [ ] **[Role] Guest role under-exercised — default join falls back to Member; Guest mask (`SPACE_VIEW` only) rarely applies unless `SetDefaultJoinRole` points to Guest.** — `src/backend/role/permissions/permissions.go`, `internal/store/roles.go`

### Cross-cutting


- [x] **[Cross-cutting] `subscription/README.md` status** — implemented billing/limit paths and the remaining product-stub gaps are explicit. — `src/backend/subscription/README.md`, `docs/PLAN.md`
- [ ] **[Cross-cutting] Analytics “partial” = server-side only by design — client RUM explicitly out of scope (`docs/features/analytics.md`); not a bug, but PLAN “partial” is architectural, not a missing backend slice.** — `docs/features/analytics.md`, `src/backend/analytics/`
- [ ] **[Cross-cutting] Notifications partial — push device creds / staging FCM already in TODO Critical/High; in-app + Realtime fan-out exist (`realtime/in_app_notification_fanout_test.go`).** — `docs/PLAN.md`, `src/backend/realtime/`

### Notification


- [x] **[Notification] `platform_enum` precedence and fallback** — `RegisterDevice` prefers recognized enum values, uses canonical legacy fallback for present unspecified/unknown enum values, and preserves legacy string-only behavior. — `protos/voice/notification/v1/notification.proto`, `src/backend/notification/internal/grpcsvc/server.go`
- [ ] **[Notification] Unauthenticated debug push recorder** — `/debug/recorded-pushes` exposes last recorded push by `profile_id` (compose/dev aid). — `src/backend/notification/debug_http.go`
- [x] **[Notification] gRPC server comment** — identifies the implemented service without stale stub wording. — `src/backend/notification/internal/grpcsvc/server.go`

### Federation


- [ ] **[Federation] Accidental Gateway REST proxy** — `src/backend/gateway/config_test.go` allows `federation` in `GATEWAY_REST_UPSTREAMS_JSON`, but `routing_test.go` blocks public paths; low risk unless someone adds transcoding routes without review.
- [ ] **[Federation] Generated-only Flutter surface** — `src/frontend/lib/gen/voice/s2s/v1/*`; no `lib/` product code for federation.
- [x] **[Federation] Repo junk in service dir** — removed `$prof` and `coverage` from git; added to `.gitignore`.
- [ ] **[Federation] No buf/gateway public API** — S2S protos correctly isolated under `protos/voice/s2s/v1/`; no accidental client exposure via REST transcoding.

### Voice


- [ ] **[Voice] Redis key layout differs from `voice-service.md` model** — docs describe `voice:session:{profile_id}` object + room sets; code uses `voice:session:{profile_id}` → `room_id` pointer + JSON blob `voice:call:{room_id}`.
- [ ] **[A2 Voice] Active-session handoff, RTC reconnect and authoritative cleanup have no contract** — `voice-service.md` defines only the invariant «один активный voice на профиль»; it does not identify a device/session principal, say whether a second device is rejected or replaces the first, define LiveKit disconnect/heartbeat/TTL reconciliation, or prescribe retry/idempotency and event/roster ordering for leave. The current serial store behavior rejects a second active room for the same `profile_id` and frees it after `RemoveParticipant` (characterized by `TestCallStore_oneActiveVoicePerProfileUntilLeave`), while `RedisCallStore.CreateCall`/`AddParticipant` perform a read-then-write active-session check without a documented atomic cross-device protocol. Do not add a reconnect or forced-disconnect implementation/test until the owner freezes the device binding, source of truth, revocation and cleanup semantics. Sources: `docs/microservices/voice-service.md`; `docs/features/voice-chat.md`; `src/backend/voice/internal/store/{call_store.go,redis_store.go}`.
- [ ] **[Voice] LiveKit DM/group JWT minimal grants** — Space voice-room JWTs now carry explicit `video.canPublish` from `VOICE_SPEAK`; DM/group JWTs still contain only `video.roomJoin` + `room` and no explicit `canPublish` / `canSubscribe` (works in compose media test, but less explicit than LiveKit best practice).
- [ ] **[Voice] Commander / raise-hand client surface is proto-only** — generated Dart gRPC stubs exist; no `lib/` product usage beyond `lib/gen/`.

### Auth


- [ ] **[Auth] IP logging for audit not implemented** — `docs/microservices/auth-service.md` § “IP logging”; `HttpAccessLogFilter` logs method/path/status only, no client IP. Files: `src/backend/auth/src/main/java/voice/backend/auth/web/HttpAccessLogFilter.java`, `docs/microservices/auth-service.md`.
- [ ] **[Auth] Internal gRPC has no caller authorization** — `ResolvePhoneHashes`, `SetAccountStatus` callable by any mesh peer without S2S auth. Files: `src/backend/auth/src/main/java/voice/backend/auth/grpc/AuthGrpcService.java`, `src/backend/moderation/internal/authclient/authclient.go`.
- [ ] **[Auth] Compose dev 2FA bypass enabled** — `AUTH_TOTP_TEST_BYPASS: "true"` in `docker-compose.yml`; acceptable for local E2E but must not leak to staging/prod manifests (staging yaml omits it — OK).

### Realtime


- [ ] **[Realtime] Unknown inbound ops silently dropped** — `ws.go` `default` branch ignores unrecognized client ops (no `error` frame).
- [ ] **[Realtime] Server does not emit WebSocket ping frames** — liveness is client `heartbeat` + 90s read deadline (`ws.go`); doc mentions “ping-pong” but implementation is app-level heartbeat only.
- [ ] **[Realtime] `CheckOrigin` always true** — `ws.go` delegates origin policy to Gateway (documented inline); defense-in-depth relies entirely on edge.

**Промпт-якорь:** Full product audit Batch 14 from docs/todo/backend.md.

---


### Multi-Profile

- [ ] **[Multi-Profile] `SwitchProfile` User RPC vs Auth session split** — User RPC returns profile + optional NATS; real session switch is Auth only; naming/docs drift (`transcode_profiles_verification.go`).
- [ ] **[Multi-Profile] Dual space membership same account not E2E-tested** — `space_members.profile_id` supports two profiles in one space; no compose/live test (friend/chat isolation covered).
- [ ] **[Multi-Profile] Voice-on-switch not E2E-tested** — `voiceBindingProfileId` + `call_error_listener` conflict dialog implemented; no live test for switch during active voice (`multi-profile.md` §войс).
- [ ] **[Multi-Profile] E2E scope narrow** — `profiles_verification_e2e_live_test` covers create+switch only; missing delete, frozen switch denial, downgrade picker, premium 3rd profile (см. [Cross-cutting] partial-feature E2E).
### Story

- [ ] **`story.lfp_created` → Matchmaking subscriber** — auto-application from LFP story (deferred per [story-service.md](../microservices/story-service.md)).
- [ ] **Feed space-member prefilter** — bulk space co-member author list (сейчас friends + self only).
- [ ] **Anonymous view (Premium)** — backend `MarkViewed.anonymous`; client UX отложен.
- [ ] **Compose expiry full chain live test** — worker → archive → purge → `DeleteFile` с `STORY_TTL_DEV` в compose.

**Промпт-якорь:** `Backend audit from docs/todo/backend.md` + сервис и приоритет.

## R23 accepted docs / open source

- [ ] **[Cross-cutting] Implement the accepted P3 Space deletion and Role retirement contract** — canonical docs fix U1-A/U2-A/U3-B/U4-B/U5-A, `K=P90D` and `B=P30D`; source still needs proto/generated ownership, Auth/Space/Role/participant migrations and workers, File reference/capability authority, RED fixtures, activation gates and end-to-end convergence evidence. This open item must not be read as shipped status.
