# Automod and report privacy recovery contract

Canonical implementation input: BE-137 c23/c24, refining c18/c19; BE-045 c13 remains authoritative.

Source decision: [c23](http://192.168.0.109:30990/?project=voice&task=BE-137#comment-c23), [c24](http://192.168.0.109:30990/?project=voice&task=BE-137#comment-c24).

Implementation boundary: `moderation/internal/recoverybundle.Inspect` reads a
read-only protected directory containing `manifest.json` and exactly its listed
regular files. Protocol 1 binds the independently supplied operation UUID,
environment, deployment generation, recovery method, store incarnations and
required artifact kinds. Each artifact identifies kind/store/path, exact length
and lowercase SHA-256, schema version 1, journal generation UUID, diagnostic
record count and opaque finite cut. Limits are 2 MiB manifest, 128 artifacts,
16 MiB per artifact, 64 MiB total and 512 directory entries. Larger recoveries
must be split into an explicitly supported bounded format before use; no
truncation is accepted. Duplicate/unknown JSON fields, extra files, symlinks,
path escape, mixed identities and unsupported metadata are rejected. Returned
bytes and metadata are copied; later file replacement cannot change the
inspected inputs. Record contents/count semantics and native domain lineage are
validated by the later domain verifier, not inferred from inventory metadata.

`Inspected` is deliberately not a verified recovery cut or a boot receipt.
It cannot prove quiescence, currentness, native replay or successful privacy
participants. The actual trusted operation adapter and coordinator must still
complete V2–V6. Neither recovery method name alone establishes its evidence.

BE-137 c21 bounded frontier decision, supplement/correction to c18-c19; BE045 c13 remains authoritative. Actor Рудеус-163, architecture/security reviewer, Astra/high focused continuation. Consumers144/156, User seam172, observer183, integrator121. No source/CI/stand/lease custody.

SOURCE AND CORRECTION
Evidence immutable8c0d1ede4c7840f596a67c603b726935c5a06847 atopbb12d52/144af171. docs/testing/async-automod-startup-restore.md sections1,4,6 explicitly allow current live databases to establish local authority on ordinary restart, while declared restore needs independent non-rolled-back recovery history. docs/DEPLOYMENT.md:1466-1477 repeats that distinction. BootGate in src/backend/moderation/internal/automod/boot_gate.go orders boot-bound phase receipts; its own comment says the coordinator must verify them. A nonzero ProofID is not a recovery verifier. report_privacy.go is absent at this immutable head, so no claim that144's newer checker was audited here.
docs/PRODUCTION_HOME_SERVER.md:159-200 documents periodic quiesced restic backups: local PVC/config/dumps and compact off-host PostgreSQL dump/config. It does NOT establish a continuous WAL archive, report recovery sink or all acknowledged post-backup erasure coverage. Current deployment state was not probed.156's terminal seal identifies a finite view and detects corruption/mixing; it does not prove independent currentness.

Autonomous implementation choice: use the EXISTING operator backup/isolated-restore boundary with a finite file-based recovery bundle and verifier. Do not add a runtime sink service, endpoint, provider, key hierarchy or per-record remote ACK prerequisite. c18-c19's independent-frontier wording was too broad if applied to every live restart: it applies to DECLARED_RESTORE or uncertain/replaced storage. Local transactional report journal is authoritative on an established current live DB. No external export/outbox is needed merely to let that ordinary boot reach P5. c18's atomic journal+report mutation, opaque content, terminal priority, provenance distinction and supported-backup retention remain mandatory.

TWO START MODES, ONE TRUST BOUNDARY
The trusted deployment/startup adapter supplies an explicit typed mode LIVE_CURRENT or DECLARED_RESTORE together with expected environment, logical stores, store incarnations and deployment/recovery generation. These are operational configuration, not user metadata or a SQL ready boolean.
LIVE_CURRENT means the supported operation kept the authoritative live storage and did not replace/roll it back. Observe the connected store, run migrations/integrity checks, capture a fresh finite local journal view, obtain fresh allowed Auth/User authority and execute normal P0-P5. Concurrent work after the finite cut follows existing transaction/lifecycle fences; startup does not wait for an ever-growing journal. A rolling process restart does not require fabricating a restore manifest. Legitimate first provisioning must be declared by the provisioning operation; an empty journal/database alone does not prove genesis.
DECLARED_RESTORE is mandatory for backup/PITR restore, volume replacement with historical contents, or uncertain continuity. An unknown/mismatched store identity/generation or a known restore marker cannot silently fall back to LIVE_CURRENT. Restore mode requires the bundle below before privacy/admission gates can open.
The mode's trust comes from the protected deployment/restore procedure, outside the application DB rollback boundary. Restore tooling must mark/fence the operation BEFORE replacing storage and preserve that record through crash/retry. App callers cannot choose mode. Ordinary configuration is not proof against a malicious operator or unannounced rollback: as c13 explicitly states, a valid old DB cannot detect its own rollback. This contract excludes that operation; it does not pretend counters, timestamps or a new UUID solve it. Do not invent a periodic external attestation requirement for normal restarts.

PRODUCER144 — LOCAL JOURNAL + FINITE EXPORT
Keep the c18 monotonic journal in Moderation's durable DB, updated in the SAME transaction as recovered report binding/cleanup. Retain subject/terminal report dispositions after raw payload deletion. No external RPC in that transaction.
Implement a local ExportReportRecoverySnapshot interface/command: input a captured operation/cut and the intended store identity; output a finite snapshot file plus ReportJournalCut. A repeatable-read snapshot captures committed rows consistently. A restore-grade FINAL cut is taken only after affected writers/transactions are fenced and drained by the restore operation; a running periodic export is merely a backup sample.
ReportJournalCut contains protocol/version, logical store+incarnation, journal generation, snapshot UUID, bounded view identifier, observed local high-water, exact record count and artifact SHA-256. Artifact records contain only existing report UUID, slot, typed binding-to-opaque-Auth-subject OR terminal disposition, and opaque provenance/evidence reference/version from c18. No raw profile/account ID/hash, descriptions or content. Define one strict serialization with fixed field types; producer and consumer share fixtures. Reject duplicate keys/rows, unknown enum/version, invalid UUIDs and conflicting identities. A SQL sequence MAX is diagnostic, not proof of completeness or commit order; aborted/gapped sequences are not missing records by themselves.
Write a complete immutable artifact, verify/read it back and publish its manifest atomically; a partial file is never a complete snapshot. This command produces evidence only, never readiness. Operational backup includes/protects the artifact and its manifest under the existing backup mechanism; actual inclusion and recovery are verified, not inferred from a path/name.
A per-record remote outbox is optional future backup plumbing, not required by this selected seam. If one already exists, an ACK/empty queue proves only that protocol's acknowledged delivery, never absence of a later unexported tail. Ordinary local P5 does not depend on an absent remote endpoint.

RESTORE PRODUCER — OPERATOR RECOVERY BUNDLE
RestoreManifestV1 is supplied as a protected read-only file set by the authorized restore procedure, not a public API or arbitrary application request. It contains:
- operation UUID, environment, destination store incarnations, declared recovery generation and exact source/store/backup lineage;
- the finite required store/cut vector: Auth terminal/lifecycle, Mod sanctions/restrictions and report journal; User/Space/Messaging evidence where their existing participant contracts require it;
- artifact inventory with kind, exact byte length/hash, schema, generation, count/cut references and backup identity; include the actual report snapshot and all required recoverable history, not just their hashes;
- trusted operation references for quiescence/drain and old-writer fencing, recovery method and target boundary, independent custody/continuity evidence, and completed isolated recovery checks.
Select one supported evidence method:
(1) CURRENT_SOURCE_CAPTURE: the still-available authoritative source is fenced/drained, its current final cut and required history are captured and protected outside the storage being rolled back, then restoration/import uses those exact artifacts. This is implementable with the existing quiesced backup/isolated restore approach.
(2) VERIFIED_PITR_HISTORY: independently retained base backup plus actual WAL/history through the proven final cut, with store/timeline ancestry and replay verification. This method is accepted only if that material really exists; it is not currently established by the production backup doc.
The latest periodic dump, a filesystem mtime, signed/hashed manifest, matching old databases or operator text saying complete=true is insufficient for a lost post-backup tail. After unexpected source loss, if all acknowledged erasure/history coverage cannot be established, that restore remains AUTHORITY_RECOVERY closed. Do not replace missing history by an arbitrary earlier target time or declared RPO loss of privacy fences.
No new platform is assumed. The actual external gap is independently preserved complete recovery material for the particular restore, plus custody/quiescence/lineage evidence. Code can export, parse, verify and replay a supplied bundle now; code cannot manufacture that missing operational evidence.

Verifier, mode-transition rules and finite tests follow in the next comment.

BE-137 c21 frontier verifier and finite handoff, continuation of c23. Normal-live authority and declared-restore evidence are deliberately different; c18/c19 are refined only as stated in c23.

CONSUMER156 — TYPED INTERNAL SEAM, NO NEW RPC
Implement an internal recovery package with these conceptual inputs:
RecoveryStart {mode, bootID, environment, deploymentGeneration, expectedStores, expectedOperationID if restore}.
LocalStoreObservation {logicalStore, incarnation, schemaVersion, journalGeneration, observedCut, integrityResult}.
RestoreBundle {strict RestoreManifestV1, actual immutable artifact files}.
TrustedOperationContext {expected restore operation/generation/store bindings, protected operator workflow/evidence references}.
Return VerifiedRecoveryCut {mode, operationID, store/cut vector, artifact identities, verification receiptID}, then bind it to THIS BootGate bootID/generation. Keep construction of the verified result inside the verifier/coordinator; a caller-supplied UUID/bool must not produce a phase receipt.

LIVE_CURRENT branch reads current local observations/integrity and finite journal census; there is no required RestoreBundle or remote sink. Continue normal terminal import, recovery participants and144 read-only checker. No default transition to LIVE_CURRENT when restore inputs are missing, malformed or mismatched. No new User recovery lookup is added to public CreateReport.
DECLARED_RESTORE branch requires actual protected TrustedOperationContext from the deployment/restore operation outside the restored DB. Expected operation/generation are selected by that operation, not taken from the bundle being checked. A fresh restore cannot reuse the completed operation identity of an older restore. This prevents accepting a valid old bundle merely because its own metadata agrees with itself.
The file adapter uses operator-installed, read-only private artifacts with strict permissions and confined inventory paths; workload/public requests cannot supply manifests or arbitrary filesystem paths. This reuses the existing trusted operator/config boundary, not a new signing service. File ownership or a signature authenticates a source; neither alone proves a current tail. The trusted operation must carry real custody/continuity/quiescence/recovery verification evidence covering the selected final cut, as c13 already requires.

Verifier steps:
V1 Strictly parse supported schema and required fields; reject duplicate/unknown fields where protocol requires strictness, invalid types/UUIDs, missing/unlisted/truncated files, inventory path escape, conflicting store/operation/generation and unsupported lineage/method. Check declared artifact lengths/hashes against actual bytes; do not follow untrusted path indirection. Runtime may choose a deployment-specific mounted path; this decision creates no host mount or file.
V2 Verify the trusted operation's expected store/backup ancestry and final cut against its actual evidence. CURRENT_SOURCE_CAPTURE needs proof that all affected writers were fenced/drained and that the supplied artifacts were captured from that current source cut. VERIFIED_PITR_HISTORY needs actual backup/history inventory and isolated replay evidence through that cut, not just a claimed high-water. Validate native store/timeline identities with the recovery tool; do not compare LSNs from unrelated timelines/stores numerically. If the adapter cannot establish this evidence, return RECOVERY_EVIDENCE_MISSING/INCOMPLETE, not VerifiedRecoveryCut.
V3 Validate the entire finite report snapshot, enum/provenance constraints, record count/hash and immutable report-slot associations. A subject-binding conflict is RECOVERY_DIVERGENCE; terminal disposition dominates an older binding but does not license guessing a different subject. Imported opaque journal state is a monotonic union. Missing coverage for an unresolved legacy row stays c18 INCOMPLETE.
V4 Verify the other required domain cuts under their existing authority. Report recovery artifacts cannot certify lost sanctions/appeals, Auth terminal/lifecycle history, User ownership or Space erasure history. Partial per-domain agreement never upgrades the whole required cut vector.
V5 Import journal/terminal evidence and apply participant cleanup idempotently before restoring access to raw report rows/outboxes. Record durable progress for crash/retry, then run144 RequirePrivacyRecovery read-only. Only coordinator-verified completion of all required phases may advance existing BootGate. SQL rows recording verified cut/progress are not persisted readiness.
V6 Recheck current boot/generation at commit/dispatch boundaries. A changed store or declared restore requires gate closure and a new boot protocol, not a ready override. Cutover tooling fences old replicas before replacement. Already-dispatched RPCs retain the existing c13 in-flight limitation.

Finite restore completion can be retried against the same operation after a crash: immutable artifacts and per-batch receipts make it idempotent, while each process has a new boot receipt. The protected operation records successful cutover only after all required recovery checks; subsequent ordinary process restarts use the resulting current live store and normal P0-P5, not an eternal dependence on an old restore manifest. A later storage rollback is a NEW declared restore. No automatic mode change caused merely by a local ready bit.

FINITE IMPLEMENTATION ORACLES (not reported PASS)
F1 Current live restart, valid local store/journal, no export bundle and no remote sink: new boot gates start closed, finite local checks/terminal import/participants finish, P5 can open. Same for a rolling process restart; no external-endpoint retry loop.
F2 Declared restore without bundle/required trusted evidence: AUTHORITY_RECOVERY, zero report activation/public effects; private issuer/liveness may run. Missing bundle never falls back to live mode.
F3 Old periodic dump has internally valid hashes/generations/counts, but no post-dump current-tail proof: reject restore completion. Auth+Mod both rolled back and mutually consistent are equally insufficient.
F4 Planned restore: fence/drain current source, export actual complete journal and required domain histories, preserve verified independent artifacts, restore an older NULL report backup, union current journal+terminal evidence, scrub before access, checker then finite coordinator completion. Repeat direct terminal report receipt without Auth subject.
F5 WAL/PITR method: wrong store/system identity, wrong timeline ancestry, missing required recovery segment, replay short of final cut, or absent isolated verification rejects. A valid completed native recovery through the proven cut is accepted; no requirement to invent a second sink.
F6 Corrupted/truncated artifact, duplicate/conflicting record, wrong count/digest/schema, unsafe path or unknown outcome cannot produce a verified receipt. Gap in a SQL sequence alone is not corruption; high-water alone cannot hide a missing record.
F7 Old signed/sealed manifest replayed under a new restore operation, wrong target incarnation/environment or mixed cut vector: reject. Trusted operation identity comes from protected workflow context, never from that old manifest itself.
F8 Crash after export temporary file, after manifest publish, during import batch or between participant completion and P5: partial export is not visible as complete; local atomic journal remains sound; next boot repeats safely and never reloads ready=true.
F9 Concurrent mutation/export: ordinary finite RR snapshot does not promise an externally current restore tail. Final restore export after fencing/drain includes all committed required records; no snapshot taken before final writer drain may claim that property.
F10 Source loss after periodic backup with no recoverable acknowledged-erasure tail: preserve a precise missing-evidence reason for that restore. Normal current live installations remain eligible for F1; no global product or all-restart hold.
F11 Late callback/worker at generation cutover is rejected; old instance cannot publish/commit under a new operation. No new per-record remote ACK, User public dependency or target existence oracle is introduced.
F12 A report snapshot is complete but Auth/Mod sanction/User-required history is not: overall required recovery remains incomplete. Conversely, when all finite required evidence exists, no unimplemented generic sink or unrelated Role check may keep P5 closed.

RESUME / OWNERSHIP
144 implements local atomic opaque journal, strict finite export/import support and journal-aware read-only privacy checker. It can proceed without a runtime sink or remote outbox. Export is an offline recovery producer, not a readiness service.
156 implements LIVE_CURRENT/DECLARED_RESTORE adapter, finite file verifier/proof type, actual P0-P5 composition and current-operation generation checks. Document actual trusted operator inputs and install boundary; do not invent VerifiedRecoveryCut from test mocks in production. New terminal seal remains useful but insufficient for restore currentness.
172 only coordinates shared canonical User ownership/lifecycle proof where c18 needs it; no Role selector/key/grant borrowing.183/121 receive the decision for their existing work/integration; neither inherits specialist source or stand custody.
Actual external restore evidence (source final-cut capture or real independent WAL/history and protected workflow receipts) must be produced during an authorized restore/backup operation. None was produced or inspected on a stand in this turn. Missing such evidence is an operational integrity limitation of that proposed restore, not a product question and not a reason to disable every ordinary live restart.
No source writes, tests, CI, GH/stand probes, key/listener provisioning, lease/resource changes or child agents. Standard implementation and future proof remain144/156.
