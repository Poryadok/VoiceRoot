# Protected voice-room creation source and evidence

Source: `f4db5563f06a3719a61f34f9b848bb85c43a7fb7`, branch `codex/pr875-role-admission`, parent source equivalent to root73531c.

Independent bot_installation source review CLEAR, including final test-only
Voice resolver setup adapter. Role, Voice, protobuf and deployment configuration
are unchanged. Exact 21 file paths/blob OIDs/content hashes are in
`f4db556-source-evidence.json`.

The protected Gateway route signs verified account/profile claims and the final
CreateVoiceRoom body. Space requests the existing pure service:space global
TEXT_CHAT_CREATE_IN_SPACE decision before local/store fences; original five-second
ingress and credential budgets survive locks and are checked before writes and
commit. Ordinary ingress and missing protected transport cannot fall back.

RED was captured on immutable preimplementation source. GREEN: Gateway and Space
short suites; actual Space TLS/mTLS/JWKS/replay/session-floor boundary; unchanged
Role exact creation listener; real native51 room/node creation and both rollback
validation points; three affected tree PG tests; all four Voice resolver tests
after explicit signed setup migration; both full module vet runs. All 19 changed
Go files exactly match the tested v3 snapshot. Owned test resources were cleaned.

Canonical CI lint qualification completed: exact
`github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.6.1` built using
Go1.26.5; binary version and module checksum recorded. Exact CI command
`golangci-lint run ./...` passed for Gateway and Space with zero issues each.
The previous cached (devel) result is retained only as historical evidence.
Binary SHA-256: `bf44279ebeb853f479227ff1374d94645d6c9f369325c799244e6508b057a2a2`.
Full provenance/result logs: `golangci-v2.6.1-provenance.log` and
`golangci-v2.6.1-ci-command.log`.

Limits: no full local Compose; native51 handler proof uses controlled Role policy,
with actual Role transport proof separate. New hosted media-stage acceptance is
not yet established. The earlier735 Go33/Flutter27 passes predate this repair.
