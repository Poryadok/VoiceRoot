# Auth live migration phase

This phase is a central-operator component, not a standalone deployment command.
It does not acquire the DEV claim or perform backup, rollout, or writer selection.
The caller supplies an already held common claim/lock, a completed authenticated
cohort readback and restore, a drained source-owned `WriterCut`, a verified
`PackagePhase`, and the immutable candidate intent. The protected Auth source
tree must match the reviewed `d11f510e50a4a86bfa2cf8707a8b087efe7d77f8` tree;
a different whole candidate commit is allowed only when that subtree is equal.
Changed Auth source, unavailable package evidence, unsupported effective Spring
configuration, or unknown database identity holds the operation.

For C1, bind the package with `bind_package`, then call `init_c1`. It verifies
the source-owned candidate image/configuration and database route, actual clean
Flyway 26, the authenticated cohort restore and qualified clone, fixed original
row markers, selected roles/catalog, the actual Job credential login, and the
pinned Flyway platform/node. The private journal and common ledger reserve both
objects as absent. Bind the resulting phase to `WriterCut.bind_migration`, call
`create_c1_objects`, then `finish_c1` and `cleanup_c1` under that same cut.
Creation and deletion use durable pending transitions and exact owned UIDs; an
unknown response requires reconciliation and cannot be replayed. Terminal
evidence covers the full prior Flyway history, 28/31/32 checksums, the V31 DDL
gate, the clone-qualified original-table row markers with the V28 normalization,
and selected role and catalog metadata. It is not a general rollback proof.

For C2, `no_op_c2` requires a fresh authenticated backup/readback/restore,
the current candidate package and configuration, the current drained cut and
actual clean history 32 matching the complete reviewed SQL catalog. It writes a
private receipt and ledger hash without creating a Job or ConfigMap. It does not
require the C2 whole source SHA, database identity, or installation timestamps
to equal C1. A changed Auth subtree or migration catalog needs a new reviewed
qualification and is not silently treated as a no-op.

`verify_completed(phase, cut, source_root)` rechecks protected package,
cohort and private/ledger receipts and returns the candidate route and target
digest for the resumed-session consumer. It does not require zero sessions or
prove that the resumed Pods use that target; the consumer must independently
verify current workload and session identities.

The cluster/controller/admission and database operator are trusted infrastructure
boundaries. Writer fencing is cooperative and is not atomic revocation. The
canonical Job POST is an authorized effect and can start SQL before its UID is
recorded. Later Job/Pod/session checks are bounded postchecks, not a preventive
barrier. Application name and Pod IP correlate sessions; they do not authenticate
them cryptographically. Unknown or proxied network identity, an unmapped writer,
an unexpected session, or an ambiguous effect must hold the operation and retain
the fence for reconciliation. The source and synthetic fixtures do not establish
that an actual DEV operation, candidate package, or rollout has completed.
