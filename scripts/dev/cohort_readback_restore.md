# Cohort readback and restore consumer

`cohort_readback.run(phase, token, key, bundle, destination, drain)` uses the
existing `CohortPhases` and its `BackupPhases` journal, fence and ledger. It
reconstructs the authenticated binding from the exact locally and durably
recorded member receipts, checks the same-invocation upload and bundle marker,
identifies the completed fixed master workflow and artifact, and performs the
full encrypted readback and extraction. The private destination is the fixed
operation directory's `cohort-readback-<run>-<challenge>`. The returned manifest
is authenticated and its canonical digest must match the protected bundle marker.

`cohort_restore_journal.run(phase, key, destination, manifest, metadata,
source_root, postgres_image, migrate_image, drain)` reserves one restore and
calls the isolated cohort qualifier. Each container create and removal uses a
durable local record and CAS ledger state under the same operation and
invocation. A lost response, changed fence, failed postcondition or uncertain
resource poisons the invocation and retains the lock for reconciliation;
neither operation may be replayed. Cleanup uses the qualifier's exact owned
container IDs and verified removal rather than mutable names. Receipt data does
not contain keys, database rows or process diagnostics.

The caller must provide genuine source metadata, the protected operation
directory, live fence and uninterrupted drain throughout these calls. A
successful result covers the selected encrypted cohort and the qualifier's
documented roles/catalog/ledger checks and cleanup; it does not prove complete
affected-set selection, full row equivalence, MinIO/NATS preservation, Auth
Flyway validation, binary compatibility or release readiness. The integration
fixture uses real PostgreSQL and Docker resources with synthetic Kubernetes,
ledger and GitHub transport; it is not live or off-node evidence.
