# Analyst 2 — Correctness Review (Batch-loop PRs ~#74–#114)

**Date:** 2026-09-02  
**Scope:** Implementation vs `docs/features/`, `docs/microservices/`, `docs/ARCHITECTURE_REQUIREMENTS.md` for admin, chat folders/QA, R3-A04 message requests, social, messaging `content_type`, notifications `message_request`, appeals, auth sessions, client navigation.  
**Git:** `git pull origin master` blocked by local Cursor hook (`block-history-rewrite.js`); review is against the current workspace tree.

---

## Summary (pass / partial / fail by domain)

| Domain | Verdict | Targeted tests |
|--------|---------|----------------|
| Chat — folders, Quick Access, archive, DM inbox (R3-A04) | **PASS** | `go test ./internal/grpcsvc/...` OK; `go test ./internal/store/...` OK |
| Messaging — `content_type`, pin limit (5), shadow send | **PARTIAL** | `go test ./internal/grpcsvc/...` OK; Forward path gap (see findings) |
| Social — contacts/favorites REST, request status | **PASS** | `go test ./internal/grpcsvc/...` OK |
| Notifications — `message_request` routing | **PARTIAL** | `delivery`/`pushcopy` OK; full package blocked (TLS/module fetch) |
| Realtime — `message_request` WS fanout | **PARTIAL** | Code + unit test present; `go test` setup failed (missing prometheus in module) |
| Moderation — appeals (`SubmitAppeal`) | **PASS** | `go test ./internal/grpcsvc/...` OK; business rules implemented |
| Moderation — shadow ban + sanction push | **PARTIAL** | Send-path integration test exists; staging wiring + forward gap |
| Auth — sessions list/revoke, delete-account UI | **PASS** | Flutter `active_sessions_screen_test` + `security_settings_screen_test` patterns verified |
| Admin — queue resolve/dismiss, OAuth assign-to-me | **PASS** | `npm test -- --run` 75/75 |
| Client — navigation / message requests / folders UI | **PARTIAL** | Flutter provider tests OK; mobile chrome / LRU polish still open per `client.md` |
| Gateway — REST transcoding (folders, friends, appeals) | **NOT RUN** | `go test` in `gateway/` fails (missing gRPC/protobuf modules in local env) |

**Totals:** 5 PASS · 5 PARTIAL · 0 hard FAIL on reviewed behavior · 1 NOT RUN (gateway local env)

---

## Findings

| Severity | Area | Issue | Spec reference | Code path | Suggested fix |
|----------|------|-------|----------------|------------|---------------|
| **High** | Messaging / moderation | `ForwardMessage` calls `CheckMessageAllowed` but **never** `IsShadowBanned` / `ghost_only`; forwarded messages always publish `message.sent` and fan out to peers. Shadow-banned users can bypass ghost delivery via forward. | `docs/features/reports.md` § Shadow ban | `src/backend/messaging/internal/grpcsvc/messaging_grpc.go` (`ForwardMessage` ~1107–1177) | Mirror `SendMessage` shadow-ban branch: set `GhostOnly`, skip `PublishMessageSent` when banned; add integration test parallel to `messaging_platform_moderation_integration_test.go`. |
| **High** | Deploy / messaging | `MODERATION_GRPC_ADDR` wired in `docker-compose.yml` for messaging but **absent** from `deploy/staging/configmap-app.yaml` and messaging Deployment env. On staging/prod, `PlatformMod` stays nil → shadow-ban and spam-mute checks silently disabled. | `docs/features/reports.md`; `docs/MICROSERVICES.md` | `src/backend/messaging/main.go` (~233–247); `deploy/staging/services.yaml` (voice-messaging env) | Add `MODERATION_GRPC_ADDR: voice-moderation:9090` to shared configmap (and prod); verify messaging pod env in smoke. |
| **Medium** | Notification / moderation | `moderation.events` consumer exists but `routeModerationNotification` calls `HandleSanctionApplied` with **empty** `recipientProfileID` → handler returns empty decision; comment admits push deferred. Users get no in-app/push on sanctions despite Batch consumer shipping. | `docs/features/reports.md` § Санкции; `docs/features/notifications.md` | `src/backend/notification/moderation_events_consumer.go` (~88–99); `internal/consumer/moderation_events.go` | Resolve `account_id` → profile via User S2S before dispatch; wire to `MessagePusher` like `message_events` consumer; add unit test with non-empty profile. |
| **Medium** | Chat / DM requests | When `SOCIAL_GRPC_ADDR` unset, `Friends`/`Contacts` are nil; `recipientInboxBucket` treats everyone as stranger (`requests`). Real friends/contacts mis-bucket until Social is wired (compose/k8s usually OK). | `docs/features/friends.md` § Незнакомец; `docs/features/text-chat.md` | `src/backend/chat/internal/grpcsvc/dm_inbox.go`; `src/backend/chat/main.go` (~66–93) | Fail-closed: return error from `EnsureDM` when Social client nil (mirror Social fail-closed pattern), or document + enforce `SOCIAL_GRPC_ADDR` in k8s readiness. |
| **Medium** | Messaging / forward | `insertForwardCommentary` always publishes `message.sent` with no shadow-ban or `PlatformMod` check. | `docs/features/reports.md` | `messaging_grpc.go` `insertForwardCommentary` (~1182–1206) | Apply same moderation gate as main send before insert/publish. |
| **Medium** | Docs drift | `docs/todo/backend.md` § Critical still claims appeals lack business rules and shadow-ban does not cut fanout; `docs/features/notifications.md` L31 still says Realtime may emit `new_message` for requests. Code shows appeals validation + `message_request` fanout shipped. | N/A (inventory accuracy) | `appeals.go`; `realtime/in_app_notification_fanout_test.go` | Update todo/feature docs in a separate docs PR (out of scope for this analyst pass). |
| **Medium** | Docs drift | `docs/PLAN.md` Telegram-parity matrix (L25) still says pin limit **50** in code; implementation uses **5** (`MaxPinsPerChat`). | `docs/features/text-chat.md` | `src/backend/messaging/internal/store/pins_store.go` | Align PLAN matrix with code (5) to avoid false regression signals. |
| **Low** | Chat / DM requests | `AcceptDMRequest` / `DeclineDMRequest` gRPC handlers show **0** coverage in `chat/cover`; logic is thin but untested at handler layer. | `docs/features/friends.md` | `src/backend/chat/internal/grpcsvc/dm_requests.go` | Add grpcsvc integration test: accept → `inbox_bucket=main`, decline → `declined`, re-contact via `PromoteDeclinedDMRecipients`. |
| **Low** | Test infra | Gateway, realtime, and full notification packages did not run locally (missing modules / TLS to proxy.golang.org). Service-scoped tests for chat/messaging/moderation/social passed. | `docs/TESTING.md` | N/A | Run `make` / CI-equivalent module sync before local gateway sweep; not a product bug. |
| **Low** | Auth UI | Password-reset screen still missing (sessions UI shipped Batch 29b). REST exists. | `docs/features/auth-and-contacts.md` | `docs/todo/client.md` L40 | Track as known client gap; no backend correctness issue. |

---

## Verified-correct highlights

- **Message requests (R3-A04):** `recipientInboxBucket` routes strangers to `requests`, friends/contacts to `main`; `PromoteDeclinedDMRecipients` on `message.sent` implements re-contact after decline; unit tests in `dm_inbox_test.go` and `dm_request_recontact_integration_test.go`.
- **Notifications `message_request`:** Notification consumer maps `inbox_bucket=requests` → `TypeMessageRequest`; Realtime fanout test `TestInAppNotificationFanouts_MessageRequestForRequestsInbox` asserts WS type `message_request` (not `new_message`).
- **Appeals:** `SubmitAppeal` validates ownership (`sanction.TargetAccountID`), 7-day window, duplicate appeal (`AlreadyExists`), publishes domain event; Gateway `POST /api/v1/moderation/appeals` → 201.
- **Shadow ban (send path):** `SendMessage` sets `ghost_only`, suppresses NATS publish, filters peer `GetMessages`; covered by `TestPlatformModeration_ShadowBannedSenderMessageHiddenFromPeer`.
- **Chat folders / Quick Access:** Store limit 15 (`maxQuickAccessSlots`); folder CRUD + membership/pin + `ListChats.folder_id` + Gateway REST tests in `transcode_chats_folders_test.go` / `transcode_chats_quick_access_test.go`.
- **Messaging `content_type`:** Column migration `000012`; `resolveSendContentType` + `validateRichPayloadAttachments` for location/article; integration tests `messaging_content_type_integration_test.go`.
- **Pin limit:** `MaxPinsPerChat = 5` matches current spec (`PLAN.md` status table L69).
- **Auth sessions UI:** `ActiveSessionsScreen` lists/revokes via `GET/POST /api/v1/auth/sessions`; guest blocked on delete-account with snackbar (`security_settings_screen.dart` L127–132).
- **Admin moderation loop:** Vitest covers resolve/dismiss, assign-to-me from OAuth session JWT, appeals approve flow (`QueuePage.test.tsx`, `AppealsPage.test.tsx`, `ReportDetail.test.tsx`).
- **Social REST:** Gateway `transcode_friends.go` exposes contacts/favorites/blocks; `PendingFriendRequest.status` for declined outgoing requests (Batch 24b).

---

## Test execution log

| Command | Result |
|---------|--------|
| `go test ./internal/grpcsvc/... -short` (chat) | **PASS** (11.6s) |
| `go test ./internal/store/... -short` (chat) | **PASS** (1.7s) |
| `go test ./internal/grpcsvc/... -short` (messaging) | **PASS** (7.5s) |
| `go test ./internal/grpcsvc/... -short` (moderation) | **PASS** (2.9s) |
| `go test ./internal/grpcsvc/... -short` (social) | **PASS** (3.0s) |
| `go test -short ./...` (realtime) | **FAIL** setup (prometheus module) |
| `go test -short ./...` (gateway) | **FAIL** setup (gRPC/protobuf modules) |
| `go test -short ./...` (notification) | **PARTIAL** (`delivery`, `pushcopy` OK; cross-package TLS fetch errors) |
| `flutter test test/message_requests_providers_test.dart test/active_sessions_screen_test.dart` | **PASS** (3 tests) |
| `npm test -- --run` (admin) | **PASS** (75 tests) |

---

*Analysis only — no changes to `docs/todo` or product code.*
