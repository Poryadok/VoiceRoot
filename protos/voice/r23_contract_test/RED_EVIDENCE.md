# R23 protobuf contract RED evidence

This oracle was authored on branch `codex/r23-contract-red` in Treehouse lease
`87e535489b1d62a6ff049786ba8cc54f`, from exact repository SHA
`edc52d46406d97283f81dfbdfc916660f50dcc69`. At the independently approved RED
checkpoint, no production proto, generated source, service source, or repository
documentation had been changed.

## Authority

- Accepted documentation merge: `2bf7e0844bda087258886b08cb77f25606ff1387`.
- `tmp/slave-driver/run/NEXT-PACKAGES.md` SHA-256:
  `1855B1DC665CA9EACDE5BA4732DC12661BA8736F0D41EE712A3075723BCC60F6`.
- `tmp/slave-driver/run/R23-DOCS-DECISION-PROPOSAL.md` SHA-256:
  `3877EBCDB9CA4E7308DF8CD3122112DC2A7FC4A7D3D8BD020ACD8AB12EEFE4F4`.
- `tmp/slave-driver/run/OWNER-DECISION-BALLOT.md` SHA-256:
  `563A79303638060B289EC438B029518E807EEBFDB63DF6E631033EE4B2B3BEB7`.
- `tmp/slave-driver/run/R23-DELETION-RETIREMENT-PLAN.md` SHA-256:
  `B6E5A242AF8420C3958B4C68764A2C3FCD98936B6161F5C194C87EDE446D1F9A`.
- Owner decision: U1-A, U2-A, U3-B, U4-B, U5-A; `K=P90D`, `B=P30D`.

The independent contract review fixed the shared lifecycle types in
`voice/common/v1/space_lifecycle.proto` under package `voice.common.v1`, and
fixed the callee wrappers, participant values, lifecycle enums, Space fields,
and event tags represented in `r23_contract_manifest.json`.

## Oracle

- `../r23_contract_manifest.json` is the machine-readable exact contract.
- `r23_contract_test.go` validates manifest self-consistency, builds the current
  protos into a JSON descriptor set with Buf, and checks file/package identity,
  syntax, dependency closure, Go/Java options, enum values and reservations,
  field names/numbers/types/labels/reservations, oneofs, optional/deprecated
  flags, services and RPC input/output FQNs, positive and negative RPC exposure,
  security/unknown-field/hash source annotations, receipt/event secret
  exclusions, additive compatibility, and a canonical selected-contract
  projection hash for every affected proto file.
- `generated_targets.json` freezes all 143 affected committed Go/Dart/Auth-mirror
  and uncommitted Java generation targets derived from the repository generation
  scripts. The generated-target test checks every committed file and R23 symbol.
- `testdata/deterministic_contract_vectors.json` contains checked-in wire bytes
  and expected domain-separated and proof-digest SHA-256 values. Its schema-bound
  corpus checks decode/re-encode stability, changed known fields, unknown values
  and order, wrappers, repeated-field order, and the FQN hash prefix. The oracle
  cross-checks every vector field number, wire kind, cardinality, unknown-field
  policy, and nested FQN against the contract manifest.
- Repository target `make r23-contract-ci` runs the oracle, and the protobuf CI
  job invokes that target. `make r23-p2-generated-parity` composes it with the
  existing Go, Dart, and Auth-mirror generation drift gates, Auth Maven
  generation/compile, executable Go and Java generated-runtime fixture consumers,
  Java-target verification, independent affected Go-module compilation, full
  `flutter analyze lib/gen`, and selected Dart public Space/File and event
  compatibility tests. The temporary consumers decode into named generated
  types and assert named, nested, and repeated values, deterministic re-encoding,
  and FQN-domain relations.
- The test module is standard-library-only and does not depend on generated R23
  code.

## Reproduction

Run from `protos/voice/r23_contract_test`:

```text
gofmt -w r23_contract_test.go
go test -run 'TestManifestSelfConsistency|TestHarnessIndexesSyntheticDescriptor|TestDeterministicContractVectors|TestGeneratedTargetsManifest' -count=1 -v
go test -run '^$' -count=1
go vet ./...
go test -run '^TestR23GeneratedRuntimeConsumers$' -count=1 -v
$env:R23_CHECK_UNCOMMITTED_JAVA='1'; go test -run '^TestR23GeneratedRuntimeConsumers$' -count=1 -v
go test -run '^TestR23ContractDescriptors$' -count=1 -v
go test -run '^TestR23GeneratedTargets$' -count=1 -v
```

The first four commands pass. The descriptor test fails, as required for RED,
because the baseline lacks the accepted R23 contract. Root failures include the
absent shared lifecycle proto; missing imports/options/reservations; absent Auth
proof, Space delete/restore/coordinator status, Chat manifest, Messaging import,
Role retirement, lifecycle event, File reference/capability/manifest/lookup, and
participant apply/purge surfaces; absent Space fields 16/17 and event arms
21/22; absent required annotations; and canonical projection mismatches.

The generated-runtime test fails separately and intentionally: its Go consumer
cannot compile the absent named common/chat/file R23 types, while its Java
consumer, after successful baseline Maven generation/compile, cannot compile
the absent named Auth R23 types. The generated-target test fails because committed Go, Dart, and Auth
mirror targets lack the R23 symbols, and the new shared lifecycle targets do not
exist. Both failure groups are product/contract absence. Manifest, synthetic
descriptor, deterministic-vector, and generated-target-manifest self-tests pass,
so neither group is a harness failure.

`git diff --check` passes. The production-path guard below prints no paths:

```text
git diff --name-only -- protos ':!protos/voice/r23_contract_manifest.json' ':!protos/voice/r23_contract_test/**' src docs
```

Independent review subsequently froze the previously unresolved Chat prepare
and page wrappers, Messaging page import receipt, File ordinary acquire/release
receipts and purge lookup, and Space coordinator status. The manifest now
asserts those exact choices; no draft or omitted R23 contract section remains.

The only non-oracle changes are the isolated Make targets and one protobuf-job
CI step that reaches `r23-contract-ci`. Unrelated workflow content is unchanged.

No commit or push was made before independent RED review.

## P2 GREEN evidence

After independent RED approval, the exact manifest contract was added to the 14
canonical proto files, copied to the Auth Maven mirror, and regenerated through
the repository's existing Go and Dart workflows. Java output remains uncommitted
under `src/backend/auth/target/`. Four Go module manifests gained only the local
dependencies required by the generated Space-to-File import; no external module
version changed and no service implementation source was edited.

The original unpacked Auth repeated-enum fixtures exposed a real deterministic
runtime difference: proto3 Java emits repeated enum values in packed form. The
independently approved correction uses packed canonical bytes and keeps the same
schema, order, wrapper, proof-digest, and FQN-domain assertions. Its authority
SHA-256 is `AF12194346ECD704AC79CA67AC04C993FA9BCA292608F01A6DBD759F18EE6656`.

The following completed successfully on the GREEN tree:

```text
make r23-contract-ci
make r23-p2-generated-parity
buf lint
buf format -d --exit-code protos
buf breaking protos --against '.git#ref=edc52d46406d97283f81dfbdfc916660f50dcc69,subdir=protos'
git diff --check
```

The P2 gate regenerated and checked committed Go and Dart output, verified the
Auth mirror, generated and compiled Auth Java, checked all 143 declared targets,
ran the Go and Java deterministic fixture consumers, compiled the affected Chat,
File, Messaging, Role, User, and Voice Go modules, analyzed `lib/gen`, and passed
the selected Flutter compatibility suite (17 tests). On Windows it uses the
repository's existing `flutter-windows-prefetch-sqlite3` prerequisite.

The final working tree contains 136 changed or new paths: 14 canonical proto
sources, one Auth mirror, 64 Go generated files, 45 Dart generated files, four Go
module manifests, and the original eight oracle/CI paths. No Go sum, Java output,
service implementation, documentation, or unrelated workflow file is included.
No commit or push was made before independent source/wire review.

The final GREEN review inputs have these SHA-256 values:

```text
6d8906268a50e705e3e8c9664d395e7b83e5f31c0430191656f295e3d8c45bac  protos/voice/r23_contract_manifest.json
c9882b423a0f37c2540652c6e633c336671a8a1f3ad1d67642892faf252d5432  protos/voice/r23_contract_test/r23_contract_test.go
0e250cdf2d5a9d5a2d0b48ff32dd95affabb95e9a31e683506449c0752d62da4  protos/voice/r23_contract_test/generated_targets.json
af12194346ecd704ac79ca67ac04c993fa9bca292608f01a6dbd759f18ee6656  protos/voice/r23_contract_test/testdata/deterministic_contract_vectors.json
73a0efeffc107d24d991b2584598e574932b6248e6a4a49cfc879011b5186a80  protos/voice/auth/v1/auth.proto
95bb4cd603848f58e07de67ae87f3f2fb926f884dc8778215f9cc22fae13ff69  protos/voice/bot/v1/bot.proto
7c6d55901148c13b78a80800308034908538b13c638f0d0f89dc6546e09095d7  protos/voice/calls/v1/calls.proto
d62ae3456dbd6a88c5a7d7780b1e1656855f1263ae6ade5ee2d3deaece9bf45c  protos/voice/chat/v1/chat.proto
7b68e2dc4ded3969cf78f7af14297bb6aae0652a2dd06e2329cfe94afe95a7bf  protos/voice/common/v1/space_lifecycle.proto
abd00ff7299892853d43d1dcace31e3f1a87498199bd4534090cbbd0f96d827f  protos/voice/events/v1/jetstream_events.proto
c058cbe3144a5d0c196ce4df56521d0cf4e63a9d60802af6da0432c653519c64  protos/voice/file/v1/file.proto
cd6ff1258312558aa94607cebfb5b0a489dd454c5d9f1d1fd7747713e071422a  protos/voice/matchmaking/v1/matchmaking.proto
7afdaea29d10f30554692d3f3d2d4011ccb448cc78a5802a07d40183d1318976  protos/voice/messaging/v1/messaging.proto
99cf98b71f6523199fd5245a2c5ee5064f483aa6adcb55f1d6708472f27cdb5e  protos/voice/notification/v1/notification.proto
66626be0b231806199639438fc474716f7aa482f80e39ee47e0f92c5e1cb77b7  protos/voice/role/v1/role.proto
16a738095076c69003cab8985dfe249bb9549561afb1629f672312ef92b4c7e2  protos/voice/search/v1/search.proto
dcf424b3f3c9f112956d437267134d58fe9a4b2e304d6f5319f3b1316f0e7333  protos/voice/space/v1/space.proto
cc60028dee9ebd243fcb8f09c2f44e827d22f7bedf118bf023560700628fcfd5  protos/voice/subscription/v1/subscription.proto
73a0efeffc107d24d991b2584598e574932b6248e6a4a49cfc879011b5186a80  src/backend/auth/src/main/proto/voice/auth/v1/auth.proto
```
