# GIS forward33 compatibility

## Purpose and context

Restore the missing GIS15–30 schema for databases already at shipped32 without
rewriting shipped31/32. GI-T40 consumes the restored public session operations.
Product authority remains `microservices/game-integration-service.md`, the
T32/T34/T37/T38 game integration acceptance sections, DATA_MODEL/DATA_STORES,
and the existing GIS owning-source contract. This is source/private PostgreSQL
work; it does not authorize shared rollout or Native qualification.

## Scope and decisions

33 recognizes exactly the shipped1–14,31,32 layout or the reconciled complete
1–32 layout. Unknown, partial, dirty32, altered guards and unsupported layouts
HOLD. The real migrator's dirty33 marker is permitted only inside this atomic
body; successful migration ends clean33. No `force` or fake marker is used.
The migrator must be the privileged catalog owner; runtime ownership refuses.
The public schema must retain PostgreSQL16/18's `pg_database_owner` owner and
exact baseline ACL: owner CREATE/USAGE, PUBLIC USAGE and migration3's runtime
USAGE, without additional grants or grant options. A changed owner or ACL
HOLDs before restoration; the real migrator leaves its attempted 33 dirty
marker while schema, ACL and retained data remain unchanged.
The complete layout keeps scoped operation keys from shipped14. Its restored15
is owned by Marusya-47 (SHA256
`41cd6c25408e15f5fe9693fc0a2d29b2224bc032982f9bccc8267a9dae1fee8e`).
Frozen15–30 manifest SHA256:
`416522de84fb9adc06b2d3c8da6c94a865b8ed9ed4cd2d2db17112d4e5d13cd4`
(v3; includes the qualified22 cancellation supplement).

No host, roster hash, lease, consent, community receipt or Native seal is
invented. Legacy provisioning/active/closing sessions lack real T32 facts and HOLD
before any schema/data mutation. Their operator needs a separately documented
owning-facts recovery decision; setting the marker or manufacturing receipts is
not recovery. Existing closed/failed sessions retain all existing bytes and
receipts; retention timestamps derive only from their owning `updated_at`, as
in restored16. The authority floor is never reset and may advance conservatively
when the restored16 updates session rows after the shipped32 triggers exist.

## Milestones and steps

- [x] Audit frozen DDL; discover15 FK incompatibility with shipped14 and obtain
  the consumer author's reconciled scoped15 before implementation.
- [x] Write/run real-driver legacy32 regression before adding33.
- [x] Pin complete schema identities; add atomic forward-only33 and catalog33.
- [x] Prove fresh/legacy schema parity, real row/receipt/floor preservation,
  dirty/partial/unknown/tampered and active-legacy HOLD with private PostgreSQL.
- [ ] Run affected source catalog checks, AST refresh, independent review and
  immutable pushed handoff to GI-T40's sole PR author.

## Validation and limits

Use the pinned `migrate/migrate:v4.18.1` driver on an owned private PostgreSQL.
PG16 is supported by `integrationtest.NewSourceMigrationFixture`; no PG16 result
claims managed DEV/PG18 parity. Repeat the bounded proof on canonical PG18 when
available. Never share a database/stack, alter candidate activation flags or
waive the Native installer. The runtime reader must observe actual clean33 and
the original pinned clock functions, never accept32 merely because33 exists.

From the complete integrated checkout:

```
rtk proxy python -B -X utf8 src/backend/migrations/game_integration_db/forward33_test.py
rtk proxy python -B -X utf8 src/backend/migrations/game_integration_db/forward33_test.py --postgres-image postgres:18-alpine
```

The runner creates a uniquely named capped private PostgreSQL container without
published ports and removes it afterward. Its optional `--migrations` directory
selects an explicit source fixture, never a live database or admission flag.
It uses the actual pinned migration driver, including dirty-marker behavior.
In `src/backend/gameintegration`, run `rtk proxy go test ./internal/registry
-run TestGISAuthoritySourceForward33 -count=1` and `rtk proxy go vet
./internal/registry` for the source version/floor guard consumer. The proof's
independent `pg_dump` comparison includes all persistent schema objects;
fingerprinting normalizes physical trigger names and sorts tied objects by
their entire value, preserving both self-referential FK UPDATE triggers.

## Progress, risks and follow-up

Marusya-52 owns migration33, its focused tests and catalog update in lease12;
Marusya-47 owns restored15–30/runtime and the coherent GI-T40 PR. Privileged
DDL custody is required throughout migration. Table locks do not fence a
competing privileged schema creator. Independent review and integrated source
catalog tests remain completion gates.

Observed final v3 proof: PostgreSQL16 six methods PASS154.622s plus the added
internal-FK/operator negative PASS14.884s; PostgreSQL18 all seven methods
PASS163.123s. Actual source catalog33/unmigrated32 tests PASS16.123s; registry
vet, gofmt and diff whitespace checks pass. A fixture setup initially combined
CREATE DATABASE/ROLE and failed PostgreSQL's implicit transaction rule; separate
statements corrected setup before the successful source consumer run.
No runtime admission, Native receipt, shared rollout or acceptance is claimed.
