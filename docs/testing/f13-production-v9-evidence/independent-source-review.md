# F13 production v9 — independent source applicability review

Reviewer: `/root/f13_review`. Result: **SOURCE ACCEPTED**, bounded to the exact source and evidence below. This is not CODE acceptance, activation authority, or full-producer completion. No service or test runtime was executed by this reviewer.

## Exact source boundary

- H9: `3a530edd97cbfe4beb9cea764eb0dd71acec2d2c`; tree `7de8699a3c6a8fc3c43a03680d8490a6af74a2ca`.
- Sole parent: `c1c340c809aa619a4ee18230b99356959fdb4a40`.
- Exactly one changed path: `scripts/ci/phase0-fixture_test.py`; all other complete tree entries are identical, including additions/deletions and modes.
- Before: `100644 blob 83744fede8beb4b837aeac070ecfe106f0bdaa78`, 159729 bytes, SHA-256 `c681dd23967350f10cdecf278dfd7b4f0b3cd55491e85dfc000845ab80a0dce3`.
- After: `100644 blob a07b4263ec3536aa31e2124e23d3c10baaab89ac`, 160640 bytes, SHA-256 `4a358406e826b70832f774e69658d444690e708e0fef0bf82f669545b7cc91ab`.
- Complete raw delta SHA-256 `298a838a58ab4f7f242272ec7f6b8aa051bb583aa6d289824fe79fb528543f12`; full-index patch bytes independently matched the packaged artifact.

The reviewer independently reconstructed these facts from Git objects with replacement objects disabled. The full D6-to-H9 inventory has exactly 1141 entries; every before/current entry and the complete raw delta hash matched. Its raw file SHA-256 is `b7bbdd161c6986fe6200369ad3181f4aeb91dbd4c517e4ebb9e0f99b31e8bbbd`. All entries retain `ORIGINAL_CONTRIBUTION_GATES_REQUIRED`; this inventory grants no blanket path or runtime acceptance.

## Fixture behavior and evidence

Read the exact source delta and independent fixture review SHA-256 `14a6b7cd47d9a40bbfb89012565e7a1ee10c711ac37f012156f2419d9456e706`. The helper preserves the nonempty-directory assertion while executing it as the actual service UID for UID65532-owned private directories. It requires GID65532/mode0700, clears supplementary groups, and drops GID/UID before enumeration. Other directories retain the direct assertion. Existence, positive file size and read-only mount checks remain. No fixture ownership/mode relaxation or production behavior change is introduced.

All 11 draft evidence references were independently hashed. All eight transported originals match their source files byte-for-byte, including lengths and hashes. Read the receipt and terminal logs: baseline reproduces two PermissionErrors; focused candidate passes 1/1; full verified suite passes 78 with one explicitly reported root-only cleanup skip; empty service-owned directory is rejected and nonempty accepted without owner/group/mode changes. The incomplete-copy full attempt is retained but explicitly excluded. Receipt SHA-256: `977f23d6f911f1e5a923c11c7da465cb8a2c3dd02cf08e938869c8d99a4464fa`. These are offline fixture results, not hosted or live-service acceptance.

## Retained Native and predecessor boundaries

Native v4 remains the exact tuple: record `60c0a1fd66d090956113c395d8447edc47b95772`, source `ef3dd7f3888bbe2792a9dcec836b2773c4e77962`, record SHA-256 `3bec6a20906f6a96ba2e77d5f4b243e415b5f0ed7f724d88e6c4643446099e53`. Independently compared the packet closure with that immutable record and checked H9 Git entries: all 99 projection paths, 472 context paths, 481 compiled paths, 89 complete immediate-directory inventories, and the entire Voice subtree are unchanged. This does not waive Native v4's existing post-record source restriction.

Production v8 predecessor record SHA-256 remains `90e6967cf14de9dbb349be06c51e10e06e8e280d8bd49c381a1ca6b2cfec1f5e` at the same R8. Existing source records and evidence were checked unchanged at H9. R8-to-c1 changes are confined to the existing ten named carrier files. H9 preserves the integrated current/full profile selector; current-profile success is not full-history proof.

## Required implementation boundary

Acceptance permits only a fixed, independently reconstructed H9/R9 applicability bridge. Actual R9 must be a direct child of H9 with exact named metadata/evidence additions; its final raw record hash and Git bindings require later verification. Current actual B/S/T, actual repository root, original event-base rules, ten executed carriers, full outside-finite-delta tree equality and all historical evidence must remain checked. Every current Native consumer, including J reconstruction and corrected-F, must validate the same typed applicability binding. Missing, unpinned or invalid applicability on this source must fail closed without falling back to old v4 acceptance. Historical predecessor reconstruction remains historical and supplies no current authority by itself.

The source draft reviewed here has SHA-256 `f336b6d86a38ca11efe5241104dabf2a4ef01c95e0cdcdf08ea5117dceffa5e8`; embedding this review will legitimately change the final record hash. Final CODE review, actual R9 pins, bounded regression results and a fresh full final-head producer terminal result remain separate obligations. No generic outside-closure exemption, future source grant, or producer PASS is accepted by this artifact.
