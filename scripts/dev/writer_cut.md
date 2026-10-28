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

Call `drain()` once. It durably reserves each effect before a UID/RV/full-spec
CAS that changes only `spec.replicas`, then reobserves the result. A terminal
drain requires controller generation/status, selected owner-chain Pod retirement,
the unchanged guard inventory and the real probe receipt. Call
`verify_drained()` around each backup step while the shared fence remains held;
the central operation must separately establish any applicable external-I/O
invariant. None of these sampled checks revokes a request already submitted.

After migrations, call `resume(targets)` with the separately admitted current
target objects, not the old objects. The full target UID/RV/spec must match,
targets and predecessor Pods must be retired, and the CAS restores only each
original replica count. A failed or unknown effect poisons the invocation,
marks the fence uncertain when possible, and requires operator reconciliation;
do not replay, auto-rollback or release the lock from absence alone.

Linux focused checks: `python3 scripts/dev/writer_cut_test.py`, followed by
`python3 scripts/dev/media_operation_guard_test.py` and
`python3 scripts/dev/media_backup_phase_test.py`. Fixtures use fake Kubernetes
and database observers; actual coherent backup remains separate evidence.
