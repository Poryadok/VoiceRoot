# ExecPlan: complete GIS owning authority source (GI-T73-GIS)

## Purpose and scope

Supply GI-T73-PUBLISHER with a protected, complete, authorization-only read of
GIS-owned state, backed by a durable transactional source clock. The existing
owner exception in `docs/PLAN.md` (Game Integrations, lines 42–67) permits GAME
implementation, including federation, while release activation remains deferred.
No DEV/STAGING rollout, provider/device acceptance, cross-service SQL or secrets.

Sources: `.agent/AGENTS.md`, `docs/PLAN.md`, `docs/features/game-integrations.md`,
`docs/microservices/game-integration-service.md`,
`docs/architecture/game-federation.md` section 6, `docs/DATA_MODEL.md`,
`docs/DATA_STORES.md`, `docs/ARCHITECTURE_REQUIREMENTS.md`, `docs/TESTING.md`,
`docs/CONTRIBUTING.md`, and the T73 entry in the game integration ExecPlan.

## Ownership and baseline

Anton-21 owns GIS source code, migration32, its tests and canonical GIS state.
Clean isolated WT58 at baseline 69cc6054dd82406e1c3f7c925dbc3466d27ea7c5;
branch `codex/anton-21-gis-authority`. Independent review is coordinated by Scout10.
No other author may write this slice. Existing worktrees/WIP remain preserved.

## Milestones and validation

- [x] Reconcile existing GAME authority exception before claim.
- [x] RED → GREEN: real migrator source clock tracks insert/update/delete,
  rollback preserves clock, floor cannot reset/delete/truncate, downgrade refuses.
- [x] RED → GREEN: canonical state validates exact environment/profile scope,
  explicit missing state, statuses/revisions, no secret/proof/receipt fields;
  unchanged bytes at time-only expiry, earlier validity cutoff for active leases.
- [x] RED → GREEN: owning read-only repeatable-read reader and revision API;
  clean catalog32, trigger/function pins and bounded pool checkout/read deadline.
- [x] Protected dedicated source-only runtime, opt-in configuration and real
  mTLS/replay/maintenance factory tests. No registration on public HTTP routes.
- [ ] Scoped full GIS and shared authoritysource tests/vet, diff check,
  Graphify AST update, independent exact-head review, normal push and PR.

Use private per-test PostgreSQL fixtures from
`integrationtest.NewSourceMigrationFixture` and the actual migration driver.
Run `go test ./internal/registry -run AuthoritySource -count=1` for owner tests,
`go test ./authoritysource -run GIS -count=1` in pkg for canonical tests,
then the relevant full module checks. Every shell command uses `rtk proxy`.

## Decisions and limits

GIS owns app/environment admission policy, bindings/authority revisions,
installation status, its resource mappings and raw binding-chat lease state,
and its session status/membership facts. Space owns community managed rights;
Auth owns SDK device/key/generation/current sessions; User owns profile facts;
Role owns permissions. GIS cannot establish those facts from local references.
No undocumented T32 lease or cross-store Space association will be invented.
Publisher must reconcile its exact Space/subject set using the other owners.

Whole-source conservative invalidation is intentional: status mutations and
insertion/deletion cannot escape a per-row maximum revision. The counter and all
facts must be read in one cut. A source read does not activate federation.

## Progress and evidence

- Actual migrator RED: `gis_authority_revision` absent after clean31 bootstrap.
  GREEN: transactional clock, rollback, floor and downgrade test passes.
- Canonical codec RED: missing state/decoder; GREEN: status/subject/missing,
  unknown-field and lease-boundary cases pass.
- Owning reader RED: missing SchemaReader methods; GREEN: fresh real migrations,
  missing/closed state, binding status-only final revoke, time-only expiry bytes,
  installation/resource/session facts, all nine clock triggers and body tampering.
- Regression RED: inconsistent app/environment tuple silently omitted; GREEN:
  both inconsistent tuples and unqualified nonempty character authority refuse.
- Actual factory RED: missing factory; GREEN: source-only mTLS service with
  signed exact requests, replay/missing principal denial and live maintenance.
- Read-only write refusal and one-second pool/schema-lock budget test passes.
- `go test ./authoritysource -count=1` passes; GIS and authoritysource vet pass.
  Full GIS module run is still in progress. Independent exact-head review,
  Graphify completion, normal push and PR remain before delivery.
