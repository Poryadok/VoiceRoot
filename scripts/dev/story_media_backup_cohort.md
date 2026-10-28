# Selected PostgreSQL cohort backup and restore

This component processes an exact sorted subset of the documented current
service-owned PostgreSQL databases plus password-free roles. The central
transaction must first derive and verify that subset from the protected runtime
snapshot, owner/database mapping and source-bound mutation plan, including
required dependent stores. The subset supplied to this helper is data, not
proof that the selection is complete. Unsupported, unexpected or ambiguous
databases must hold the operation; they must not be silently dropped. Legacy
`gateway_db`, deferred federation and multiple PostgreSQL engines are not
qualified by this component.

The authenticated binding includes an exact sources map for all database
members and roles. Every database carries the same nonempty engine observation
and server-start value plus its own unique positive database OID; roles carry
the same engine and server-start value. A
multiple-engine operation needs separately qualified role backups and is
rejected. This structural comparison does not independently establish that
the observed engine/session mapping is true. Only the transaction's verified
resolver, live guard and uninterrupted cross-service drain can do that.

`story_media_backup_cohort.export(root,key,binding,databases,producer)` streams
each selected database and `roles` into separate private encrypted files;
`verify` checks the authenticated exact set, and `decrypt` verifies a private
ciphertext snapshot before emitting any plaintext. The existing operation
fence, durable per-member pending/uncertain journal, separate key custody and
actual off-node artifact transport remain mandatory. Operation IDs support only
the separately authorized `media-c1-<32hex>` and `media-c2-<32hex>` forms.

`story_media_backup_cohort_finalize.finalize` checks the exact per-member
receipt identities, source engine/server start/database OIDs and unique attempt
nonces and binds their canonical hashes into the encrypted manifest. The caller
must first verify each real receipt against its protected journal and ledger;
this helper does not establish that the receipts represent real executions.
`story_media_backup_cohort_custody` packages the exact authenticated set in
`cohort-backup.tar`, verifies the pre-upload bytes, and verifies a complete
GitHub Actions artifact readback before private extraction. The fixed artifact
name is `voice-cohort-backup-<operation_id>-<challenge>`; its ZIP contains only
that tar and no key. The central operator owns actual upload, trusted
run/head/challenge binding, protected key custody and recovery on uncertainty.
The separate media-only artifact format is not interchangeable with this one.

`story_media_backup_cohort_restore.qualify` consumes a verified encrypted set,
protected source metadata, source-root SQL inventory, pinned images and the
mandatory resource journal. It actually restores roles/databases into a new
networkless PostgreSQL 16 container, compares selected roles/catalog/ledger,
and handles two cases: unchanged schemas are restored and checked again without
starting a migration tool; changed Go schemas run the pinned golang-migrate
binary and check the final clean ledger. Changed Auth/Flyway migrations are not
yet qualified and fail closed before creating restore resources. Unchanged Auth
compares its existing Flyway history; it does not run Flyway validation or
prove Auth binary compatibility. The exact source migration file inventory is
hash-checked even for the unchanged path.

The result is only selected-database isolated restore/catalog/ledger evidence.
It does not establish a complete affected set, an actual off-node copy, service
data invariants, application binary readiness, cross-service consistency or
MinIO/NATS preservation. Role settings/grantors and exhaustive schema/function
equivalence are outside the selected catalog comparison. Resource creation and
cleanup ambiguity must remain in the caller's durable journal for recovery;
no cleanup by a mutable name or automatic retry is authorized.

Linux fixture tests exercise synthetic selections, roles, a synthetic Auth
Flyway history and populated Voice P schema with both no-op and changed Go
migrations. They do not establish actual runtime mapping, live custody or
production admission. Source, test and image identities are recorded in the PR.
