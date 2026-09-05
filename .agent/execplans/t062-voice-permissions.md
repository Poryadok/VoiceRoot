# ExecPlan: T-062 Voice role-permission enforcement

## Purpose

Close the documented Voice Service authorization gap so a Space voice-room
participant cannot unmute/speak or acquire organizer/floor powers unless Role
Service authorizes the matching voice-room permission.  Role Service outages
must deny the protected action; a participant must still be able to mute
themself.  The issued LiveKit token must make the same publish decision, so an
otherwise admitted listener cannot bypass the Voice RPC through direct media
publication. Space owners retain the documented bypass because Role Service
returns their permission check as allowed.

## Context

- Docs: `docs/PLAN.md` marks voice chat partial and roles core-live, with voice
  permission enforcement still incomplete. `docs/microservices/role-service.md`
  defines `VOICE_SPEAK` as own audio and `VOICE_MUTE_OTHERS` as muting others,
  and defines the Owner effective-permissions bypass. `docs/microservices/voice-service.md`
  assigns Voice Service Role Service checks. `docs/features/roles.md` assigns
  organizers microphone and floor control. `docs/features/voice-chat.md`
  defines raid/school speak restriction and organizer-issued floor.
- UI contract: `docs/design/screen-controls.md` exposes self mute/unmute and
  only exposes participant mute to a caller with `VOICE_MUTE_OTHERS`.
- Backlog source: `docs/todo/backend.md` identifies the exact remaining gap:
  Voice wires Role Service and enforces join/share, but not `VOICE_SPEAK` or
  `VOICE_MUTE_OTHERS` on speak/mute paths.
- Code: `src/backend/voice/internal/grpcsvc/voice_grpc.go`,
  `voice_commander.go`, `role_guard.go`, and `voice_room.go`; Role gRPC adapter
  is `src/backend/voice/internal/s2s/role_permissions.go`. Existing test
  conventions are in `internal/grpcsvc/*_test.go` and
  `internal/s2s/role_permissions_test.go`.
- Constraints: no Docker, network, generated-proto change, or main-checkout
  mutation. Work is on `fix/voice-role-permission-enforcement` in clean WT9 at
  base `a3b06b52`.

## Scope

- In: Space voice-room `VOICE_SPEAK` enforcement on the explicit unmute state
  transition and the enabled commander broadcast path; `VOICE_MUTE_OTHERS`
  enforcement before enabling commander mode and before every floor-control
  mutation; unavailable or missing Role dependency fails closed for those
  protected actions; role-adapter coverage for the new permission.
- In: Preserve self-mute without a Role check and preserve existing DM/group
  behavior, which has no Space role context. Preserve Owner authorization by
  forwarding the regular Role Service check instead of adding a local owner
  exception.
- In: `GetJoinToken` for Space voice rooms maps a denied `VOICE_SPEAK` check to
  a LiveKit `canPublish=false` grant, preserves publish for an allowed/Owner
  response, and fails closed when the checker is absent or unavailable.
- Out: A new target-participant mute RPC (the proto has none), move-room
  implementation (currently unimplemented), UI wiring, and unrelated voice
  lifecycle work. Previously-issued LiveKit JWTs are not revoked before their
  configured TTL; the canonical docs do not define a token refresh/revocation
  contract.

## Inventory of ingress paths before tests

| Path | Mutation/capability | T-062 disposition |
| --- | --- | --- |
| `UpdateVoiceState` | Caller changes own mute/deafen/video state | Require `VOICE_SPEAK` only when `is_muted` is explicitly false in a Space voice room; self-mute remains allowed. |
| `GetJoinToken` | Mints LiveKit join token | For a Space voice room, `VOICE_SPEAK` allow/Owner gets `canPublish=true`; an explicit deny gets `canPublish=false`; missing/unavailable Role service fails closed. DM/group remains publish-capable. |
| `SetCommanderMode(enabled)` | Gives caller organizer flag | Require `VOICE_MUTE_OTHERS` for Space voice rooms before enabling. |
| `SetBroadcasting(enabled)` | Commander begins speaking/broadcasting | Require both `VOICE_MUTE_OTHERS` and `VOICE_SPEAK` for Space voice rooms before enabling. |
| `GrantFloor` / `RevokeFloor` | Mutates another participant's floor | Require `VOICE_MUTE_OTHERS` for Space voice rooms before mutation; do not accept existing initiator/commander state as a substitute. |
| `RaiseHand` / `LowerHand` | Caller changes own hand state | No other-participant mutation; no new permission gate. |
| `JoinVoiceRoom` / screen share | Join/share | Existing guards; retain their scope. |
| `MoveToVoiceRoom` | Would move participant | Server remains unimplemented, so it is not a bypass. |
| Gateway REST mappings | Transcode directly to the handlers above | No alternate authorization implementation; handlers remain the enforcement point. |

## Acceptance criteria

1. A Space voice-room participant denied `VOICE_SPEAK` cannot explicitly
   unmute, and no state is changed. The same participant may explicitly mute
   themself without a Role call.
2. A Space voice-room participant denied `VOICE_MUTE_OTHERS` cannot enable
   commander mode, start broadcasting, grant floor, or revoke floor; these
   mutations fail before the store update.
3. A missing or unavailable Role checker denies every T-062 protected Space
   action. DM/group calls keep existing behavior because they have no role
   scope.
4. The Role adapter sends `VOICE_SPEAK` and `VOICE_MUTE_OTHERS` with the
   `space_id`, `profile_id`, and `voice_room_id`; a Role allow (including the
   Owner response) succeeds without a local bypass.
5. Existing join/share and self-service paths continue to pass.
6. A denied Space participant receives no publish-capable LiveKit token; an
   allowed/Owner-shaped response receives `video.canPublish=true`; unavailable
   Role Service receives no token. DM/group token behavior remains publish-capable.

## Milestones

- [x] Read governing docs and inventory all direct gRPC, Gateway, and commander ingress paths.
- [x] Add documentation-derived regression tests only; demonstrate expected RED failures; commit the RED phase.
- [x] Obtain independent fresh test review and resolve any critical finding before production changes. The first review found P1 RED gaps (allow/Owner-shaped behavior, `EnsureMuteOthers` deny/unavailable adapter behavior, optional/non-Space state-patch regressions); the correction pass and fresh rereview found no P1/critical findings.
- [x] Implement the smallest checker/interface and handler guards; run focused GREEN tests and mutation checks.
- [x] Update the backlog wording/ExecPlan progress, commit GREEN, obtain fresh implementation review, and run bounded verification.

## Detailed steps

1. Extend only the voice Role-permission boundary with a speak check and test
   double support. Keep Role gRPC request metadata and voice-room override
   context consistent with `EnsureVoiceJoin`.
2. In a new or focused existing grpcsvc test file, construct Space voice-room
   calls and assert denied, unavailable, self-mute, allow/owner-shaped allow,
   and unchanged-state outcomes. Add commander/floor cases to
   `voice_commander_test.go`; add adapter request/error cases to
   `role_permissions_test.go`. Do not edit production code in this stage.
3. Run `CGO_ENABLED=0 go test` against the focused packages and preserve the
   failing assertions caused by the missing production guards. Commit the RED
   tests as an isolated commit.
4. Have a fresh reviewer check docs coverage, false positives, fixture realism,
   test-only diff, and whether all inventory paths are addressed.
5. Add the minimum production guards after `requireActiveCall`/`requireCall`
   has established the caller and call context. Gating must precede
   `UpdateVoiceState`. Map deny/unavailable into fail-closed gRPC errors using
   the service's existing screen-share style; do not special-case owner or
   permit a Space path when the checker is nil.
6. Add documentation-derived `GetJoinToken` grant tests: decode the issued JWT
   and check denied, allow/Owner-shaped allow, unavailable, and a non-Space
   regression. Commit these RED tests before changing the token issuer or RPC.
7. Have a fresh reviewer check the new RED tests for real grant decoding,
   denial-vs-unavailability coverage, and non-Space preservation.
8. Run the focused tests after each small change; run a bounded mutation
   check by locally negating/removing each new guard one at a time and observing
   a focused regression fail, restoring the exact source afterward with
   `apply_patch` only.
9. Update this plan and the specific `docs/todo/backend.md` residual wording
   to describe the closed T-062 gap without claiming unimplemented LiveKit or
   targeted-mute work. Commit the minimal GREEN implementation and docs.
10. Obtain a fresh independent implementation review. Fix every critical
   finding in a new commit; repeat focused checks and review if needed.

## Validation

- [x] `$env:GOPROXY='off'; $env:CGO_ENABLED='0'; go test ./internal/grpcsvc -run 'TestVoiceGRPC(.*Role|.*Speak|.*Commander|.*Floor|.*VoiceRoom)'`
- [x] `$env:GOPROXY='off'; $env:CGO_ENABLED='0'; go test ./internal/s2s -run 'TestGRPCRolePermissions'`
- [ ] `$env:GOPROXY='off'; $env:CGO_ENABLED='0'; go test ./...` from `src/backend/voice` — internal packages pass, but root package cannot resolve uncached `github.com/prometheus/client_golang` with network disabled.
- [x] `$env:GOPROXY='off'; $env:CGO_ENABLED='1'; go test -race ./internal/grpcsvc ./internal/s2s`
- [x] Mutation evidence: removal/negation of each final unmute, commander-enable, broadcast-speak, and floor-control guard made its focused regression fail before the exact source was restored with `apply_patch`.
- [x] `$env:GOPROXY='off'; go vet ./internal/grpcsvc ./internal/s2s`; `git diff --check`.

## Progress

- [x] WT9 lease created with holder `T-062-voice-permissions`; base and clean status verified before branch creation.
- [x] Created branch `fix/voice-role-permission-enforcement`.
- [x] Documentation read and ingress inventory completed.
- [x] RED tests added. The focused grpcsvc command fails because ungated Space
  mutations return OK instead of `PERMISSION_DENIED`; the s2s test is an
  intentional compile-RED because `EnsureVoiceSpeak` and its sentinel do not
  exist yet.
- [x] Fresh RED test review completed; three P1 test gaps were found.
- [x] Add reviewed RED gaps (allow behavior, `EnsureMuteOthers` adapter deny/unavailable, optional/non-Space state patches), then obtain a fresh follow-up test review. Production code remains untouched.
- [x] Implemented minimal GREEN guards only after accepted RED review: added `EnsureVoiceSpeak`, explicit-unmute and enabled-broadcast checks, and fail-closed Space commander/floor checks.
- [x] Focused GRPC + Role adapter tests, race tests, vet, and four guard mutations completed; full module test is blocked only by the uncached Prometheus dependency while network is prohibited.
- [x] GREEN commit and fresh implementation review. No P0/P1 findings; accepted
  and corrected the sole P2 stale test-double comment in a follow-up commit.

## Decisions

- Role checks apply only when the active call is a Space voice room with both
  `space_id` and `voice_room_id`; the repository has no Role scope for DM or
  group calls.
- Owner bypass is delegated to `RoleService.CheckPermission`, matching the
  documented effective-permissions algorithm; the Voice service must not infer
  ownership from initiator or commander state.
- An explicit `is_muted=false` is the observable unmute transition because the
  proto field is optional; absent mute patches are not treated as unmute.

## Risks and follow-ups

- LiveKit tokens currently lack explicit publish/subscribe grants. This task
  guards the documented service RPC state/commander paths only; enforcing media
  publication independently requires a compatible token-refresh design.
- There is no target-profile mute RPC in the current proto despite the UI
  control inventory. Such an RPC needs its own contract and TDD task.
- No Docker/network checks run by user instruction. Focused host Go tests and
  race tests are the bounded evidence.
