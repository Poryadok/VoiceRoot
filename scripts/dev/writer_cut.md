# DEV writer cut consumer contract

The central operation owns the admitted writer-to-database mapping, real
database observation, common claim/lock, backup, and migration. This component
does not discover all writers or establish generic external-I/O quiescence.
Use only with the existing shared Journal, Fence, `KubectlTransport(journal)`
and its `KubeLedger`; no independent live invocation is authorized here.

Put `writer_cut` in the immutable private journal intent before constructing
`WriterCut(journal, fence, client, lock, namespace_uid, admission, probe)`:

- `controllers`: nonempty list of exact admitted Deployment/StatefulSet
  Kubernetes objects with namespace, name, UID, resourceVersion, full spec and
  explicit integer replica count. The central owner qualifies selection.
- `guards`: `jobs`, `cronjobs`, `hpas`, each the complete current namespace
  list projected by `writer_cut.inventory(items)`. This retains owner, identity,
  spec and status and holds if that full inventory changes. `linked` has `jobs`
  and `cronjobs` arrays of affected UIDs, qualified by the central owner.
  Linked Jobs must be terminal, linked CronJobs suspended and inactive; an HPA
  targeting any selected controller holds. This code does not suspend them.
- `session_scope`: the pinned affected engine/database/role scope, qualified by
  the central owner. `probe(scope)` must query actual bounded read-only session
  and transaction state and return exactly `scope`, `active_sessions` and
  `active_transactions`, with both counts integer zero. Synthetic callbacks are
  test fixtures, not runtime evidence.

For the established DEV PostgreSQL engine, the central caller can bind
`writer_cut_pg_probe.inspect(scope, cut._guard)` to `probe`. Its immutable scope
is `{engine, databases, database_oids, server_start}`: the engine is the exact
guarded Kubernetes snapshot, databases are the qualified sorted affected set,
and every name is bound to its observed PostgreSQL OID. The observer counts all
remaining sessions on those databases, including idle sessions, and adds prepared
transactions to the transaction count. It rejects missing DB/OID associations,
engine restart, insufficient `pg_read_all_stats` access or guard loss. This is
one sampled observation; selection completeness, held writer fencing and any
applicable unsettled external I/O remain the central caller's responsibility.

Call `drain()` once. It durably reserves each effect before a UID/RV/full-spec
CAS that changes only `spec.replicas`, then reobserves the result. A terminal
drain requires controller generation/status, selected owner-chain Pod retirement,
the unchanged guard inventory and the real probe receipt. Call
`verify_drained()` around each backup step while the shared fence remains held;
the central operation must separately establish any applicable external-I/O
invariant. None of these sampled checks revokes a request already submitted.

Before the first planned backend Deployment configuration change, open the
separate producer Journal and call `bind_config_plan(config_journal)`. The plan
must bind the same operation, namespace, candidate source and cut invocation;
its ordered User/Notification/Realtime services must all be selected writers.
The producer owns the fixed configuration policy and calls
`accept_config_successor(config_journal, service)` immediately after each
completed effect in that order. WriterCut reconstructs the complete original
cut-to-successor link using the fixed producer verifier, exact private
source/target/patch/receipt and shared ledger records. It rechecks the accepted
prefix, live zero-replica specs, pods and sessions around subsequent work.
Resume holds until every service in the bound plan is accepted. The separate
config Journal must remain open and unchanged for the previously accepted
entries through resume; unaccepted pending/changed effects are not proof.
`current_cut_proof()` revalidates the complete plan and returns full current
zero-replica specs with UID, generation and cut/successor journal references.
It is private input to the image phase before its first mutation. Once an image
effect changes a spec, the image phase must retain that bound input and verify
its own per-workload successor plus the unchanged cut/config journal and ledger;
the pre-image live check no longer describes that changed workload.

For the fixed Auth and Realtime migration producers, bind their authenticated
source plan with `bind_migration(kind, phase, source_root)` before the Job
create. The producer records pending before POST and the accepted Job UID after
POST before calling `verify_migration_running()`. The cut checks source-owned
Job and Pod validators, the unchanged unrelated Job inventory, pinned node and
scoped session observation. `finish_migration()` requires the source terminal
receipt, a Complete Job, its original successful Pod and zero migration
sessions. Auth cleanup is accepted only after the producer's exact UID removal
receipts and absence checks. A lost response remains uncertain and is not replayed.
The existing cooperative fence and trusted Kubernetes admission authorize the
POST; later IP/application-name matching correlates sessions and does not prove
cryptographic peer identity or provide atomic revocation. Unknown or ambiguous
network attribution holds the operation.

Before restoring writers after a registered migration, bind the authenticated
inventory via `bind_resume_inventory(inventory_phase, token)`. The token remains
in memory; only the receipt digest enters the protected journal and ledger. The
fixed resumed-session observer must qualify source routes and owned Pod identities:
it is preflighted before any resume CAS, and after the first CAS it can accept
only sessions belonging to completed, verified resume effects. Missing observer
or source evidence holds before mutation. `current_resumed_proof()` returns
private exact completed controller receipts for that observer, never the pending
effect currently being submitted.

After migrations, call `resume(targets)` with the separately admitted current
target objects, not the old objects. The target UID/generation/full spec must match,
targets and predecessor Pods must be retired, and the CAS restores only each
original replica count. A failed or unknown effect poisons the invocation,
marks the fence uncertain when possible, and requires operator reconciliation;
do not replay, auto-rollback or release the lock from absence alone.

When the selected writers include Space, Chat or Realtime, `resume` refuses the
bulk path. Bind the qualified source-owned post-image plan with
`prepare_activation(phase, token)`, inspect `verify_activation_stage()`, and call
`activate_next()` only for a controller stage. At a health stage the named fixed
source phase runs its own actual observation, then
`accept_activation_health(name, phase, token)` binds its completed receipt.
Space HTTPS must be confirmed before Realtime, publisher HTTPS before Chat, and
Chat health before Realtime. `finish_activation()` completes only after the full
ordered prefix and all required barriers. Source and health receipts are checked
again before each CAS; the shared ledger and private records are checked at the
final pre-CAS boundary. Phase tokens stay in memory. A failed or uncertain step
holds the operation and must not be replayed.

Target preflight binds UID, generation and full spec to the caller's admitted
target; status-only resourceVersion changes are allowed. The subsequent patch
uses the fresh observed resourceVersion as an exact CAS test. Reobservation
requires the same UID/full spec and the precise expected generation transition,
even if status advances resourceVersion. `resume` returns the invocation and an
effects map keyed by `writer-resume-<kindlower>-<name>`: each entry has kind,
name, restored replicas, nonce, receipt hash, UID, spec hash, response and fresh
resourceVersions, and generation. The consumer must verify those against the
private journal, complete owned ledger map and its authenticated image target.
Full specifications remain private. Before each next effect and at completion,
the implementation compares the entire writer effect map with its journal.

Linux focused checks: `python3 scripts/dev/writer_cut_test.py`, followed by
`python3 scripts/dev/media_operation_guard_test.py` and
`python3 scripts/dev/media_backup_phase_test.py`. Fixtures use fake Kubernetes
and database observers; actual coherent backup remains separate evidence.
