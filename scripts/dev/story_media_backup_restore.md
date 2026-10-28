# Isolated media backup restore and migration

`qualify_restored_set(root, key, binding, manifest, source_metadata,
migration_root, postgres_image, migrate_image, journal)` verifies the encrypted
members and authenticated final manifest before it creates isolated Linux
Docker resources. It restores password-free roles and the three database dumps
into a newly created networkless PostgreSQL 16 container, compares the selected
source role/catalog/ACL/ledger metadata, and runs the pinned `golang-migrate`
binary on the verified source SQL files. It checks each final migration ledger
version and dirty flag. It verifies removal of its own exact container IDs and
anonymous volume before returning a receipt; failures never restore over a live
database.

The caller must provide the actual final binding and manifest established by
the fenced operation, plus protected `source_metadata` with exact `catalog`,
`roles` and `migrations` objects. Canonical SHA-256 of each object must match
the authenticated `restore_inputs` binding; the caller is responsible for
capturing those metadata from the actual pre-migration cut and validating their
provenance. The original source SHA, artifact/receipt identity, PostgreSQL image,
migration image and migration binary SHA must be pinned to reviewed inputs.
Image references are immutable digests and must already resolve locally to the
verified Linux image IDs. The exact migration file inventory and hashes cover
the baseline version's `down` file and every present `up`/`down` file through
target, so golang-migrate can identify its starting version. Missing numeric
versions are not invented. Passwords and database rows must not be included in
source metadata or receipts.

The resource journal is mandatory: `begin(kind, exact_name, labels)` durably
records intent and returns a 32-hex attempt nonce; `created(kind, nonce, exact_id,
inspected_labels)` records verified ownership; `begin_remove(kind, exact_id)`
records deletion intent; `removed(kind, exact_id, verified_absent)` records
confirmed container and owned-volume absence; `uncertain(kind, nonce)` retains
recovery authority when creation or cleanup is unknown. A lost creation reply
is not permission to delete a resource by its name hint or to retry. The
operator must preserve the lock and reconcile any journal uncertainty.

The receipt binds operation, snapshot, source, final manifest hash, image refs
and IDs, migration tool hash and final versions. Its scope is only isolated
roles/catalog/migrations and verified cleanup. Catalog checks cover public
relations and their owners/ACLs, schema/default grants and migration history;
they are not an exhaustive schema, function, type or data comparison. The roles
check covers selected role attributes and memberships, not passwords, role
settings or grantor identity. Service data invariants, application binaries,
cross-service consistency, actual off-node custody and MinIO/NATS preservation
remain separate requirements. A synthetic fixture or receipt alone cannot
authorize a DEV or STAGING rollout.

`story_media_backup_restore_test.py` exercises the input and ownership failures.
`story_media_backup_restore_integration_test.py` uses owned, networkless Linux
PostgreSQL fixtures, verifies an actual invalid restore failure and cleanup,
and tests `golang-migrate` on populated File 13, Story 4 and Messaging 27
schemas to 17/8/38 with retained fixture rows. It uses synthetic bindings and
source metadata, not live data or a real external artifact. The pinned local
test images and Docker socket runner are documented in the test source and
`story_media_backup.md`.
