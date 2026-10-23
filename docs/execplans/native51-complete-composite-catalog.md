# ExecPlan: Native51 complete-composite catalog and atomic installation

## Purpose

Install a distinct source-qualified Native51 catalog before production readiness,
preserving complete fresh catalog observations and unchanged Ban effects/budgets.
This plan is implementation WIP, not admission or runtime acceptance.

## Context

Base ca31c7e630b4afc82992d681788708045cd555bb. Decisions: acceptance66591bfe,
finite type759b82c3, atomicityda078296; consumer Isaac-2 allocated migration51 and
ACKed41 exact paths. Author Isaac-26-SH owns task NATIVE51-CATALOG-INSTALLER-26,
WT31 feature/isaac26-native51-catalog-installer. Documentation authority:
Space service Native readiness/Ban contract, DATA_MODEL/DATA_STORES,
ARCHITECTURE_REQUIREMENTS, TESTING and CONTRIBUTING. Original M44/M45/M50,
Transition50:107 and full107/307/retry test bodies remain byte-identical.

## Scope

The new profile includes complete source-derived PG16/18 composite/type/output
qualification, full row JSON encoding, exact source pins, genuine old50-parent
CAS and production installer continuation. Exact finite51 dispatch only; no
unknown-version inheritance, partial catalog, callback/cache removal, RPC/provider
change, force marker or adoption of unknown installations. Existing325 fresh calls,
full cohort/effects, monotonic<5s and owner<=2s remain mandatory.

## Milestones

- [x] Freeze exact source/path/ownership and single-transaction protocol.
- [x] Author source/phase/qualification tests before implementation; execute only
      when a separately approved Linux source-check window exists.
- [x] Implement finite classifier and separately pinned catalog51 definition.
- [x] Implement real migrate4.18.1/file-source adapter and atomic51 transition.
- [x] Connect genuine production44→45→50→51 receipts and restart HOLD.
- [ ] Freeze exact source; independent review before any runtime request.
- [ ] Separately qualify current candidate raw stability/type/transition/installer
      on PG16/18, then unchanged full107/307/retry under original resources.
- [ ] Independent final review, normal feature commit/push and integration offer.

## Detailed Steps

The pinned external driver still executes through50, including unchanged real
Transition45/50. Only51 uses the real pinned Go migrate engine with source/file
and a narrow transaction-bound database.Driver. Its Lock owns one ReadCommitted
transaction, standard migration session advisory lock and same-key xact lock,
marker/epoch/full relation locks, actual same-run trusted50 facts and drain.
The real engine selects exact51 SQL and calls dirty history→Run→clean history.
DDL, full new source checks, genuine old50 reconstruction, epochCAS and marker
changes remain in that single transaction. Driver.Unlock releases only its own
framework session lock; it NEVER commits. Explicit outer CommitQualified runs
only after engine.Migrate(51) returns nil, exact completed phase and final fresh
checks. Any source/control/driver/unlock/context/window failure rolls back all
three. Old50 child and old transition semantics are unchanged.

The pinned engine can return a lock timeout without joining Driver.Lock. The
adapter owns a cancelable acquisition context and a completion signal. Error
handling and Close first prevent late starts, cancel in-flight acquisition and
join it before accessing worker state or releasing its backend. Concurrent
cleanup calls serialize. No late Lock can publish state after Close returns.

The full source-derived classifier runs before unknown datum/row serialization.
It admits only compiled complete layouts/dependencies and controlled-source
missing arrays: M6 bool[true], M21 int8[0], M23 bool[false]. Source-derived NULL
requires consistent metadata and qualified lineage. Actual complete array_send
bytes bind the three finite type/value/shape cases; arbitrary/custom/future
values remain unavailable. Every complete catalog field is still serialized.

Finite type proof is one input. Admission requires exact independently reviewed
v2 proof plus current definition/session stability qualification. An absent or
unbound qualification denies BEFORE51 mutation and before installer complete.
There is no runtime flag or caller-supplied admission receipt. Do not label the
normalized prototype screen raw digest stability.

Fresh provision rejects ANY existing database. Verify is read-only and requires
actual independently retained SAME image/source/executable/migrator/inventory/
target completed evidence with real45/50/51 transitions plus fresh full51 checks.
Missing/partial/old50 evidence, ambiguous commit or crash-before-complete means
HOLD; no automatic resume, marker reset, dirty force or endpoint reseal.

## Validation

The current source-check ACK permits bounded private Linux compile/unit/vet,
without PG/network services. Execution is pending, never claimed from source
inspection. Separately ACKed PG16/18
phase failure/rollback/concurrency/source/type/session/currentness checks.
For each injected failure after body/CAS/marker, body+epoch+marker must all revert.
No skipped tests. Complete real installer image/source chain must reach51 before
any readiness. Original full107/307/retry files are hashed and run unchanged on
the genuinely admitted51 candidate; old7996.396s remains separately labeled RED.

## Progress

- Exact41-path source-only scope and outer-commit protocol ACKed.
- Pinned real migrate4.18.1 and exact required transitive source downloaded/
  checksummed; no tests/PG/Ban run at the source-authoring checkpoint.
- Complete87-file SQL inventory, finite51 dispatch, controller evidence/tests,
  actual-candidate session/unknown-datum tests and real-engine failure/lock
  retention tests authored. These are implementation milestones, not test PASS.
- Actual finite type-probe PG16/18 PASS and independent11 CLEAR reported; binding
  exact final evidence and production candidate remains part of implementation.
- No rawN current-candidate stability or performance result exists.
- Actual R0v4 on77f1384 returned PG16 test FAIL; the retained assertion identity
  and after-failure catalog state are UNKNOWN. No rollback claim follows from
  cleanup. Independent normalized oracle PASS does not qualify the Go test.
- Read-only diagnosis found a raw-preimage/digest contract mismatch: bare source
  assets were treated as wrapped digest SQL, including the candidate RETURN,
  reconstructed old50 parent and proof oracle. Two offline Linux contract tests
  fail on77f source (tree348371b4); this is a separate proven regression, not a
  reconstruction of the discarded R0 failure. The correction uses explicit raw
  preimages and UTF8 SHA256 envelopes without changing the complete projections.
- R0 assertions now emit fixed check/session/error enums without DSN, SQL, raw
  errors or catalog values. All11 session controls and semantic/narrowed/raw
  stability checks remain; actual digest/oracle length32 is additionally checked.
  No new PG/R0 run is authorized by these source changes.
- Independent11 found the pinned engine's unjoined timeout in f1816572. That
  immutable packet is retained. The successor owns cancellation/join and adds
  an actual pinned-engine timeout negative blocked at real pgxpool acquisition,
  plus a Close-before-Lock negative; neither creates a connection or native proof.

## Decisions

One PostgreSQL transaction is required for51 DDL/epoch/history; the superseded
split goto51/postinstall CAS is not implemented. Source/file/real engine history
hooks provide canonical migration selection, not a manual metadata stamp.
Driver.Unlock never commits, including engine errors outside the adapter.

## Risks And Follow-Ups

Classifier evaluation order and complete builtin closure must be independently
reviewed. RawN must be stable whenever original canonical preimage is stable on
the same cut across supported sessions. Performance is unmeasured; no deadlines
or325 observation sites may be reduced to obtain success. FullCI/Native/P3/
provider/deployment acceptance remains separate. No Windows Go test execution.
