# Auth account-deletion database clock

## Purpose and scope

Account deletion must use Auth's database time after obtaining the account row
lock, within the same owning JDBC transaction as the account mutation and durable
deletion operation. Caller time must not shift the original schedule, and an
unrelated active Spring transaction must not qualify an autocommit connection.

This is a bounded `BE-019` correction. It does not implement the complete A4
producer, User historical coverage, new lifecycle transport or activation.

Governing sources are [Auth](../microservices/auth-service.md),
[account recovery](../features/auth-and-contacts.md#удаление-аккаунта--gdpr),
[data stores](../DATA_STORES.md) and [testing](../TESTING.md).

## Existing implementation and change

`TransactionalAccountDeletionOperationStarter` already couples account
deactivation/session epoch, the durable operation and the session-epoch floor in
one transaction. Previously its schedule came from the caller's clock. The
repository now obtains `accounts ... FOR UPDATE` before a separate
`clock_timestamp()` statement on the same transaction-bound Auth connection.
The existing mutation/outbox/floor logic and idempotent retry remain in place.

The clock helper requires an active Spring transaction, a `ConnectionHolder`
bound to its own JDBC datasource and JDBC autocommit disabled before issuing SQL.
An unrelated datasource transaction can set the global active flag and even
synchronize an Auth connection while leaving autocommit enabled; neither state
alone qualifies the clock sample.

No schema, migration, protobuf, public RPC, credential, permission, producer
subject or runtime capability is added. Auth remains Java/Spring. Existing
schedule, operation ID and restore-token hash survive retries unchanged.

## Source provenance

The reviewed implementation comes from immutable source commits
`56610364b422139a11b564927340854908e4d7e0` (database-owned post-lock clock) and
`e60be4ffbb64e40dd72fbe11d5308428c5bdedf2` (datasource-bound transaction guard).
The current-master package carries their four final Java source/test files;
the wider A4/User contract and coverage-integrity work is separate.

The source components received independent Anton-140 SOURCE CLEAR. The guard
receipt is `BE-019` c3. Packaging must bind that review to the new immutable head
and verify the Java blobs remain identical; it is not a new acceptance claim.

## Validation and causal evidence

All Auth runtime tests ran in Linux with JDK 25 and owned PostgreSQL 16 fixtures.

- The original held-row test first failed when caller time seven days ahead was
  persisted outside the post-lock database-time bounds. It passed after the
  database clock change; the retry control preserved the original operation.
- The transaction-binding RED ran three tests: cold and prewarmed Auth
  connections inside an unrelated datasource transaction both incorrectly
  returned a clock, while the real held-row transaction control passed.
- The guard GREEN ran seven tests, zero failures/errors/skips: producer four,
  session-floor rollback one and durable repository two. Session 44093 exited 0.
- Source `git diff --check` and the current-diff contract passed. Package-specific
  applicability and hosted checks must use the actual package head/base.

From `src/backend/auth`, the focused Linux command is:

```sh
mvn -B -Dtest=AccountDeletionProducerJdbcIntegrationTest,AccountDeletionEpochFloorJdbcIntegrationTest,AccountDeletionOperationRepositoryJdbcIntegrationTest -DfailIfNoTests=true test
```

Portable retained reports and the independent-review manifest are under
`tmp/swarm/anton148-a4-producer-evidence/` in the shared engineering workspace.
The original RED evidence remains immutable; no skipped test, PubAck or source
review is presented as runtime/provider acceptance.

## Progress and remaining work

- [x] Document the existing transaction boundary and write causal tests first.
- [x] Implement the database-owned post-lock clock and datasource-bound guard.
- [x] Run the focused Linux regression set and obtain independent source review.
- [ ] Bind the composed package to its immutable current-master head and required
  hosted checks, then follow the normal merge/delivery workflow.

Full cycle/revision/decision history, protected current/terminal producer reads,
complete User ownership provenance, all-writer coverage closure, transport,
backup/rollback enforcement and A4 acceptance remain open. Source metadata pins
or an empty User scan do not establish whole-account history. The held Auth
migration lineage is not folded into this clock-only package.
