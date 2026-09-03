# TODO — Backend

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


- [ ] **[Space] Owner locked: `GetAuditLog`, `SearchPublicSpaces`, `ListTemplates`, `CreateFromTemplate` — runtime `Unimplemented` (embed). `DeleteSpace`/`TransferOwnership` shipped (T-011); `JoinSpace`/`LeaveSpace` живые; owner не может leave без transfer** — `protos/voice/space/v1/space.proto`; `src/backend/space/internal/grpcsvc/`
- [x] **[Space] Space Pro cache never synced — `space_db.space_subscriptions` comment says “synced from Subscription”; only test seed `UpsertSpaceSubscription` writes; Subscription writes `subscription_db` only** — **done:** NATS consumer `space/internal/subscriptionconsume` + S2S `SyncSpaceProSubscription` write entitlement cache; `SeedSpaceProActive` remains test helper only.
- [ ] **[Space] `entry_requirement` не исполняется — JoinSpace отвергает любой requirement ≠ `none` (`FailedPrecondition`), нет captcha/questions/mod-approval queue** — `src/backend/space/internal/grpcsvc/join.go`, `invites.go`
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


- [ ] **[User] OAuth verification bypasses User Service** — Twitch link writes `profiles` via `JdbcUserVerificationSync` (`src/backend/auth/src/main/java/voice/backend/auth/userdb/JdbcUserVerificationSync.java`), not `SetVerification` (`src/backend/user/internal/grpcsvc/user_verification.go`). No `user.verified` NATS publish on OAuth path (`src/backend/user/internal/userevents/jetstream.go`). `EnsurePrimaryProfile` / `GetSettings` — **есть** (`user_settings.go`), не заводить снова.

### Analytics


- [ ] **[Analytics] Event loss on ClickHouse failure / crash** — NATS messages are consumed and acked before durable CH write; failed flushes only re-queue in process memory (`d:\Git\Voice\src\backend\analytics\internal\consumer\runner.go`, `d:\Git\Voice\src\backend\analytics\internal\buffer\accumulator.go`). Process restart after a failed flush drops data permanently.

### Matchmaking


- [ ] **[Matchmaking] Party snapshot из voice roster отсутствует** — `PartyStore` stub; `StartSearch` валидирует `partySize=1`. Нет сброса очереди при leave/join войса (`docs/features/matchmaking.md`). Pairwise `rolesCompatible` уже требует **distinct** roles (`criteria/criteria.go`); live 10-stack matcher + `RolesDistinct` на полном лобби — тонко (нет compose на seeded 10-slot).
- [x] **[Matchmaking] Platform MM ban fail-closed + S2S** — `StartSearch` / matcher fail-closed when `BanStore` nil (`platform_ban_degradation_test.go`, `worker_ban_degradation_test.go`, **#73**); Moderation `mm_ban` → `ApplyPlatformMMBan` / revoke (`sanctions.go`).
### Role


- [x] **[Role] Voice Service never wires Role Service — `Roles` is nil in prod; `VOICE_JOIN`, `VOICE_SPEAK`, `VOICE_MUTE_OTHERS`, etc. are not enforced on join/speak/mute. Only `EnsureScreenShare` exists and is unused without a Role client.** — **partial:** `voice/main.go` wires `Roles` via `ROLE_GRPC_ADDR`; `EnsureVoiceJoin` + `EnsureScreenShare` enforced on join/share (`role_guard.go`, `voice_room.go`). Compose/Flutter `VOICE_JOIN` deny live shipped (`TestComposeVoiceJoinDeny_live`, #14). Still open: `VOICE_SPEAK` / `VOICE_MUTE_OTHERS` (and related) not enforced on speak/mute paths.
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


- [ ] **[Auth] DeleteAccount tombstone неполный** — RPC/REST `DeleteAccount`/`RestoreAccount` **есть**. **ListChats** скрывает DM с удалённым peer (Chat → Auth `ListDeletedAccountIds`, Batch 31d). Остаётся: системное «Пользователь удалён» в DM-треде; `email_verify` OTP; UI — [client.md](client.md).


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
- [ ] **[File] Infected-file: ConfirmUpload может вернуть 200 с `scan_result=infected`** — нет Notification fan-out; клиент не показывает блок.

### Protos/Pkg


- [ ] **[Protos/Pkg] Analytics subscription mapping remains partial** — runner already subscribes to `social.events`, `role.events`, `file.events`, `subscription.events`, and `moderation.events`; only deferred `federation.events` is absent. The Subscription mapper handles plan-started/payment events, not expiry/downgrade. — `src/backend/analytics/internal/consumer/runner.go`; `src/backend/analytics/internal/adapters/domain.go`; `docs/MICROSERVICES.md`
- [ ] **[Protos/Pkg] Space event catalog residual drift** — `ChatStreamEvent` now has payloads for invite, member joined/left, updated and deleted events. Remaining gaps vs [space-service.md](../microservices/space-service.md): no `space.member_banned` payload; no voice-room created/deleted payloads; `SpaceUpdated` carries only `space_id`, not `changed_fields`.
- [ ] **[Protos/Pkg] Go `pb/` codegen sync asymmetry** — `scripts/dev/sync-pb-from-gen.sh` syncs 7 trees (`analytics`, `chat`, `file`, `messaging`, `role`, `user`, `voice`); 10+ packages hub under `src/backend/voice/pb/voice/` and `src/backend/user/pb/voice/`. No CI drift check (unlike `make buf-dart-check` for `src/frontend/lib/gen/`). Stale committed stubs possible after proto edits.
- [ ] **[Protos/Pkg] `pkg/` resilience gap** — `docs/MICROSERVICES.md` requires circuit breaker on all gRPC calls; `src/backend/pkg/grpcclient/` only provides `dial.go` (`DialTarget`) and `wait.go`. No breaker/retry/mTLS helpers.
- [ ] **[Protos/Pkg] `pkg/` auth metadata fragmentation** — Gateway contract in `src/backend/gateway/transcode_grpc.go` (`x-voice-user-id`, `x-voice-profile-id`, …), but 12+ per-service `internal/authctx/` copies (`src/backend/*/internal/authctx/`). Only partial shared helpers: `src/backend/pkg/guestguard/`, `src/backend/pkg/correlation/`, `src/backend/pkg/jwt/` (edge validation, not inbound gRPC claim parsing).

### Space


- [ ] **[Space] `GetAuditLog` unimplemented while `audit_log` is written for ban/kick/timeout only** — `src/backend/migrations/space_db/000004_moderation.up.sql`, `src/backend/space/internal/store/{moderation,members}.go`. Gateway **мапит** moderation ExportAuditLog (не hardcoded `[]`).
- [x] **[Space] `UpdateSpace` writes visibility / entry_requirement / questions / mm_config** — `space.go`; Role `SPACE_MANAGE_SETTINGS` (`requireSpacePermission`). Tree CRUD — `requireSpaceTreeManage`, не owner-only.
- [ ] **[Space] `mm_config` / `entry_questions` never loaded — not in `SpaceRow`, not in `spaceRowToProto`** — `src/backend/space/internal/store/space.go`, `src/backend/space/internal/grpcsvc/proto.go`
- [ ] **[Space] Tree node Pro limit (500) not implemented — hardcoded `MaxTreeNodes = 50`, no entitlement check (unlike member cap)** — `src/backend/space/internal/store/tree.go`
- [ ] **[Space] Catalog indexing fragile — Search hydrator calls `GetSpace` (member-only); `SearchPublicSpaces` Unimplemented; ranking verified-first нет; нет `space.updated` re-index** — `src/backend/search/internal/deps/deps.go`, `chat_space_indexer.go`
- [ ] **[Space] NATS events incomplete vs spec** — join/leave/kick publish `space.member_joined` / `space.member_left`; update/delete publish `space.updated` / `space.deleted`. Remaining: `space.member_banned` is absent; voice-room created/deleted publisher methods are no-ops; membership/invite publish failures still pass through no-op `logInviteEventFailure`; Search does not reindex `space.updated` — `src/backend/space/internal/spaceevents/`, `src/backend/search/internal/indexer/chat_space_indexer.go`.
- [ ] **[Space] Member timeout not enforced downstream — `IsProfileTimedOut` exists, unused outside Space** — `src/backend/space/internal/store/moderation.go`
- [ ] **[Voice] JoinVoiceRoom: `voice_room_id ∈ space_id` не проверяется** — `voice_room.go`

### Moderation


- [x] **[Moderation] `mm_ban` S2S to Matchmaking** — `ApplySanction` type `mm_ban` → `Matchmaking.ApplyPlatformMMBan`; revoke → `RevokePlatformMMBan` (`sanctions.go`). Peer-scoped `BanFromMM` remains separate API.
- [ ] **[Moderation] Auto-moderation diverges from spec** — `CheckMessage` only detects ≥3 links; no repeated-message detection; no 1h timed mute (second spam hit → permanent block for that pattern only, no window); no “first 10 messages after mute” pass.
- [ ] **[Moderation] Report threshold audience is static env, not object audience** — `MODERATION_PLATFORM_AUDIENCE_SIZE` (default 1000) drives 1% calc; spec calls for relative threshold vs target’s audience.
- [ ] **[Moderation] Admin audit export пуст только если store пуст** — Gateway `writeModerationAuditExportJSON` мапит `ExportAuditLog` (`transcode_moderation_admin.go`). Не hardcoded `[]`.
- [ ] **[Moderation] Temp ban expiry does not restore Auth** — `expires_at` respected in SQL for active lookup, but no job/handler calls `Auth.SetAccountStatus(active)` on expiry; only explicit revoke/approved appeal clears suspension.
- [x] **[Moderation] Sanction notification push delivery** — **done (T-013):** consumer resolves profiles, applies policy with **presence skipped**, sends `system` push copy so online recipients are not silent-dropped (`moderation_events_consumer.go`).
- [ ] **[Moderation/Notification] T-023 — sanction `system` in-app contract and Realtime fan-out** — current moderation consumer resolves `target_account_id` to profiles, intentionally skips presence, and sends push so online users are not silent-dropped. Still undefined/absent: Notification→Realtime transport, system-sanction payload, transport dedupe, final account→profiles semantics, and Flutter `system` presentation. This is a narrow temporary exception, not a presence rule for every system notification — [notification-service.md](../microservices/notification-service.md).

### Social


- [x] **[Social] Contacts/favorites REST** — Gateway `GET/POST /api/v1/friends/contacts`, `GET/POST /api/v1/friends/favorites` shipped (**Batch 23a**). **Remaining:** `SyncPhoneContacts` UX.
- [ ] **[Social] Block friendship cascade fails open when profile resolution is unavailable** — happy path is implemented: `BlockAccount` resolves both accounts' profile sets and calls transactional `BlockAccountAndSeverFriendships`, which deletes accepted/pending/declined rows between non-empty sets (`block_cascade_integration_test.go`). Residual: when `AccountProfiles` is nil or either `ProfileIDsForAccount` call fails, the handler passes an empty set and the store inserts the block without deleting friendship rows (`social_blocks.go`, `blocks.go`).
- [x] **[Social] Outgoing request status exposed** — `PendingFriendRequest.status` (`pending` | `declined`) in proto + `ListFriendRequests` mapping; Flutter outgoing requests tab shows declined label — **Batch 24b** (`friends.md`).
- [x] **[Social] `allow_friend_requests` service integration test** — **done:** `allow_friend_requests_integration_test.go` covers stranger invite denial. Remaining compose-live coverage is tracked below under Social test gaps.

### User


- [ ] **[User] Premium animated GIF avatar is a dead path** — premium gate in `user_avatar.go` but `image/gif` rejected by `r2avatar/validate.go` (`TestValidateUploadParams_rejectsGifInPhase1`); conflicts with `docs/features/user-profile.md`. `GetSettings`/`UpdateSettings` **есть** (`user_settings.go`). `GetPrivacySettings` ownership check **есть** (non-S2S → `GetOwnedProfile`).
- [ ] **[User] `SetPrimaryProfile` отсутствует** — `is_primary` только bootstrap; phone search всегда primary.
- [ ] **[User] Verification V1 incomplete (Auth + User boundary)** — Twitch only in `LinkedAccountsService` (`src/backend/auth/src/main/java/voice/backend/auth/service/LinkedAccountsService.java`); YouTube in DB schema only (`src/backend/auth/src/main/resources/db/migration/V3__linked_identities.sql`); no partner-status recheck cron (`docs/features/verification.md`).
- [ ] **[User] NATS contract gaps** — `user.presence_changed` **published** but JetStream proto lacks `old_status`/`new_status`; missing `user.game_detected`, `user.settings_changed` ([user-service.md](../microservices/user-service.md)); `PublishProfileUpdated` / `PublishVerified` emit stub `ProfileCreated` without `changed_fields` / `verification_type`; `PublishProfileSwitched` drops `old_profile_id` (`src/backend/user/internal/userevents/jetstream.go`).
- [ ] **[User] Durable `last_seen_at` (PostgreSQL)** — spec requires PG persistence for header; code Redis-only TTL 5 min — [presence.md](../features/presence.md), [user-service.md](../microservices/user-service.md). Sub-bullet: privacy filter at read time when `show_last_seen` lands.
- [ ] **[User] `show_last_seen` privacy enforcement** — add `show_last_seen` to `privacy_settings` proto/DDL; filter `last_seen_at` in `GetPresence`/`GetBulkPresence` per viewer — [user-service.md](../microservices/user-service.md) — **P0**
- [ ] **[User] JetStream `presence_changed`: `old_status`/`new_status` in proto + publisher** — event table spec vs stub payload in `jetstream_events.proto` / `userevents/jetstream.go` — **P0**
- [ ] **[User] Homoglyph-normalized search not implemented** — anti-spoof on create only (`src/backend/user/internal/store/verification.go`); `SearchProfilesAfter` uses raw `ILIKE` (`src/backend/user/internal/store/profile_search.go`); spec requires normalized lookup (`docs/features/verification.md`).
- [ ] **[User] Premium custom status not gated** — `UpdatePresence` accepts `custom_status` for all tiers (`src/backend/user/internal/grpcsvc/user_presence.go`); spec: Premium only. `UpdateProfile.custom_status` не persist в DDL (только Redis presence).

### Analytics


- [ ] **[Analytics] Health dashboard under-spec vs docs** — event type defaults to `api_request` (PR #125 / master); product still under-delivers latency/error/WS KPIs from `docs/features/analytics.md` / analytics-service.md (`query.go`).
- [ ] **[Analytics] Product dashboard under-spec** — `product` returns `dau` as `uniqExact(user_id_hashed)` over the whole range (default 30d), not daily DAU; missing MAU/WAU/onboarding completion per `docs/features/analytics.md` / `docs/microservices/analytics-service.md` (`d:\Git\Voice\src\backend\analytics\internal\store\query.go`).
- [ ] **[Analytics] Grafana vs REST registration mismatch** — Grafana panel counts `profile_created` (`d:\Git\Voice\deploy\observability\grafana\dashboards\voice-analytics-product.json`); REST `product` dashboard counts `user_registered` (`d:\Git\Voice\src\backend\analytics\internal\store\query.go`).
- [ ] **[Analytics] DoD ingest path untested** — Live tests only check RBAC 200/403 and export HTTP 200; no assertion that `message.sent` → ClickHouse row within 60s (`d:\Git\Voice\src\backend\gateway\compose_analytics_live_test.go`, `d:\Git\Voice\src\backend\gateway\compose_analytics_export_live_test.go` vs `docs/features/analytics.md` DoD §1).
- [ ] **[Analytics] Silent no-op without ClickHouse** — `CLICKHOUSE_DSN` unset → service starts, ingest buffers, nothing persisted (`d:\Git\Voice\src\backend\analytics\main.go`); k8s secret refs are `optional: true` (`d:\Git\Voice\deploy\staging\services.yaml`, `d:\Git\Voice\deploy\prod\services.yaml`).
- [ ] **[Analytics] Weak prod hash-key guard** — Missing `ANALYTICS_ID_HASH_KEY` falls back to dev default (`d:\Git\Voice\src\backend\analytics\main.go`).

### Matchmaking


- [x] **[Matchmaking] Decline semantics vs spec** — `handleMatchDecline` party-aware (declining party cancelled, others continue searching); cross-party IT in `grpcsvc/match_test.go` (PR #4). Compose live: `TestComposeMatchmakingCrossPartyDecline_live` (#14).
- [ ] **[Matchmaking] MM rating privacy off in compose** — `main.go` wires `RatingPrivacy` only when `USER_GRPC_ADDR` / `SOCIAL_GRPC_ADDR` / `SPACE_GRPC_ADDR` are set; `docker-compose.yml` matchmaking service omits these (unlike k8s `envFrom` on `deploy/staging/configmap-app.yaml`). Local stack exposes ratings without privacy checks.
- [ ] **[Matchmaking] Match squad not ephemeral** — Squad creates a normal group chat + group voice (`squad/grpc_clients.go`). `CompleteMatch` only updates MM DB (`grpcsvc/rating.go`); no Chat/Voice teardown. Contradicts “auto-delete when all leave” (`docs/features/matchmaking.md`).
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
- [ ] **[Bot] Deferred follow-up uses wrong `ChatRef` type** — `lookupInteraction` always returns `CHAT_TYPE_CHANNEL` (`internal/grpcsvc/interaction.go`), breaking deferred `SendBotMessage` / `CompleteInteraction` for group (and DM) chats.
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
- [ ] **[Messaging] Group/channel view counts absent** — `text-chat.md` requires per-message view counter; no model/RPC beyond DM-style `read_receipts`.
- [ ] **[Messaging] `ForwardMessage` attachment `validateRichPayload` gaps vs `SendMessage`** — shadow-ban / ghost_only on forward+commentary closed (PR #126/#131); remaining: attachment `validateRichPayload` parity with SendMessage (`messaging_grpc.go` `ForwardMessage`).
- [ ] **[Messaging] Read-state APIs DM-typed only** — `MarkRead` / `GetReadState` / `GetBulkReadState` / `GetChatListMetadata` use `validateChatRefDM`; explicit `group`/`channel` refs rejected while `GetMessages` accepts all types.
- [ ] **[Messaging] `content_type`: article, location, video_note, music** — **partial (parallel track):** `messages.content_type` column + `SendMessage`/`Message.content_type` proto; location/article send without `file_id`; `video_note`/`music` payload validation still open — [messaging-service.md](../microservices/messaging-service.md) — **P0**
- [ ] **[Messaging] `schedule_message`, `send_when_online`, `send_silent`** — **doc contract in** [messaging-service.md](../microservices/messaging-service.md) (`SendMessageRequest`, `scheduled_messages`, worker). Not yet in proto/code. — **P0**
- [x] **[Messaging] `GetChatListMetadata` preview DTO** — **done (Batch 13 + parallel track):** `last_message_content_type` from durable `messages.content_type` with attachment inference fallback; `is_outgoing` + `delivery_state` shipped (Batch 12).
- [x] **[Messaging] Durable `last_message_delivery_state`** — `read_receipts.last_delivered_message_id`, consumer on `message.delivery_ack`, derivation in `GetChatListMetadata` (Batch 12).
- [ ] **[Messaging] `UpdateScheduledMessage` RPC + handler** — edit pending scheduled row; proto + integration test — [messaging-service.md](../microservices/messaging-service.md) — **P0**
- [ ] **[Messaging] File processed → preview refresh consumer** — NATS handler on `file.processed` to update list metadata / invalidate cache — [messaging-service.md](../microservices/messaging-service.md)
- [ ] **[Messaging/Subscription] Premium multi-reaction limit enforcement** — after subscription entitlement doc lands
- [ ] **[Messaging] `message.sent` event** — **partial (parallel track):** JetStream `MessageSent.content_type` shipped; `send_silent` + scheduled metadata still open.

### Search


- [x] **[Search] Reverse-direction / bidirectional block on SearchUsers/SearchGlobal** — `filterProfileHits` + `AccountPairBlocked`. Remaining: User `SearchProfiles` (`/api/v1/users/search`) и SQL `BlockedAccountIDs` pre-filter (outgoing) — post-filter закрывает.
- [ ] **[Search] JetStream `DeliverNew` → no historical backfill — consumers only index events after subscription; deploy/reset leaves `search_db` empty for past messages/profiles unless manual per-chat reindex.** — `src/backend/search/internal/indexer/consumer.go`
- [ ] **[Search] Index update failures silently acked — handler logs `search index update failed` but does not `Nak`; failed upserts are lost permanently.** — `src/backend/search/internal/indexer/consumer.go`
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
- [x] **[Chat] Archive side-effects** — auto-unarchive on incoming DM message to archived membership (Chat `message.sent` consumer → `AutoUnarchiveDMRecipients`) — **done (Batch 20)**.
- [x] **[Chat] Archive integration test** — auto-unarchive when incoming DM message arrives — **done (Batch 20)**; main/archive inbox list regression — **done (Batch 15)**.
- [x] **[Chat] Gateway REST** — folder RPCs + `GET /chats?folder_id=` (**Batch 19**): `GET/POST /api/v1/chats/folders`, `PATCH/DELETE …/folders/{id}`, `POST/DELETE …/folders/{id}/chats`, `PUT …/chats/order`, `POST/DELETE …/chats/{chatId}/pin`; Quick Access REST — **done (Batch 17)**; `inbox=archive` on `GET /chats` — **done Batch 15**.

### Telegram-parity audit — open CODE (2026-08-28)

Источник: `tmp/telegram-ux-audit/AUDIT.md` (DOC closed). ID — для трассировки с audit tracker.

- [x] **[Chat] R3-A04 — Message requests bucketing** — **done (Batch 21a):** `EnsureDM` sets recipient `inbox_bucket` via Social friends/contacts S2S (`HasContact`); `message.sent` consumer promotes `declined` → `requests` on re-contact — `dm_inbox.go`, `dm_request_recontact.go`.
- [x] **[Messaging] R3-A05 — `PinMessage` permission gap** — standalone `group`/`channel` without `space_id`: deny pin for `member` role (owner/admin allowed); space chats still use Role `TEXT_CHAT_PIN_MESSAGES` — [messaging-service.md](../microservices/messaging-service.md).
- [x] **[Messaging] R3-A06 — `validateAttachments` blocks rich payloads** — **done (Batch 31a):** `content_type` branches for location/article (no File row) and file-backed rich types (`sticker`, `gif`, `music`, `video_note`) with payload shape + File metadata validation — `messaging_grpc.go`, tests — [messaging-service.md](../microservices/messaging-service.md).
- [x] **[Chat] R3-A12 — Standalone `channel` chats** — `CreateChat` without `space_id` creates standalone channel with creator as `chat_members` owner (`CreateChannelChat`); space channels unchanged — **Batch 26b**. **Batch 14:** membership `channel` rows appear in `ListChats` main inbox SQL.
- [x] **[Chat] R3-A14 — `CreateChat`/`UpdateChat` proto fields ignored** — `topic` persisted on create; `topic`/`threads_enabled`/`allow_user_main_feed` on `UpdateChat`; channels updatable — `group.go`, store `UpdateGroupChat` — **Batch 27b**.
- [ ] **[Chat] R3-A15 — `chats.allow_guests` behavior not wired** — migration `000007` adds the chat-level guest admission flag required by [auth-and-contacts.md](../features/auth-and-contacts.md), but Chat has no proto field, mutation handler, or admission enforcement for it. Keep the column; define and implement the contract.
- [x] **[Chat] R3-A16 — `ListChats` space merge bugs** — unified SQL pagination for space chats on page 2+ (`listChatsPageMainWithSpaces` UNION in store; gRPC passes `spaceIDs` on every page). Prior partial fixes: Batch 13 archived filter + hydration; Batch 16 pagination.
- [ ] **[Chat/Messaging/File] Stickers/GIF wire (R2-A32, R4-04)** — expand checklist: `chat_db` migrations `sticker_packs`/`stickers`/`profile_installed_packs`; Chat RPCs (`ListInstalledStickerPacks`, `InstallStickerPack`, `SearchGifs`, …); Gateway REST ([api-gateway.md](../microservices/api-gateway.md) § Stickers and GIF); ~~Messaging proto `STICKER`/`GIF` + send validation~~ **Messaging send validation done (Batch 31a)**; File `UPLOAD_INTENT_STICKER`/GIF transcode; `ListSharedMedia` `STICKERS` kind extension — **P0**
- [x] **[Messaging] Durable delivery consumer** — **done (Batch 12):** Realtime JetStream `message.delivery_ack` publish (Batch 11) + Messaging consumer → `last_delivered_message_id`; list ✓✓ via `GetChatListMetadata.last_message_delivery_state`.
- [x] **[Realtime] R3-A27 — @mention notification payload naming** — WS `mention` op uses `profile_id` (not `user_id`) in `dispatchMentionAdded` (Batch 11).
- [ ] **[User] R3-A19 — Presence WS privacy filter (code)** — Realtime `presence_update` must apply `show_online` / omit `last_seen` per viewer; publish `old_status`/`new_status` delta on `user.presence_changed` — doc in [presence.md](../features/presence.md); code gaps in User + Realtime.
- [x] **[Notification] R3-A23/R4-A15 — `message_request` type in code** — **done (Batch 22a):** `TypeMessageRequest` in notification delivery; push/in-app route by recipient `inbox_bucket=requests` via Chat `ListMembers.inbox_bucket` S2S; Realtime WS fan-out emits `message_request` (not `new_message`) for requests inbox — [notification-service.md](../microservices/notification-service.md). **Client toggle:** [client.md](client.md) Batch 22b.

### Chat — other


- [ ] **[Chat/Messaging/File] Stickers + GIF** — **P0**, 0 code: `[Chat]` pack store + provider search RPC; `[Messaging]` `STICKER`/`GIF` send payload + composer contract; `[File]` animated asset processing — superseded single-line below
- [ ] **[Chat] Стикер-паки / GIF / voice-note first-class — 0 кода** — see `[Chat/Messaging/File] Stickers + GIF` above; voice-note via `[File]` upload category
- [ ] **[File] Upload intent/category: video vs video_note** — proto field + processing branch in `ConfirmUpload` — composer video-note flow — **P0**
- [x] **[Chat] `MuteChat` / `ArchiveChat`** — `mute_archive.go`.
- [x] **[Chat] Group `last_message_at` never updated from message stream** — **done:** `TouchLastMessageAt` updates `type IN ('dm','group','channel')` (`dm.go`); store IT `last_message_at_integration_test.go`.
- [x] **[Chat] Group last_message_at from message stream** — **done:** same as above.
- [ ] **[Chat] `ListChats` omits `Chat.topic`** — list SQL/mapping already includes `e2e_enabled`, `space_id`, `slow_mode_seconds`, `threads_enabled` and `allow_user_main_feed`; only `topic` is absent from the selected `ChatRow` fields — [chat-service.md](../microservices/chat-service.md).
- [x] **[Chat] `UpdateChat` ignores thread settings** — **done (Batch 27b):** `threads_enabled` / `allow_user_main_feed` persisted via `UpdateGroupChat`.
- [x] **[Chat] `UpdateChat` rejects channels** — **done (Batch 27b):** `UpdateChat` allows `group` and `channel`; topic/thread flags via Chat API.
- [ ] **[Chat] Subscription S2S not integrated** — doc dependency (`docs/microservices/chat-service.md`); limit hardcoded `GroupMemberLimit = 500` (`src/backend/chat/internal/store/group.go`). No subscription-tier differentiation.
- [ ] **[Chat] Group admin role unused** — schema allows `owner|admin|member`; only `owner` may `RemoveMember` / `UpdateChat` (`src/backend/chat/internal/grpcsvc/group.go`). No code assigns `admin`. Conflicts with `docs/features/text-chat.md` admin powers.

### Notification


- [ ] **[Notification] `friend_request` delivery зависит от Social NATS** — publisher + `social_events_consumer.go` есть; проверить wiring `NATS_URL` на notification в k8s. Тихие часы/settings **пишутся в БД** (`store/settings.go`) — клиентский dual-write: [client.md](client.md).
- [ ] **[Notification] `send_silent` consumption** — read flag from `message.sent`; suppress push sound/badge rules; in-app policy — [notification-service.md](../microservices/notification-service.md)
- [ ] **[Notification] Quiet hours semantics test** — assert in-app still delivered when push blocked (`ApplyQuietHours`); document as intended — `quiet_hours_test.go`
- [x] **[Notification] `message_request` / stranger type** — **done (Batch 22a):** `message_request` wire type + per-recipient routing from `chat_members.inbox_bucket` — [notification-service.md](../microservices/notification-service.md)
- [ ] **[Notification] `reply` marked ✓ but not implemented — no `reply` type in message consumer or Realtime in-app fanout; thread replies are treated as `new_message`.** — `docs/features/notifications.md`, `src/backend/notification/message_events_consumer.go`, `src/backend/realtime/in_app_notification_fanout.go`
- [ ] **[Notification] Matchmaking/voice push ignores presence — handlers hardcode `IsOnline: false`; no `EnrichDecision` / User gRPC check → online users still get push (messages path does check).** — `src/backend/notification/internal/consumer/matchmaking_events.go`, `src/backend/notification/matchmaking_events_consumer.go`, `src/backend/notification/voice_events_consumer.go`
- [ ] **[Notification] `system` in-app / Gateway gaps (T-023)** — Moderation NATS consumer produces `system` push for sanctions and narrowly skips presence until an in-app path exists. Still missing/undefined: Notification→Realtime transport + payload + dedupe, final account→profiles semantics, Flutter presentation, other system producers, and Gateway REST exposure — `src/backend/notification/moderation_events_consumer.go`; `src/backend/notification/internal/grpcsvc/server.go`; `src/backend/gateway/transcode_notifications.go`; `src/backend/realtime/`
- [x] **[Notification] Multi-replica duplicate push risk — per-pod durable consumer name (`notif_<hostname>_mod`) on moderation stream caused duplicate delivery across replicas; all notification JetStream consumers now use cluster-wide `SharedDurable` names (moderation → `notif_mod`).** — **done (Batch 31c):** `src/backend/notification/internal/consumer/durable.go`, `moderation_events_consumer.go`, `main.go`

### Federation


- [ ] **[Federation] Hollow pod on every staging/prod deploy** — `voice-federation` is Tier-1 restart in `scripts/staging/rollout-app-tier.sh`; image built/pushed on every `master` push via `.github/workflows/ci.yml` (`staging-images-push`) and `scripts/ci/staging-image-catalog.json`. Burns CI/CD + cluster resources with no product surface.
- [x] **[Federation] `federation_db` documented but never provisioned** — `docs/DATA_STORES.md`, `docs/microservices/federation-service.md` now mark `federation_db` as planned/deferred; still absent from `docker/postgres/initdb.d/`, `scripts/dev/compose-migrate-all.sh`, `src/backend/migrations/`, `deploy/templates/` until implementation.
- [x] **[Federation] Prometheus scrape misconfigured** — federation scaffold now exposes GET `/metrics` via `pkg/promhttp`; k8s annotations unchanged.
- [ ] **[Federation] Spec ↔ proto drift (implementation trap)** — when work starts, docs and contracts disagree:
- [ ] **[Federation] `federation.events` contract is dead** — `docs/CONTRACT_MATRIX.md` lists Federation → Analytics/Role/Moderation; zero publishers/consumers in `src/backend/analytics/`, `src/backend/role/`, `src/backend/moderation/`, `src/backend/federation/`.

### Story


- [x] **[Story] `show_stories = Nobody` global privacy bypass** — CreateStory caps explicit visibility to `show_stories` floor; `canViewStory` denies when floor is Nobody. Path: `src/backend/story/internal/grpcsvc/story.go` (`capCreateStoryVisibility`, `canViewStory`).
- [ ] **[Story] Anonymous view leaks viewer in NATS** — `MarkViewed` always calls `PublishStoryViewed` with `viewer_profile_id` even when `anonymous=true`; contradicts [stories.md](../features/stories.md) §Анонимный просмотр. Paths: `src/backend/story/internal/grpcsvc/story.go`, `src/backend/story/internal/storyevents/jetstream.go`.
- [ ] **[Story] No `media_file_id` ownership / story-context validation** — any UUID accepted; video duration checked only when File client is wired. Path: `src/backend/story/internal/grpcsvc/story.go` (`CreateStory`, `CreateLookingForParty`); File story context exists in `src/backend/file/internal/grpcsvc/file_grpc.go` but Story does not enforce it.
- [ ] **[Story] Feed degrades to global scan when Social fails** — `GetStoryFeed` falls back to `ListActiveStoriesPaginated` (all active rows) if `ListFeedAuthorIDs` errors; only post-filtered by `canViewStory`. Path: `src/backend/story/internal/grpcsvc/story.go` (`GetStoryFeed`); related: `src/backend/story/internal/privacy/friends.go`, `src/backend/gateway/compose_stories_degradation_live_test.go` (checks liveness only).
- [ ] **[Story] `DeleteStory` orphans R2 media** — soft-delete only; purge worker targets `expired_at IS NOT NULL`, so early-deleted stories never reach `RunArchivePurgeOnce` / `FileDeleter`. Paths: `src/backend/story/internal/store/store.go` (`DeleteStory`), `src/backend/story/internal/jobs/jobs.go`.
- [x] **[Story] Moderation cannot hide stories from feeds** — `HideStoryFromFeed` + `hidden_from_feed_at`; non-author feed/GetStory filtered. Residual: Moderation `ResolveReport` does not yet call Story hide RPC. Paths: `src/backend/story/`; `protos/voice/story/v1/story.proto`.
- [ ] **[Story] GET reactions REST missing** — gRPC `GetStoryReactions` exists; Gateway only maps `POST …/reactions`. Flutter `getStoryReactions` GET will 404. Paths: `src/backend/gateway/transcode_stories.go`; `src/frontend/lib/backend/stories_client.dart`.

### Voice


- [ ] **[Voice] Unimplemented gRPC (proto + gateway exposed, server returns `Unimplemented`)** — `MoveToVoiceRoom`. Required by `voice-chat.md` (room moves). Commander / raise-hand / GrantFloor shipped (П.11 / VC-07 product).
- [ ] **[Voice] Один active voice session на профиль across devices** — спека [platforms.md](../features/platforms.md); steal/kick first не дожат.
- [x] **[Voice] S2S deps declared in spec but not wired in `main.go`** — **done (wire):** `voice/main.go` sets `Roles` (`ROLE_GRPC_ADDR`), `SpacePro` (`SUBSCRIPTION_GRPC_ADDR`), `SpaceMembers` (`SPACE_GRPC_ADDR`) when env present; compose sets all three. Remaining gaps: speak/mute role bits, roster NATS events (sibling Voice bullets).
- [ ] **[Voice] Missing NATS events vs `voice-service.md` / Analytics** — never published: `voice.call_started`, `voice.participant_joined`, `voice.participant_left`. Publisher surface stops at incoming/accepted/declined/missed/ended/state/screen-share. Analytics adapter expects `call_started`.
- [ ] **[Voice] Space voice join/leave publishes no roster events** — no `participant_joined` / `participant_left` / `voice.state_changed` on `JoinVoiceRoom` / `LeaveVoiceRoom`; Realtime consumer has no handlers for those subjects anyway.
- [ ] **[Voice] Staging LiveKit WebRTC likely broken without ops beyond WS smoke** — `deploy/staging/infra.yaml` sets `use_external_ip: true` but no `node_ip` (compose uses explicit `node_ip: 127.0.0.1` in `deploy/livekit/livekit.yaml`). Signaling is `wss://` via Ingress; RTC is NodePort **30881/TCP + 30882/UDP** on the node — not validated by `scripts/staging/smoke-staging.sh` (WS probe only).
- [ ] **[Voice] No LiveKit Server SDK room lifecycle** — docs say create/close rooms via SDK; implementation only mints JWT (`internal/livekit/token.go`). Rooms rely on implicit LiveKit auto-create; no explicit teardown.

### Auth


- [ ] **[Auth] OTP Redis throttling not implemented in Auth** — `docs/ARCHITECTURE_REQUIREMENTS.md` assigns OTP attempt throttling to Auth Redis; only JWT blacklist is wired. Password-reset REST **есть** (`POST /api/v1/auth/password/reset`, `OtpService.resetPassword`); Flutter UI нет — [client.md](client.md).
- [ ] **[Auth] Resend на staging/prod** — `ResendMailSender` есть; без `RESEND_API_KEY` → `NoopMailSender`. [ci.md](ci.md).
- [x] **[Auth] NATS `user.guest_converted` not wired in compose/staging** — **done (compose):** `AUTH_NATS_URL` + `depends_on: nats` in `docker-compose.yml`; convert publishes + `TestComposeConvertGuestNATS_live`. Staging env still worth verifying separately.
- [ ] **[Auth] Linked-accounts list is a stub** — `GET /api/v1/auth/linked-accounts` returns `[]` in both Auth REST and Gateway transcoding; `linked_identities` table unused by Java. Twitch OAuth mock-only (`mock-code`).
- [ ] **[Auth] Password change (logged-in) + revoke-all-refresh not implemented** — reset-via-OTP есть; нет change-password для сессии. UI reset — [client.md](client.md).

### Realtime


- [ ] **[Realtime] Subscription bootstrap limited to DM only** — `docs/microservices/realtime-service.md` requires auto-subscribe to all active chats, spaces, and friend presence; implementation only pages `ListChats` for `CHAT_TYPE_DM` in `dm_chat_lister_grpc.go` and registers those in `ws.go`. Groups/spaces rely on client `subscribe`; no friends-presence subscription model.
- [ ] **[Realtime] Live friend presence over WS is incomplete** — `presence_update` is fan-out to same-profile tabs and chat subscribers (`ws.go`, `ws_hub.go`). Friends not in a shared chat subscription do not receive live updates while WS is up (Flutter stops REST polling when connected — `src/frontend/lib/state/presence_providers.dart`). Proto has `PresenceChange` (`protos/voice/events/v1/jetstream_events.proto`) but User publisher has no presence subject (`user/internal/userevents/jetstream.go`) and Realtime has no `user_events` consumer.
- [ ] **[Realtime] In-app `notification` targets WS-subscribed profiles, not chat membership** — `in_app_notification_fanout.go` uses `hub.profileIDsSubscribedToChat(chatID)` as the recipient set. Connected group members who have not subscribed to that chat miss `notification` (and may miss `message_create` too).
- [x] **[Realtime] `delivery_ack` Redis cross-instance fanout** — **done:** `redis_fanout.go` publishes/consumes `fanoutMsgDeliveryAck`; `ws.go` publishes on client `delivery_ack`; compose live `TestComposeDeliveryReceipts_live`. Residual: dedicated cross-instance integration test — см. test-gaps ниже.
- [x] **[Realtime] JetStream `message.delivery_ack` publisher** — client `delivery_ack` → `message.delivery_ack` on `message.events` via `delivery_ack_publisher.go` (Batch 11); Messaging durable consumer **done (Batch 12)** → `last_delivered_message_id`.
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
- [ ] **[Subscription/Notification] Grace reminder delivery and duplicate safety remain incomplete — Subscription emits `subscription.grace_reminder` on D1/D3/D7 and sent-day bookkeeping suppresses ordinary sequential repeats, but publish-before-mark has no atomic claim or `Nats-Msg-Id`, so crashes/concurrent replicas can duplicate. Notification only validates recognized days, logs consumption, and ACKs; no push/email dispatch or client presentation.** — `src/backend/subscription/internal/sweeper/sweeper.go`; `src/backend/subscription/internal/store/grace_reminders.go`; `src/backend/subscription/internal/subscriptionevents/jetstream.go`; `src/backend/notification/subscription_events_consumer.go`
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


- [ ] **[Space] ChatLookup S2S hardening (Agent batch)** — set `x-voice-internal-caller=space` on Chat GetChat; Warn on lookup failures instead of silent skip; add unit/mock test for enrichment; optional batch GetChat to avoid N+1 (`chat_lookup.go`, `main.go`). Closed High wiring via PR #129.
- [ ] **[Space] No audit rows for tree CRUD, invite revoke, space settings, role changes (spec lists these)** — `src/backend/space/internal/store/tree.go`, `src/backend/space/internal/grpcsvc/invites.go`
- [ ] **[Space] `RevokeInvite` / `ListInvites` owner-only — `CreateInvite` uses `SpaceManageInvites`; revoke/list use `requireSpaceOwner`** — `src/backend/space/internal/grpcsvc/invites.go`
- [x] **[Space] Invite/public join membership event** — both join paths call `finalizeMembership`, which publishes `space.member_joined` after a new membership is created (`invites.go`, `join.go`, `spaceevents/jetstream.go`).
- [x] **[Space] Kick/leave membership event** — `KickMember` and `LeaveSpace` publish `space.member_left` after removal (`members.go`, `join.go`, `spaceevents/jetstream.go`). Residual Space event gaps remain in the dedicated NATS item above.
- [ ] **[Space] No gateway REST for leave/join-public/delete/transfer/audit/templates** — `src/backend/gateway/transcode_spaces.go`, `transcode_spaces_members.go`
- [ ] **[Space] Flutter client gaps — `spaces_client.dart` has no leave/join-public/transfer/audit/delete** — `src/frontend/lib/backend/spaces_client.dart`
- [ ] **[Space] Test holes — no integration tests for unimplemented RPCs; tree update/delete/category update/voice update/delete/RemoveTreeNode thin coverage** — `src/backend/space/internal/grpcsvc/*_integration_test.go`
- [ ] **[Space] Stale README still says “scaffold / out of scope”** — `src/backend/space/README.md`
- [ ] **[Space] `logInviteEventFailure` no-op — publish failures silently dropped** — `src/backend/space/internal/grpcsvc/invites.go`

### Moderation


- [x] **[Moderation] Service README status** — describes the implemented core and keeps residual gaps explicit.
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
- [ ] **[Social] Flutter client surface incomplete** — `friends_client.dart` now has contacts/favorites (**Batch 23b**), `listBlocked`/`unblockAccount` + Blocked tab (**Batch 24a**), `syncPhoneContacts` stub + Contacts tab action (**Batch 25a**), QR add friend UI (**Batch 26a**). Gateway exposes phone sync (`transcode_friends.go`). **Deferred:** live camera QR scanner (paste link works).
- [ ] **[Social] No live/E2E for friend-request privacy denial** — `privacy_actions_e2e_live_test` / `compose_privacy_actions_live_test.go` exercise DM/calls/files, not `POST /api/v1/friends/invitations`.
- [ ] **[Social] Stale service README** — `src/backend/social/README.md` still claims health-only scaffold; contradicts implemented gRPC + migrations.

### User


- [ ] **[User] `SearchProfiles` ignores discoverability privacy** — no `allow_friend_requests` / phone-search enforcement (`src/backend/user/internal/grpcsvc/user_search.go`); comment still references pre-privacy DDL.
- [ ] **[User] `UpdateProfile.custom_status` ignored** — comment "not persisted in current DDL" (`src/backend/user/internal/grpcsvc/user.go`); only Redis presence path works.
- [ ] **[User] Org DNS verification lifecycle thin** — unlimited pending rows, no expiry/TTL (`src/backend/user/internal/store/verification.go`).
- [x] **[User] `README.md` status** — describes the implemented RPC surface and keeps residual gaps explicit (`src/backend/user/README.md`).

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
- [x] **[Role] No live E2E for voice room overrides — store/grpc tests exist; no compose/Flutter E2E for `VOICE_JOIN` deny (UI strings exist).** — **done (VOICE_JOIN deny):** `TestComposeVoiceJoinDeny_live` + Flutter VOICE_JOIN deny (#14).
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
- [ ] **[Cross-cutting] Distributed tracing absent** — v1 uses `request_id` in logs; deferred per [ADR 003](../adr/003-distributed-tracing-deferred.md). — `docs/features/observability.md`, `deploy/observability/`

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
- [ ] **[Story] `AddToHighlight` allows active stories** — spec says “from archive”; store only checks author ownership, not `expired_at`. Path: `src/backend/story/internal/store/store.go` (`AddToHighlight`).
- [ ] **[Story] Archive purge worker delayed first run** — 24h ticker, no startup `RunArchivePurgeOnce`. Path: `src/backend/story/internal/jobs/jobs.go` (`StartArchivePurgeWorker`).
- [ ] **[Story] Weak content validation** — no required `text_content` for `text`, no required `media_file_id` for `photo`/`video`. Path: `src/backend/story/internal/grpcsvc/story.go` (`CreateStory`).
- [ ] **[Story] `game_tag` unvalidated** — free string, no Matchmaking catalog lookup. Path: `src/backend/story/internal/grpcsvc/story.go`.
- [ ] **[Story] No compose/live E2E for LFP** — Gateway unit test only. Paths: `src/backend/gateway/transcode_stories_test.go`; no `compose_*lfp*` / Flutter LFP create flow in CI ([`.github/ci/e2e-features.yml`](../../.github/ci/e2e-features.yml)).
- [ ] **[Story] Stale service README** — still says “scaffold”. Path: `src/backend/story/README.md`.

### Voice


- [x] **[Voice] `GetVoiceStates` populates commander/floor fields** — `is_commander`, `hand_raised`, `has_floor`, `is_broadcasting` in store + GetVoiceStates + state events (П.11 / VC-07).
- [ ] **[Voice] DM `StartCall` skips chat membership check** — `ensureChatMember` used for group/space paths only; DM only checks privacy + callee. Wrong `chat_id` can be attached.
- [ ] **[Voice] `RedisCallStore` has zero unit/integration tests** — staging/prod use Redis (`VOICE_REDIS_ADDR`); all store tests hit `MemoryCallStore`.
- [ ] **[Voice] Group voice cap mismatch in microservice doc** — `voice-service.md` mentions groups up to **500**; code hard-caps room at **32** (`MaxGroupVoiceParticipants`). Tests document 32 as intentional (`voice_grpc_group_test.go`); doc is stale.
- [ ] **[Voice] E2E coverage gaps vs PLAN “shipped”** — present: DM signaling (`TestComposeVoiceCall1to1_live`), optional bidirectional audio (`compose_voice_call_media_live_test.go`), Flutter `group_voice` / `spaces_voice` / `screen_share` API tests. Missing: compose live test for **space** voice + screen share with Role guard; no staging **RTC/media** smoke; `group_voice` E2E never exercises `LeaveCall` multi-participant behavior.
- [ ] **[Voice] `ListExpiredRinging` on Redis uses `KEYS`** — `voice:call:*` scan; risky under load.
- [ ] **[Voice] Stale service README** — still says “scaffold / out of scope” while PLAN marks voice shipped.

### Auth


- [ ] **[Auth] NATS event matrix mostly unimplemented** — `docs/microservices/auth-service.md` lists `user.registered`, `user.logged_in`, `user.logged_out`, `user.2fa_enabled`, `user.account_deleted`, `user.account_restored`; `AuthEventPublisher` only defines `user.guest_converted`. Files: `src/backend/auth/src/main/java/voice/backend/auth/events/AuthEventPublisher.java`, `src/backend/auth/src/main/java/voice/backend/auth/events/NatsAuthEventPublisher.java`.
- [ ] **[Auth] Disable 2FA not implemented** — No RPC/REST to turn off TOTP or invalidate backup codes after enrollment. Sessions list/revoke **есть** (`GET /api/v1/auth/sessions`, `TestComposeAuthSessions_live`); Flutter UI — [client.md](client.md).
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
- [x] **[Realtime] Module README status** — describes the implemented WebSocket/fan-out core and keeps residual gaps explicit (`src/backend/realtime/README.md`).

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
- [ ] **[Subscription] Premium cosmetic gaps outside Subscription module — e.g. custom status not persisted; anonymous view tracked separately in `docs/todo/backend.md`** — `src/backend/user/internal/grpcsvc/user.go`; `docs/todo/backend.md` (Anonymous view)
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
- [ ] **[Space] Transfer ownership 2FA/password — spec requirement; proto has bare `TransferOwnershipRequest` only** — `docs/features/spaces.md`, `protos/voice/space/v1/space.proto`
- [ ] **[Space] Member `nickname` in schema, no update RPC** — `src/backend/migrations/space_db/000001_init.up.sql`, `protos/voice/space/v1/space.proto`
- [ ] **[Space] QR join — product doc only, no Space API** — `docs/features/spaces.md`
- [ ] **[Space] Space-level `mm_config` for matchmaking — column exists, unused** — `src/backend/migrations/space_db/000001_init.up.sql`
- [ ] **[Space] `allow_guests` column (migration 000006) — only checked on invite join; no admin API to toggle** — `src/backend/migrations/space_db/000006_allow_guests.up.sql`, `src/backend/space/internal/store/invite.go`

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
- [ ] **[Analytics] gRPC ingest omits lag metric** — `IngestLag` only set on NATS path (`d:\Git\Voice\src\backend\analytics\internal\grpcsvc\ingest.go` vs `d:\Git\Voice\src\backend\analytics\internal\consumer\runner.go`).
- [ ] **[Analytics] `DeliverNew` only** — New durable consumers skip backlog (`d:\Git\Voice\src\backend\analytics\internal\consumer\runner.go`); acceptable per spec but limits replay/backfill.
- [ ] **[Analytics] Export CSV omits hashed IDs** — Export columns exclude `user_id_hashed` / `profile_id_hashed` (`d:\Git\Voice\src\backend\analytics\internal\grpcsvc\query.go`).
- [ ] **[Analytics] User-level activity gap** — `message_sent` hashes `profile_id` only; `user_id_hashed` empty → DAU/retention undercount messengers (`d:\Git\Voice\src\backend\analytics\internal\adapters\domain.go`).
- [ ] **[Analytics] Gateway health telemetry off by default** — `GATEWAY_ANALYTICS_SAMPLE_RATE` default 0 (`d:\Git\Voice\src\backend\gateway\analytics_telemetry.go`); health dashboard needs explicit enablement.
- [ ] **[Analytics] CH schema doc naming** — Docs use `user_id`/`profile_id`; DDL uses `user_id_hashed`/`profile_id_hashed` (`docs/microservices/analytics-service.md` vs `d:\Git\Voice\docker\clickhouse\init\001_events.sql`).

### Role


- [x] **[Role] Server comment** — `RoleGRPC` now identifies the implemented service without stale red-phase wording. — `src/backend/role/internal/grpcsvc/server.go`
- [x] **[Role] README status** — describes the implemented role and permission surface. — `src/backend/role/README.md`
- [ ] **[Role] `NamesFor` order non-deterministic — map iteration; awkward for UI/tests expecting stable lists.** — `src/backend/role/permissions/permissions.go`
- [ ] **[Role] No test for dual-scope effective mask — chat + `voice_room_id` together in one `GetEffectiveMask` call.** — `src/backend/role/internal/store/`, `internal/grpcsvc/` tests
- [x] **[Role] `CreateRole` hierarchy validation — non-owner with `SPACE_MANAGE_ROLES` may create only below their top role; equal/higher denial leaves no role or event, Owner bypass is covered.** — `src/backend/role/internal/grpcsvc/roles.go`, `roles_manage_integration_test.go`
- [ ] **[Role] Guest role under-exercised — default join falls back to Member; Guest mask (`SPACE_VIEW` only) rarely applies unless `SetDefaultJoinRole` points to Guest.** — `src/backend/role/permissions/permissions.go`, `internal/store/roles.go`

### Cross-cutting


- [x] **[Cross-cutting] `subscription/README.md` status** — implemented billing/limit paths and the remaining product-stub gaps are explicit. — `src/backend/subscription/README.md`, `docs/PLAN.md`
- [ ] **[Cross-cutting] Analytics “partial” = server-side only by design — client RUM explicitly out of scope (`docs/features/analytics.md`); not a bug, but PLAN “partial” is architectural, not a missing backend slice.** — `docs/features/analytics.md`, `src/backend/analytics/`
- [ ] **[Cross-cutting] Notifications partial — push device creds / staging FCM already in TODO Critical/High; in-app + Realtime fan-out exist (`realtime/in_app_notification_fanout_test.go`).** — `docs/PLAN.md`, `src/backend/realtime/`

### Notification


- [ ] **[Notification] `platform_enum` in proto ignored — `RegisterDevice` only uses string `platform`.** — `protos/voice/notification/v1/notification.proto`, `src/backend/notification/internal/grpcsvc/server.go`
- [ ] **[Notification] Unauthenticated debug push recorder** — `/debug/recorded-pushes` exposes last recorded push by `profile_id` (compose/dev aid). — `src/backend/notification/debug_http.go`
- [ ] **[Notification] DEPLOYMENT doc drift — references `internal/apns/config.go` (file is `http_sender.go`) and `APNS_PRIVATE_KEY` as canonical env name.** — `docs/DEPLOYMENT.md`, `src/backend/notification/internal/apns/http_sender.go`
- [x] **[Notification] gRPC server comment** — identifies the implemented service without stale stub wording. — `src/backend/notification/internal/grpcsvc/server.go`

### Federation


- [ ] **[Federation] Accidental Gateway REST proxy** — `src/backend/gateway/config_test.go` allows `federation` in `GATEWAY_REST_UPSTREAMS_JSON`, but `routing_test.go` blocks public paths; low risk unless someone adds transcoding routes without review.
- [ ] **[Federation] Generated-only Flutter surface** — `src/frontend/lib/gen/voice/s2s/v1/*`; no `lib/` product code for federation.
- [x] **[Federation] Repo junk in service dir** — removed `$prof` and `coverage` from git; added to `.gitignore`.
- [ ] **[Federation] No buf/gateway public API** — S2S protos correctly isolated under `protos/voice/s2s/v1/`; no accidental client exposure via REST transcoding.

### Voice


- [ ] **[Voice] Redis key layout differs from `voice-service.md` model** — docs describe `voice:session:{profile_id}` object + room sets; code uses `voice:session:{profile_id}` → `room_id` pointer + JSON blob `voice:call:{room_id}`.
- [ ] **[Voice] LiveKit JWT minimal grants** — only `video.roomJoin` + `room`; no explicit `canPublish` / `canSubscribe` (works in compose media test, but less explicit than LiveKit best practice).
- [ ] **[Voice] Commander / raise-hand client surface is proto-only** — generated Dart gRPC stubs exist; no `lib/` product usage beyond `lib/gen/`.

### Auth


- [ ] **[Auth] IP logging for audit not implemented** — `docs/microservices/auth-service.md` § “IP logging”; `HttpAccessLogFilter` logs method/path/status only, no client IP. Files: `src/backend/auth/src/main/java/voice/backend/auth/web/HttpAccessLogFilter.java`, `docs/microservices/auth-service.md`.
- [ ] **[Auth] Internal gRPC has no caller authorization** — `ResolvePhoneHashes`, `SetAccountStatus` callable by any mesh peer without S2S auth. Files: `src/backend/auth/src/main/java/voice/backend/auth/grpc/AuthGrpcService.java`, `src/backend/moderation/internal/authclient/authclient.go`.
- [ ] **[Auth] Compose dev 2FA bypass enabled** — `AUTH_TOTP_TEST_BYPASS: "true"` in `docker-compose.yml`; acceptable for local E2E but must not leak to staging/prod manifests (staging yaml omits it — OK).

### Realtime


- [x] **[Realtime] Coverage artifacts committed** — удалены ровно шесть tracked-артефактов (`$prof`, `coverage`, `coverage_profile`, `coverage_profile.out`, `notif_cov`, `notif_cov.out`) из `src/backend/realtime/`; `.gitignore` теперь предотвращает их повторное появление.
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
