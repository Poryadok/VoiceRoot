# BE-085 ordered source projection correction

## Purpose and decision

Independent139 found P1 on frozen8febb291: file-ID-only Chat persistence admitted
both A and B for one actor/provider identity. The contract requires one current
selection and forbids an old operation replacing a newer admitted binding.
The existing actor generations do not order selections within that generation.

Astra101 resolves order at File's first successful pending-admission transaction:
one head per actor-generation/provider scope, positive monotonic revision,
pending File/immutable receipt/head advance atomically. Retry/status never
advances. New admitted B supersedes A even if B pending or subsequently fails;
failure before admission does not supersede. UI click chronology is not claimed.

Chat uses a disposable random selector-slot incarnation captured before remote
Status/Import, no URL or operation ledger. Save updates only that incarnation:
greater revision advances, equal requires exact tuple, lower rejects. Retirement
deletes the captured incarnation so a delayed response cannot reinstall into a
new empty slot. File owns the permanent watermark; Chat cache is not freshness
authority. Validate always reads File's current proof and rechecks the exact
cached slot. Missing cache fails closed; Resolve can hydrate only File-attested
current nonterminal state. No new implicit import or lookup permission is added.

## Protocol and lifecycle

Public Resolve envelope1 remains compatible but always calls private File2.
Private Chat source Validate accepts2 only; destination-only observation stays1.
File response proof fields are13/14 on Import/Status,15/16 on Validate (Lookup
wraps it); Chat Validate response7/8. Positive selection/current revisions are
mandatory for admission, with current equality. Never infer order from UUID,
timestamps, arrival or source hash. Protocol1/default/zero denies admission.

Only qualified asset-selector terminal cleanup may use both zero revisions and
both empty Chat generations: exact tuple, terminal status and nonzero opaque
terminal_fence_id. It cannot become Import/op status/READY/lookup admission.
Retirement compares captured slot/revision/File identity. Legacy unordered rows
remain historical and inadmissible; no ordering backfill or message/ref/blob
deletion. Forward Chat migration29 leaves published28 immutable.

Empty slots contain only gate IDs/generations, purpose-HMAC selector aliases and
random incarnation. Request failure/cancel removes only the same still-empty
slot. Ordinary bounded eviction can remove orphan/in-flight slots immediately
under actor/selector locks; late Save then fails and retries fresh File status.
This is cache maintenance, not File execution fencing or a retention TTL.

## Progress and validation

- [x] Confirm139 finding against exact implementation and contract155-159.
- [x] Obtain101's first-admission, disposable-slot and protocol2 decision.
- [x] Agree exact fields with95; normal-merge standalone protocol-only
  `19ad40c1b77dee9716d1af040f67447c101a448e`, direct parent d3, excluding executor89ce.
- [x] Demonstrate real PG A→B supersession failure before behavior changes.
- [x] Add HMAC slots/current projection/forward migration29 and bounded eviction.
- [x] Translate public envelope to strict private2; enforce fresh File proof,
  current equality and update-only captured incarnation in Chat service/client.
- [x] Cover delayed A after B, B admitted before Chat projection, B terminal and
  cache deletion/recreation, failure-before-admission, no fallback after failed B,
  actor purge/restore, key rotation, legacy/zero proof and current-scope isolation.
- [ ] Rerun actual PG/race/TLS, affected component/contract checks, then request
 139 bounded rereview at corrected immutable head with full-result delivery gates.

The initial assembled suite and unchanged isolated PG test both hit the3-second
production lock timeout under8 contenders. All success assertions and production
guard remain unchanged. Added phase/timing and isolated wait-graph diagnostics
retain every contender before failure cleanup; that diagnostic run passed2.67s.
Earlier failures remain nonPASS evidence, not erased by that run. Diagnosis and
final exact-tree verification continue with the correction.

Working correction evidence: actual PostgreSQL supersession test failed against
the old behavior (A still found after B); corrected catalog package race passes
45 tests, including the final legacy negative and purge cleanup. Chat short
passes279 tests/12 packages; Chat lint reports0 issues. The isolated actor-gate
test rerun also passes without changing its eight-contender success conditions
or production lock timeout. Buf lint/format pass; broader final checks and
immutable rereview remain pending. The service tests control response delivery,
not a replacement File admission algorithm: File ordering behavior itself stays
95-owned and requires its own implementation/review.

No live provider, fake product data, executor uptake, E2E/sticker activation,
environment mutation, deployment or acceptance is authorized by this fix.
