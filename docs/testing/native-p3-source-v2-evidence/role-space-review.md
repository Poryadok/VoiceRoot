# Role/Space applicability review — candidate 9df1fa83

Read-only review; **not a new Native-P3 authority record or approval of 579 paths**.
Candidate: `9df1fa83e2cb8741c7fdfc1003658131ccc267b7`.
Historical record/source: `docs/testing/native-p3-source-v1.json`,
`fa2eac69f0262409b0ab0f55fd76d200220bc2c7` (94 projection, 460 context).
Exact manifest hashes, before/after blob OIDs and classified path inventory:
[f13-9df-role-space-inventory.json](f13-9df-role-space-inventory.json).

## Existing record drift

The reviewed Role/Space/shared-package/canonical Space/common subset contains
368 changed paths: **4 existing context entries, zero existing projection entries,
364 outside the old record**. Preserve those classifications. Outside-record
contributions need their original router/source qualification; do not promote
them into Native-P3 projection merely because current CI needs them.

| Existing context path | Candidate blob | Applicability |
| --- | --- | --- |
| `src/backend/pkg/authoritysource/postgres.go` | `6a55b91475379c29a3cbd4fc62218d9233f2db66` | Existing fenced catalog checks extracted for same-transaction reuse and batched. Required relation, trigger namespace/type/args, function body hashes and per-function uniqueness remain checked. This changes compiled shared code, so old binary identity cannot be claimed. |
| `src/backend/pkg/authoritysource/server.go` | `935a35be35fff369c54b2b407375746dfac0887c` | Adds qualified schema2 only for Role/Space; existing schema1 and owner-specific selectors remain. It does not activate a Voice relay or confer membership. |
| `src/backend/pkg/authoritysource/server_test.go` | `ce7e363d969fdc968d643d759ef122d073a8880f` | Negative unknown schema changes from2 to3, reflecting explicit schema2 support. New `qualified_scope_test.go` supplies Role/Space-only discrimination; do not treat moving the negative alone as proof. |
| `src/backend/pkg/integrationtest/authority_migrator.go` | `69da62cf16975e4c4a111f443e28241652f2f8af` | Explicit PG-image fixture helper and bounded sanitized migrator diagnostics. Default PG16 behavior is retained; no production authority path. Actual PG18 Native51 evidence exercises the helper. |

`postgres_integration_test.go` additionally discriminates ambiguous overloads
that would preserve total row count while omitting a required function. It is
new context evidence, not an automatic addition to the old manifest.

## Source-bound evidence available in this session

- `6313cc6e5..9df1fa83` has **no changes** under `src/backend/role`,
  `src/backend/pkg/authoritysource`, or `src/backend/migrations/role_db`.
  `wt47-workload-mm-native-role25.log` records actual native25/current Space51
  PASS69.513s, seven Messaging and five Voice decisions, Space SETTINGS/MM
  owner/member checks, and original observation/JWT expiry. This supports those
  unchanged component facts. Later generated Space/common inputs differ, so it
  is not proof of an identical current compiled binary or a fresh F13 matrix.
- `5eb311240..9df1fa83` has **no changes** in `pkg/roleoverrideprincipal`,
  Space member/bot authority handlers/store, native Bot publication, their native
  fixture, or Space migrations. `bot-message-space/green.log` PASS17.978s proves
  current native Bot echo and stale/cancel/reinstall/wrong-kind/chat/budget
  rejection; `green-botinstallation.log` PASS31.571s proves real signed native
  add/remove; `green-protected.log` PASS16.811s preserves Role PREPARING behavior.
  The first RED reached the missing-echo assertion. Signature/mTLS/replay tests,
  scoped lint and independent f13 source review are recorded in `space-evidence.md`.
- All proof-file paths and SHA256 values are in the companion JSON. Executing
  migrations through51 does not prove every account-ban, purge or recovery path.

## No additional Voice A3 or retirement authority

Role's changed Voice grant reader adds fresh qualified Space observations and
post-commit expiry checks. It does not change Voice provider registration, relay
leases, delivery or retirement receipts. Role admission cleanup retention only
extends evidence compaction; it is not a new retirement decision.

The actual Role retirement handler/store and Space lifecycle coordinator/manifest
remain byte-identical from fa2eac69 to9df1fa83; their exact OIDs are in the JSON.
Space lifecycle storage does add native membership capture around schedule,
proof receipt and purge. That is a substantive original Space contribution,
**not proven by the Bot-send test and not blanket-qualified here**. Its separate
source/protocol evidence must remain attached to its original contribution.

## Remaining qualification gates

1. Whole-unit reviewer must settle current Voice projection/context drift and
   generated/module closure; this review covers neither all902 changed paths
   nor all579 router denials.
2. Keep old eight A/B results historical. Reuse only explicitly unchanged facts;
   no current whole-matrix, runtime A3 activation or deployment claim follows.
3. Any new Native-P3 dependency entry needs current Voice import/compiled-closure
   justification. Most Role/Space changes belong to their own admission/native
   catalog protocols, not a broader dormant Voice authority exemption.

No source files changed and no Auth/Chat stopped scenario or runtime test ran
during this audit. This inventory is classification/evidence mapping, not a
368-file fresh implementation signoff.
