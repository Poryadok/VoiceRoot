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
