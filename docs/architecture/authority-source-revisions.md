# Owner revisions for complete authority snapshots

Space migration `000024_authority_revision` and Role migration
`000015_authority_revision` extend the existing v11 per-Space Voice counters.
They retain all previous counter/outbox records and use the same invalidation
event schema. Each mutation emits a coarse Space wildcard. Changes moving a row
between Spaces invalidate both scopes in UUID order. These events prompt fresh
reads; they do not contain a complete policy snapshot.

Space coverage includes Space fields, membership, bans, timeouts, voice rooms,
tree/category mappings, subscriptions, ownership journal, lifecycle aggregate,
deletion tombstones, community owner authority/recovery and community roster.
Role coverage includes role definitions, member assignments, chat and voice
overrides, ownership transfer, retirement and deletion fence heads. Cascading
role deletion invalidates the parent Space even when override triggers can no
longer resolve the deleted role. Auth/User/GIS and resource placement revisions
remain separate owning-source requirements.

Counters start at one and advance exactly once per bump. Changing their Space,
rewinding/removing them or truncating them is rejected. Owner source tables and
the immutable outbox also reject TRUNCATE, which would bypass row triggers.
The migration locks every covered relation, refuses RLS/policies, rewrite rules,
inheritance and unlogged tables, and refuses outbox evidence ahead of a missing
or lower counter. Activation advances every known scope once, including saved
floors for deleted Spaces and legacy fence-only scopes. Previous outbox payloads
and protected receipts remain unchanged.

Both migrations are forward-only. DOWN fails and leaves the migrator in dirty
maintenance state; it preserves the schema and floors. Recovery requires the
verified matching database/runtime backup bundle. Do not force the dirty marker
to run an older runtime without diagnosing the failed transition.

The owning-source reader must capture complete state and the revision together
in a repeatable-read transaction. The publisher then rereads every owner's
monotonic token and accepts only an unchanged vector. Validity starts before the
first read and is bounded by two seconds; changed, missing, expired or regressed
tokens prevent renewal. Time-dependent restrictions also bound validity. This
does not authorize publication until all owner snapshots, protected transport,
scope validation and canonical resource/application/session bindings are covered.
See [the Federation contract](game-federation.md#6-authorization-snapshot-и-live-изменения)
and [the sprint acceptance ledger](../testing/game-integrations-exec-plan.md).

Real PostgreSQL regressions cover authority mutations, both update scopes,
floor retention and activation. The pinned golang-migrate test exercises both
complete owner catalogs, clean upgrades and refused downgrade. Combined producer
and media enforcement acceptance remains a separate gate.
