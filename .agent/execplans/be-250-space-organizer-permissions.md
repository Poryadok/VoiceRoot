# ExecPlan: Space voice organizer permission presentation

## Purpose and context

BE-250 retains a missing client slice: group organizer controls exist, but Space
controls do not expose current Role permission or denial. Canon:
`docs/features/voice-chat.md`, `docs/features/roles.md`,
`docs/microservices/role-service.md`, and the accepted T-062 gates recorded in
`docs/todo/backend.md`. The current Voice handler enforces `VOICE_MUTE_OTHERS`
for commander enable and grant/revoke floor, and both that permission and
`VOICE_SPEAK` for broadcast enable. Disabling commander/broadcast remains possible
after permission loss. Raise/lower hand requires call membership, no moderator bit.

## Scope and risk

Modify the existing Flutter panel and focused widget tests; document its mirror
of server gates. Use the current Space/voice-room locator and authenticated viewer
for existing permission queries, never current shell selection or role names.
Loading/error/missing scope must not enable privileged actions. Group controls and
backend authorization remain authoritative. No new organizer role, RPC, permission
bit, live environment or media acceptance is introduced.

## Milestones / detailed steps

- [x] Write and run widget cases before production changes: pending/deny/error,
  allowed moderator, separate broadcast speaking permission, stop after revoke,
  group behavior, scoped query identity and stale async completion.
- [x] Wire current permission queries and explanatory disabled states. Sanitize
  failed actions with existing localized common/permission messages; ignore stale
  results after room/auth/profile changes.
- [ ] Run targeted widget and call-provider regressions plus affected analysis;
  update graph after code; inspect diff and obtain independent permission review.
- [ ] Commit/push/PR; preserve exact CI and remaining runtime acceptance gaps in TT.

## Validation

`flutter test test/voice_organizer_permissions_test.dart` drives RED/GREEN.
`flutter test test/call_providers_test.dart test/space_permissions_test.dart`
covers relevant existing behavior. Analyze changed files; required hosted Flutter
checks validate the complete component. No new dependency resolution unless
needed. Tests use in-memory/fake transport; no real microphone/SFU or deployment.

## Decisions and progress

Use disabled controls with existing localized explanations, matching existing
permission UI conventions. Do not reuse the shared resolver's loading-as-allowed
behavior for privileged controls. Author owns tests and implementation; one final
independent review covers permission binding and async results.

RED first exposed enabled controls while permission was pending. A second cycle
exposed invisible server denial and a stale action refreshing a replacement room.
A third cycle exposed stale denial after only the voice binding owner changed.
Each failure was fixed without weakening its assertion. GREEN: nine new widget
cases plus existing call/provider permission suites, 32 tests total, no skips
(`flutter test --no-pub`, terminal exit 0). Windows SQLite fixture was seeded by
the repository checksum-verifying prefetch script; no dependency versions changed.
Affected `flutter analyze --no-pub` passes with no issues (exit 0). Two initial
brace-style diagnostics were corrected. Incidental generated plugin registration
changes from pub get were restored; no dependency or platform registration change
belongs to this slice. `graphify update .` was invoked; completion is recorded in
TT separately. Independent review and hosted CI are still required before merge.

## Risks and follow-ups

Role/Voice remains final authority after a UI check; a race may still produce a
server deny, which must be visible without raw diagnostics. This change does not
claim continuous backend permission invalidation, media-token revocation or A2
vertical acceptance. Broader organizer-role provisioning remains separate.
