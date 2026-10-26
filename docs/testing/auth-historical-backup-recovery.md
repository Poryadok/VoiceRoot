# Historical Auth matching-backup recovery fixture

GI-T77-AUTH-RECOVERY prepares recovery evidence for the T77 consumer. Auth remains
Java/Spring. This fixture does not authorize any live Auth, DEV or STAGING repair.

The earlier SDK-only Flyway catalog put identity at V15 and omitted the canonical
refresh-profile migration. Current source refuses that history. Recovery restores
the matching database/runtime bundle; it never converts old history into current
history. Do not use Flyway repair/baseline, golang-migrate force, version renumbering,
direct history edits or a fabricated missing migration.

The test creates an isolated PostgreSQL 16 target with the actual historical
Flyway loader and saves SDK identity, device, conversion and owner receipt rows.
It produces a PostgreSQL custom archive from that target. The fixture binds the
archive to exact catalog filenames/content and the captured public-table rows.
Capture requires complete checksum-bearing successful SQL history with exactly
one applied row per release migration, no BASELINE row and no canonical marker.
An actual Flyway-created synthetic baseline is rejected before archive creation.
Current catalog mismatch and archive modification refuse before target creation.
A separate empty `auth_recovery` database receives a single-transaction restore.
Every public table, including every Flyway history field, must match the captured
snapshot before validation and again after the matching loader's no-op migration.
The source database remains unchanged. Failures retain fixture maintenance state.
No archive or receipt content is printed or persisted in the repository.

Run from `src/backend/auth`:

```powershell
rtk proxy mvn -B '-Dtest=AuthMigrationCatalogJdbcIntegrationTest,AuthMigrationCatalogContractTest' test
```

The snapshot comparison preserves PostgreSQL logical values and their JSONB
serialization, including receipt content and loader metadata. It does not assert
physical datafile or dump byte identity after restoration. The archive digest
checks the original dump bytes; it is an in-process fixture pin, not an externally
authenticated backup manifest.

For an actual operator recovery, keep Auth drained and unavailable throughout
inspection, backup selection, empty-target restore and reconciliation. Retain the
failed database and its real history without edits. The selected backup must be
target-generated and verified against its exact runtime/catalog and recovery point;
resolve any post-backup writes before serving. Reject missing or mismatched bundle
provenance. Verify database rows, immutable receipts, full engine history, clean
migration state and application readiness under that exact runtime before routing
traffic. This document provides no live commands or production credentials.

The fixture's `ready` field means matching historical Flyway validation and no-op
readiness only. It does not start an Auth server or prove HTTP/gRPC admission,
current V31 DDL-gate recovery, canonical Go history recovery, external backup
provenance, or rollout compatibility. Those remain open before full T77 sign-off.
