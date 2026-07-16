# TODO — Backend

[← Индекс](../TODO.md)

Микросервисы, Gateway (backend), protos, NATS, compose live verification.

Аудит микросервисов (Go/Java), Gateway backend, protos/pkg, NATS. Источник: product audit 2026-07-14; дубликаты roadmap — только ссылка.

## Critical

### Subscription


- [ ] **[Subscription] Checkout is a stub — no Paddle Billing API; returns `checkout.paddle.test` URLs; no real purchase path** — `src/backend/subscription/internal/grpcsvc/subscription.go` (`CreateCheckoutSession`, `CreateSpaceCheckoutSession`)
- [ ] **[Subscription] CloudPayments not implemented — СНГ provider entirely missing** — `src/backend/subscription/internal/grpcsvc/subscription.go` (`HandleCloudPaymentsWebhook` → `Unimplemented`); no `internal/billing/cloudpayments.go`
- [ ] **[Subscription] JWT `subscription_tier` stuck at `free` — Auth uses in-memory resolver; gateway forwards JWT tier to User (not Subscription Service). Premium webhook updates DB but profile/GIF/banner limits still see `free`. File upload works only because gateway overrides tier for File** — `src/backend/auth/src/main/java/voice/backend/auth/config/AuthBeans.java`; `src/backend/auth/src/main/java/voice/backend/auth/service/InMemorySubscriptionTierStore.java`; `src/backend/gateway/auth.go`; `src/backend/gateway/subscription_tier.go` (File-only override); `src/backend/user/internal/grpcsvc/user.go`, `user_avatar.go`
- [ ] **[Subscription] Space Pro purchase does not affect Space/Voice — webhook writes `subscription_db.space_subscriptions`; Space reads `space_db.space_subscriptions`; no sync/NATS consumer in prod (tests use `SeedSpaceProActive`)** — `src/backend/subscription/internal/store/store.go` (`ActivateSpacePro`); `src/backend/migrations/space_db/000005_space_subscriptions.up.sql`; `src/backend/space/internal/store/invite.go` (`memberCapTx`); `src/backend/space/internal/grpcsvc/space.go` (`SeedSpaceProActive`)
- [ ] **[Subscription] Voice Space Pro cap never applied in prod — `SpacePro` lookup not wired in `main.go`; room cap stays 32** — `src/backend/voice/main.go`; `src/backend/voice/internal/grpcsvc/voice_room.go`; `src/backend/voice/internal/grpcsvc/subscription_voice_limits_test.go` (mock only)

### File


- [x] **[File] No server-side SHA-256 verification** — `ConfirmUpload` stores client hash without reading R2 and recomputing; upload integrity is trust-on-client (`d:\Git\Voice\src\backend\file\internal\grpcsvc\file_grpc.go`, `d:\Git\Voice\src\backend\file\internal\store\files_store.go`).
- [x] **[File] Retention not enforced** — spec: 90d free / forever paid; only E2E uploads get `expires_at` (`d:\Git\Voice\src\backend\file\internal\grpcsvc\file_grpc.go` L163–167). No cron/worker, no `MarkExpired`, no R2 purge, no expired placeholder path (`d:\Git\Voice\src\backend\file\internal\store\files_store.go` has no expiry ops).
- [x] **[File] `DeleteFile` is DB-only** — marks `deleted`, never deletes R2 objects (original, `converted_r2_key`, `thumbnail_r2_key`) (`d:\Git\Voice\src\backend\file\internal\grpcsvc\file_grpc.go` L382–400; `d:\Git\Voice\src\backend\file\internal\r2file\r2file.go` has no `DeleteObject`).
- [x] **[File] Download serves original, not processed asset** — `GetFileURL` always presigns `row.R2Key`, ignoring `converted_r2_key` despite spec “original not stored; user downloads processed version” (`d:\Git\Voice\src\backend\file\internal\grpcsvc\file_grpc.go` L236–237; `d:\Git\Voice\docs\features\file-storage.md`).

### Protos/Pkg


- [ ] **[Protos/Pkg] Split NATS wire format vs `jetstream_events.proto`** — `protos/voice/events/v1/jetstream_events.proto` defines protobuf envelopes, but publishers diverge:
- [ ] **[Protos/Pkg] Corrupt invite event payload** — `src/backend/space/internal/spaceevents/jetstream.go` `PublishInviteCreated` publishes `ChatStreamEvent_SpaceCreated` with `invite_code` written into `owner_profile_id` on subject `space.invite_created`. No matching oneof in `jetstream_events.proto`.
- [ ] **[Protos/Pkg] Social stream proto with zero publisher** — `SocialStreamEvent` in `protos/voice/events/v1/jetstream_events.proto`, but `src/backend/social/` has no JetStream publisher. `docs/MICROSERVICES.md` lists Social as publisher of `social.events` → Analytics/Notification/Chat/Federation.

### Space


- [ ] **[Space] 8 proto RPCs have no handlers — runtime `Unimplemented`: `DeleteSpace`, `SearchPublicSpaces`, `JoinSpace`, `LeaveSpace`, `TransferOwnership`, `ListTemplates`, `CreateFromTemplate`, `GetAuditLog`** — `protos/voice/space/v1/space.proto`; handlers only in `src/backend/space/internal/grpcsvc/{space,invites,members,moderation,tree,bot_member,co_members}.go`
- [ ] **[Space] Public-space join impossible — `spaces.md` requires free join for `public`; only `JoinByInvite` exists** — `src/backend/space/internal/grpcsvc/invites.go` (no `JoinSpace`)
- [ ] **[Space] Space Pro cache never synced — `space_db.space_subscriptions` comment says “synced from Subscription”; only test seed `UpsertSpaceSubscription` writes; Subscription writes `subscription_db` only** — `src/backend/migrations/space_db/000005_space_subscriptions.up.sql`, `src/backend/space/internal/store/entitlement.go`, `src/backend/subscription/internal/store/store.go`
- [ ] **[Space] `entry_requirement` never enforced — DB column + proto field exist; join path ignores phone/captcha/questions/manual** — `src/backend/migrations/space_db/000001_init.up.sql`, `src/backend/space/internal/store/invite.go`, `src/backend/space/internal/grpcsvc/invites.go`
- [ ] **[Space] Social block check on join missing — spec dependency “Social — проверка блокировок при join” not called** — `docs/microservices/space-service.md`; `src/backend/space/internal/grpcsvc/invites.go`, `src/backend/space/main.go` (Social wired only for invite privacy friends)

### Moderation


- [ ] **[Moderation] Auto report threshold does not enforce shadow ban** — `maybeAutoShadowBan` only writes `auto_mod_log`; never inserts `shadow_ban` sanction, so `IsShadowBanned` stays false after threshold. Contradicts `docs/features/reports.md` / `docs/microservices/moderation-service.md`; enforced by test expecting zero sanctions.
- [ ] **[Moderation] `moderation.events` not published** — NATS wired only for `analytics.moderation.*`; no `moderation.report_created`, `sanction_applied`, `appeal_*`, `auto_action` per `docs/microservices/moderation-service.md` / `docs/CONTRACT_MATRIX.md`. Notification has no moderation consumer.
- [ ] **[Moderation] Appeals not exposed to users** — no Gateway HTTP for `SubmitAppeal`; Flutter `VoiceModerationClient` only has `createReport`; no appeal UI under `src/frontend/lib/ui/`. Product requires profile “Account” appeal form (`docs/features/reports.md`).
- [ ] **[Moderation] `SubmitAppeal` stores profile ID as account ID** — uses `profileIDFromMetadata` (`x-voice-profile-id`) for `appellant_account_id`; Gateway sends account as `x-voice-user-id`. Breaks ownership checks and Auth sync on approval.

### Social


- [ ] **[Social] No `social.events` JetStream publisher** — spec in `docs/microservices/social-service.md` (`social.friend_request`, `social.friend_accepted`, `social.friend_removed`, `social.user_blocked`, `social.contact_added`, `social.contacts_synced`); zero NATS code under `src/backend/social/`. Breaks `docs/CONTRACT_MATRIX.md` publisher contract; Notification has `friend_request` type (`src/backend/notification/internal/delivery/types.go`) but no `social.events` consumer.
- [ ] **[Social] Privacy fail-open when User S2S absent** — `ensureFriendRequestPrivacy` / `ensurePhoneSearchPrivacy` return `nil` if `Privacy` / `PhoneSearchPrivacy` is nil (`src/backend/social/internal/grpcsvc/social_friends.go`, `phone_contacts.go`); `main.go` only wires User client when `USER_GRPC_ADDR` is set. Misconfig bypasses `allow_friend_requests` and `allow_phone_search`.
- [ ] **[Social] `SendFriendInvitation` ignores account blocks** — no `IsBlocked` check in `src/backend/social/internal/grpcsvc/social_friends.go`; blocked parties can still send/see pending invites while DM is gated elsewhere (`src/backend/chat/internal/grpcsvc/chat_dm.go`).

### User


- [ ] **[User] `EnsurePrimaryProfile` gRPC not implemented** — contract in `docs/microservices/user-service.md` / `protos/voice/user/v1/user.proto`; Auth bootstraps via direct JDBC instead (`src/backend/auth/src/main/java/voice/backend/auth/userdb/JdbcPrimaryProfileProvisioner.java`). User service has no handler (`src/backend/user/internal/grpcsvc/user.go` missing; stub in `src/backend/user/pb/voice/user/v1/user_grpc.pb.go`).
- [ ] **[User] OAuth verification bypasses User Service** — Twitch link writes `profiles` via `JdbcUserVerificationSync` (`src/backend/auth/src/main/java/voice/backend/auth/userdb/JdbcUserVerificationSync.java`), not `SetVerification` (`src/backend/user/internal/grpcsvc/user_verification.go`). No `user.verified` NATS publish on OAuth path (`src/backend/user/internal/userevents/jetstream.go`).

### Analytics


- [ ] **[Analytics] Raw PII in ClickHouse `properties`** — Subscription direct telemetry writes plaintext `account_id` into `properties_json` instead of HMAC-hashed `user_id_hashed` (`d:\Git\Voice\src\backend\subscription\internal\grpcsvc\subscription.go`). Violates `docs/features/analytics.md` § privacy.
- [ ] **[Analytics] Event loss on ClickHouse failure / crash** — NATS messages are consumed and acked before durable CH write; failed flushes only re-queue in process memory (`d:\Git\Voice\src\backend\analytics\internal\consumer\runner.go`, `d:\Git\Voice\src\backend\analytics\internal\buffer\accumulator.go`). Process restart after a failed flush drops data permanently.

### Matchmaking


- [ ] **[Matchmaking] `pending_accept` never expires** — Sweeper only expires `status = 'searching'` (`timeout/sweeper.go`, `store/sessions.go::ListSearchingExpired`). After `mm.match_found`, sessions move to `pending_accept` with a unique “one active per profile” index (`migrations/matchmaking_db/000004_matches.up.sql`). Ghosting the accept popup blocks `StartSearch` indefinitely; `CancelSearch` also rejects non-`searching` sessions (`grpcsvc/search.go`).
- [ ] **[Matchmaking] Role-required 10-stack matching is broken** — Seeded modes use `slots: 10` with `roles_required: true` (`migrations/matchmaking_db/000001_init.up.sql`), but `criteria.Compatible` requires **identical** `self.role` (`criteria/criteria.go::rolesCompatible`, asserted in `criteria/compat_test.go`). A full Dota/Valorant lobby cannot form with distinct roles.
- [ ] **[Matchmaking] Moderation `mm_ban` not enforced** — Moderation defines `mm_ban` sanctions (`moderation/internal/grpcsvc/sanctions.go`, `moderation/internal/store/sanctions.go`), but Matchmaking only checks peer `mm_bans` at match time (`matcher/worker.go`, `store/bans.go`). Platform MM bans do not block `StartSearch` or matching.

### Role


- [ ] **[Role] Voice Service never wires Role Service — `Roles` is nil in prod; `VOICE_JOIN`, `VOICE_SPEAK`, `VOICE_MUTE_OTHERS`, etc. are not enforced on join/speak/mute. Only `EnsureScreenShare` exists and is unused without a Role client.** — `src/backend/voice/main.go`, `src/backend/voice/internal/grpcsvc/role_guard.go`, `src/backend/voice/internal/grpcsvc/voice_grpc.go` (`JoinCall`)
- [ ] **[Role] Chat send overrides are API-only — `TEXT_CHAT_SEND_MESSAGES` deny via `chat_overrides` is computed in Role Service but Messaging `SendMessage` never calls `CheckPermission` / `HasChatPermission` for send; E2E only probes `/api/v1/roles/check`.** — `src/backend/messaging/internal/grpcsvc/messaging_grpc.go`, `src/frontend/test/custom_roles_e2e_live_test.dart`

### Cross-cutting


- [ ] **[Cross-cutting] JWT `subscription_tier` never syncs from billing — production Auth bean is `InMemorySubscriptionTierStore` (comment: “optional NATS-backed sync” but no consumer). After Paddle webhook, `/subscription/me` and Gateway file path see premium, but JWT still `free` until re-login — and re-login still won’t update tier. Breaks `DATA_MODEL.md` (“source of truth — Subscription”).** — `src/backend/auth/src/main/java/voice/backend/auth/config/AuthBeans.java`, `.../InMemorySubscriptionTierStore.java`, `.../AuthService.java`; consumers: `src/backend/user/internal/grpcsvc/user.go`, `src/backend/gateway/auth.go`
- [ ] **[Cross-cutting] Space Pro entitlement duplicated, not synced — webhook writes `subscription_db.space_subscriptions` (`subscription/internal/grpcsvc/subscription.go`); Space enforces caps from `space_db.space_subscriptions` (`space/internal/store/entitlement.go`). No S2S/event sync on `subscription.activated` / `space_pro`. Live Space Pro billing does not raise member cap.** — `src/backend/migrations/subscription_db/000001_init.up.sql`, `src/backend/migrations/space_db/000005_space_subscriptions.up.sql`, `src/backend/space/internal/store/entitlement.go`
- [ ] **[Cross-cutting] `subscription.events` bus missing — `docs/CONTRACT_MATRIX.md` / `docs/MICROSERVICES.md` list stream with subscribers Analytics, User, Space, File; code only publishes `analytics.subscription.*` from Subscription. Blocks cross-service tier/limit propagation.** — `src/backend/subscription/internal/grpcsvc/subscription.go` (`publishPaymentEvent`), `docs/CONTRACT_MATRIX.md`
- [ ] **[Cross-cutting] Web JWT in WS query string (no ticket) — documented security follow-up not implemented; access tokens can hit proxy/CDN logs on web reconnect.** — `docs/ARCHITECTURE_REQUIREMENTS.md` (§ WS auth), `src/frontend/lib/backend/realtime_client.dart`, Gateway WS proxy

### Messaging


- [ ] **[Messaging] `GetMessage` RPC not implemented** — proto + S2S callers exist, handler missing (`UnimplementedMessagingServiceServer` only). Breaks Search reindex body fetch and Notification push preview.
- [ ] **[Messaging] `ForwardMessage` bypasses channel/thread send policy** — does not call `threadPolicyDeps().validateSend`; forwards land in main feed without `posted_as_chat` even when channel requires it.
- [ ] **[Messaging] E2E forward policy gap** — forwards copy `content` + attachments but never set `is_e2e`, never run `validateE2ESend` on target chat; E2E ciphertext can appear as plaintext forward in non-E2E chats.

### Search


- [ ] **[Search] `ReindexChat` indexes E2E message bodies — live indexer skips `IsE2E` events, but reindex pages `GetMessages` and upserts all rows with no E2E filter; ciphertext becomes searchable.** — `src/backend/search/internal/indexer/reindex_chat.go`, `src/backend/search/internal/deps/deps.go` (`ListChatMessages`)
- [ ] **[Search] `SearchGlobal` `matched_chats` leaks chat titles — `SearchChats` scans all `chat_search_documents`; results are not intersected with `AccessibleChatIDs` (only message hits are scoped). Non-members can discover group/DM titles by substring.** — `src/backend/search/internal/grpcsvc/search.go`, `src/backend/search/internal/grpcsvc/store_adapters.go` (`ProjectionChatAccess.SearchChats`), `src/backend/search/internal/store/profile_space_search.go` (`SearchChats`)

### Chat


- [ ] **[Chat] Group min-size invariant not enforced on kick/leave** — `AddGroupMembers` enforces `MinGroupMembers = 3` (`src/backend/chat/internal/store/group.go`), but `RemoveGroupMember` / `LeaveGroupChat` have no post-delete count check (`src/backend/chat/internal/store/group.go`, `src/backend/chat/internal/grpcsvc/group.go`). A group can end with 1–2 members, contradicting `docs/features/text-chat.md`.
- [ ] **[Chat] E2E pre-key gate bypass when Messaging is unset** — `setChatE2E` only calls `E2EPreKeyGate` when wired (`src/backend/chat/internal/grpcsvc/chat_e2e.go`); `main.go` wires it only if `MESSAGING_GRPC_ADDR` is set. Misconfigured deploy enables E2E without bundle verification (`docs/microservices/messaging-service.md`).

### Notification


- [ ] **[Notification] K8s FCM env names don’t match code — service reads `FCM_CREDENTIALS_JSON`; deploy injects `FCM_PROJECT_ID` + `FCM_SERVICE_ACCOUNT_JSON`. With secrets filled, sender still stays noop.** — `src/backend/notification/internal/fcm/config.go`, `deploy/staging/services.yaml`, `deploy/prod/services.yaml`, `deploy/staging/secret.example.yaml`
- [ ] **[Notification] K8s deployment omits all `APNS_*` env vars — only FCM keys are mounted; APNs/VoIP always noop in cluster even if secrets exist.** — `deploy/staging/services.yaml`, `deploy/prod/services.yaml`
- [ ] **[Notification] APNs secret key name mismatch — secrets use `APNS_PRIVATE_KEY`; code reads `APNS_AUTH_KEY` / `APNS_AUTH_KEY_PATH`.** — `deploy/staging/secret.example.yaml`, `src/backend/notification/internal/apns/http_sender.go`

### Voice


- [ ] **[Voice] Space voice rooms: no membership enforcement in runtime** — `ensureSpaceMember` is a no-op when `SpaceMembers == nil`; `main.go` never wires `SpaceMembers` despite `SPACE_GRPC_ADDR` in configmap/compose. Any profile with `voice_room_id` + `space_id` can join.
- [ ] **[Voice] `LeaveCall` ends the whole session for group voice** — aliases to `EndCall` (sets `CALL_STATUS_ENDED` + `call_ended` for all). One leaver kills the group call for everyone; should use `RemoveParticipant` like space rooms.

### Auth


- [ ] **[Auth] TOTP at-rest encryption uses hardcoded dev key when unset** — `TotpService.resolveKey()` falls back to `DEFAULT_DEV_KEY` if `auth.totp.encryption-key` is blank; staging `deploy/staging/services.yaml` and `deploy/staging/secret.example.yaml` have no `AUTH_TOTP_ENCRYPTION_KEY`. Files: `src/backend/auth/src/main/java/voice/backend/auth/service/TotpService.java`, `deploy/staging/services.yaml`, `deploy/staging/secret.example.yaml`.
- [ ] **[Auth] DeleteAccount / RestoreAccount / VerifyOTP in proto but unimplemented** — RPCs declared in `protos/voice/auth/v1/auth.proto` and `docs/microservices/auth-service.md`; no handlers in `AuthGrpcService`, no logic in `AuthService`, no REST. GDPR soft-delete / 30-day grace (`docs/features/auth-and-contacts.md`) blocked. Files: `protos/voice/auth/v1/auth.proto`, `src/backend/auth/src/main/java/voice/backend/auth/grpc/AuthGrpcService.java`, `src/backend/auth/src/main/java/voice/backend/auth/service/AuthService.java`.


## High

### Subscription


- [ ] **[Subscription] Cancel / Resume unimplemented — gRPC returns `Unimplemented`; Flutter cancel UI calls missing REST route** — `src/backend/subscription/internal/grpcsvc/subscription.go`; `src/frontend/lib/backend/subscription_client.dart` (`/subscription/cancel`); `src/frontend/lib/ui/settings/subscription_settings_screen.dart`; `src/backend/gateway/transcode_subscription.go` (no `cancel` handler)
- [ ] **[Subscription] `subscription.events` NATS stream not published — spec events (`plan_started`, `plan_expired`, `downgrade`, …) absent; only `analytics.subscription.*` telemetry** — `docs/microservices/subscription-service.md`; `docs/CONTRACT_MATRIX.md`; `src/backend/subscription/internal/grpcsvc/subscription.go` (`publishPaymentEvent`); `src/backend/subscription/main.go`
- [ ] **[Subscription] Grace period never expires — `grace_period_end` set on failed payment; no job/webhook path to `cancelled`/`expired` after 7 days** — `src/backend/subscription/internal/store/store.go` (`MarkPaymentFailed`); no sweeper in `src/backend/subscription/main.go`
- [ ] **[Subscription] Paddle webhook lifecycle incomplete — only `subscription.activated` + `subscription.payment_failed`; no renew, cancel, pause, or period-end handling** — `src/backend/subscription/internal/grpcsvc/subscription.go` (`HandlePaddleWebhook`)
- [ ] **[Subscription] Downgrade not driven by subscription lifecycle — `ApplyDowngradeProfiles` exists but nothing emits `subscription.downgrade` or triggers picker on expiry** — `src/backend/subscription/internal/grpcsvc/subscription.go`; `src/backend/subscription/internal/grpcsvc/user_profile_downgrade.go`; `src/frontend/lib/ui/profile/profile_downgrade_picker_screen.dart`
- [ ] **[Subscription] Gateway: no CloudPayments webhook route** — `src/backend/gateway/transcode_subscription.go` (only `webhooks/paddle`)

### File


- [ ] **[File] SHA-256 deduplication missing** — no hash lookup, no reuse of existing R2 key; `file_references` table absent (spec model in `d:\Git\Voice\docs\microservices\file-service.md`; only `files` in `d:\Git\Voice\src\backend\migrations\file_db\000001_init.up.sql`). Acknowledged in `d:\Git\Voice\src\backend\file\README.md`.
- [ ] **[File] NATS `file.events` not implemented** — no publisher for `file.uploaded`, `file.processed`, `file.scan_infected`, `file.downloaded` (`d:\Git\Voice\src\backend\file\`; contract in `d:\Git\Voice\docs\CONTRACT_MATRIX.md`, `d:\Git\Voice\docs\microservices\file-service.md`). `file.expired` implemented in `internal/fileevents`. Blocks Messaging “preview update after conversion”.
- [ ] **[File] No async worker / `processing` status** — conversion runs inline in `ConfirmUpload`; `processing` never set (`d:\Git\Voice\src\backend\file\internal\grpcsvc\file_grpc.go`; `d:\Git\Voice\docs\microservices\file-service.md` pipeline).
- [x] **[File] Originals kept after image processing** — processed keys written, source `r2_key` not removed (`d:\Git\Voice\src\backend\file\internal\imgproc\webp.go`; contradicts `d:\Git\Voice\docs\features\file-storage.md`).
- [ ] **[File] `CheckQuota` ignores premium** — always returns `r2file.MaxFreeFileBytes` as limit (`d:\Git\Voice\src\backend\file\internal\grpcsvc\file_grpc.go` L449–454); README says subscription quotas beyond free tier are out of scope.
- [ ] **[File] Gateway REST gaps** — no transcoding for `ListFiles` / `CheckQuota` (`d:\Git\Voice\src\backend\gateway\transcode_files.go`); proto RPCs exist but no HTTP surface.
- [ ] **[File] Infected-file notification missing** — scan marks `failed`/`infected` but no NATS/Notification fan-out (`d:\Git\Voice\docs\microservices\file-service.md`).

### Protos/Pkg


- [ ] **[Protos/Pkg] Analytics consumer gap vs MICROSERVICES matrix** — `src/backend/analytics/internal/consumer/runner.go` subscribes to 8 domain streams only; missing `social.events`, `role.events`, `file.events`, `subscription.events`, `moderation.events`, `federation.events` despite publisher/subscriber table in `docs/MICROSERVICES.md`.
- [ ] **[Protos/Pkg] Space event catalog drift** — `docs/microservices/space-service.md` documents `space.member_joined/left`, `space.updated/deleted`, `space.member_banned`, etc.; `jetstream_events.proto` `ChatStreamEvent` has only `ChatCreated`, `ChatMemberChanged`, `SpaceTreeChanged`, `SpaceCreated`; most space subjects are unimplemented in code (grep shows no `space.member_joined` publishers).
- [ ] **[Protos/Pkg] Go `pb/` codegen sync asymmetry** — `scripts/dev/sync-pb-from-gen.sh` syncs 7 trees (`analytics`, `chat`, `file`, `messaging`, `role`, `user`, `voice`); 10+ packages hub under `src/backend/voice/pb/voice/` and `src/backend/user/pb/voice/`. No CI drift check (unlike `make buf-dart-check` for `src/frontend/lib/gen/`). Stale committed stubs possible after proto edits.
- [ ] **[Protos/Pkg] `pkg/` resilience gap** — `docs/MICROSERVICES.md` requires circuit breaker on all gRPC calls; `src/backend/pkg/grpcclient/` only provides `dial.go` (`DialTarget`) and `wait.go`. No breaker/retry/mTLS helpers.
- [ ] **[Protos/Pkg] `pkg/` auth metadata fragmentation** — Gateway contract in `src/backend/gateway/transcode_grpc.go` (`x-voice-user-id`, `x-voice-profile-id`, …), but 12+ per-service `internal/authctx/` copies (`src/backend/*/internal/authctx/`). Only partial shared helpers: `src/backend/pkg/guestguard/`, `src/backend/pkg/correlation/`, `src/backend/pkg/jwt/` (edge validation, not inbound gRPC claim parsing).

### Space


- [ ] **[Space] `GetAuditLog` unimplemented while `audit_log` is written for ban/kick/timeout only** — `src/backend/migrations/space_db/000004_moderation.up.sql`, `src/backend/space/internal/store/{moderation,members}.go`
- [ ] **[Space] `UpdateSpace` ignores `visibility`, `entry_requirement`, `entry_questions_json`, `mm_config_json` from proto** — `src/backend/space/internal/grpcsvc/space.go`, `src/backend/space/internal/store/space.go`
- [ ] **[Space] `mm_config` / `entry_questions` never loaded — not in `SpaceRow`, not in `spaceRowToProto`** — `src/backend/space/internal/store/space.go`, `src/backend/space/internal/grpcsvc/proto.go`
- [ ] **[Space] Tree node Pro limit (500) not implemented — hardcoded `MaxTreeNodes = 50`, no entitlement check (unlike member cap)** — `src/backend/space/internal/store/tree.go`
- [ ] **[Space] Tree mutations owner-only, not Role permissions (`ManageChannels` etc.)** — `src/backend/space/internal/grpcsvc/tree.go` (`requireSpaceOwner`)
- [ ] **[Space] `ProfileAccounts` not wired in prod — `BanMember` account→profile eviction scan skipped when nil** — `src/backend/space/main.go`, `src/backend/space/internal/grpcsvc/moderation.go`
- [ ] **[Space] `ChatLookup` not wired in prod — `ListSpaceTree` text_chat `display_name` enrichment dead** — `src/backend/space/main.go`, `src/backend/space/internal/grpcsvc/chat_lookup.go`, `src/backend/space/internal/grpcsvc/tree.go`
- [ ] **[Space] NATS events incomplete vs spec — only `space.created`, tree, voice room, `invite_created`; missing `space.updated/deleted`, `member_joined/left`, `member_banned`** — `src/backend/space/internal/spaceevents/publisher.go`, `docs/microservices/space-service.md`
- [ ] **[Space] Catalog indexing fragile — Search hydrator calls `GetSpace` (member-only); no `space.updated` re-index on visibility change** — `src/backend/search/internal/deps/deps.go`, `src/backend/search/internal/indexer/chat_space_indexer.go`
- [ ] **[Space] Member timeout not enforced downstream — `IsProfileTimedOut` exists, unused outside Space** — `src/backend/space/internal/store/moderation.go`
- [ ] **[Space] Voice linkage gap (cross-service) — Space creates `voice_rooms` + tree nodes correctly, but Voice `JoinVoiceRoom` does not verify `voice_room_id ∈ space_id`; `SpaceMembers` guard not wired in Voice `main.go`** — `src/backend/space/internal/store/tree.go`, `src/backend/voice/internal/grpcsvc/voice_room.go`, `src/backend/voice/main.go`

### Moderation


- [ ] **[Moderation] `mm_ban` sanction is local-only** — type allowed in DB/handlers but `ApplySanction` never calls Matchmaking `BanFromMM` / `GetMMBanStatus`; MM bans remain peer-scoped in Matchmaking, not platform moderation.
- [ ] **[Moderation] Auto-moderation diverges from spec** — `CheckMessage` only detects ≥3 links; no repeated-message detection; no 1h timed mute (second spam hit → permanent block for that pattern only, no window); no “first 10 messages after mute” pass.
- [ ] **[Moderation] Report threshold audience is static env, not object audience** — `MODERATION_PLATFORM_AUDIENCE_SIZE` (default 1000) drives 1% calc; spec calls for relative threshold vs target’s audience.
- [ ] **[Moderation] Admin audit export is a stub** — always `{"entries":[]}`.
- [ ] **[Moderation] Appeals lack business rules** — no validation that appellant owns sanction; no 7-day submission window; no duplicate-appeal error mapping (DB `UNIQUE(sanction_id)` → 500).
- [ ] **[Moderation] Temp ban expiry does not restore Auth** — `expires_at` respected in SQL for active lookup, but no job/handler calls `Auth.SetAccountStatus(active)` on expiry; only explicit revoke/approved appeal clears suspension.
- [ ] **[Moderation] No sanction notifications** — Notification service has zero moderation integration despite `moderation.events` → Notification in contract matrix.

### Social


- [ ] **[Social] Contacts RPCs unimplemented** — `AddContact`, `RemoveContact`, `ListContacts` still default to `UnimplementedSocialServiceServer` (`src/backend/social/internal/grpcsvc/social_friends.go` embed); no handlers, no `contacts` table (`src/backend/migrations/social_db/000001_init.up.sql` only has `friendships` + `blocks`).
- [ ] **[Social] Favorites unimplemented** — `SetFavorite`, `ListFavorites` same unimplemented state; `friends.md` “Избранные” list has no backend.
- [ ] **[Social] `SyncPhoneContacts` does not create contacts** — `src/backend/social/internal/grpcsvc/phone_contacts.go` only filters Auth hash matches by privacy and returns IDs; no DB write, no `contact_added` / `contacts_synced` events. Conflicts with `docs/features/friends.md` (phone-book users become contacts).
- [ ] **[Social] Block does not cascade to graph** — `BlockAccount` in `src/backend/social/internal/store/blocks.go` inserts block row only; accepted friendships and pending/declined rows in `friendships` are untouched.
- [ ] **[Social] Outgoing request status not exposed** — store tracks `pending` vs `declined` (`src/backend/social/internal/store/friendships.go` `PendingFriendOutgoing.Status`), but `ListFriendRequests` maps only `profile_id` + `created_at` (`social_friends.go`); `protos/voice/social/v1/social.proto` `PendingFriendRequest` has no status field. Clients cannot tell declined from pending despite `friends.md`.
- [ ] **[Social] No test for `allow_friend_requests` enforcement** — privacy hook exists (`social_friends.go:356`) but no integration test (unlike phone sync in `src/backend/social/internal/grpcsvc/phone_search_privacy_integration_test.go`); no compose live test denying stranger invite.

### User


- [ ] **[User] `GetSettings` / `UpdateSettings` unimplemented** — proto + gateway surface exist; handler missing (`src/backend/user/pb/voice/user/v1/user_grpc.pb.go`, no impl in `src/backend/user/internal/grpcsvc/`).
- [ ] **[User] Premium animated GIF avatar is a dead path** — premium gate in `src/backend/user/internal/grpcsvc/user_avatar.go` but `image/gif` rejected by `src/backend/user/internal/r2avatar/validate.go` + `validate_test.go` (`TestValidateUploadParams_rejectsGifInPhase1`); conflicts with `docs/features/user-profile.md` and PLAN **shipped** user-profile.
- [ ] **[User] `banner_url` persisted but not exposed** — DB + `UpdateProfile` write (`src/backend/user/internal/store/profile.go`, `user.go`) but `rowToProto` omits `BannerUrl` (`src/backend/user/internal/grpcsvc/user.go`); proto has field (`protos/voice/user/v1/user.proto`).
- [ ] **[User] Verification V1 incomplete (Auth + User boundary)** — Twitch only in `LinkedAccountsService` (`src/backend/auth/src/main/java/voice/backend/auth/service/LinkedAccountsService.java`); YouTube in DB schema only (`src/backend/auth/src/main/resources/db/migration/V3__linked_identities.sql`); no partner-status recheck cron (`docs/features/verification.md`).
- [ ] **[User] NATS contract gaps** — missing `user.presence_changed`, `user.game_detected`, `user.settings_changed` (`docs/microservices/user-service.md`); `PublishProfileUpdated` / `PublishVerified` emit stub `ProfileCreated` without `changed_fields` / `verification_type`; `PublishProfileSwitched` drops `old_profile_id` (`src/backend/user/internal/userevents/jetstream.go`).
- [ ] **[User] Homoglyph-normalized search not implemented** — anti-spoof on create only (`src/backend/user/internal/store/verification.go`); `SearchProfilesAfter` uses raw `ILIKE` (`src/backend/user/internal/store/profile_search.go`); spec requires normalized lookup (`docs/features/verification.md`).
- [ ] **[User] Premium custom status not gated** — `UpdatePresence` accepts `custom_status` for all tiers (`src/backend/user/internal/grpcsvc/user_presence.go`); spec: Premium only (`docs/microservices/user-service.md`, `docs/features/user-profile.md`).
- [ ] **[User] `GetPrivacySettings` lacks ownership check** — any caller with `profile_id` can read settings (`src/backend/user/internal/grpcsvc/privacy.go`); S2S/gRPC exposure risk (Gateway uses `me/privacy` only — `src/backend/gateway/transcode_users.go`).

### Analytics


- [ ] **[Analytics] Health dashboard always empty for Gateway telemetry** — Query counts `gateway_request` (`d:\Git\Voice\src\backend\analytics\internal\store\query.go`); Gateway publishes `api_request` (`d:\Git\Voice\src\backend\gateway\analytics_telemetry.go`).
- [ ] **[Analytics] Retention windows wrong** — D1/D7/D30 SQL uses same-day windows (`timestamp < cohort_date + N`) instead of day+N return (`d:\Git\Voice\src\backend\analytics\internal\store\query.go`). Product retention KPIs will be misleading.
- [ ] **[Analytics] Product dashboard under-spec** — `product` returns `dau` as `uniqExact(user_id_hashed)` over the whole range (default 30d), not daily DAU; missing MAU/WAU/onboarding completion per `docs/features/analytics.md` / `docs/microservices/analytics-service.md` (`d:\Git\Voice\src\backend\analytics\internal\store\query.go`).
- [ ] **[Analytics] Grafana vs REST registration mismatch** — Grafana panel counts `profile_created` (`d:\Git\Voice\deploy\observability\grafana\dashboards\voice-analytics-product.json`); REST `product` dashboard counts `user_registered` (`d:\Git\Voice\src\backend\analytics\internal\store\query.go`).
- [ ] **[Analytics] DoD ingest path untested** — Live tests only check RBAC 200/403 and export HTTP 200; no assertion that `message.sent` → ClickHouse row within 60s (`d:\Git\Voice\src\backend\gateway\compose_analytics_live_test.go`, `d:\Git\Voice\src\backend\gateway\compose_analytics_export_live_test.go` vs `docs/features/analytics.md` DoD §1).
- [ ] **[Analytics] REST date range not wired** — Gateway transcoding never passes `from`/`to` query params to gRPC (`d:\Git\Voice\src\backend\gateway\transcode_analytics.go`); Admin client also omits them (`d:\Git\Voice\src\admin\src\api\analytics.ts`).
- [ ] **[Analytics] Silent no-op without ClickHouse** — `CLICKHOUSE_DSN` unset → service starts, ingest buffers, nothing persisted (`d:\Git\Voice\src\backend\analytics\main.go`); k8s secret refs are `optional: true` (`d:\Git\Voice\deploy\staging\services.yaml`, `d:\Git\Voice\deploy\prod\services.yaml`).
- [ ] **[Analytics] Weak prod hash-key guard** — Missing `ANALYTICS_ID_HASH_KEY` falls back to dev default (`d:\Git\Voice\src\backend\analytics\main.go`).

### Matchmaking


- [ ] **[Matchmaking] Decline semantics vs spec** — `handleMatchDecline` abandons the proposal and `ResetToSearching` + re-enqueues **all** participants (`grpcsvc/match.go`). Spec (`docs/features/matchmaking.md`): own-party decline resets own party; foreign-party decline lets acceptors continue; decliner should not silently keep searching. No cross-party decline test (`grpcsvc/match_test.go` only covers solo-duo decline).
- [ ] **[Matchmaking] Peer-ban check fail-open** — Matcher logs and continues on `IsPairBanned` DB errors (`matcher/worker.go`), allowing banned pairs to match under store outage.
- [ ] **[Matchmaking] MM rating privacy off in compose** — `main.go` wires `RatingPrivacy` only when `USER_GRPC_ADDR` / `SOCIAL_GRPC_ADDR` / `SPACE_GRPC_ADDR` are set; `docker-compose.yml` matchmaking service omits these (unlike k8s `envFrom` on `deploy/staging/configmap-app.yaml`). Local stack exposes ratings without privacy checks.
- [ ] **[Matchmaking] Match squad not ephemeral** — Squad creates a normal group chat + group voice (`squad/grpc_clients.go`). `CompleteMatch` only updates MM DB (`grpcsvc/rating.go`); no Chat/Voice teardown. Contradicts “auto-delete when all leave” (`docs/features/matchmaking.md`).
- [ ] **[Matchmaking] `UpdateGame` mutates catalog config for any caller** — Any authenticated user can change `config_json` (`grpcsvc/server.go`, `store/games.go`). Conflicts with user-game immutability (`docs/features/matchmaking.md`) and moderator-only catalog edits (`docs/features/game-catalog.md`).

### Role


- [ ] **[Role] Verification roles not implemented — auto roles on verification status (Steam, rank, etc.) per `docs/features/roles.md` / `docs/microservices/role-service.md`; no code in Role or User/Auth integration.** — `docs/features/roles.md` § «Верификационные роли»; `src/backend/role/` (absent)
- [ ] **[Role] Voice chat organizer role not implemented — no system/custom role, no permission bits, no Voice-side organizer powers (mute, floor, raise-hand).** — `docs/features/roles.md` § «Организатор войс-чата»; `src/backend/role/permissions/permissions.go`
- [ ] **[Role] Override targets not validated S2S — `SetChatOverride` / `SetVoiceRoomOverride` accept arbitrary UUIDs; doc dependency on Space/Chat validation is missing.** — `src/backend/role/internal/grpcsvc/roles.go`, `roles_manage.go`; `docs/microservices/role-service.md` § «Зависимости»
- [ ] **[Role] `MODERATION_MANAGE_REPORTS` unused — bit exists; Moderation service has no `CheckPermission` integration.** — `src/backend/role/permissions/permissions.go`; `src/backend/moderation/`
- [ ] **[Role] `SPACE_MANAGE_MATCHMAKING` unused — no Role checks in Matchmaking service.** — `src/backend/matchmaking/`
- [ ] **[Role] Many text-chat permission bits not enforced downstream — Messaging checks mentions, threads, pins only; not send/media/embed/files/reactions/slow-mode/manage-messages. Chat checks `TEXT_CHAT_VIEW`, slow-mode/settings only.** — `src/backend/messaging/internal/grpcsvc/messaging_grpc.go`, `threads_policy.go`; `src/backend/chat/internal/grpcsvc/roles.go`, `space_membership.go`
- [ ] **[Role] `SPACE_MANAGE_SETTINGS` bypassed — Space `UpdateSpace` is owner-only, not role-based; Admins with the flag cannot update space metadata.** — `src/backend/space/internal/grpcsvc/space.go`
- [ ] **[Role] Admin ≡ Owner on effective mask — `GetEffectiveMask` short-circuits Admin to `AllMask()`; Admin system role mask is also `all`. Doc algorithm step 5 («кроме Owner-specific») has no distinct owner bits, so Admin is functionally Owner for all 42 flags.** — `src/backend/role/internal/store/roles.go` (`GetEffectiveMask`), `permissions/permissions.go` (`SystemRoles`); `docs/microservices/role-service.md` § «Вычисление effective permissions»

### Bot


- [ ] **[Bot] Inbound chat message events → bot webhook/poll not implemented** — `docs/microservices/bot-service.md` describes `NATS: message in whitelisted chat → Bot Service`; code only **publishes** `bot.events` (`internal/botevents/jetstream.go`, wired in `main.go`), no consumer/subscriber anywhere under `src/backend/bot/`.
- [ ] **[Bot] Deferred follow-up uses wrong `ChatRef` type** — `lookupInteraction` always returns `CHAT_TYPE_CHANNEL` (`internal/grpcsvc/interaction.go`), breaking deferred `SendBotMessage` / `CompleteInteraction` for group (and DM) chats.
- [ ] **[Bot] Redis gRPC rate limiter fails open** — on Redis error, requests proceed unlimited (`internal/ratelimit/redis_limiter.go`); staging sets `BOT_REDIS_ADDR` in `deploy/staging/services.yaml`.
- [ ] **[Bot] Token / webhook-secret rotation does not invalidate active sessions** — `RegenerateToken` / `RegenerateWebhookSecret` only update DB (`internal/store/store.go`, `internal/grpcsvc/bot.go`); no hub deferred-token purge or polling invalidation per `docs/features/bots.md` §tokens.

### Cross-cutting


- [ ] **[Cross-cutting] Subscription lifecycle incomplete cross-service — `CancelSubscription`, `ResumeSubscription`, `HandleCloudPaymentsWebhook` → `Unimplemented`; `GetBillingHistory` empty; grace-period user notifications not wired to Notification. PLAN marks subscription partial.** — `src/backend/subscription/internal/grpcsvc/subscription.go`, `docs/features/subscription.md`
- [ ] **[Cross-cutting] `CheckLimit` unused outside Subscription — no runtime gRPC callers in Chat/Space/User/File for documented caps (profiles, space/chat counts, etc.). Enforcement is ad hoc: File via Gateway live `GetSubscription`, User via JWT tier, Chat has no subscription client.** — `src/backend/subscription/internal/grpcsvc/subscription.go`, `src/backend/gateway/subscription_tier.go`, `src/backend/user/internal/grpcsvc/user.go`
- [ ] **[Cross-cutting] Resilience claims vs code — `MICROSERVICES.md` promises circuit breakers + NATS DLQ; no `gobreaker`/DLQ in `src/backend/`. Tier-0 degradation is partial (Gateway file tier fallback only).** — `docs/MICROSERVICES.md`, `src/backend/` (absence)
- [ ] **[Cross-cutting] No E2E for Space Pro billing path — smoke/full cover personal premium + file limits (`compose_billing_live_test.go`, `billing_e2e_live_test.dart`); zero `space_pro` webhook → invite/member-cap tests.** — `src/backend/gateway/compose_billing_live_test.go`, `.github/ci/e2e-features.yml`
- [ ] **[Cross-cutting] E2E smoke skips core messaging cross-cut — tier-2 smoke omits `ws_resume`, `message_delivery`, `in_app_notifications` (full/nightly only). Two-layer delivery not gated on every master push.** — `.github/ci/e2e-features.yml`, `docs/TESTING.md`
- [ ] **[Cross-cutting] No device `integration_test` driver suite — live coverage is host `flutter test`; `src/frontend/integration_test/` remains aspirational. Mobile push/VoIP/deep-link acceptance not automatable as documented.** — `src/frontend/integration_test/README.md`, `docs/TESTING.md`
- [ ] **[Cross-cutting] Federation: staging/CI vs local compose — Federation scaffold in CI/staging (`deploy/staging/services.yaml`, `Makefile` `GO_SERVICES`); not in `docker-compose.yml` app profile. Deferred product but deployable — env parity gap.** — `deploy/staging/services.yaml`, `docker-compose.yml`, `src/backend/federation/main.go`
- [ ] **[Cross-cutting] Admin vs PLAN — `PLAN.md` lists Admin as “зарезервировано”; `src/admin/` ships moderation queue + product analytics pages with CI job. Cross-cutting staff product surface undocumented in PLAN.** — `docs/PLAN.md`, `src/admin/`

### Messaging


- [ ] **[Messaging] `message.forwarded` NATS event missing** — spec lists it; publisher/stream only has `message.sent` on forward.
- [ ] **[Messaging] `ForwardMessageRequest.commentary` ignored** — proto + Flutter client send it; server never creates commentary message.
- [ ] **[Messaging] “Copy as new message” / forward without attribution** — spec feature; no proto field or server path (always `type=forward` + attribution).
- [ ] **[Messaging] Forward-author privacy block not enforced** — spec says user can forbid forwarding their messages; no `allow_forward` in `privacy.md`, no check in `ForwardMessage`.
- [ ] **[Messaging] Group/channel view counts absent** — `text-chat.md` requires per-message view counter; no model/RPC beyond DM-style `read_receipts`.
- [ ] **[Messaging] `ForwardMessage` skips SendMessage guards** — no moderation/slow-mode, no `checkAttachmentPrivacyForSend`, no `checkDMBlocksForSend` / `checkDMPrivacyForSend` on target, no `validateAttachments`, `chat_type` defaults to `dm`.
- [ ] **[Messaging] Read-state APIs DM-typed only** — `MarkRead` / `GetReadState` / `GetBulkReadState` / `GetChatListMetadata` use `validateChatRefDM`; explicit `group`/`channel` refs rejected while `GetMessages` accepts all types.

### Search


- [ ] **[Search] Reverse-direction block not enforced — only viewer’s outgoing `ListBlocked` accounts excluded; User Service uses bidirectional `IsBlocked`. Per `docs/features/privacy.md`, a user blocked by someone can still find them in Search.** — `src/backend/search/internal/deps/deps.go` (`SocialBlocks`), `src/backend/search/internal/store/profile_space_search.go` (`SearchProfiles`)
- [ ] **[Search] JetStream `DeliverNew` → no historical backfill — consumers only index events after subscription; deploy/reset leaves `search_db` empty for past messages/profiles unless manual per-chat reindex.** — `src/backend/search/internal/indexer/consumer.go`
- [ ] **[Search] Index update failures silently acked — handler logs `search index update failed` but does not `Nak`; failed upserts are lost permanently.** — `src/backend/search/internal/indexer/consumer.go`
- [ ] **[Search] Chat/space projection staleness after create — indexer handles only `ChatCreated` / `SpaceCreated`; no handlers for group rename (`UpdateGroupChat`), space update (`UpdateSpace`), visibility change, or `SpaceTreeChanged`.** — `src/backend/search/internal/indexer/chat_space_indexer.go`; upstream: `src/backend/chat/internal/grpcsvc/group.go`, `src/backend/space/internal/grpcsvc/space.go`
- [ ] **[Search] `ReindexChat` not admin-gated — spec (`docs/microservices/search-service.md`) says admin; any authenticated profile with read access can trigger full chat backfill. No Gateway HTTP route.** — `src/backend/search/internal/grpcsvc/search.go` (`ReindexChat`); absent from `src/backend/gateway/transcode_search.go`

### Chat


- [ ] **[Chat] Folders API entirely unimplemented** — `ListFolders`, `CreateFolder`, `UpdateFolder`, `DeleteFolder` fall through to `UnimplementedChatServiceServer` (`src/backend/chat/internal/grpcsvc/chat.go` embeds default server). No `folders` / `folder_chats` migrations under `src/backend/migrations/chat_db/`. Conflicts with `docs/microservices/chat-service.md` responsibilities and `docs/features/navigation.md`.
- [ ] **[Chat] `MuteChat` / `ArchiveChat` unimplemented** — RPCs unimplemented (`src/backend/chat/pb/voice/chat/v1/chat_grpc.pb.go`); DB columns `muted_until` / `is_archived` exist (`000001_init.up.sql`) and `ListChats` filters archived rows (`src/backend/chat/internal/store/list_chats.go`), but nothing can set them. Blocks archive UX in `docs/features/text-chat.md` while PLAN marks text-chat **shipped**.
- [ ] **[Chat] `DeleteChat` unimplemented** — proto + gRPC handler exist; no `ChatGRPC.DeleteChat` (`src/backend/chat/internal/grpcsvc/`).
- [ ] **[Chat] `ListChats` omits channels** — SQL filters `c.type IN ('dm', 'group')` (`src/backend/chat/internal/store/list_chats.go:87,97`). Space channels are invisible in the main inbox despite `chat-service.md` listing Channels as a folder dimension.
- [ ] **[Chat] Group `last_message_at` never updated from message stream** — `message_activity_consumer.go` calls `TouchLastMessageAt`, which updates only `type = 'dm'` (`src/backend/chat/internal/store/dm.go:119`). Group rows in `ListChats` won’t reflect real activity ordering.
- [ ] **[Chat] `UpdateChat` ignores thread settings** — proto has `threads_enabled` / `allow_user_main_feed` (`chat.pb.go`); handler only passes name/avatar/slow_mode (`src/backend/chat/internal/grpcsvc/group.go`). Defaults tested (`thread_settings_integration_test.go`); runtime toggles impossible.
- [ ] **[Chat] `UpdateChat` rejects channels** — `row.Type != "group"` guard (`src/backend/chat/internal/grpcsvc/group.go:97`). Channel topic/slow-mode changes via Chat API blocked.
- [ ] **[Chat] Subscription S2S not integrated** — doc dependency (`docs/microservices/chat-service.md`); limit hardcoded `GroupMemberLimit = 500` (`src/backend/chat/internal/store/group.go`). No subscription-tier differentiation.
- [ ] **[Chat] Group admin role unused** — schema allows `owner|admin|member`; only `owner` may `RemoveMember` / `UpdateChat` (`src/backend/chat/internal/grpcsvc/group.go`). No code assigns `admin`. Conflicts with `docs/features/text-chat.md` admin powers.

### Notification


- [ ] **[Notification] `notification_settings` / `quiet_hours` tables exist but are unwired — gRPC returns hardcoded `enabled: true`; `SetQuietHours` no-op; runtime uses `PermissivePolicyLoader` (mute/DND never applied to push).** — `src/backend/migrations/notification_db/000001_init.up.sql`, `src/backend/notification/internal/grpcsvc/server.go`, `src/backend/notification/internal/delivery/policy.go`, `internal/store/` (only `device_tokens.go`)
- [ ] **[Notification] `friend_request` marked ✓ in feature table but no delivery path — no `social.events` consumer, no `FriendRequest` event in protos (only `FriendAdded`), Social doesn’t call Notification, no Realtime fanout; client expects `friend_request_id` in push data only.** — `docs/features/notifications.md`, `protos/voice/events/v1/jetstream_events.proto`, `src/backend/social/internal/grpcsvc/social_friends.go`, `src/frontend/lib/state/push_notification_handler.dart`
- [ ] **[Notification] `reply` marked ✓ but not implemented — no `reply` type in message consumer or Realtime in-app fanout; thread replies are treated as `new_message`.** — `docs/features/notifications.md`, `src/backend/notification/message_events_consumer.go`, `src/backend/realtime/in_app_notification_fanout.go`
- [ ] **[Notification] Matchmaking/voice push ignores presence — handlers hardcode `IsOnline: false`; no `EnrichDecision` / User gRPC check → online users still get push (messages path does check).** — `src/backend/notification/internal/consumer/matchmaking_events.go`, `src/backend/notification/matchmaking_events_consumer.go`, `src/backend/notification/voice_events_consumer.go`
- [ ] **[Notification] `system` notifications have no producer — `SendNotification` gRPC exists but no other service calls it; no NATS consumer; not exposed on Gateway REST.** — `src/backend/notification/internal/grpcsvc/server.go`, `src/backend/gateway/transcode_notifications.go`
- [ ] **[Notification] Multi-replica duplicate push risk — per-pod durable consumer name (`notif_<hostname>_msg** — mm

### Federation


- [ ] **[Federation] Hollow pod on every staging/prod deploy** — `voice-federation` is Tier-1 restart in `scripts/staging/rollout-app-tier.sh`; image built/pushed on every `master` push via `.github/workflows/ci.yml` (`staging-images-push`) and `scripts/ci/staging-image-catalog.json`. Burns CI/CD + cluster resources with no product surface.
- [ ] **[Federation] `federation_db` documented but never provisioned** — `docs/DATA_STORES.md`, `docs/microservices/federation-service.md` declare `federation_db`; absent from `docker/postgres/initdb.d/01-init-databases.sh`, `scripts/dev/compose-migrate-all.sh`, `src/backend/migrations/`, `deploy/templates/`.
- [ ] **[Federation] Prometheus scrape misconfigured** — `deploy/staging/services.yaml` / `deploy/prod/services.yaml` annotate `prometheus.io/path: "/metrics"` on `voice-federation`, but `src/backend/federation/health.go` exposes only `/health` → scrape 404s / noisy alerts.
- [ ] **[Federation] Spec ↔ proto drift (implementation trap)** — when work starts, docs and contracts disagree:
- [ ] **[Federation] `federation.events` contract is dead** — `docs/CONTRACT_MATRIX.md` lists Federation → Analytics/Role/Moderation; zero publishers/consumers in `src/backend/analytics/`, `src/backend/role/`, `src/backend/moderation/`, `src/backend/federation/`.

### Story


- [ ] **[Story] `show_stories = Nobody` global privacy bypass** — `canViewStory` skips the User privacy floor when `floor.IsNobody()`; `CreateStory` allows explicit `visibility: everyone` without capping to global setting. Path: `src/backend/story/internal/grpcsvc/story.go` (`canViewStory`, `CreateStory`, `storyPrivacyFloor`).
- [ ] **[Story] Anonymous view leaks viewer in NATS** — `MarkViewed` always calls `PublishStoryViewed` with `viewer_profile_id` even when `anonymous=true`; contradicts [stories.md](../features/stories.md) §Анонимный просмотр. Paths: `src/backend/story/internal/grpcsvc/story.go`, `src/backend/story/internal/storyevents/jetstream.go`.
- [ ] **[Story] No `media_file_id` ownership / story-context validation** — any UUID accepted; video duration checked only when File client is wired. Path: `src/backend/story/internal/grpcsvc/story.go` (`CreateStory`, `CreateLookingForParty`); File story context exists in `src/backend/file/internal/grpcsvc/file_grpc.go` but Story does not enforce it.
- [ ] **[Story] Feed degrades to global scan when Social fails** — `GetStoryFeed` falls back to `ListActiveStoriesPaginated` (all active rows) if `ListFeedAuthorIDs` errors; only post-filtered by `canViewStory`. Path: `src/backend/story/internal/grpcsvc/story.go` (`GetStoryFeed`); related: `src/backend/story/internal/privacy/friends.go`, `src/backend/gateway/compose_stories_degradation_live_test.go` (checks liveness only).
- [ ] **[Story] `DeleteStory` orphans R2 media** — soft-delete only; purge worker targets `expired_at IS NOT NULL`, so early-deleted stories never reach `RunArchivePurgeOnce` / `FileDeleter`. Paths: `src/backend/story/internal/store/store.go` (`DeleteStory`), `src/backend/story/internal/jobs/jobs.go`.
- [ ] **[Story] Moderation cannot hide stories from feeds** — reports accepted (`target_type: story` in Moderation/Gateway), but Story has no moderation flag/consumer; [stories.md](../features/stories.md) §Модерация expects hide/remove. Paths: `src/backend/story/` (no integration); `src/backend/moderation/internal/grpcsvc/reports.go`.
- [ ] **[Story] GET reactions REST missing** — gRPC `GetStoryReactions` exists; Gateway only maps `POST …/reactions`. Flutter `getStoryReactions` GET will 404. Paths: `src/backend/gateway/transcode_stories.go`; `src/frontend/lib/backend/stories_client.dart`.

### Voice


- [ ] **[Voice] Unimplemented gRPC (proto + gateway exposed, server returns `Unimplemented`)** — `SetCommanderMode`, `RaiseHand`, `LowerHand`, `MoveToVoiceRoom`. Required by `voice-chat.md` (commander, raise hand, room moves).
- [ ] **[Voice] S2S deps declared in spec but not wired in `main.go`** — no `Roles` (Role Service), `SpacePro` (Subscription), or `SpaceMembers` (Space Service). Effects:
- [ ] **[Voice] Missing NATS events vs `voice-service.md` / Analytics** — never published: `voice.call_started`, `voice.participant_joined`, `voice.participant_left`. Publisher surface stops at incoming/accepted/declined/missed/ended/state/screen-share. Analytics adapter expects `call_started`.
- [ ] **[Voice] Space voice join/leave publishes no roster events** — no `participant_joined` / `participant_left` / `voice.state_changed` on `JoinVoiceRoom` / `LeaveVoiceRoom`; Realtime consumer has no handlers for those subjects anyway.
- [ ] **[Voice] Staging LiveKit WebRTC likely broken without ops beyond WS smoke** — `deploy/staging/infra.yaml` sets `use_external_ip: true` but no `node_ip` (compose uses explicit `node_ip: 127.0.0.1` in `deploy/livekit/livekit.yaml`). Signaling is `wss://` via Ingress; RTC is NodePort **30881/TCP + 30882/UDP** on the node — not validated by `scripts/staging/smoke-staging.sh` (WS probe only).
- [ ] **[Voice] No LiveKit Server SDK room lifecycle** — docs say create/close rooms via SDK; implementation only mints JWT (`internal/livekit/token.go`). Rooms rely on implicit LiveKit auto-create; no explicit teardown.

### Auth


- [ ] **[Auth] OTP / password-reset flow missing end-to-end** — `otp_codes` DDL exists (`V1__auth_schema.sql`, `000001_init.up.sql`) but no send/verify service, no `/api/v1/auth/otp/*` in Auth; Gateway treats OTP routes as **public** and rate-limits them (`routing.go`, `ratelimit.go`) → 404 upstream. Files: `src/backend/auth/src/main/resources/db/migration/V1__auth_schema.sql`, `src/backend/gateway/routing.go`, `src/backend/gateway/ratelimit.go`.
- [ ] **[Auth] OTP Redis throttling not implemented in Auth** — `docs/ARCHITECTURE_REQUIREMENTS.md` assigns OTP attempt throttling to Auth Redis; only JWT blacklist is wired. Files: `src/backend/auth/src/main/java/voice/backend/auth/security/RedisTokenBlacklist.java`, `docs/ARCHITECTURE_REQUIREMENTS.md`.
- [ ] **[Auth] Resend email integration absent** — Documented dependency in `docs/microservices/auth-service.md` (verification, password reset); no Resend client or mail sender in `src/backend/auth/`.
- [ ] **[Auth] NATS `user.guest_converted` not wired in compose/staging** — Publisher only when `auth.nats.url` set (`AuthEventsConfiguration.java`); absent from `docker-compose.yml` auth env and `deploy/staging/services.yaml` → `NoopAuthEventPublisher` in default stacks. Files: `src/backend/auth/src/main/java/voice/backend/auth/config/AuthEventsConfiguration.java`, `docker-compose.yml`, `deploy/staging/services.yaml`. *(Related to TODO.md Batch 6 NATS test item, but env wiring is a separate gap.)*
- [ ] **[Auth] Linked-accounts list is a stub** — `GET /api/v1/auth/linked-accounts` returns `[]` in both Auth REST and Gateway transcoding; `linked_identities` table unused by Java. Files: `src/backend/auth/src/main/java/voice/backend/auth/rest/AuthRestController.java`, `src/backend/gateway/transcode_profiles_verification.go`, `src/backend/auth/src/main/resources/db/migration/V3__linked_identities.sql`.
- [ ] **[Auth] Twitch verification OAuth is mock-only** — `LinkedAccountsService.completeTwitchCallback()` accepts only `mock-code`, uses hardcoded token; link start returns static URL without client_id/state. Files: `src/backend/auth/src/main/java/voice/backend/auth/service/LinkedAccountsService.java`, `src/backend/auth/src/main/java/voice/backend/auth/rest/AuthRestController.java`.
- [ ] **[Auth] Password reset cannot work for convert-guest recovery** — Spec/TODO calls for self-service reset; no password-change or reset API exists. *(TODO.md Batch 6 “convert-guest recovery” covers product need; Auth implementation gap is new detail.)* Files: `src/backend/auth/src/main/java/voice/backend/auth/rest/AuthRestController.java`, `docs/features/auth-and-contacts.md`.

### Realtime


- [ ] **[Realtime] Subscription bootstrap limited to DM only** — `docs/microservices/realtime-service.md` requires auto-subscribe to all active chats, spaces, and friend presence; implementation only pages `ListChats` for `CHAT_TYPE_DM` in `dm_chat_lister_grpc.go` and registers those in `ws.go`. Groups/spaces rely on client `subscribe`; no friends-presence subscription model.
- [ ] **[Realtime] Live friend presence over WS is incomplete** — `presence_update` is fan-out to same-profile tabs and chat subscribers (`ws.go`, `ws_hub.go`). Friends not in a shared chat subscription do not receive live updates while WS is up (Flutter stops REST polling when connected — `src/frontend/lib/state/presence_providers.dart`). Proto has `PresenceChange` (`protos/voice/events/v1/jetstream_events.proto`) but User publisher has no presence subject (`user/internal/userevents/jetstream.go`) and Realtime has no `user_events` consumer.
- [ ] **[Realtime] In-app `notification` targets WS-subscribed profiles, not chat membership** — `in_app_notification_fanout.go` uses `hub.profileIDsSubscribedToChat(chatID)` as the recipient set. Connected group members who have not subscribed to that chat miss `notification` (and may miss `message_create` too).
- [ ] **[Realtime] `delivery_ack` has no Redis cross-instance fanout** — `ws.go` only calls local `hub.broadcastToProfile` for `message_delivered`. Unlike `mark_read` / typing / presence, there is no `redis_fanout.go` publish path; sender on another Realtime instance won't see delivery acks.
- [ ] **[Realtime] Redis connection registry is write-only** — `redis_registry.go` `Register`/`Unregister` are called from `ws.go` but never read for routing. Doc describes `{profile_id → [instance_id, conn_id]}` registry for multi-instance fanout (`realtime-service.md`); actual cross-instance path is Redis Pub/Sub + per-instance NATS durables only.
- [ ] **[Realtime] `role_events` consumer lacks JetStream boot retry** — `role_events_consumer.go` subscribes directly; other consumers use `subscribeJetStreamWithRetry` (`jetstream_subscribe.go`). Cold-start before `role_events` stream exists → goroutine exits permanently (`main.go` logs error, no restart).
- [ ] **[Realtime] Blocking send on call fanout can stall NATS handlers** — `ws_hub.go` `profileFanoutBlocks` uses blocking `reg.fanout <- env` (no timeout/drop) for `call_*` / screen-share ops; a full fanout buffer (32) can block the NATS consumer goroutine.
- [ ] **[Realtime] `REALTIME_INSTANCE_ID` missing in k8s** — Compose sets it (`docker-compose.yml`); `deploy/staging/services.yaml` / `deploy/prod/services.yaml` do not. Each pod restart generates a new UUID (`main.go`), leaving orphan JetStream durables (`rt_<id>_msg`, etc.) and breaking stable instance identity for ops/lag metrics.
- [ ] **[Realtime] Readiness does not probe Redis/NATS** — `health.go` always returns `ok`; pod can be Ready while Redis Pub/Sub or all NATS consumers are down (ephemeral + message fanout silently degraded).

### Multi-Profile

- [ ] **[Multi-Profile] Soft-deleted profiles still count toward limit** — `CountByAccountID` has no `deleted_at IS NULL` filter (`src/backend/user/internal/store/profile.go`); blocks re-create after delete per archive semantics in `docs/features/multi-profile.md`.
- [ ] **[Multi-Profile] Auth `switch-profile` bypasses User service** — `AuthService.switchActiveProfile` reissues JWT via `JdbcProfileSwitchValidator` only; does not call `User.SwitchProfile` → no `user.profile_switched` NATS on client path (downstream Search/analytics; см. [User] NATS gaps, [Search] ProfileSwitched).
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
- [ ] **[Subscription] Grace-period notifications missing — spec: reminders on days 1, 3, 7** — `docs/features/subscription.md`; no code in `src/backend/subscription/` or Notification integration
- [ ] **[Subscription] Activation ignores billing period / provider metadata — always `monthly`, synthetic `provider_subscription_id`** — `src/backend/subscription/internal/store/store.go` (`ActivatePremium`, `ActivateSpacePro`)
- [ ] **[Subscription] `billing_events.amount` / `currency` never written** — `src/backend/migrations/subscription_db/000001_init.up.sql`; `src/backend/subscription/internal/store/store.go` (`insertBillingEventTx`)
- [ ] **[Subscription] No Flutter Space Pro checkout / management** — `src/frontend/lib/backend/subscription_client.dart`; `src/frontend/lib/ui/settings/subscription_settings_screen.dart` (Premium only)

### File


- [ ] **[File] “WebP” conversion is JPEG bytes** — `encodeJPEG` with `.webp` key suffix and `Content-Type: image/jpeg` (`d:\Git\Voice\src\backend\file\internal\imgproc\webp.go`); spec requires WebP re-encode (`d:\Git\Voice\docs\features\file-storage.md`).
- [ ] **[File] Post-conversion size caps not enforced** — spec ≤5 MB images/GIF, ≤15 MB video, ≤10 MB docs; no post-process size check (`d:\Git\Voice\docs\features\file-storage.md`).
- [ ] **[File] Flutter drops thumb/converted keys** — `fileMetadataFromProto` never maps `thumbnail_r2_key` / `converted_r2_key` to `previewUrl` (`d:\Git\Voice\src\frontend\lib\backend\proto_mappers.dart` L452–462); unit test expects `previewUrl == null` (`d:\Git\Voice\src\frontend\test\files_client_test.dart` L122).
- [ ] **[File] Conflicting live test** — `file_image_thumb_e2e_live_test.dart` expects `previewUrl` non-empty (`d:\Git\Voice\src\frontend\test\file_image_thumb_e2e_live_test.dart` L55) while mapper never sets it.
- [ ] **[File] ClamAV E2E likely ineffective** — live test uses `eicar.com` + `text/plain` (`d:\Git\Voice\src\frontend\test\file_clamav_infected_e2e_live_test.dart`); `shouldScan` only matches `.exe`/`.zip`/`.bat` + zip/exe MIME (`d:\Git\Voice\src\backend\file\internal\grpcsvc\file_grpc.go` L588–596) — scan skipped, confirm may succeed.
- [ ] **[File] `ListFiles` chat filter unimplemented** — `filter_chat` → `FailedPrecondition` (`d:\Git\Voice\src\backend\file\internal\grpcsvc\file_grpc.go` L408–410).
- [x] **[File] Free-tier `expires_at` not set** — non-E2E free uploads have `expires_at = NULL`; retention cron has nothing to query (`d:\Git\Voice\src\backend\file\internal\grpcsvc\file_grpc.go` L163–167).
- [ ] **[File] No thumb URL presign helper** — clients get R2 keys in metadata but no `GetThumbnailURL` / presign for `thumbnail_r2_key`.
- [ ] **[File] Attachment lifecycle partial** — Messaging validates `ready` + chat link + scan (`d:\Git\Voice\src\backend\messaging\internal\grpcsvc\messaging_grpc.go` L312–359), but no `file_references`, no expiry placeholder UX (`d:\Git\Voice\docs\features\file-storage.md` “кучка костей”), no message preview refresh on `file.processed`.

### Protos/Pkg


- [ ] **[Protos/Pkg] Auth proto duplication without sync gate** — canonical `protos/voice/auth/v1/auth.proto` vs copy `src/backend/auth/src/main/proto/voice/auth/v1/auth.proto` (already diverges in comments); no CI compare step.
- [ ] **[Protos/Pkg] Federation protos orphaned from service** — `protos/voice/s2s/v1/s2s.proto`, `federation_management.proto` codegen to Flutter (`src/frontend/lib/gen/voice/s2s/`) and Go hubs, but `src/backend/federation/go.mod` depends only on `voice/backend/pkg` (scaffold; deferred per `docs/PLAN.md`).
- [ ] **[Protos/Pkg] `common.proto` under-specified** — `protos/voice/common/v1/common.proto` has pagination only; no shared idempotency/actor/ref types despite `docs/ARCHITECTURE_REQUIREMENTS.md` idempotency key and `messaging.proto` inline idempotency contract.
- [ ] **[Protos/Pkg] Analytics taxonomy vs proto** — `docs/MICROSERVICES.md` analytics examples include `file_downloaded`, `space_left`, `voice_room_created`, `message_forward`, notification push metrics; no corresponding arms in `jetstream_events.proto` and no publishers found.

### Space


- [ ] **[Space] No audit rows for tree CRUD, invite revoke, space settings, role changes (spec lists these)** — `src/backend/space/internal/store/tree.go`, `src/backend/space/internal/grpcsvc/invites.go`
- [ ] **[Space] `RevokeInvite` / `ListInvites` owner-only — `CreateInvite` uses `SpaceManageInvites`; revoke/list use `requireSpaceOwner`** — `src/backend/space/internal/grpcsvc/invites.go`
- [ ] **[Space] `JoinByInvite` does not publish `space.member_joined`** — `src/backend/space/internal/grpcsvc/invites.go`, `src/backend/space/internal/spaceevents/`
- [ ] **[Space] Kick/leave do not publish `space.member_left` or decrement path events** — `src/backend/space/internal/grpcsvc/members.go`, `src/backend/space/internal/store/members.go`
- [ ] **[Space] No gateway REST for leave/join-public/delete/transfer/audit/templates** — `src/backend/gateway/transcode_spaces.go`, `transcode_spaces_members.go`
- [ ] **[Space] Flutter client gaps — `spaces_client.dart` has no leave/join-public/transfer/audit/delete** — `src/frontend/lib/backend/spaces_client.dart`
- [ ] **[Space] Test holes — no integration tests for unimplemented RPCs; tree update/delete/category update/voice update/delete/RemoveTreeNode thin coverage** — `src/backend/space/internal/grpcsvc/*_integration_test.go`
- [ ] **[Space] Stale README still says “scaffold / out of scope”** — `src/backend/space/README.md`
- [ ] **[Space] `logInviteEventFailure` no-op — publish failures silently dropped** — `src/backend/space/internal/grpcsvc/invites.go`

### Moderation


- [ ] **[Moderation] Stale service README** — still claims “scaffold / out of scope”.
- [ ] **[Moderation] `ListReports` pagination incomplete** — proto has `next_cursor`; handler never sets it.
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
- [ ] **[Social] Flutter client surface incomplete** — `src/frontend/lib/backend/friends_client.dart` has `blockAccount` only; no `listBlocked`, `unblockAccount`, `syncPhoneContacts`, favorites. Gateway exposes blocks list + phone sync (`src/backend/gateway/transcode_friends.go`).
- [ ] **[Social] No live/E2E for friend-request privacy denial** — `privacy_actions_e2e_live_test` / `compose_privacy_actions_live_test.go` exercise DM/calls/files, not `POST /api/v1/friends/invitations`.
- [ ] **[Social] Stale service README** — `src/backend/social/README.md` still claims health-only scaffold; contradicts implemented gRPC + migrations.

### User


- [ ] **[User] `SearchProfiles` ignores discoverability privacy** — no `allow_friend_requests` / phone-search enforcement (`src/backend/user/internal/grpcsvc/user_search.go`); comment still references pre-privacy DDL.
- [ ] **[User] `UpdateProfile.custom_status` ignored** — comment "not persisted in v1 DDL" (`src/backend/user/internal/grpcsvc/user.go`); only Redis presence path works.
- [ ] **[User] Org DNS verification lifecycle thin** — unlimited pending rows, no expiry/TTL (`src/backend/user/internal/store/verification.go`).
- [ ] **[User] `README.md` stale** — claims "other RPCs still unimplemented" (`src/backend/user/README.md`).

### Analytics


- [ ] **[Analytics] Dashboard types missing** — Only `product`, `engagement`, `revenue`, `health`, `moderation` in query layer; no `search` / `voice` / `federation` despite spec tables (`d:\Git\Voice\src\backend\analytics\internal\store\query.go`, `docs/microservices/analytics-service.md`).
- [ ] **[Analytics] Funnel `onboarding` not implemented** — Only `registration` funnel (`d:\Git\Voice\src\backend\analytics\internal\store\query.go`); proto/admin reference registration only (`d:\Git\Voice\protos\voice\analytics\v1\analytics.proto`, `d:\Git\Voice\src\admin\src\pages\FunnelsPage.tsx`).
- [ ] **[Analytics] `GetMetrics` filters ignored** — Proto `filters` map never applied (`d:\Git\Voice\src\backend\analytics\internal\grpcsvc\query.go`, `d:\Git\Voice\protos\voice\analytics\v1\analytics.proto`).
- [ ] **[Analytics] MVs unused by query API** — `dau_mv` / `events_by_type_mv` created in DDL (`d:\Git\Voice\docker\clickhouse\init\001_events.sql`) and used in Grafana, but REST queries scan raw `voice.events` (`d:\Git\Voice\src\backend\analytics\internal\store\query.go`).
- [ ] **[Analytics] Thin test coverage** — No tests for `consumer`, `grpcsvc`, or `store/query`; integration tier only tests `InsertBatch` (`d:\Git\Voice\src\backend\analytics\internal\store\clickhouse_integration_test.go`). Existing unit tests: adapters, buffer, hash, health only.
- [ ] **[Analytics] Admin UI partial** — Product table + registration funnel + export only; no engagement/revenue/retention/search/voice pages (`d:\Git\Voice\src\admin\src\pages\`).
- [ ] **[Analytics] Grafana dashboards partial** — Only product, engagement, ingest (`d:\Git\Voice\deploy\observability\grafana\dashboards\`); missing revenue/health/moderation/search/voice panels from spec.
- [ ] **[Analytics] `role_events` stream not consumed** — Custom-role activity absent from adapters (`d:\Git\Voice\src\backend\analytics\internal\consumer\runner.go` vs `d:\Git\Voice\src\backend\role\internal\roleevents\jetstream.go`).
- [ ] **[Analytics] Engagement metrics shallow** — Voice “minutes”, MM sessions, active spaces, stories use coarse event counts or are absent (`d:\Git\Voice\src\backend\analytics\internal\store\query.go`, `d:\Git\Voice\src\backend\analytics\internal\adapters\domain.go`).
- [ ] **[Analytics] Export live test doesn’t verify audit log** — Comment claims audit path; test only checks HTTP 200 (`d:\Git\Voice\src\backend\gateway\compose_analytics_export_live_test.go` vs DoD §3 in `docs/features/analytics.md`).
- [ ] **[Analytics] Prod ClickHouse DDL manual** — Comment-only apply path vs staging Job automation (`d:\Git\Voice\deploy\prod\infra.yaml` vs `d:\Git\Voice\scripts\staging\apply-clickhouse-init.sh`).
- [ ] **[Analytics] Doc drift: Redis buffer** — `docs/PLAN.md` L84 and `docs/ARCHITECTURE_REQUIREMENTS.md` mention Redis buffer; implementation and `docs/DATA_STORES.md` say in-memory only (`d:\Git\Voice\src\backend\analytics\internal\buffer\accumulator.go`).
- [ ] **[Analytics] Stale service README** — Still describes scaffold/health-only (`d:\Git\Voice\src\backend\analytics\README.md`).

### Matchmaking


- [ ] **[Matchmaking] `GamesPlayed` is rating count, not match count** — `UpsertPlayerRating` increments `total_ratings_received` but API exposes it as `games_played` (`store/ratings.go`, `grpcsvc/rating.go`). Spec model separates `total_matches` vs `total_ratings_received` (`docs/microservices/matchmaking-service.md`).
- [ ] **[Matchmaking] Ratings cannot be skipped** — `validateStars` enforces 1–5 only (`store/ratings.go`). Spec allows skip per participant (`docs/features/matchmaking.md`).
- [ ] **[Matchmaking] `mm.player_banned` never published** — Stream subject registered (`mmevents/publisher.go`) but `BanFromMM` does not emit it (`grpcsvc/rating.go`).
- [ ] **[Matchmaking] Popular-games ordering missing** — `ListGames` sorts by `created_at DESC` (`store/games.go`). Spec wants popularity by active queue depth (`docs/features/game-catalog.md`).
- [ ] **[Matchmaking] Matcher scans ≤100 games** — `Worker.RunOnce` lists `PageSize: 100` active games once (`matcher/worker.go`). Additional catalog pages never polled.
- [ ] **[Matchmaking] `CreateGame` lacks `icon_url` / `external_id`** — Columns exist (`migrations/matchmaking_db/000001_init.up.sql`, `store/games.go`) but `CreateGame` only persists name+config (`grpcsvc/server.go`).
- [ ] **[Matchmaking] `mm.search_cancelled` ad-hoc JSON** — Not proto like other MM events (`mmevents/publisher.go` comment). Contract inconsistency for Notification subscribers.
- [ ] **[Matchmaking] Party / voice-derived MM absent** — `PartyStore` is a stub (`store/parties.go`); `StartSearch` always validates `partySize=1` (`grpcsvc/search.go`, `criteria/criteria.go`). Voice join/leave reset flow from spec not implementable yet.
- [ ] **[Matchmaking] Test gaps for prod-scale modes** — No matcher test for seeded 10-slot games or role-diversity matching (`matcher/worker_test.go` uses custom 2-slot Duo only).

### Role


- [ ] **[Role] `color`, `is_mentionable` in DB, not in API — columns in migration; absent from proto, store scans, REST responses.** — `src/backend/migrations/role_db/000001_init.up.sql`, `protos/voice/role/v1/role.proto`, `src/backend/role/internal/store/roles.go`, `grpcsvc/roles.go`
- [ ] **[Role] No live E2E for voice room overrides — store/grpc tests exist; no compose/Flutter E2E for `VOICE_JOIN` deny (UI strings exist).** — `src/backend/role/internal/store/roles_custom_integration_test.go`, `roles_manage_integration_test.go`; `src/frontend/lib/ui/space/space_tree_panel.dart`
- [ ] **[Role] Owner role lifecycle unguarded — Owner can assign Owner to others (`CanManageRole` owner bypass); no block on revoking last Owner.** — `src/backend/role/internal/store/roles.go` (`CanManageRole`, `AssignMemberRole`)
- [ ] **[Role] `ReorderRoles` skips hierarchy for system roles — managed roles reorder without `CanEditRole` / position vs actor checks.** — `src/backend/role/internal/grpcsvc/roles_manage.go` (`ReorderRoles`)
- [ ] **[Role] Override removal not published — `RemoveChatOverride` / `RemoveVoiceRoomOverride` emit no NATS events; Realtime consumer won't invalidate clients.** — `src/backend/role/internal/grpcsvc/roles_manage.go`, `internal/roleevents/publisher.go`, `src/backend/realtime/role_events_consumer.go`
- [ ] **[Role] `BootstrapSpaceRoles` floods events — publishes `role.created` for all 5 system roles on every bootstrap.** — `src/backend/role/internal/grpcsvc/roles.go`
- [ ] **[Role] S2S RPCs unauthenticated — `BootstrapSpaceRoles`, `CheckPermission`, `GetEffectivePermissions`, `DeleteRolesCreatedByProfile` trust network boundary (not exposed via Gateway, but no service auth).** — `src/backend/role/internal/grpcsvc/roles.go`, `roles_cleanup.go`, `main.go`
- [ ] **[Role] `created_at` wrong in API — `roleRowToProto` uses `timestamppb.Now()` instead of DB `created_at`.** — `src/backend/role/internal/grpcsvc/roles.go`
- [ ] **[Role] Federation role sync — listed in role-service deps; Federation deferred, no SyncSnapshot path.** — `docs/microservices/role-service.md`; `src/backend/federation/`

### Bot


- [ ] **[Bot] `ListInstalledBots` mislabels chat types** — all whitelist refs hardcoded to `CHAT_TYPE_CHANNEL` (`internal/grpcsvc/interaction.go`).
- [ ] **[Bot] `DeleteBot` is soft-disable only** — `status = 'disabled'` (`internal/grpcsvc/bot.go`); `bot_space_installations` / `bot_chat_whitelist` rows remain.
- [ ] **[Bot] `UpdateBot` ignores proto fields** — only name/description updated; `avatar_url`, `scopes_json` from `UpdateBotRequest` ignored (`internal/grpcsvc/bot.go`).
- [ ] **[Bot] Archive chat not implemented** — `TEXT_CHAT_CREATE_IN_SPACE` docs say create/**archive** (`docs/features/bots.md`); only `CreateBotChat` exists (`internal/grpcsvc/bot_c.go`).
- [ ] **[Bot] Manifest option types not validated** — allowed types (`string`, `integer`, `user`, `channel`, `role`, `attachment` in `docs/features/bots.md`) not checked in `internal/manifest/manifest.go`.
- [ ] **[Bot] Channel install skips `Chat.AddMembers`** — `InstallBotInSpace` `continue`s on channel refs (`internal/grpcsvc/interaction.go`); bot actor may not join channel chats.
- [ ] **[Bot] Autocomplete skips offline check** — `ExecuteSlashInteraction` gates on presence; `AutocompleteSlashOption` does not (`internal/grpcsvc/autocomplete.go` vs `interaction.go`).

### Cross-cutting


- [ ] **[Cross-cutting] Inconsistent entitlement resolution — Gateway live-calls Subscription only for File (`gateway/subscription_tier.go`); User/Chat trust JWT metadata. Premium UX fragmented after payment.** — `src/backend/gateway/subscription_tier.go`, `src/backend/user/internal/authctx/authctx.go`
- [ ] **[Cross-cutting] Compose infra version drift — local Postgres 16 / Redis 7 (`docker-compose.yml`) vs target Postgres 18 / Redis 8 (`docs/MICROSERVICES.md`). Staging/prod parity risk for migrations and Redis features.** — `docker-compose.yml`, `docs/MICROSERVICES.md`
- [ ] **[Cross-cutting] Partial-feature integration E2E missing — no cross-smoke for: premium → profile banner/GIF/3rd profile; premium → Story anonymous view; subscription grace → push/email; bot slash → in-app notification (only isolated feature tests).** — `.github/ci/e2e-features.yml`, `src/frontend/test/`
- [ ] **[Cross-cutting] `profiles_verification` / `encryption_dm` not in smoke — PLAN partial/shipped-opt-in; smoke has `encryption_key_backup` only, not DM encryption or verification flows.** — `.github/ci/e2e-features.yml`, `docs/PLAN.md`
- [ ] **[Cross-cutting] gRPC mTLS not wired — admitted in `docs/DEPLOYMENT.md`; `MICROSERVICES.md` security section still states mTLS between services. Staging relies on NetworkPolicy + `BOT_GRPC_GATEWAY_ONLY`.** — `docs/DEPLOYMENT.md`, `deploy/templates/network-policy-voice-bot.yaml`
- [ ] **[Cross-cutting] Distributed tracing absent — `MICROSERVICES.md` stack lists OTel+Jaeger; `docs/features/observability.md` defers tracing (v1 = `request_id` in Loki). Implementation gap between architecture table and v1 observability spec — staging debug relies on logs only.** — `docs/features/observability.md`, `deploy/observability/` (no Jaeger/OTel)

### Messaging


- [ ] **[Messaging] `ListThreads` pagination stub** — `ThreadList.next_cursor` in proto never populated; store has limit-only query.
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
- [ ] **[Search] No search query length limit — User `SearchProfiles` caps at 128 chars; Search gRPC accepts unbounded queries.** — `src/backend/search/internal/grpcsvc/search.go`
- [ ] **[Search] Privacy audience not enforced on profile discovery — `SearchProfiles` ignores `viewer`; no `privacy_settings` integration (User Service notes same v1 gap, but Search has its own projection).** — `src/backend/search/internal/store/profile_space_search.go`

### Chat


- [ ] **[Chat] `ListChats` returns partial `Chat` objects** — list query omits `e2e_enabled`, `space_id`, `slow_mode_seconds`, thread flags (`src/backend/chat/internal/store/list_chats.go` vs `chatRowToProto` in `src/backend/chat/internal/grpcsvc/chat_dm.go`). List UI can’t show E2E state without `GetChat`.
- [ ] **[Chat] NATS event surface incomplete vs doc** — published: `chat.created`, `chat.member_changed` (`src/backend/chat/internal/chatevents/jetstream.go`). Not published: `chat.updated`, `chat.deleted`, granular `member_added`/`removed`/`left` (`docs/microservices/chat-service.md` table).
- [ ] **[Chat] S2S enrichment fails open** — Messaging errors logged and zeroed (`src/backend/chat/internal/grpcsvc/list_chats.go:77-81`). Documented degradation, but no metric/alert on enrichment skip.
- [ ] **[Chat] No integration tests for unimplemented RPCs** — no tests for mute/archive/folders/delete (expected given stubs); no red tests documenting expected behavior.
- [ ] **[Chat] README stale** — `src/backend/chat/README.md` still claims “scaffold / health only”; contradicts full gRPC implementation.

### Notification


- [ ] **[Notification] Email (Resend) channel missing — spec lists auth-only email via Resend; zero code in Notification (Auth also has `otp_codes` DDL but no Resend sender).** — `docs/microservices/notification-service.md`, `src/backend/notification/` (no email package), `src/backend/auth/src/main/resources/db/migration/V1__auth_schema.sql`
- [ ] **[Notification] Redis rate limiting not implemented — spec mentions rate limiting; Redis used only for grouping.** — `docs/microservices/notification-service.md`, `src/backend/notification/internal/grouping/store.go`, `main.go`
- [ ] **[Notification] Analytics telemetry incomplete — only `analytics.notification.push_sent` on gRPC `SendNotification`; NATS-driven pushes don’t publish; no `push_delivered` / `push_clicked`.** — `src/backend/notification/internal/grpcsvc/server.go`, `docs/microservices/notification-service.md`
- [ ] **[Notification] `APNS_VOIP_TOPIC` in deploy unused — VoIP sender uses `APNS_BUNDLE_ID` as topic, not separate VoIP topic from secrets.** — `deploy/staging/secret.example.yaml`, `src/backend/notification/internal/apns/voip_sender.go`
- [ ] **[Notification] APNs E2E proves registration only, not delivery — unlike FCM compose test with `RecordSender` + debug endpoint.** — `src/frontend/test/apns_e2e_live_test.dart`, `src/frontend/test/fcm_delivery_e2e_live_test.dart`, `src/backend/notification/debug_http.go`
- [ ] **[Notification] No NATS/JetStream integration tests in Notification service — consumer wiring untested end-to-end at service boundary (Gateway has register-device live test only).** — `src/backend/notification/` (no `*_integration_test.go` for consumers), `src/backend/gateway/compose_notification_live_test.go`
- [ ] **[Notification] JetStream `DeliverNew()` on all consumers — restarts skip in-flight/backlog; at-least-once redelivery behavior not covered by explicit ack/nak handling.** — `src/backend/notification/*_events_consumer.go`

### Federation


- [ ] **[Federation] Not in local compose stack** — `docker-compose.yml` has no `federation` service; `GATEWAY_GRPC_UPSTREAMS_JSON` omits it (lines ~793). Contrast: `analytics` is wired. `Makefile` still lists `federation` in `GO_SERVICES` — builds locally but no runtime wiring.
- [ ] **[Federation] No k8s migrate job** — unlike shipped services, no `federation_db` template in `deploy/templates/`; first real impl needs DB bootstrap path.
- [ ] **[Federation] No downstream product hooks** — federated spaces/auth/search/moderation described in `docs/features/federation.md`, `docs/features/search.md` §owners — no `federat*` code in `src/backend/space/`, `src/backend/search/`, `src/backend/auth/`, `src/backend/notification/`.
- [ ] **[Federation] No control-plane surface** — `FederationManagementService` in `protos/voice/s2s/v1/federation_management.proto` (mTLS, admin ops) — no server, no `src/admin/` UI.
- [ ] **[Federation] K8s manifest lacks gRPC port** — `deploy/staging/services.yaml` `voice-federation` Service exposes only `:8080`; spec requires gRPC S2S (`docs/microservices/federation-service.md`). Scaffold-consistent today, but manifest won't work when gRPC lands without edit.
- [ ] **[Federation] `docs/todo/backend.md` L257 partially stale** — claims no staging Deployment; `deploy/staging/services.yaml` now has `voice-federation`. Still correct that `deploy/staging/configmap-app.yaml` / `deploy/prod/configmap-app.yaml` omit federation from `GATEWAY_GRPC_UPSTREAMS_JSON`.

### Story


- [ ] **[Story] `visibility_audience` not writable via API** — DB column + read path exist; `CreateStoryRequest` has no audience JSON; `visibilityFromRequest("custom")` stores `privacy.Nobody()`. Blocks real space/custom per-story audience (Batch 7 covers Flutter picker; backend contract gap). Paths: `protos/voice/story/v1/story.proto`, `src/backend/story/internal/grpcsvc/audience.go`, `src/backend/migrations/story_db/000002_visibility_audience.up.sql`.
- [ ] **[Story] Highlights lack `visibility_audience` JSONB** — only coarse `visibility` TEXT; no space multiselect per [stories.md](../features/stories.md) §Highlights. Paths: `src/backend/migrations/story_db/000001_init.up.sql`, `src/backend/story/internal/grpcsvc/story.go` (`canViewHighlight`).
- [ ] **[Story] `AddToHighlight` allows active stories** — spec says “from archive”; store only checks author ownership, not `expired_at`. Path: `src/backend/story/internal/store/store.go` (`AddToHighlight`).
- [ ] **[Story] Archive purge worker delayed first run** — 24h ticker, no startup `RunArchivePurgeOnce`. Path: `src/backend/story/internal/jobs/jobs.go` (`StartArchivePurgeWorker`).
- [ ] **[Story] Weak content validation** — no required `text_content` for `text`, no required `media_file_id` for `photo`/`video`. Path: `src/backend/story/internal/grpcsvc/story.go` (`CreateStory`).
- [ ] **[Story] `game_tag` unvalidated** — free string, no Matchmaking catalog lookup. Path: `src/backend/story/internal/grpcsvc/story.go`.
- [ ] **[Story] No compose/live E2E for LFP** — Gateway unit test only. Paths: `src/backend/gateway/transcode_stories_test.go`; no `compose_*lfp*` / Flutter LFP create flow in CI ([`.github/ci/e2e-features.yml`](../../.github/ci/e2e-features.yml)).
- [ ] **[Story] Stale service README** — still says “scaffold”. Path: `src/backend/story/README.md`.

### Voice


- [ ] **[Voice] `GetVoiceStates` omits proto fields** — never populates `is_commander`, `hand_raised` (fields exist in proto; state store has no fields for them).
- [ ] **[Voice] DM `StartCall` skips chat membership check** — `ensureChatMember` used for group/space paths only; DM only checks privacy + callee. Wrong `chat_id` can be attached.
- [ ] **[Voice] `RedisCallStore` has zero unit/integration tests** — staging/prod use Redis (`VOICE_REDIS_ADDR`); all store tests hit `MemoryCallStore`.
- [ ] **[Voice] Group voice cap mismatch in microservice doc** — `voice-service.md` mentions groups up to **500**; code hard-caps room at **32** (`MaxGroupVoiceParticipants`). Tests document 32 as intentional (`voice_grpc_group_test.go`); doc is stale.
- [ ] **[Voice] E2E coverage gaps vs PLAN “shipped”** — present: DM signaling (`TestComposeVoiceCall1to1_live`), optional bidirectional audio (`compose_voice_call_media_live_test.go`), Flutter `group_voice` / `spaces_voice` / `screen_share` API tests. Missing: compose live test for **space** voice + screen share with Role guard; no staging **RTC/media** smoke; `group_voice` E2E never exercises `LeaveCall` multi-participant behavior.
- [ ] **[Voice] `ListExpiredRinging` on Redis uses `KEYS`** — `voice:call:*` scan; risky under load.
- [ ] **[Voice] Stale service README** — still says “scaffold / out of scope” while PLAN marks voice shipped.

### Auth


- [ ] **[Auth] NATS event matrix mostly unimplemented** — `docs/microservices/auth-service.md` lists `user.registered`, `user.logged_in`, `user.logged_out`, `user.2fa_enabled`, `user.account_deleted`, `user.account_restored`; `AuthEventPublisher` only defines `user.guest_converted`. Files: `src/backend/auth/src/main/java/voice/backend/auth/events/AuthEventPublisher.java`, `src/backend/auth/src/main/java/voice/backend/auth/events/NatsAuthEventPublisher.java`.
- [ ] **[Auth] Active sessions / “Активные устройства” API missing** — `refresh_tokens` + `device_info` stored; no list/revoke-other-sessions endpoint per `docs/features/auth-and-contacts.md`. Files: `src/backend/auth/src/main/java/voice/backend/auth/repository/JdbcRefreshTokenRepository.java`, `docs/features/auth-and-contacts.md`.
- [ ] **[Auth] Password change + revoke-all-refresh not implemented** — Spec: password change deletes all refresh tokens; no change-password RPC/REST and no bulk revoke in `AuthService`. Files: `docs/features/auth-and-contacts.md`, `src/backend/auth/src/main/java/voice/backend/auth/service/AuthService.java`.
- [ ] **[Auth] Disable 2FA not implemented** — No RPC/REST to turn off TOTP or invalidate backup codes after enrollment.
- [ ] **[Auth] YouTube linked identity not implemented** — DDL allows `youtube` in `linked_identities`; only partial Twitch path exists. File: `src/backend/migrations/auth_db/000004_linked_identities.up.sql`.
- [ ] **[Auth] Guest TTL sweeper lacks real JDBC tests** — `GuestAccountLifecycleIntegrationTest` only checks bean exists and invokes `sweep()` without DB assertions; comment admits gap. File: `src/backend/auth/src/test/java/voice/backend/auth/GuestAccountLifecycleIntegrationTest.java`.
- [ ] **[Auth] Guest sweeper deletes guests with `last_online_at IS NULL`** — First sweep can soft-delete never-touched guests (e.g. legacy rows). File: `src/backend/auth/src/main/java/voice/backend/auth/repository/JdbcAccountRepository.java` (`deactivateExpiredGuests`).
- [ ] **[Auth] gRPC token context via `lastAccessToken` atomic** — `enable2FA`, `verify2FA`, `putE2EKeyBackup`, `getE2EKeyBackup`, `convertGuest` rely on in-process `lastAccessToken` when metadata missing; unsafe for concurrent direct gRPC. File: `src/backend/auth/src/main/java/voice/backend/auth/grpc/AuthGrpcService.java`.
- [ ] **[Auth] `auth-service.md` doc drift** — Missing/incorrect vs code: `SwitchActiveProfile`, `SetAccountStatus`, `ResolvePhoneHashes`, OAuth2 (developer-portal + admin), `backup_codes`, `last_online_at`, `linked_identities`; E2E migration cited as `V4` but Flyway uses `V4__e2e_key_backups.sql` + golang `000005`. File: `docs/microservices/auth-service.md`. *(Partial overlap with TODO.md “convert-guest doc auth-service.md” — that item is narrower.)*
- [ ] **[Auth] `src/backend/auth/README.md` migration section stale** — Still says Flyway “single migration V1”; repo has `V1`–`V5` and golang `000001`–`000006`. File: `src/backend/auth/README.md`.

### Realtime


- [ ] **[Realtime] Doc metrics vs implementation** — `realtime-service.md` lists `realtime.events.delivered`, `realtime.events.fanout_latency`, `realtime.reconnects`; `metrics.go` exposes only connections, connect counters, hello histogram, NATS lag. (`docs/features/observability.md` documents the implemented set — drift between service doc and observability spec.)
- [ ] **[Realtime] WS protocol surface differs from service doc** — Documented server ops `member_add` / `member_remove` not emitted; `chat_events_consumer.go` maps membership to `chat_update` with `change`. Undocumented ops in code: `message_read`, `message_delivered`, `message_pinned` / `message_unpinned`, `role_update`, `screen_share_started` / `screen_share_stopped`, `mention`.
- [ ] **[Realtime] `resume` is intentionally a no-op** — `ws.go` ignores `last_s` (aligned with `ARCHITECTURE_REQUIREMENTS.md`: catch-up via Messaging REST). No `resume_ack`; client cannot confirm server-side handling (acceptable per arch, but undocumented in protocol table).
- [ ] **[Realtime] Six separate NATS connections per instance** — `main.go` opens one connection per consumer + lag poller (no shared `*nats.Conn`), increasing reconnect churn and FD usage at scale.
- [ ] **[Realtime] Test gaps for newer paths** — No tests for `role_events_consumer.go`, `matchmaking_events_consumer.go` (integration), `delivery_ack` / cross-instance `message_delivered`, or `user_presence_updater_grpc.go` (gRPC path untested in this module).
- [ ] **[Realtime] Presence E2E is REST-only** — `presence_e2e_live_test.dart` checks `getPresence` API, not WS live fanout between friends (PLAN marks presence “shipped”).
- [ ] **[Realtime] `presence_update` status not validated in Realtime** — `ws.go` accepts any non-empty string; canonical enum normalization happens only in User gRPC (`user/internal/grpcsvc/user_presence.go`).
- [ ] **[Realtime] Stale module README** — `src/backend/realtime/README.md` still describes a health-only scaffold; contradicts `docs/PLAN.md` and actual code.

### Multi-Profile

- [ ] **[Multi-Profile] No create/delete profile rate limits** — anti-abuse spec in `docs/features/multi-profile.md`; no throttling in `CreateProfile` / `DeleteProfile` (`src/backend/user/internal/grpcsvc/user.go`).
- [ ] **[Multi-Profile] Premium vanity `@username` (no `#1234`) not implemented** — all profiles get 4-digit discriminator (`src/backend/user/internal/store/profile.go`); monetization in `docs/features/multi-profile.md`.
- [ ] **[Multi-Profile] Additional phone per profile not implemented** — spec: доп. номер на профиль (не основной); only account `accounts.phone` → primary profile (`PhoneHashResolver`, `auth.proto` S2S).
- [ ] **[Multi-Profile] Transfer contact between profiles not implemented** — spec §контакты: перевести контакт в нужный профиль после phone-add; depends on Contacts RPCs ([Social] Contacts RPCs).
- [ ] **[Multi-Profile] Per-profile notification policy incomplete** — push tokens per `profile_id` (`notification/.../device_tokens.go`); `PermissivePolicyLoader` default-open; inactive-profile DND not enforced end-to-end (`multi-profile.md` §статусы и уведомления).

## Low

### Subscription


- [ ] **[Subscription] README outdated — still describes health-only scaffold** — `src/backend/subscription/README.md`
- [ ] **[Subscription] Default webhook secret in prod path — `test-webhook-secret` if `PADDLE_WEBHOOK_SECRET` unset** — `src/backend/subscription/internal/billing/paddle.go`
- [ ] **[Subscription] Duplicate `DELETE` in `ActivatePremium`** — `src/backend/subscription/internal/store/store.go` (lines 95–99)
- [ ] **[Subscription] E2E / test gaps — no live CloudPayments, cancel, grace expiry, Space Pro webhook→join, or billing history** — `src/frontend/test/billing_e2e_live_test.dart`; `src/backend/gateway/compose_billing_live_test.go`; `src/backend/subscription/internal/grpcsvc/webhook_integration_test.go`; `src/backend/subscription/internal/grpcsvc/subscription_handlers_test.go`
- [ ] **[Subscription] Premium cosmetic gaps outside Subscription module — e.g. custom status not persisted; anonymous view tracked separately in `docs/todo/backend.md`** — `src/backend/user/internal/grpcsvc/user.go`; `docs/todo/backend.md` (Anonymous view)
- [ ] **[Subscription] Doc/constant drift — free space join 100 vs 50; free voice 360p vs 480p in different docs; not unified in limits** — `docs/features/subscription.md`; `docs/microservices/subscription-service.md`; `src/backend/subscription/internal/testfixtures/limits.go`

### File


- [ ] **[File] Proto lifecycle enum** — no distinct `expired`; `expired` DB status maps to `FILE_LIFECYCLE_STATUS_DELETED` (`d:\Git\Voice\protos\voice\file\v1\file.proto`, `d:\Git\Voice\src\backend\file\internal\grpcsvc\file_grpc.go` L674–675).
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
- [ ] **[Space] Transfer ownership 2FA/password — spec requirement; proto has bare `TransferOwnershipRequest` only** — `docs/features/spaces.md`, `protos/voice/space/v1/space.proto`
- [ ] **[Space] Member `nickname` in schema, no update RPC** — `src/backend/migrations/space_db/000001_init.up.sql`, `protos/voice/space/v1/space.proto`
- [ ] **[Space] QR join — product doc only, no Space API** — `docs/features/spaces.md`
- [ ] **[Space] Space-level `mm_config` for matchmaking — column exists, unused** — `src/backend/migrations/space_db/000001_init.up.sql`
- [ ] **[Space] `allow_guests` column (migration 000006) — only checked on invite join; no admin API to toggle** — `src/backend/migrations/space_db/000006_allow_guests.up.sql`, `src/backend/space/internal/store/invite.go`

### Moderation


- [ ] **[Moderation] `GetAutoModStats` semantics weak** — counts `auto_mod_log` rows, not messages scanned; `CheckMessage` does not increment checked counter.
- [ ] **[Moderation] Spam mute action taxonomy mismatch** — logs `mute` / `mute_permanent` actions; docs/model use `mute` / `shadow_ban`; Messaging only blocks when pattern re-matches.
- [ ] **[Moderation] Appeal proto omits `reviewed_at` / `review_notes`** — stored in DB, not returned to clients.
- [ ] **[Moderation] Limited unit coverage** — `automod_unit_test.go` only link-flood + threshold math; no unit tests for sanctions/appeals handlers (integration tests only).
- [ ] **[Moderation] Federation moderation** — documented in `moderation-service.md`, not implemented (federation deferred per PLAN).

### User


- [ ] **[User] Guest audience in User service is implemented for presence** — `show_online` / `show_game_status` + `include_guests` tested (`src/backend/user/internal/grpcsvc/privacy_integration_test.go`); `show_mm_rating` / `show_stories` enforced in Matchmaking/Story, not User (by design per `docs/features/privacy.md` enforcement path). Flutter per-field `include_guests` — waves A–J (2026-07-15).

#### Multi-Profile — audit (2026-07-15)

Спека: [multi-profile.md](../features/multi-profile.md). PLAN: **partial** (User, Auth). Аудит кода + сверка с TODO — ниже только открытое.

**Связанные пункты в других секциях (не дублировать):** [Subscription] JWT `subscription_tier` stuck `free` (лимит 5 профилей); Downgrade lifecycle + `ProfileDowngradePickerScreen`; [User] `EnsurePrimaryProfile` gRPC; NATS `user.profile_switched` gaps; [Search] `ProfileSwitched` not indexed; [Cross-cutting] premium → 3rd profile E2E; [Social] Contacts RPCs / phone-sync.

**Уже в коде (не заводить повторно):** `CreateProfile` + preset + `accent_color` + privacy seed; `ListMyProfiles` / `GET /api/v1/users/profiles`; `POST /api/v1/auth/switch-profile`; soft-delete `DeleteProfile` (gRPC); `ApplyDowngradeProfiles` + `frozen_at`; desktop `ProfileSwitcher` + mobile `ProfileAvatarSwitcher`; `profile_context_controller` (WS reconnect, MM cancel, space exit); accent theme + migration; voice `voiceBindingProfileId` + conflict dialog; account-level blocks; friend/chat isolation live tests (`compose_profile_isolation_live_test`, `profiles_verification_e2e_live_test`).

### Analytics


- [ ] **[Analytics] Prometheus names vs spec** — Code: `analytics_ingest_*`, `analytics_clickhouse_insert_latency_seconds` (`d:\Git\Voice\src\backend\analytics\internal\metrics\metrics.go`); docs: `analytics.ingest.events_per_second`, `analytics.ingest.batch_size` (`docs/microservices/analytics-service.md`). No `events_per_second` or `batch_size` histogram.
- [ ] **[Analytics] gRPC ingest omits lag metric** — `IngestLag` only set on NATS path (`d:\Git\Voice\src\backend\analytics\internal\grpcsvc\ingest.go` vs `d:\Git\Voice\src\backend\analytics\internal\consumer\runner.go`).
- [ ] **[Analytics] `DeliverNew` only** — New durable consumers skip backlog (`d:\Git\Voice\src\backend\analytics\internal\consumer\runner.go`); acceptable per spec but limits replay/backfill.
- [ ] **[Analytics] Export CSV omits hashed IDs** — Export columns exclude `user_id_hashed` / `profile_id_hashed` (`d:\Git\Voice\src\backend\analytics\internal\grpcsvc\query.go`).
- [ ] **[Analytics] User-level activity gap** — `message_sent` hashes `profile_id` only; `user_id_hashed` empty → DAU/retention undercount messengers (`d:\Git\Voice\src\backend\analytics\internal\adapters\domain.go`).
- [ ] **[Analytics] Gateway health telemetry off by default** — `GATEWAY_ANALYTICS_SAMPLE_RATE` default 0 (`d:\Git\Voice\src\backend\gateway\analytics_telemetry.go`); health dashboard needs explicit enablement.
- [ ] **[Analytics] CH schema doc naming** — Docs use `user_id`/`profile_id`; DDL uses `user_id_hashed`/`profile_id_hashed` (`docs/microservices/analytics-service.md` vs `d:\Git\Voice\docker\clickhouse\init\001_events.sql`).

### Role


- [ ] **[Role] Stale server comment — `RoleGRPC` still says «red-phase: Unimplemented stubs only».** — `src/backend/role/internal/grpcsvc/server.go`
- [ ] **[Role] README understates implementation — lists health only; contradicts actual surface.** — `src/backend/role/README.md`
- [ ] **[Role] `NamesFor` order non-deterministic — map iteration; awkward for UI/tests expecting stable lists.** — `src/backend/role/permissions/permissions.go`
- [ ] **[Role] No test for dual-scope effective mask — chat + `voice_room_id` together in one `GetEffectiveMask` call.** — `src/backend/role/internal/store/`, `internal/grpcsvc/` tests
- [ ] **[Role] `CreateRole` position not validated — actor can create custom role at position ≥ own top (hierarchy only on edit/assign, not create).** — `src/backend/role/internal/grpcsvc/roles.go` (`CreateRole`)
- [ ] **[Role] Guest role under-exercised — default join falls back to Member; Guest mask (`SPACE_VIEW` only) rarely applies unless `SetDefaultJoinRole` points to Guest.** — `src/backend/role/permissions/permissions.go`, `internal/store/roles.go`

### Cross-cutting


- [ ] **[Cross-cutting] `subscription/README.md` stale — still says “scaffold”; contradicts PLAN partial + billing E2E.** — `src/backend/subscription/README.md`, `docs/PLAN.md`
- [ ] **[Cross-cutting] Analytics “partial” = server-side only by design — client RUM explicitly out of v1 (`docs/features/analytics.md`); not a bug, but PLAN “partial” is architectural, not a missing backend slice.** — `docs/features/analytics.md`, `src/backend/analytics/`
- [ ] **[Cross-cutting] Notifications partial — push device creds / staging FCM already in TODO Critical/High; in-app + Realtime fan-out exist (`realtime/in_app_notification_fanout_test.go`).** — `docs/PLAN.md`, `src/backend/realtime/`

### Notification


- [ ] **[Notification] `platform_enum` in proto ignored — `RegisterDevice` only uses string `platform`.** — `protos/voice/notification/v1/notification.proto`, `src/backend/notification/internal/grpcsvc/server.go`
- [ ] **[Notification] `GetNotificationSettings` / `UpdateNotificationSettings` don’t persist — update echoes request; get ignores DB scopes/`mute_until`/`suppress_types`.** — `src/backend/notification/internal/grpcsvc/server.go`
- [ ] **[Notification] Unauthenticated debug push recorder — `/debug/recorded-pushes` exposes last recorded push by `profile_id` (compose/dev aid).** — `src/backend/notification/debug_http.go`
- [ ] **[Notification] DEPLOYMENT doc drift — references `internal/apns/config.go` (file is `http_sender.go`) and `APNS_PRIVATE_KEY` as canonical env name.** — `docs/DEPLOYMENT.md`, `src/backend/notification/internal/apns/http_sender.go`
- [ ] **[Notification] gRPC still labeled “Phase-6 stub” in server while substantial logic exists — misleading for reviewers.** — `src/backend/notification/internal/grpcsvc/server.go`

### Federation


- [ ] **[Federation] Accidental Gateway REST proxy** — `src/backend/gateway/config_test.go` allows `federation` in `GATEWAY_REST_UPSTREAMS_JSON`, but `routing_test.go` blocks public paths; low risk unless someone adds transcoding routes without review.
- [ ] **[Federation] Generated-only Flutter surface** — `src/frontend/lib/gen/voice/s2s/v1/*`; no `lib/` product code for federation.
- [ ] **[Federation] Repo junk in service dir** — `src/backend/federation/$prof`, `src/backend/federation/coverage` (not source; shouldn't ship).
- [ ] **[Federation] No buf/gateway public API** — S2S protos correctly isolated under `protos/voice/s2s/v1/`; no accidental client exposure via REST transcoding.

### Voice


- [ ] **[Voice] Redis key layout differs from `voice-service.md` model** — docs describe `voice:session:{profile_id}` object + room sets; code uses `voice:session:{profile_id}` → `room_id` pointer + JSON blob `voice:call:{room_id}`.
- [ ] **[Voice] LiveKit JWT minimal grants** — only `video.roomJoin` + `room`; no explicit `canPublish` / `canSubscribe` (works in compose media test, but less explicit than LiveKit best practice).
- [ ] **[Voice] Commander / raise-hand client surface is proto-only** — generated Dart gRPC stubs exist; no `lib/` product usage beyond `lib/gen/`.

### Auth


- [ ] **[Auth] IP logging for audit not implemented** — `docs/microservices/auth-service.md` § “IP logging”; `HttpAccessLogFilter` logs method/path/status only, no client IP. Files: `src/backend/auth/src/main/java/voice/backend/auth/web/HttpAccessLogFilter.java`, `docs/microservices/auth-service.md`.
- [ ] **[Auth] Internal gRPC has no caller authorization** — `ResolvePhoneHashes`, `SetAccountStatus` callable by any mesh peer without S2S auth. Files: `src/backend/auth/src/main/java/voice/backend/auth/grpc/AuthGrpcService.java`, `src/backend/moderation/internal/authclient/authclient.go`.
- [ ] **[Auth] `GuestConvertNatsEventIntegrationTest` is a false positive** — Asserts `user.guest_converted` without calling convert; `RecordingAuthEventPublisherImpl.publishedSubjects()` always includes subject. File: `src/backend/auth/src/test/java/voice/backend/auth/GuestConvertNatsEventIntegrationTest.java`. *(Likely duplicate of TODO.md Batch 6 “NATS user.guest_converted”.)*
- [ ] **[Auth] Compose dev 2FA bypass enabled** — `AUTH_TOTP_TEST_BYPASS: "true"` in `docker-compose.yml`; acceptable for local E2E but must not leak to staging/prod manifests (staging yaml omits it — OK).

### Realtime


- [ ] **[Realtime] Coverage artifacts committed** — `$prof`, `notif_cov`, `notif_cov.out`, `coverage_profile`, `coverage_profile.out`, `coverage` under `src/backend/realtime/` are tracked in git (local profiling noise).
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
