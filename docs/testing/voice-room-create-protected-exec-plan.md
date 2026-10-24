# Protected Space voice-room creation

## Contract and scope

The final live media stage at source 73531c1 failed creating a Space voice room:
the ordinary Space Role client reached the protected Role endpoint without its
required transport and principal. Role already accepts the exact pure Space
service decision `TEXT_CHAT_CREATE_IN_SPACE`; its source remains unchanged.

Gateway must sign the final CreateVoiceRoom body using verified account/profile
claims and route only to the existing private Space lifecycle connection.
Space authenticates that exact delegated request, obtains the signed Role
decision before acquiring Space mutation fences, and preserves the ingress
five-second budget through the database ownership locks and writes. Ordinary
ingress and missing protected dependencies fail closed. Existing name, capacity,
room/node atomicity and events remain intact.

Owned production files: Gateway auth.go, routing.go, transcode.go,
transcode_spaces_tree.go and new space_voice_create.go; Space lifecycleprincipal
admission_call.go/interceptor.go, grpcsvc/tree.go and voice_create_permission.go,
store/tree.go. Associated boundary and existing semantic fixtures are updated by
the same author. No Role, protobuf, Compose or Voice runtime changes.

## Verification

- Author boundary regression, observe RED on original source.
- Verify exact Gateway claims/body dispatch and no ordinary/HTTP fallback.
- Verify signed Space actor, body/JTI/ordinary rejection and finite original
  budget with Role outside local locks.
- Exercise actual PostgreSQL voice-room and tree-node writes and expired
  prewrite rollback; retain existing tree behavior coverage through explicit
  signed fixture identity.
- Reuse actual TLS/mTLS Role creation listener tests without source changes.
- Scoped Go short tests, vet and lint; independent review before publication.

No local full Compose or stopped Auth/Chat scenario is part of this work.

## Progress

2026-10-23: synchronized WT47 to exact root735 through ordinary merge e8db2e99a.
Boundary RED tests authored; isolated Linux runner assigned execution only.

RED observed on an immutable backend snapshot: Gateway's private dispatch was
absent and Space never reached the protected Role decision. Logs are retained
under ignored `tmp/pr875-repair/voice-create/` in the main checkout.

Implementation and explicit signed semantic-fixture adapters are complete.
Independent source review by bot_installation is CLEAR, including the later
authentication-negative fixture body correction. Role source is unchanged.
Linux Gateway and Space short suites and the unchanged protected Role creation
listener test passed. The actual PostgreSQL native51 signed-handler test passed
(15.79s), as did the three affected tree tests. Two independent Voice resolver
fixtures still used unsigned room setup; their setup alone now uses the same
explicit signed test actor, and all four resolver cases passed on rerun.
Independent review also cleared this fixture-only follow-up. Final Gateway and
Space `go vet ./...` passed; both scoped golangci-lint runs reported zero issues.
All 19 changed Go files match the immutable tested v3 snapshot byte-for-byte.
Its aggregate SHA-256 over 729 Gateway/Space Go files is
`745e2f873ef7ed802b3462af0133d1f0242f02c671527a800157d89e237de49c`.
Owned test containers were cleaned up. The native51 handler test uses a controlled Role decision
and is not a full Role25 end-to-end claim; Role's protected creation listener runs
separately with actual TLS/mTLS/JWKS and replay verification.
