# B1–B4 evidence reuse, scoped source review

Comparison: admitted Native v4 source `ef3dd7f3888bbe2792a9dcec836b2773c4e77962`
to target `b2b566f1103e29058da0e8b786cbd8f0dd028d83`. Final H must retain these
exact entries or be independently re-reviewed. This review establishes bounded
historical evidence reuse, not a fresh B execution or complete v5 source PASS.
The current A evidence, wire review and complete current closure remain separate.

`b-bindings.json` SHA-256
`80533229bd5699d880363820e235b384803aab05d9840a0bd145cb4922da2270` binds
mode/type/OID/SHA256 for all 30 Voice migrations, exact B clause source paths,
Voice PostgreSQL fixture helpers and integrationtest helpers at both sources.
All 36 rows match except `src/backend/pkg/integrationtest/migrations.go`. Its
new branch matches Messaging 015 and applies Messaging 038. The 014 probe uses
`startSpaceMediaAdmissionPostgres`, with explicit disposable DSN in both original
receipts, which calls `spacegatetest.StartPostgres`: that branch reads Voice 001
directly; the fixture reads Voice 002–010, 013, 014 and 015 directly using
`os.ReadFile` and pool Exec. It does not call `ApplySQLFile`, read Messaging 015
or include Voice 012. Literal 014 execution reads 014 DOWN directly and the B1
roundtrip additionally reads 014 UP. New Messaging 038 is therefore not a
transitive SQL input for those explicit-DSN probes. There is no implication for
other fixtures or default Testcontainers branch.

Independently hashed all 15 historical proof files listed in tracked
`docs/testing/native-p3-source-v2-evidence/historical-proof-bindings.json` and
the separately corroborating B3 receipt; all match the binding (see JSON).
The H53 source archive matches its receipt's SHA-256
`a6fc957847f5cf809ae12c09133e51c734af5da3b8d7dfb3c2115c2ff7352221`.
The v2 independent source review already examined H53 → 9df native changes and
accepts old literal 014 evidence and separately bound 012 supplement. The
v4 → target source-entry and directory/closure review adds the current edge;
it does not rewrite the historical source, claim a new runtime or eliminate
normal generated-dependency/current-H evidence requirements.

* **B1: reusable within exact old 014 scope.** External driver
  `native_b1_qualification_probe_test.go` SHA-256
  `c0e9420989758bbc8ec453a5df4be5303d19e167247d5129fda965d7a35898d1`,
  receipt `80d9dfa81c04bb56624b14de25ecc8301980f6197dd730bd353fa082a51c6ca3`,
  JSONL `5474245e56f1c5b3d64f1e51f2e0f3d0962bc0dbb13c2d79003f7d24605daff3`,
  supplement `752f8a6456fecec12291f983b231652262ade78c05fece94cac12fff6e45a90d`.
  The recorded test is `TestQualificationP3EmptyDownPreservesUnrelatedSchemaAndData`:
  unrelated schema, FK, index, view, JSON and public sentinel survive literal
  014 DOWN/UP. One test and package PASS, zero skips; no claim that 012 was in
  that fixture or that this driver ran on the new source.
* **B2: reusable within the retained-class 014 scope.** Original snapshot matrix
  receipt `51a606a3693e128c2cc3021d8cc54b309fd45ef4c9d8ed107ec27ecf90e04654`,
  JSONL `cd885785fb4a76444a0b86583addeffdfacdecfccb68bc3f37894f6d25d141ef`,
  proof `a595d15486b4ea688865e5e3a3fe8e5165ad67934916e214f3915bec12fe050c`,
  records 26 test PASS, zero skips. `TestP3DownRetainsIndependentLifecycleEvidence`
  includes 12 retained classes, with exact snapshot retention. Independent B3
  driver additionally checks a valid grant and FK parents with no lifecycle
  targets/receipts: DOWN refuses and preserves state. Historical results only.
* **B3: reusable within exact 014 synchronization scope.** Original external
  probe SHA-256 `9e70f9220e1b0caf57ab1f9184d3627a95a005024df386798f479325aaad88e2`,
  pinned JSONL `876e945ba10c771884fbdaeefd549202b1e9c55d74800d38aa338ce1df5a361f`,
  corroborating previously unpinned receipt
  `4a3313e231914cb488e02e8bf329f9080906dd4e283f48349f451324648119d6`.
  Four probe tests and package passed with no skips: grant-only, observed
  DOWN-first writer exclusion, actual dependent-view DDL rollback, and executor
  lock-timeout rollback preserving writer. The ordinary writer-first lock wait
  is in the historical matrix. Timeout was an explicit fixture executor setting,
  not a migration default.
* **B4: reusable within literal 013 retention/no-MM020 scope.** The matrix above
  covers retained outbox/counter and inherited snapshot guard; 013 DOWN entry
  `100644 blob 47f8f4249efb5051731086e24d5855130ac1cb19` and Voice fixture
  remain identical. The fixture uses only Voice migrations; no MM020 import.

The separately admitted 9df 012 current supplement
`voice-ordinary-down.jsonl`, SHA-256
`ac60b8611d1cfa33c21ba63c9003c8e43eb2302613ffe72ae3d6f2b606630790`,
records first writer journal/snapshot, inherited snapshot rejection and empty
roundtrip. The later v4 and target preserve 012 migration and ordinarycall tree;
inspect exact 9df → v4 provenance in the v2 record, and retain changed common
compiled inputs in current closure. This is not the 014 unrelated-schema proof.
Fresh H tests may supplement these old proofs; do not relabel them as fresh.
