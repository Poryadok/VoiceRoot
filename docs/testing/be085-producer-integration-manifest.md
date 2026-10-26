# BE-085 protected producer integration manifest

This is source assembly and review preparation, not feature activation or acceptance.
Anton-93 owns this integration branch and Chat/additive generated/documentation
resolutions. Anton-95 retains File/Messaging behavior ownership. Anton-67 owns
delivery-child bookkeeping only. PR915's earlier six-path review does not cover
the complete producer stack.

## Frozen inputs before assembly

| Input | Exact version | Inclusion and review evidence |
| --- | --- | --- |
| Fresh master | `258b48d0b4506e118b833a194642142fb5847c2e` | Clean Treehouse allocation base; includes PR881. |
| Current PR907 stack | `00f34bb1ddfc3723dea9310a2ac4b85b65258689` | Confirmed by integrator67; includes reviewed PR910/914 and the scheduled test correction. |
| Plaintext GIF producer PR898 | `708e54d2264f8f05aea3b529a26fd9b653dec154` | Root explicitly pins this reviewed producer, already an ancestor of the95 interface lineage. Do not substitute current PR898 head without scope review. |
| Compatible File/Messaging prerequisite | `d3d0e5fadba2dd2cf488b7abc07327a429f1f942` | Directly confirmed95. Versioned schema, terminal proof and typed consumer are compatible. Excludes95's later storage/executor work. |
| Intact Chat/Gateway producer | `70a9b06a0918c40cffa53582ae4b8d30df72f3fa` | Frozen original PR915 source; includes its recorded prerequisite ancestry. No cherry-pick stripping or history rewrite. |

PR907 review chain: [combined actor authority](https://github.com/Poryadok/VoiceRoot/pull/907#issuecomment-6405061038),
[integration applicability at12073d25](https://github.com/Poryadok/VoiceRoot/pull/907#issuecomment-6405878104),
[scheduled test source at4ecbc4e](https://github.com/Poryadok/VoiceRoot/pull/907#issuecomment-6405932101).
Final00f34bb is the ordinary merge of the scheduled correction into that stack;
new assembly still requires exact-result review and CI.

Review precision for95: independent97 CLEAR covers only byte fetcher
`985f774fea80c936cc449862c1b6e156c2079249`, not blanket schema/typed-consumer,
database, import executor or worker signoff. Contract agreement with93/Astra101
is technical design agreement, not independent implementation review. Do not
merge8eda or currently uncommitted import behavior into this package.

## Execution plan

- [x] Root authorized this source packaging stage; BE-085 is workflow2,
  in_progress, owner Anton-93; historical assignee remains unchanged.
- [x] Fresh isolated WT85 received from capacity115, lease
  `f2ea0e0323155e797dd4e4479febc3e2`, holder `Anton-93-BE085-integration`;
  resolved path, clean state and exact258b48d base verified.
- [x] Freeze this manifest before any dependency merge on
  `codex/anton93-be085-producer-integration`.
- [x] Normal-merge the exact inputs above in order. An already-contained input
  is a verified no-op, not a new source uptake. Preserve original authorship.
- [x] Record actual merge parents, conflicts, source-owner resolutions and
  complete remaining delta against the frozen/current master.
- [ ] Run affected Chat/Gateway/User and generated-consumer checks, migration
  and protected auth/concurrency cases appropriate to actual conflict changes.
- [ ] Freeze complete resulting source, request whole-result security/source
  review55 via scout10 and exact-head CI through delivery67.

## Review scope and handoff

Actual normal merge receipts:

| Step | Result |
| --- | --- |
| Manifest before merges | `289c30b2ce8545f281d10b1b1de98f985b8ef72c` |
| PR907 into fresh master | `725f2ab1854564a0919038f4392c0d69d4009a2c` |
| Exact plaintext producer | `929f9e19d6f24798ac479a67c329fa4da62ac6b0` |
| Exact95 interface/consumer | `a54b5466eb6f91405995ffd5a432a840c276de31` |
| Intact70a9 source | `4fb4ff2aac4c53d38ba901797e517973efdd2b15` |

Only the last merge required manual resolutions, both documentation-only:
`docs/TESTING.md` had the identical catalog-actor link with differing line
endings; `user-service.md` had the same CatalogActor/Messaging sections in
different order/line endings plus the newer PR914 owner-only challenge readback
on the integration side. Both retain the exact current907 bytes. No source,
proto, generated, File or Messaging conflict was manually resolved.

Unrestricted scoped diffs verify Chat plus the new Gateway selection handler/tests
and Chat catalog proto are byte-identical to70a9; File implementation/File proto
are byte-identical to95d3; User internal/main/proto/socialprincipal and generated
User Go/Dart messages are byte-identical to90700f34bb. These are source-equivalence
facts, not independent security or execution passes for this assembled tree.

Whole source review base is929f9e19 (after907/898, before95): the actual result
adds74 paths/17460 lines, mostly schemas/generated bindings. The own-producer-only
comparison a54b5466→4fb4ff2 changes51 paths/5429 additions/158 deletions; it must
not hide95's shared protocol/typed-consumer review requirement. Source owner95
has been asked to inspect Messaging composition; final full review55 was
requested through scout10 while exact-tree checks are prepared.

The reviewer must cover the complete new Chat catalogassets package, actor
HMAC keyring/migration28, account Save/purge generation synchronization, protected
RPC/client/listener/runtime, native membership/E2E/Role destination checks,
profile/File lifecycle reconciliation and Gateway delegated selection transport.
The actual remaining File/Messaging protocol/typed-consumer changes require95's
ownership review and independent coverage; reviewed User prerequisites must
remain intact. Six Gateway paths or merely the last70a9 commit are insufficient.

Original evidence is recorded in BE-085 c10/c11 and attachment
`2d667069a3bdd30f076532f08c32e6b3`; those developer checks are historical source
evidence, not passes for the new assembled head. Producer runtime remains with
an absent live resolver. Actual File byte import/settlement/ready metadata and
Messaging send proof remain separate unfinished consumer integration work.

## Decisions, surprises and outcomes

The first new lease attempt failed because all83 slots were occupied/dirty;
capacity115 allocated a new supported slot. Main checkout was not edited.
PR915 has multiple merge bases and a147-path historical three-dot diff; retargeting
it to907 is not a six-file source package. Assembly therefore uses a new branch
from current master and normal merges of pinned complete source.

No live provider/key, E2E catalog source disclosure, installed-sticker authority,
environment mutation, deployment, runtime qualification or acceptance is part
of this source checkpoint. Destination ACL and lifecycle/reference guards must
remain fail closed. UNKNOWN snapshots and generic File errors never authorize
purge; cache bindings never grant blob retention or history authority.
