# Phase0 private signer inventory test repair

Author: bot_installation. Base: c1c340c809aa619a4ee18230b99356959fdb4a40. Only modified source file: scripts/ci/phase0-fixture_test.py, SHA256 4a358406e826b70832f774e69658d444690e708e0fef0bf82f669545b7cc91ab. Root owns final commit/push; no author commit or deployment.

Hosted failure and local immutable baseline both fail because the host test user cannot list service-owned Chat and Space signer directories (UID/GID65532, mode0700). The generator's permissions are intentional. The test now checks private directory metadata and performs its nonempty assertion after clearing supplementary groups and switching the child to that numeric owner. It does not chmod/chown or return credential content. Ordinary directories retain direct enumeration. Existing bind presence, file size and read-only assertions remain.

Local verification used Linux UID1000 with passwordless sudo, offline Compose config only, and the complete c1 checkout plus the single frozen candidate overlay. All7388 tracked files were verified present/readable, including five tracked tmp files. No Docker Compose services were started.

| Check | Result | Log SHA256 |
|---|---|---|
| Immutable original test (c681dd23967350f10cdecf278dfd7b4f0b3cd55491e85dfc000845ab80a0dce3) | RED: one test, two PermissionErrors for Chat/Space,12.825s | red.log: f22d09c52ff63b7e649577dc5b8051cbbfe99f76cba813ccc10827d9a9af0320 |
| Focused candidate bind-source test | GREEN1/1,10.427s | focused-green.log: 03552dfdcce379d7e84f0cd3abbcbae04ae330e317a61c013c5e80795fe1db82 |
| Full candidate fixture suite | GREEN78 tests,48.716s; one existing root-only cleanup test skipped under UID1000; service-UID fixture ran and passed | full-verified.log: f45e5fbcab6fe216e6c24b14767d822aed04343404dbb7021b73d918feaca143 |
| Controlled helper oracle | Empty directory rejected; nonempty accepted; both owner65532:65532/mode0700 unchanged before/after | helper-negative.log: 4b7e1e134c7517fc91ff3ed0e0189d67a88feec7e05467221694e00d9b485549 |

The first local full-suite attempt raced an incomplete source copy and is excluded from acceptance evidence; its log is retained separately. Host/import setup errors are likewise not counted as RED. Independent source review was CLEAR from role_admission; final reviewer artifact is separate. git diff --check passed. Graphify98381 completed exit0 (existing header parse and visualization size warnings only). This proves offline test behavior, not deployed service acceptance or live Compose behavior.
