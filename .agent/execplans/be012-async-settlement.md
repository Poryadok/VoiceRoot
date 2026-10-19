# BE012 async provision settlement

## Purpose and authority

Execute the accepted BE012-ASYNC-PROVISION-RESET-v1 (Astra169 BE012c9,
attachment1535c157ae3296729f9b20686baeddde). A roster cancellation must not hide
possibly created Chat/Voice resources, issue a previously unissued provider after
abandonment, or activate a changed party. Generic RPC errors prove no absence.
BE011c16 MEMBER-ACCOUNT-AUTHORITY-v1 additionally requires fresh private tuples
and transactional all-account platform-ban checks at each forward boundary.

Actual author Рудеус-147 / Sol6.1 medium, fresh focused continuation. Atomic
supporting-task claim e2 and explicit boundary acceptance c3 preserve genuine90
BE059c12/functionc1 historical custody; BE059 full scope/CI remains90. No other
provider writer is active.145 owns candidate/private authority/ban guard/main,
022 and queue projection;021 and provider settlement are owned147.

## Checkout and resources

Safely returned old WT38 only after immutable BE012 checkpoint2b920a0fd37b502384e02f091b928e3969ebbd8d,
bundle verification and clean/no-host-process state. Fresh actual develop
06935825edaeca8db253934c8bb4f4e892940456 was read immediately before new branch.
WT31 lease1e45594404be44c8e28f57daba0a7889, holderrudeus-147-be012-async,
branch codex/be012-async-settlement. Normal merges restore exact own checkpoint
and14568ac9bdfb85d9f126c691de5f804404981d5399a; no history rewrite.

Retained owned Linux runner1ea2c8b and portless PG93e0 from the prior checkpoint;
external proof D:/Git/Voice/tmp/swarm/rudeus147-be012. No host tests/listeners,
firewall change, GH probe or shared DEV/STAGING claim. Own CI/source custody
persists until actual supported receiving acceptance.

## Implementation sequence

1. Migration021: monotone operation disposition/revision and per-provider
   NOT_ISSUED/MAY_HAVE_CREATED/RECEIPT_KNOWN markers with permanent first dispatch
   identity/time. Existing pre-marker frozen requests migrate conservatively;
   do not infer no dispatch from a missing receipt. No dispatch side table:
   existing candidate full-operation JSON automatically includes these fields.
2. Extend compensation required set to []/[chat]/[chat,voice], immutable after
   abandonment. Required-but-unknown creation has an explicit unresolved slot;
   no missing participant means success. Materialize request bytes only after a
   validated creation receipt, once and with exact frozen teardown identity.
3. Short caller-Tx latch invoked by every preactivation abandonment, including
   roster reset, decline/deadline and historical abandoned-intent reconciliation.
   Active-match winner retains its existing lifecycle policy. Rollback restores
   match, party/session state, markers and compensation evidence together.
4. Common coordinator uses fresh PartySourceGate before intent, each forward
   dispatch/retry and final activation, request and both background scans. Unknown
   performs no forward mutation/cancellation. Commit dispatch marker before RPC.
   New account guard API acquires sorted shared account advisory locks before
   match/completeparty/completesession/proposal/operation locks; sanction writers
   hold exclusive matching locks. Separate READ COMMITTED ban read after locks,
   VerifyProvisionCandidateTx and transition commit in one local transaction;
   no network IO under locks. Consume actual145 immutable API before enablement.
5. ABANDONED settlement bypasses fresh party eligibility (revoked subjects must
   not strand cleanup). Replay only an already-issued exact original Create to
   obtain its original receipt and tear it down; never issue NOT_ISSUED Voice
   after late Chat receipt. Accept valid late receipts while compensating,
   preserve conflicting evidence/alert, clean independent known resources first.
6. Complete only after every frozen required provider has valid creation AND
   teardown receipts. No activation/usable IDs/mm.match_completed from abandoned
   settlement. Existing final-leave completion/compaction rules remain distinct.
7. Use fresh bounded10s provider attempts and existing durable exponential retry
   (1s+jitter,5min cap), progress15min/contract mismatch signals. Old unmarked
   dispatchers and credentials require actual cold retirement before activation;
   migration or a configuration boolean cannot certify this.

## Risk proof and delivery

- [ ] Real PG RED/GREEN marker-before-dispatch and same-Tx abandon rollback.
- [ ] Before-intent and beforeChat no-provider close; known/unknownChat with
      unissuedVoice seals chat-only obligations, never startsVoice.
- [ ] Issued unknownVoice plus knownChat: cleanupChat proceeds independently;
      aggregate remains pending until exactVoice creation and teardown receipts.
- [ ] Both receipts beforeactivation, concurrent accepter/worker/reset, all
      marker/RPC/receipt/cleanup crash boundaries, duplicate late callbacks.
- [ ] Generic terminal/malformed response stays unresolved/mismatch; no fake
      absence. Legacy migration and cold-writer retirement acceptance separated.
- [ ] Actual Chat/Voice Create replay after teardown cannot restore access/media.
- [ ] Missing/unknown candidate or all-account guard restrictive; ban races
      serialize under the accepted account-first local lock order.
- [ ] Affected Linux suites/vet/lint, generated/canonical checks and graph update;
      independent review, portable full proof and accepted121 integration/CI.

Prior native205 CLEAR and existing PG/consumer/reset/gate proofs are retained and
reused where input/risk is unchanged. Old baseline/harness failures remain in
PROOF.md; a prior local PASS is not hosted CI, coverage certification or rollout.

## Storage checkpoint 2026-10-18T23:30Z

Immutable test-first45c018b6ef35455be25b7b0554f6bad7f473dddf reproduced
same-caller-transaction abandonment RED (expected ABANDONED, actual LIVE).
The current storage delta adds monotone dispatch/disposition fences, dispatch-
backed unresolved compensation slots, strict prerequisite Chat receipt before
Voice dispatch, replay binding checks and no synthetic aggregate for no dispatch.
Historical requests are conservatively possibly-issued; receipt absence never
becomes proof of absence. Migration DOWN refuses any operation evidence; empty
UP/DOWN/UP restores the predecessor constraints.

Linux PG results in external rudeus147-be012/:
- settlement-red.*: retained initial RED,7.009s.
- settlement-green.*: caller-Tx latch/rollback GREEN1,0skip,11.768s.
- settlement-monotone-red.*: direct SQL reopened disposition (RED preserved).
- settlement-monotone-green.*: monotone storage fence GREEN1,0skip.
- settlement-unissued-red.*: synthetic aggregate for no dispatch (RED preserved).
- settlement-storage-v2-green.*: no-dispatch closure plus rollback/roundtrip
  GREEN3,0skip,14.953s.
- settlement-obligation-red.*: fake unresolved aggregate without abandoned
  dispatch evidence was accepted (RED preserved).
- settlement-storage-v4-green.*: final storage/replay/evidence guards
  GREEN3,0skip,29.636s; exact input file SHA256 inventory retained alongside.

This is storage groundwork, not the complete risk matrix above. No production
reset callers, fresh candidate/ban-guard transition, late-receipt materialization,
provider settlement dispatcher, independent review or activation is qualified.
The original commit process has committed45c018b6 and remains in its automatic
shared-object repack (own chain50924/884/38008/40896/42264/50644); no maintenance
process or shared Git configuration was killed/changed. Current source changes
remain on the sole owned WT31 lease while useful PG work proceeds.

Late Chat receipt extension: settlement-late-chat-red.* preserved the original
compensating-state rejection. Existing Chat/Voice receipt methods now take the
match fence before operation, retain all original request/receipt validation,
require an issued marker in021 mode, advance RECEIPT_KNOWN/revision and accept
exact abandoned replay. Atomic compensation materialization fills only the
pre-frozen teardown identity and moves WAITING_FOR_CREATION to NOT_STARTED.
Migration trigger functions preserve predecessor definitions under pre021 names
for guarded empty rollback; existing non-NULL bytes, receipts and terminal states
stay immutable. A late Chat receipt never changes Voice's NOT_ISSUED marker.

settlement-late-chat-green.*: Linux PG4PASS/0skip64.202s includes the three
storage tests and existing full durable provisioning/receipts/lifecycle test.
Exact source SHA inventory is recorded before run. No mock provider absence or
coordinator/public activation claim. Graphify update had no output; only owned
Python35320/launcher47360 were stopped after the bounded window, wrappers exited0;
this is NOT a Graphify PASS. Git maintenance finished normally;45c018b6 process
terminal0 confirmed, no shared maintenance/configuration change.

Next: known Chat cleanup while Voice creation remains unknown, durable conflict
evidence, receipt/teardown lock-order compatibility; consume145 immutable
2a7079832dd893cf944de5a22f6356484b1830dc private tuple/Accounts/022 guard APIs by
normal merge. Fresh ban Check stays after inner locks, with account locks outer.

2026-10-18T23:46Z: known-Chat/unknown-Voice real PG RED11.672s demonstrated
that the latch had left known cleanup waiting. The latch now materializes every
already-known required provider in its own transaction. Teardown receipt and
compaction preparation acquire match -> operation before aggregate/intent locks;
record-compaction receipt remains intent-only and does not climb the lock order.
The new case cleans Chat while Voice remains WAITING_FOR_CREATION, forbids any
compaction yet, accepts late Voice creation/cleanup, then closes both with the
match still abandoned and zero completion events; exact receipt replay stays closed.
Additional premature-close RED preserved (direct SQL could hide unresolved work);
021 now forbids closing abandoned issued work without known creation bindings
and a complete aggregate for the exact required set.

settlement-independent-v2-green.*: 5 real PG PASS/0skip26.716s, including
existing full provisioning/lifecycle and storage roundtrip. Input is immutable
1445e74d811b7c9cf0393dc3203f3ccf5cd9c0f9 plus exact listed source SHA deltas;
no actual provider replay or enabled worker recovery is claimed. Remaining named
work: durable conflict evidence/alerts, common validated intent/dispatch/activation,
every reset/deadline/decline latch, abandoned recovery scan and real provider tests.

2026-10-19T00:22Z forward store checkpoint (worker still unwired):

- `ProvisionValidationProof` consumes the actual private partysource attempt via
  Hash/Matches without a store->partysource import cycle. All three local APIs
  acquire READ COMMITTED account guard before match -> complete parties ->
  complete sessions -> proposals -> operation, then fresh ban Check, Verify and
  transition/commit; no external IO occurs in the transaction.
- Validated intent uses protocol1 with both providers NOT_ISSUED. Dispatch
  authorization commits MAY_HAVE_CREATED before returning a ticket; retries keep
  permanent first-dispatch identity/time and advance revision, invalidating the
  consumed cut even when dispatch state was already MAY_HAVE_CREATED.
- Final activation requires exact operation/protocol/LIVE/both-known bindings
  and the same fresh proof/guard. Legacy public activation cannot bypass protocol1.
  Caller-Tx extraction retains original immutable request/receipt validation and
  predecessor legacy-schema behavior in its explicitly legacy entry points.
- 021 post-UP INSERT fence rejects new unmarked legacy intents. Historical
  conservative backfill stays separate; this is not proof old in-flight writers
  stopped or that any missing receipt means no effect.

Actual proof partition (external JSONL/stderr/exit and source SHA inventories):
- settlement-dispatch-red: initial stub Pending RED; dispatch-v2-green:
  3PGPASS/0skip24.170s (non-initiating member ban, nil/stale proof, marker/abandon).
- settlement-intent-refactor-regression: original full lifecycle PASS1/0skip17.846s.
- settlement-activation-red: missing final gate RED; unmarked-writer-red:
  upgraded schema previously accepted protocol0 intent (RED retained).
- settlement-forward-green: 10PGPASS/0skip (partysearch15.048s, store58.105s),
  including activation/ban/stale cut, both abandonment/activation winner outcomes,
  provider receipt/cleanup and guarded migration cases.
- settlement-proof-reuse-red: a retry reused its proof, RED retained; revision
  fix covered by full settlement-party-package run:21top+10subPASS,1topFAIL,
  0skip156.376s. The sole failure was own older gate Snapshot fixture missing
  newly required private authority fields; production validator unchanged.
  Only that fixture changed, then settlement-party-fixture-green PASS1/0skip14.985s.
  Original full-package FAIL remains preserved, not relabelled a full-run PASS.
- settlement-forward-short:230PASS/176explicitPGskip/0FAIL; forward-vet store,
  squad and partysearch terminal0. Host lint diagnostic terminal1 reports only
  the three existing unwired roster-runtime helpers; not a lint PASS and its
  working input overlapped new skeleton edits, so no immutable certification.
- Second Graphify update again produced no output; only owned40100/49756 stopped,
  wrapper terminal0; NOT a graph update PASS. Actual WT31 lease/holder unchanged
  on readback, no foreign process/lease/resource changed.

Remaining: real typed local-ban denial -> exact affected-party reset (145 narrow
private error accessor is publishing), every source/decline/deadline abandonment
latch and historical repair, durable provider mismatch evidence/alerts, request
and both background coordinator paths, exact abandoned Create settlement and
actual provider replay/no-resurrection proofs. LFP169c23 is a separate typed
MM-owned consent source; no NULL/native/SOLO compatibility bypass was added.
222/121 dormant A3 relay repair remains independent; later delivery still needs
220v6 plus145coverage and147 actual productPG commit-beforeACK/lostACK evidence.

2026-10-19 00:35Z checkpoint: immutable forward boundary2710dd1 plus normal merge typed-denial17733 (includes191 coverage dependencies, no new activation). Native reset real PG first harness assertion mismatch retained as settlement-native-reset-red, corrected assertion then actual LIVE-vs-ABANDONED RED in v2-red; same-Tx hooks GREEN1/0skip8.215s. Cancellation outbox CHECK failure proves reset and latch rollback; successful reset freezes Chat-only issued set, never Voice. Deadline/decline/common reset now latch under existing locks; explicit AbandonMatch uses same locked transaction, keeps active/completed winners. No-operation old schema path does not create synthetic obligations; existing operation missing dispatch metadata fails closed.
Legacy closed unknown-Voice upgrade RED reproduced23.356s. UP now holds operations+aggregate locks and refuses sealed legacy obligations that need reconciliation, before any schema mutation. GREEN2PG/0skip40.811s including existing backfill/rollback proof; failed migration preserves old closed evidence and installs no dispatch column. Reset regression3PG/0skip31.783s (decline durability, history abandonment, original latch), scoped vet terminal0. Proof names settlement-native-reset-*, settlement-reset-regression.*, settlement-legacy-upgrade-*, settlement-reset-vet.*. Original failed results retained. This is storage qualification only; common worker/recovery/PG+broker acceptance and independent review remain open. No stage/GH access, custody147/WT31 retained.

Graphify AST update bounded with no output; verified owned child46760/launcher48112 stopped after bound. No graph PASS claimed.

2026-10-19 00:50Z coordinator slice: PartySourceGate.ValidateAttempt preserves the actual private ValidatedAttempt, with ValidateProvision compatibility wrapper. Exact typed PlatformBanDenial evidence maps only affected immutable captured parties; generic/foreign/empty evidence remains Unknown. Real PG2 PASS/0skip6.829s (private gate and prior unknown/atomic reset). This narrow mapping was implemented before its integrated regression; no test-first RED is claimed for that mapping.

Provider request and both resume scans now use the common authority gate at intent, each live forward dispatch and final activation. Missing validator RED reached Chat RPC; GREEN1PG10.964s prevents any RPC/marker. Expanded missing-intent request and crash scan also leave zero intents. Terminal Voice status after MAY_HAVE_CREATED retains possible dispatch; no Chat-only compensation is inferred. Compensating operations remain discoverable, reload durable metadata and replay only previously issued exact requests to obtain original creation receipts for teardown. Cleanup does not need revoked eligibility; no NOT_ISSUED provider can be added after abandon and public resource IDs require ACTIVE. Real PG replay RED1Voicecall -> GREEN1PG5.726s; an intervening test-only completion-events query column error is retained in first green log, corrected without production relaxation. Late unknown Chat proof with 10s bounded context initially RED(no deadline); GREEN2PG includes exact late Chat cleanup materialization/no new Voice and expanded unknown/missing-intent gates. These provider clients are doubles: this proves coordinator/PG protocol, not real provider absence/tombstone convergence.

Scoped coordinator vet0 and full Linux production build0. Pre-final-timeout coordinator short230PASS/180explicitPGskip/0FAIL, historical forward short proof retained. Actual provider stores, durable provisioning claim/backoff/operator visibility, final immutable independent review, fully assembled main/cold coverage/NATS-v6 and real PG+broker commit-before-ACK/lost-ACK remain OPEN. Source/CI147 and same WT31 lease; no runtime/stage enablement claimed.

Provider-attempt GREEN2PG/0skip11.161s; scoped final vet0. Host golangci terminal1: exactly the same three unused voice_roster_runtime symbols pending145 main consumer, no suppression/no lint PASS. Current AST update again bounded/no output; exact owned child21420/launcher37804 stopped after parent-chain verification, foreign graph processes untouched. Diffcheck0. No runtime readiness inferred.

2026-10-19 01:10Z retry checkpoint: normal merge145 ac946 complete-roster CreateProposal as d4ea55d389d12bc87729f569e17674384903383e; no edits its proposal hunks. New Linux immutable retry-base archive is d4ea55 plus owning deltas. Durable claims serialize provisioning/compensating attempts, increment generation/attempt count, bound whole attempt55s under60s claim, retain10s independent provider calls; claims are scheduling only, never effect/absence evidence. Persisted exponential jitter uses existing1s..5m helper. Scheduling/release CAS cannot overwrite a successor claim, non-finite/private error classes rejected; concurrent8 claimants produce exactly1 winner. Claim/backoff storage stub RED3.629s -> GREEN; worker immediate second Voice RED2.310s -> GREEN2PG18.485s.

No-progress counts use15min DB time; actual first dispatch/new receipt/first abandonment advances progress, retries/claims do not. Contract-status classes produce an operator-attention error with finite counts/no IDs. Progress RED6.060s -> GREEN1PG10.682s. Worker+store focused race5PG/0skip PASS13.709s including8-way claim race, affected request/missing-intent idle-backoff/eligible retry, exact ABANDONED replay and late Chat/no new Voice. Idle scans during durable backoff return0 work; once due the same authority gate still returns Unknown/no RPC. Finite error-class mismatch found by inspection then unit RED->GREEN0skip; unknown transport classes now persist as unknown rather than rejected teardown-only spelling. Final scopedvet0, production build0; exact9-file Linux/host SHA comparison0differences in settlement-retry-source-sha256. No repeat of unchanged source CLEAR. Current retry lint was bounded/no output: verified own golangci46800(parent37696 with owning log) stopped, no lint PASS; prior explicit3-unused diagnostics retained. AST bounded/nooutput own44128/33188 stopped; no AST PASS.

Remaining concrete c9 risk: known contradictory creation receipts need durable per-provider CONTRACT_MISMATCH evidence and a sticky page/closure fence; the current last_error_class is retry diagnostics and must not be represented as that complete mechanism. Known provider cleanup receipts must still persist independently, while unresolved mismatch prevents aggregate/saga completion. Generic terminal statuses remain unknown, never absence. Prepare a coherent supporting function for that accepted contract, consume it in147 coordinator; no new architecture or absence oracle. Real provider delayed/concurrent replay-after-teardown/compaction, actual MM PG+NATS C4/C5, assembled startup and independent source acceptance remain OPEN. Same WT31/runner+portlessPG; no DEV/STG/GH access; source/CI147 retained.

2026-10-19 01:35Z real runtime seam qualification: new additive provider tests passed actual Linux PG with race, no production provider change. Chat package reported10.844s and Voice8.804s, one test each/0skip. Both delay an original request in transport, race exact Create arrivals with real teardown, then release original after terminal state: retained receipt bytes match and no Chat members/Voice projection/media-Ensure resurrection. After valid protected synthetic compaction attestation, concurrent same-Create arrivals return tombstones without fabricated creation/absence receipt. Voice SQL ownership/receipt state is real PG with full owning migration chain; media/projection/Auth adapters remain in-process, not physical LiveKit or owning/game-drain qualification. Existing sequential native PASS was not repeated/widened.

Product C4/C5 now actual PG+native pinnedNATS2.12.12 race1PASS/0skip11.496s in settlement-product-pg-ack-v4. Product MatchStore.ApplyVoiceRosterEvent blocks at an actual cancellation-outbox table lock; independent PG sees no committed inbox and broker ACKfloor0/pending1. Forced outbox CHECK failure rolls back real inbox/party/session changes and is not ACKed. Retry commits inbox+2 cancellations+generation reset; test closes actual consumer TCP after COMMIT before product ACK. Native broker retains pending; a new production Consumer binds same unchanged durable/Created/config, redelivery ACKs exact committed inbox with one row/two cancellations/unchanged generations. Original cutF1/L3/M1,targetS2 and consumed sequence/ACK are checked. Broker image immutable2ca98656a279b2d88cfdf2b8c3f0d5d7f3941ae9dc2ab12ebaa92d83e0f4ccdb, actualversion2.12.12; mutable2.12-alpine was2.12.15 and was not used for this proof.

Preserved harness failures: first C4 compile used nonexistent SourceParty type; v2/v3 failed port-isolation guard before product test because testcontainers0.35 auto-merges image ports after HostConfigModifier. Source inspection established this behavior. v4 explicitly shares ONLY supplied owned Linux runner namespace, asserts emptyHostConfig.PortBindings and uses loopback; all fixture brokers terminated and no firewall/settings changed. No product failure inferred. Fixture uses no installed ACL qualification; accepted220/121 authority, real installation and145 assembly remain separate gates. All three affected package vets0; three new source files match Linux/host SHA exactly. C9 mismatch function BE012-CREATION-MISMATCH-EVIDENCE published AVAILABLE with exact69a2 baseline, no fake receiver or waiting. Text attachment c11 returned internal_error, not retried; immutable Git+portable TT comments preserve evidence. Source/CI147 retained.

### 2026-10-19: sticky creation mismatch continuation

Original author147 atomically claimed BE012-CREATION-MISMATCH-EVIDENCE e2;
explicit acceptance c1 retains the same primary WT31 lease, no foreign handoff.
Actual develop8afe8e90285a was read before this continuation. Searches at rev3749
and consumer145 confirmed MM023 unreserved; 021 and 022 remain unchanged.

MM023 adds per-provider first response bytes/hash/time to the immutable operation.
The row already binds original operation/provider/frozen request. Store API checks
the original operation ID and request digest under match -> operation locks.
Exact witness replay is idempotent; another response cannot replace the first.
Direct SQL cannot clear/rewrite it or newly activate/close/issue another provider.
Generic terminal status, timeout and absent response remain uncertain, not absence
or a positive contradictory receipt. The protected coordinator records actual
receipt bytes on invalid ID/binding; database failures remain failures, never a
durable-success claim. No response bytes or resource IDs enter the finite alert.

Both LIVE coordinator boundaries and validated storage transitions fence forward
dispatch/activation. An ABANDONED already-issued replay still obtains original
receipts solely for cleanup. Each known teardown receipt commits independently;
the last receipt leaves the aggregate pending when negative evidence exists.
No completion event or compaction intent is then produced. A contradiction arriving
after an ordinary closure is retained and pages without reopening the operation or
rewriting its sealed provider set. PrepareCompaction checks even an already-frozen
intent. Operator counts use sticky facts, not replaceable last_error_class, and
include closed operations with late evidence. No automatic reconciliation clears
these facts. Runtime startup requires the new schema; no activation is claimed.

Real Linux PG/race evidence (all logs preserved under tmp/swarm/rudeus147-be012):
- Initial schema/stub RED5.481s: evidence column missing; then GREEN15.342s.
- Old coordinator production path RED3.680s: ErrConflict without sticky witness;
  corrected common path GREEN5.531s, actual invalid Chat receipt preserved, fresh
  source cut cannot authorize retry, restart/Unavailable cannot erase alert.
- Old teardown implementation with new evidence schema RED14.842s: final receipt
  transaction rolled back on completion guard. Narrow predicate correction makes
  both receipts persist and leaves aggregate pending.
- Final storage3 PG tests PASS40.210s/0skip: independent final cleanup, late closed
  mismatch, unchanged ordinary valid settlement path. Evidence DOWN refusal and
  direct SQL immutability/completion guards are exercised.
- Four coordinator regression PG tests PASS29.467s/0skip: uncertain terminal Voice,
  source Unknown, abandoned exact Chat replay/no new Voice, progress-clock rules.
  Terminal status alone now correctly does not count as a sticky contradiction.
- Empty MM023 UP/DOWN/UP real PG PASS12.041s. Scoped vet0 and full MM build0.
- Scoped short:24 PASS/108 PG skips/0fail, explicitly not runtime PG coverage.
- CodeGraph was consulted first; frozen index required current-source reads.
  Graphify update bounded without output; Treehouse identified own cwd processes,
  CIM proved15316 -> 57384 -> 50412; only verified own child/launcher stopped.
  No AST PASS. No Windows test executable/listener/firewall changes, stands or GH.

Source/CI remains147 until an actual accepted handoff. Independent review of this
new production delta, complete settlement source, full assembly, installed NATS
rights/JWT/TLS, owning media absence, cold cut and rollout remain separate gates.
Earlier285 CLEAR applies only to exactbee8 additive tests/proof fidelity.
