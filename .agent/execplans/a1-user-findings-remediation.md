# ExecPlan: A1 user-reported acceptance findings

## Purpose

Resolve the current owner-reported A1 remediation slices, then continue exact-SHA
CI and staging acceptance without resetting or wiping staging data. Keep Space
hierarchy expansion out of scope; A1 covers category controls and the
API-backed text-chat create flow in an existing Space.

## Context and sources

- User findings: `C:/Users/Sergey/.codex/attachments/74f668ec-03bc-4f1a-8d71-d033642a74b8/Pasted text.txt`.
- Product/architecture: `docs/PLAN.md` A1 DoD; `docs/features/privacy.md`;
  `docs/features/text-chat.md`; `docs/microservices/chat-service.md`;
  `docs/microservices/user-service.md`; `docs/ARCHITECTURE_REQUIREMENTS.md`;
  `docs/TESTING.md`; `docs/REPOSITORIES.md`.
- User explicitly requires a real nickname on an already existing DM row while
  blocked. This narrows the privacy UX contract: profile details and blocked
  messages remain hidden; only the title of that existing DM remains usable.
- Existing local batch on PR #596 already addresses friend-removal fanout,
  inbox/unread reconciliation, invite origin/routing,
  regular-account onboarding, attachment download, and directional blocked
  group/channel history and Realtime delivery. Treat all as local until safely
  deployed and rechecked on staging.
- The archive action is available in each chat's row overflow and has passed a
  local check. Keep staging discoverability acceptance open;
  do not list archive as a missing capability.
- The owner has now requested that a pending-email account which logs in after
  password reset receive a verification resend automatically. This is a new
  requested behavior update, not a previously established product contract.
  Reuse the existing Auth resend throttle and document the new behavior with
  the relevant feature contract when implementation is ready.
- Latest exact-head CI failure is Messaging integration: group/channel history
  now performs account-owner lookups for the documented block visibility
  policy, but older tests still assert that all non-DM account mapping is
  forbidden; a channel history fixture lacks the required block policy and
  correctly fails closed.
- Staging deploy remains gated by the active-generation NATS ACL proof and
  state-preserving migration evidence. Do not change proof variables, rotate
  NATS roots, reset namespaces, or wipe data to bypass the gate.

## Acceptance criteria

1. Resolve the pasted findings across friend removal and request delivery,
   inbox/unread/read reconciliation, invite generation/join,
   regular-account onboarding, attachment download, and directional block
   visibility. Keep each item classified as local implementation, local
   verification, or staging acceptance; a local fix or passing test is not
   staging proof.
2. Incoming notification delivery does not duplicate an inbox item or leave
   stale unread/preview state when the selected chat is already open. Preserve
   the regression's red-then-green evidence in the local test record.
3. Category controls and Space text-chat creation use the documented APIs and
   complete an API-backed text channel create flow. Keep tests-in-progress
   classified as unverified until they pass. Canonical tree reorder and
   category placement are being added. Category rename/delete remain
   unsupported through API Gateway because PATCH/DELETE routes are absent,
   despite the Space protobuf RPCs and target documentation; keep that gap
   explicit unless those routes are implemented.
4. A web invite URL survives unauthenticated login and nickname completion and
   joins the original invite afterward; native sharing uses the configured
   application origin. Test both retention and resulting membership.
5. A password-correct login for an email-pending account following password
   reset automatically resends verification through Auth's existing throttle.
   Treat this as the owner's newly requested product behavior and retain the
   existing rate-limit guarantees.
6. An existing DM row and header keep the peer's actual nickname after either
   direction of block; public profile/search privacy remains enforced.
7. A blocked sender's group/channel history and realtime events stay hidden in
   the viewer-to-sender direction, and unblocked events/messages remain visible.
8. Unblock updates the affected client state without a full page reload; verify
   both Friends/profile presentation and any already-open shared-chat history.
   Blocking a peer while a shared chat is open must scrub that peer's cached
   messages fail-closed when the filtered REST history refresh fails or the
   client is offline. This cache-scrub fix is being implemented and needs a
   regression that proves blocked messages disappear without a successful
   network refresh.
9. Group/channel visibility checks use a Social-owned profile-pair block
   checker, but non-DM history never runs User profile-owner/Auth deleted-
   account DM lifecycle lookups or returns DM-only peer state. Missing Social
   policy dependencies continue to fail closed.
10. Update the privacy and service contract docs, generated protobuf sources,
   and the A1 evidence checklist together with implementation. Keep untested
   acceptance items open, including reset-password resend/delivery, invite
   acceptance from another account, attachment download/hash, read-cursor and
   reconnect/history proof, archive discoverability in staging, and
   soft-delete/recovery.
11. Continue remaining acceptance after CI: test only against the current staging
   build until a safe exact-SHA deployment path is available; do not claim local
   or CI checks as staging acceptance.

## Scope and likely files

- Chat/User/Social protobuf contracts and generated Go/Flutter sources; Chat
  ListChats service/adapters/tests; User internal title-only lookup/tests;
  Social profile-pair block lookup and User adapter; Messaging S2S adapter,
  filter, runtime wiring, and integration tests; Flutter list model, mapping,
  title renderer, and regression tests.
- Messaging integration fixtures/assertions that preserve the non-DM no-
  User/Auth lookup contract while testing the new Social privacy seam and its
  fail-closed behavior.
- Flutter social action invalidation and chat-history cache refresh only if
  trace confirms it is necessary for the reported no-reload unblock symptom.
- `docs/features/privacy.md`, relevant Chat/User/Messaging service docs, and
  `docs/todo/a1-staging-acceptance-2026-09-27.md`.

## Red-green sequence

1. Add an incoming-notification regression that first demonstrates the duplicate
   or stale selected-chat inbox state (RED), then passes after reconciliation
   (GREEN). Preserve the focused test evidence for both states.
2. Add category and Space text-chat control tests against the existing APIs.
   Exercise the complete text-channel create flow through API responses; the
   current test work remains in progress until GREEN.
3. Add invite-flow coverage that starts from a web invite URL, crosses an
   unauthenticated login and nickname step, and then joins the original invite.
   Verify native share uses the configured origin rather than a fixed host.
4. Add Auth login coverage for a password-correct, email-pending account after
   password reset. Verify automatic resend uses the existing throttle and its
   rate-limit response; this behavior is newly requested by the owner.
5. Add a narrow DM-title regression that fails when public `GetProfile` is
   unavailable, while existing DM membership still supplies the true title;
   test both participant directions and non-member rejection at the service
   boundary. Run the focused tests RED.
6. Add contract-first tests for a directional Social profile-pair block lookup
   and a Chat-only title lookup. Generate contracts, confirm the new tests fail
   at the unimplemented RPCs, then implement Social account resolution via its
   protected User profile adapter and a title-only User RPC that only Chat can
   call. Chat requests titles only for peers in authorized DM rows. Regenerate
   all outputs and rerun focused Go and Flutter tests.
7. Add a no-reload unblock regression for the exact stale surface found during
   code tracing. If the stale data is cached message history, assert that
   invalidation/refetch happens without exposing content while the block is
   active. Run focused tests RED, implement, then GREEN.
   Add a corresponding block/offline regression: seed cached shared-chat
   messages from the blocked peer, fail the REST refresh, and assert the cache
   is scrubbed before rendering.
8. Replace Messaging's User profile-to-account + account-block history path
   with the Social profile-pair checker. Keep the non-DM no-User/Auth lookup
   assertions intact. Wire explicit allow-all Social test policy into generic
   channel fixtures; keep missing-dependency tests fail closed. Run targeted
   Messaging tests and then the package.
9. Run scoped frontend/backend/protobuf verification, update Graphify, inspect
   the full diff, commit and push to PR #596, then wait for exact-head CI.

## Validation

- Short Go suites for Chat, Social, User, Messaging and Realtime; affected
  focused tests and package suites.
- Flutter CI (1,239 passed, 98 skipped), targeted analysis, Space and privacy
  regressions, `make buf-ci buf-breaking`, and `git diff --check`.
- `make ci-script-tests` was attempted but cannot run on this host because
  `jq` is not installed (`scripts/ci/resolve-staging-matrix.sh` requires it).
- Generated protobuf sync and exact-head CI remain pending.
- GitHub PR #596 exact-head checks; staging acceptance only after an approved
  data-preserving deploy at the exact SHA and live browser verification.

## Progress

- [x] Read the pasted findings and repository instructions; review product,
  service, testing, and contribution sources.
- [x] Resolve asymmetric friend removal and live inbox/request/read-state
  reconciliation; focused Flutter/provider tests pass.
- [x] Preserve invite links through auth/profile gates, use the configured
  application origin, and add the resulting Space join routing; focused tests
  and targeted analysis pass.
- [x] Auto-resend verification after password-correct login for an
  email-pending account using Auth's existing throttle; Auth tests pass. Live
  mail delivery remains staging-only proof.
- [x] Avoid showing the guest save-profile prompt to regular accounts; focused
  onboarding tests and targeted analysis pass.
- [x] Add Space category creation, API-backed text-chat creation, category
  placement, and permitted tree reorder. Client/widget tests cover uncertain
  network outcomes and duplicate prevention; focused analysis passes.
- [x] Add attachment download tap handling; widget tests and targeted analysis
  pass. Staging download/hash/restart proof remains open.
- [x] Add title-only authorized DM peer names when blocked, and directional
  block filtering for group/channel history and Realtime. Social owns the
  profile-pair block decision; missing policy dependencies fail closed.
- [x] Fix unblock refresh and blocked-message cache leakage with a shared
  serialized cache mutation barrier across profiles/rooms; focused cache and
  privacy suite passes (37 tests).
- [x] Run integrated local checks: short Go suites for Chat/Social/User/
  Messaging/Realtime; `make buf-ci buf-breaking`; `make flutter-ci` (1,239
  passed, 98 skipped); focused cache/privacy and Space suites; targeted Flutter
  analysis; `git diff --check`.
- [x] Regenerate Go/Dart protobuf outputs; `make buf-go-pb-check` confirms Go
  outputs match the proto sources. `make buf-ci buf-breaking` passed before
  commit.
- [x] Push the initial implementation batch to PR #596 as `6d1310ab9` and
  evidence updates as `4db8b6322`.
- [x] Use exact-head CI feedback to fix stale unused Messaging test doubles,
  explicitly allow Social policy in ordinary direct-service integration
  fixtures, and keep a failed inbox page/cursor intact until explicit retry
  after MarkRead or resumed activity. The inbox regression failed before the
  fix and passes afterward.
- [x] Rerun `make golangci-ci` across all 20 Go modules; `make flutter-ci`
  (1,239 passed, 98 skipped); focused InboxReconciler (16), notifications and
  chat (40), Messaging short tests and lint; and Graphify update.
- [ ] Commit/push this CI-feedback batch and obtain exact-head CI. The previous
  code run `36935069635` was superseded while the docs-only commit was pushed;
  its findings are resolved locally but need confirmation on a fresh head.
- [ ] Continue live acceptance only against the deployed exact SHA after the
  state-preserving NATS gate is resolved.

The active-generation NATS proof and state-preserving migration gate still
block staging deployment. Do not promote local tests or the current deployed
build's observations into staging acceptance. The current staging build still
shows the known regular-account profile setup prompt, confirming that the local
fix has not been deployed.

## Risks and decisions

- Public `GetProfile` intentionally denies blocked visibility; do not use
  `GetProfiles` or the ownership-only resolver as a privacy bypass. The new
  User method returns only the display name, requires exact internal caller
  `chat`, and Chat supplies only peer IDs from authorized DM memberships.
- The block store is account-scoped. Social resolves profile ownership through
  its existing protected User adapter, evaluates the directional account
  block, and returns only a boolean to Messaging; Messaging must not learn the
  account IDs or widen its User lifecycle lookup contract.
- Staging cannot receive this batch until NATS state-preservation evidence is
  satisfied; work on source, tests, and read-only staging checks continues.
