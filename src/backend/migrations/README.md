# SQL migrations (DATA_SCOPE v1)

### Auth identity additive source profile (not applied/qualified)

The original172 current444 author unit retains V1..26 / canonical1..27 and adds
V28/000029 plus V31/000032. Reserved Mod V27,29,30 / canonical28,30,31 are not
part of this unit; never fabricate their history or renumber published source.
Both V28 UP bodies are identical. Flyway executes V31 in its actual migration
transaction; canonical000032 explicitly wraps the same body in BEGIN/COMMIT.
The source contract tests check the inventory and envelope, not actual loaders.
Real two-loader equivalence and privileges remain a required native gate.

Install the barrier/event trigger through the already authorized trusted
maintenance channel before activating barrier-aware replicas. The runtime role
stays non-superuser and cannot install event triggers. Future protected DDL must
take the exclusive barrier FIRST in the SAME actual loader transaction and hold
it through commit; a preflight connection is insufficient. Canonical000032 DOWN
unconditionally refuses removal with55000 after exclusive entry. Restore uses a
verified matching runtime/database, never forced marker repair.

Event-trigger control is a PostgreSQL automatic-gate exception. The fixed-source
`scripts/ops/auth-identity-maintenance.py` acquires exclusive access, checks the
known preimage, recreates the exact trigger and verifies the enabled postimage in
one transaction. Its exclusive lock drains/fences participating owner calls;
old nonparticipating replicas must already be drained. Shared-role/database or
unannounced privileged bypass is not covered. This stage executes no installer,
maintenance, SQL or DDL and qualifies no environment.

Per-database folders for the first migration wave ([docs/DATA_SCOPE_V1.md](../../../docs/DATA_SCOPE_V1.md)). Filenames follow [golang-migrate](https://github.com/golang-migrate/migrate) (`NNNNNN_name.up.sql` / `.down.sql`); use domain-based names, not phase numbers.

| Directory | Database | Owner (typical) |
|-----------|----------|-----------------|
| `auth_db/` | `auth_db` | Auth Service — DDL also in Flyway ([`src/backend/auth/src/main/resources/db/migration/`](../auth/src/main/resources/db/migration/)); **one tool per DB** (see below) |
| `user_db/` | `user_db` | User Service |
| `social_db/` | `social_db` | Social Service |
| `chat_db/` | `chat_db` | Chat Service |
| `messaging_db/` | `messaging_db` | Messaging Service |
| `file_db/` | `file_db` | File Service |
| `bot_db/` | `bot_db` | Bot Service |
| `game_integration_db/` | `game_integration_db` | Game Integration Service |
| `voice_db/` | `voice_db` | Voice Service — `000001_room_lifecycle`, затем `000002_redis_divergence` |

Apply against the matching database only; do not run one folder against another DB ([docs/OPERATIONS.md](../../../docs/OPERATIONS.md)).

## `auth_db` (Auth): Flyway vs golang-migrate

Guest-conversion rollback `000009` takes the owning table's exclusive lock
before checking for pending work, and retains it through the drop in one DO
statement. It first refuses isolation levels other than READ COMMITTED: an
inherited REPEATABLE READ snapshot could hide a pending row even after the lock.
Concurrent admitted `PENDING_USER`/`PENDING_EVENT` writers must commit
before that check; their work refuses rollback. Empty/completed-only tables may
still roll back. Run `python scripts/dev/auth9_rollback_test.py` from the repo
root for the real PostgreSQL 16 two-connection regression and DOWN/UP controls.
The regression includes an inherited empty REPEATABLE READ snapshot followed by
a separately committed pending row. It requires Docker and creates only its
own disposable container, with no host ports or shared database access. Set
`AUTH9_ROLLBACK_ARTIFACT_DIR` to export source-before-run SHA256 and source bytes
plus a JSONL trace of the executed SQL and raw SQL stdout/stderr.

`auth_db` is owned by the Auth service. The same schema can be applied in **two** ways; use **exactly one** per database (mutual exclusion — do not run golang-migrate on `auth_db` and then start Auth with default Flyway on the same DB, or Flyway will try `V1` against existing tables and fail).

| Path | Who applies | Current ordered layout | Auth startup |
|------|-------------|------------------------|--------------|
| **A — Flyway (default)** | Auth on boot | `V1__auth_schema.sql` … `V25__sdk_conversion_owner_receipts.sql` | `AUTH_FLYWAY_ENABLED` omitted or `true` |
| **B — golang-migrate** | Ops / CLI / Docker `migrate` | `000001_init` … `000026_sdk_conversion_owner_receipts`, each with `.up.sql` and `.down.sql` | `AUTH_FLYWAY_ENABLED=false` |

**Equivalence (current schema):** Flyway `V1` contains the initial schema and
the `refresh_tokens.access_jti` change represented by golang-migrate
`000001_init` followed by `000002_refresh_tokens_access_jti`. Every later
Flyway revision maps in order to the next golang-migrate revision (`V2` →
`000003`, …, `V25` → `000026`). Keep these layouts in lockstep for future
Auth-owned DDL.

Refresh-token profile binding retains canonical V15 / 000016. The SDK chain now
starts at V16 / 000017, with identical UP bodies in both catalogs through the
conversion-owner receipts. SDK DOWN refuses downgrade and preserves authority
evidence; recovery requires a verified matching database/runtime backup.

The earlier SDK-only V15 / 000016 history collides with master's refresh addition.
Existing histories require inspection and a matching-release recovery plan;
renumbering source files must not be applied as a history rewrite. Flyway checksum
validation and golang-migrate's dirty maintenance refusal remain enabled. Never
repair/baseline/force a marker to bypass that mismatch. See [Auth's database
instructions](../auth/README.md#database). Complete catalog tests use the actual
Flyway loader and pinned `migrate/migrate:v4.18.1` against private PostgreSQL 16,
including historical refusals and preserved SDK identity/operation/receipt rows.

**Examples below** run migrate only for **Go-owned** databases (`user_db`, `social_db`, `chat_db`, `messaging_db`). For `auth_db`, use Path A (start Auth) or Path B (migrate then Auth with Flyway disabled) — see [Auth README](../auth/README.md).

Example with [migrate](https://github.com/golang-migrate/migrate) CLI for a Go-owned DB (adjust password/host):

```text
migrate -path src/backend/migrations/user_db -database "postgres://voice:voice@localhost:5432/user_db?sslmode=disable" up
```

Repeat for `social_db`, `chat_db`, `messaging_db`, `file_db`, `bot_db`, and other Go-owned folders under `src/backend/migrations/`, except Space's native installer below (see table in [DATA_STORES.md](../../../docs/DATA_STORES.md)).

**Compose (all Go-owned DBs):** from repo root after `docker compose up -d`:

```bash
make compose-migrate-all
```

Runs `scripts/dev/compose-migrate-all.sh`: `chat_db` + `messaging_db` (E2E encryption DDL), `bot_db`, `story_db`, then `user_db`, `social_db`, `file_db`, `space_db`, `role_db`, `notification_db`, `matchmaking_db`, `search_db`, `moderation_db`, `gateway_db`, `subscription_db`, `voice_db`. **`auth_db`** — Path A (default): Flyway on Auth boot; golang-migrate only with `VOICE_MIGRATE_AUTH_DB=1` (Path B, then `AUTH_FLYWAY_ENABLED=false`). Granular: `make compose-migrate-e2e`, `compose-migrate-bot`, `compose-migrate-story`, `compose-migrate-voice`.

The `compose-db-init` service runs on every Compose app startup: it creates any
missing Go-owned databases and applies their migrations before dependent apps
start, including existing Postgres volumes. The lifecycle migration DOWN refuses destructive teardown while evidence rows exist; archive or remove evidence through an
explicit operator workflow before attempting destructive teardown.

### Space Native44/45/50/51 provisioning

`space_db` exclusively uses Space's production `cmd/native-migrate`, built from
the repository root with `src/backend/space/Dockerfile.native-migrate`. The
bootstrap image compiles the same command and includes its separate pinned
migrate4.18.1 executable. It never uses the generic legacy patch, force or `up`
branches for Space. SQL and existing transition implementations/pins are unchanged.

The command accepts a TCP PostgreSQL URI (`SPACE_NATIVE_ADMIN_URL`) or the
existing PG environment. It normalizes one connection contract before mutation:
admin/verifier/migrate use the same resolved authority and TLS inputs with fixed
`postgres`/`space_db` paths. Keyword DSNs, service indirection, query identity
aliases and duplicate keys reject. URI options are limited to the four common
TLS keys, `connect_timeout` and `application_name`; default TLS is `require`,
verified modes require an explicit CA file, and `allow`/`prefer` are unsupported
by the pinned child driver. Retained evidence over1MiB rejects in its entirety.

For an absent target, the administrative deployment run owns actual
`CREATE DATABASE space_db`, validates its complete compiled SQL byte inventory,
executes real migrate `goto44`, verifies the complete fixed44 source and actual
M44-created epoch/seal, executes exact45 and the existing Go Transition45,
then real50 and existing Transition50, and verifies full clean50 as the genuine
parent of the distinct51 stage below.
Current inventory has no46–49; the command never fabricates markers or blindly
applies a future migration. A changed inventory requires explicit qualification.

The distinct `native_catalog_rowjson_51` successor is qualification-gated WIP:
finite PG16/18 type evidence alone is not universal/custom/future admission or
current-candidate raw digest stability. Without the compiled independently
qualified source/session binding, the installer returns HOLD before creation
and cannot emit complete. M44/M45/M50 SQL, source pins and Transition50's original
byte-equality probe remain unchanged prerequisites.

After genuine old50, the real pinned migrate4.18.1 Go engine and canonical file
source drive51 through a transaction-bound driver. Exact51 SQL, full source/type
qualification, genuine sameOID original50-parent epoch CAS and migration history
commit in ONE PostgreSQL transaction. Framework Unlock never commits; the outer
executor commits only after engine success and final fresh source/target/drain
checks. Generic child `goto51`, manual marker/force and standalone postinstall
reseal are unavailable. Any precommit failure rolls back body, epoch and marker
together. Final51 verification and actual linked45/50/51 receipts are required
before complete/readiness, ordinary enrollment or new serving grants.

Unknown existing, partial or ambiguously committed installations remain HOLD.
Provision cannot auto-resume an existing database; read-only verify requires the
owning controller's original completed SAME image/source/executable/migrator/
inventory/target evidence including51, followed by fresh full51 qualification.
An old50 completed Job or DB-stored seal cannot manufacture that history.
Full107/307/retry effects,325 fresh catalog observations and original5s/2s budgets
remain separate mandatory gates; no successor performance PASS is asserted.

The controller keeps applications and independent privileged DDL actors stopped
throughout this window. The command checks its session, cooperating-installer
advisory lock, created target identity and foreign sessions; these checks are
not a fence against a hostile privileged DDL actor. Read-only source inspection
does not prove installation history or authorize adoption.

JSON-lines output retains creation/system/database/OID/role identity, ordered
real seals/transition receipts, exact executable/migrator/SQL/source identities
and a final `complete` event. Source identity covers actual compiled Go source
bytes, excluding uncompiled test files; the release controller additionally
records its actual image digest and SCM revision. Never replace these records
with an expected live-catalog hash or a database marker.

`make compose-migrate-all` retains successful fresh output as a controller-owned
0600 artifact `tmp/native-space-installer/prior.jsonl` and its observed Docker
image ID beside it. `compose-db-init` consumes that same artifact read-only,
which connects the two actual CI entrypoints. `SOURCE_REVISION`, when supplied,
must be identical for both builds. A direct app bootstrap without the earlier
command emits its real output to the retained Docker logs; without subsequently
retained authenticated controller evidence, a later existing-target run HOLDs.
`SPACE_NATIVE_PRIOR_EVIDENCE` selects actual retained output, not a trust flag.

Staging/production and DEV require a release-owner-selected
`SPACE_NATIVE_INSTALLER_IMAGE` platform image digest. The controllers retain
the authenticated successful native Job UID/spec, matching actual Pod image ID
and complete output in immutable `voice-space-native-installer-evidence`, then
always run a fresh read-only50 verifier. Source/image/inventory changes reject
cached reuse. Old generic SQL-only success is not eligible evidence.

Unknown precreated empty/44/45 databases, dirty states, lost maintenance windows
and incomplete output HOLD without repair. Known50 repeat requires original
controller custody and exact source/executable/migrator/inventory/target/seal
binding; it emits `verified`, preserves original receipts and never calls a
transition. Automatic crash resume/adoption is not supported. See
[deployment procedure](../../../docs/DEPLOYMENT.md#space-native-installer) and
[ExecPlan](../../../docs/execplans/native50-production-installer.md).

Voice migration `000002_redis_divergence.down.sql` refuses to run when any
incident exists, including resolved history. Evidence must be exported and
handled by an explicitly reviewed retention/resolution procedure before any
later destructive operation; R22.3 adds no generic cleanup path.

**E2E encryption (compose Path A):** `e2e_key_backups` via Auth Flyway `V4__e2e_key_backups.sql` on boot. `chat_db` / `messaging_db` DDL (`e2e_enabled`, `is_e2e`, `e2e_prekey_bundles`) via idempotent `docker/postgres/incremental_*.sql.snippet`, or `make compose-migrate-e2e` for golang-migrate on Go-owned DBs.

Path B for `auth_db` only (then set `AUTH_FLYWAY_ENABLED=false` for Auth):

```text
migrate -path src/backend/migrations/auth_db -database "postgres://voice:voice@localhost:5432/auth_db?sslmode=disable" up
```

## Space DB tree audit registry

The Space migration head after `000024_authority_revision` is
`000025_tree_audit_actions`. Its UP migration replaces only
`audit_log_closed_registry`, retaining every action already accepted by
`000015_audit_ledger` and adding the documented tree shapes:
`tree_node_upserted` targets `tree_node` and carries `kind`, exactly one
`chat_id` or `voice_room_id`, optional `category_id`, `sort_order`,
`is_pinned`, and optional `pin_order`; `tree_node_removed` targets `tree_node`
with `{}`; `tree_reordered` targets `space` with `count`. The DOWN migration
locks `audit_log` and raises before changing the constraint while any of those
tree actions remain, preserving their audit and outbox evidence. This records
source migration behavior only; it does not authorize applying it to a live DB.
PostgreSQL validates the revised CHECK while holding `ACCESS EXCLUSIVE` on
`audit_log`; schedule that source migration in a controlled window because the
lock duration depends on the existing audit table size.

## Role protected authority reads

Role `000016_sdk_authority_revision` adds a global, permanent SDK session/grant
revision alongside the existing per-Space Role floor. Both sources invalidate
protected Role reads; SDK/session TRUNCATE and counter rewind/removal are refused.
Its DOWN refuses downgrade with SQLSTATE55000 and preserves all source data and
floors. Protected source registration requires a clean exact catalog16; a dirty
or unknown later marker keeps complete reads unavailable until an explicitly
compatible runtime is installed. Recover with a verified matching database/runtime
backup rather than forcing metadata or removing guards. Actual pinned-driver and
private-listener tests cover upgrade, refusal, and post-registration maintenance.

## Without local CLI (Docker)

If the `migrate` binary is not installed, use the official image [`migrate/migrate`](https://hub.docker.com/r/migrate/migrate) on the same Docker network as Compose Postgres (project network is usually `voice_default` when started from the repo root; service hostname is `postgres`).

From repo root, PowerShell (paths use `/` for the volume so Docker accepts them on Windows). **Go-owned DBs** (safe alongside Auth using default Flyway on `auth_db`):

```powershell
cd d:\Git\Voice
$dbs = @("user_db", "social_db", "chat_db", "messaging_db", "file_db")
foreach ($d in $dbs) {
  docker run --rm --network voice_default `
    -v "d:/Git/Voice/src/backend/migrations/${d}:/migrations" migrate/migrate `
    -path /migrations `
    -database "postgres://voice:voice@postgres:5432/${d}?sslmode=disable" up
}
```

**Path B — `auth_db` only** (omit if Auth applies schema via Flyway; requires `AUTH_FLYWAY_ENABLED=false` when starting Auth):

```powershell
docker run --rm --network voice_default `
  -v "d:/Git/Voice/src/backend/migrations/auth_db:/migrations" migrate/migrate `
  -path /migrations `
  -database "postgres://voice:voice@postgres:5432/auth_db?sslmode=disable" up
```

- Adjust `d:/Git/Voice` to your clone path. Credentials must match [`.env`](../../../.env.example) / [docker-compose.yml](../../../docker-compose.yml) (`POSTGRES_USER` / `POSTGRES_PASSWORD`).
- One-off `down 1` (same pattern): replace the trailing `up` with `down 1`.
- If your Compose project name differs, check the network with `docker network ls` and pass `--network <name>` accordingly.
