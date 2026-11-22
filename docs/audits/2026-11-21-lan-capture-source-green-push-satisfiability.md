# LAN baseline capture satisfiability — no green full-graph master push in the 24-name catalog era

- **Agent:** OpenChamber session (model qwen3.8-flash-next-coder-iq1_m, working dir
  `D:\Git\Voice`, fresh worktree branch `docs/lan-capture-satisfiability-20261121`), audit nick
  **LAN-Capture-Source-Green-Push-Satisfiability-20261121**, TT actor `Codex-LAN-Capture-Debt`.
- **Purpose:** decide whether the LAN reviewed-baseline route can even start tonight. The route requires a
  current-master source capture bound to a push-event CI run that satisfies the `_ci` contract in
  `scripts/staging/nats-rollout-preservation/source_authority.py`. This audit states, from code and dispatcher-verified
  GitHub Actions evidence, that no such run exists in the 24-image catalog era, so `capture_source` and
  `capture_equivalent_master_source` fail closed before any node ceremony begins.

## Scope and method

- **Contract:** read the current master source at fresh-worktree head `origin/master` and cited line numbers directly:
  `scripts/staging/nats-rollout-preservation/source_authority.py` and `deploy/staging/services.yaml`.
- **CI evidence:** dispatcher-verified `gh api` scan of CI workflow id `263230665`
  (` .github/workflows/ci.yml` ) for master push runs since 2026-10-17, newest first, plus the documented
  limitation that one scan call returned HTTP 502. This pass did not re-scan every job row; it used the recorded
  run/job summary table as GH evidence and re-verified the repo-side contract and 24-name authority surface locally.
- **No mutation in this audit:** docs-only. No CI dispatch, no LAN node/root sudo, no product-code, CI workflow,
  staging inventory, catalog, image, or ceremony primitive changed.

## Ground truth (contract, verified by reading)

### `_ci` contract

`_ci` is at `source_authority.py:145-198`. For the chosen run it requires:

1. The run must be a **push-event** CI run of `.github/workflows/ci.yml` against `master` whose `head_sha`
   equals the requested `source_sha` (`:147-152`; specifically `run['event'] != 'push'` at `:149` and
   `run['head_sha'] != sha` at `:147`).
2. `ci-gate` and `staging-stack-lock` must each appear **exactly once** and be completed-success jobs for the
   same run, head sha, and run attempt (`:157-162`).
3. For every observed job whose status is completed, the conclusion must be `success` or `skipped`, except at
   most one `deploy-staging / deploy` failure when the parent run is `in_progress` or `failure`
   (`:166-197`; the one allowed exception is at `:176-177`, and the completed-failure revalidation is at
   `:178-197`). Therefore any other red, cancellation, or skipped required lock breaks the contract.

`capture_source` at `source_authority.py:551-554` pins current `master` equal to `source_sha` by passing
`source_sha` as `master_sha`, so the repository pin must be exactly the captured source.

`capture_equivalent_master_source` at `source_authority.py:557-577` may bind an older successful CI only after
`_equivalent_master_source` at `source_authority.py:452-472` proves:

- the source commit is an ancestor of current master (`:454-461` compare/ancestor checks), and
- `src/backend` is byte-equal as a tree blob, and `.github/workflows/ci.yml`,
  `scripts/ci/backend-docker-context.txt`, and
  `scripts/ci/staging-image-catalog.json` are byte-equal blob-to-blob
  (`:462-469`).

`capture_source` and `capture_equivalent_master_source` both fail closed if no CI run satisfies `_ci`; neither
can be rescued by `deploy-staging / deploy` failure handling because all catalog-era runs have other reds.

### 24-name authority surface

- `source_authority.py:31-33` `IMAGE_NAMES` has 24 entries:
  `analytics`, `bot`, `chat`, `file`, `gateway`, `matchmaking`, `messaging`,
  `moderation`, `notification`, `realtime`, `role`, `search`, `social`, `space`, `story`,
  `subscription`, `user`, `voice`, `auth`, `web`, `developer-portal`, `admin`,
  `gameintegration`, `nats-hub-config-renderer`.
- The new 24-name era marker is `gameintegration`, present in the staging deployment manifest at
  `deploy/staging/services.yaml:883` (`name: voice-gameintegration`) and again as a Service object at
  `deploy/staging/services.yaml:932` (`name: voice-gameintegration`).

## GitHub Actions evidence — dispatcher-verified summary

The CI workflow is `263230665`. Full-graph master push runs mean the CI graph has at least 100 jobs; auto-subset
runs in this scan were below 100. The table is **newest first**. It records run id, head sha prefix, created time,
total observed jobs, `staging-stack-lock` conclusion, and count of jobs whose conclusion is neither `success`
nor `skipped`.

| Run id | head sha (first 8) | created | jobs | `staging-stack-lock` | non-`success`/`skipped` |
|---:|---:|---|---:|---|---:|
| 44934578838 | eb341dd8 | 2026-11-21T18:16Z | 151 | success | 10 |
| 44589215159 | e5f0dba7 | 2026-11-19 | 133 | skipped | 11 |
| 44329501457 | 3c2a24e3 | 2026-11-18 | 133 | skipped | 11 |
| 44317546200 | 994983cd | 2026-11-18 | 133 | skipped | 10 |
| 44305446757 | 1b20b398 | 2026-11-18 | 133 | skipped | 9 |
| 44153952853 | d4af0992 | 2026-11-17 | 132 | success | 11 |
| 43787631870 | 9d810e91 | 2026-11-16 | 132 | success | 10 |
| 43768627883 | 269f642b | 2026-11-15 | 132 | skipped | 10 |
| 43695825108 | e3337cb9 | 2026-11-15 | 132 | skipped | 10 |
| 43641582322 | f974b501 | 2026-11-14 | 135 | success | 10 |
| 43626614882 | cfed3c85 | 2026-11-14 | 132 | skipped | 14 |
| 43463924560 | 50cb633b | 2026-11-13 | 132 | skipped | 13 |
| 43205490394 | 547509ef | 2026-11-12 | 132 | success | 9 |
| 43177246533 | 35a2b317 | 2026-11-12 | 150 | success | 8 |
| 43162122204 | 5a91c596 | 2026-11-12 | 132 | success | 7 |
| 43146084189 | 4126f43f | 2026-11-12 | 132 | success | 8 |
| 43138769006 | dd627953 | 2026-11-12 | 132 | cancelled | 12 |
| 43133061485 | 9750619f | 2026-11-12 | 132 | cancelled | 10 |
| 43131921081 | 02cea8e9 | 2026-11-12 | 32 | cancelled | 27 |
| 43127978172 | 87c0515f | 2026-11-12 | 132 | cancelled | 17 |
| 43126807465 | 5e7ad8a5 | 2026-11-12 | 132 | cancelled | 89 |
| 43099468032 | 197106ea | 2026-11-11 | 132 | success | 10 |
| 43052524086 | bdc9ec87 | 2026-11-11 | 132 | success | 16 |
| 43022260323 | cd330482 | 2026-11-11 | 132 | success | 8 |
| 42999756889 | abe618c3 | 2026-11-11 | 132 | success | 8 |
| 42515255047 | 5a6f6112 | 2026-11-08 | 132 | skipped | 3 |
| 42395318933 | d77ce7d3 | 2026-11-07 | 132 | skipped | 7 |
| 42390258054 | 1e9378a8 | 2026-11-07 | 138 | cancelled | 47 |

Scan scope:

- 2026-10-17..2026-11-06: 14 master push runs, **zero full-graph** runs; all auto-subset rows were below 100 jobs.
- 2026-11-07..2026-11-21: the 28 rows above are the full-graph master push rows.
- Total master push runs in 2026-10-17..2026-11-21: **42**, of which **0** satisfy the `_ci` green contract.

Green 66-job auto-subset pushes observed in the catalog era with lock skipped are **illegal LAN capture sources**:

| Run id | head sha (first 8) | created |
|---:|---:|---|
| 44890379063 | c4e1b3fc | 2026-11-21 |
| 44883609252 | a4a43e84 | 2026-11-21 |
| 44873202146 | a33959f9 | 2026-11-21 |
| 44786511718 | 7d7808a7 | 2026-11-20 |
| 44779492594 | 0f4a040e | 2026-11-20 |
| 44698828922 | fd97cec7 | 2026-11-20 |
| 44696834460 | 818d5cb5 | 2026-11-20 |
| 44696039339 | 68427f97 | 2026-11-20 |

They are not accepted capture sources because `_ci` requires `staging-stack-lock` completed-success (`:157-162`),
and these rows have `staging-stack-lock` skipped. They are also auto-subset, not full-graph CI.

**Limitation:** one `gh api` call in the dispatcher scan returned HTTP 502. The scan result used here is the recorded
GH summary, not a fresh re-scan of all rows from this audit pass.

## Findings

1. **No 24-name catalog-era master push satisfies `_ci`.** Every full-graph master push row has at least
   `3` non-`success`/`skipped` jobs and at most one allowed `deploy-staging / deploy` failure cannot cover
   those rows. Where `staging-stack-lock` is skipped/cancelled, the contract fails even earlier at the
   required-success job checks (`source_authority.py:157-162`).
2. **The 66-job green auto-subset pushes are not capture candidates.** They are green-looking only for the
   affected subset; they fail `_ci` because `staging-stack-lock` is skipped, and the LAN baseline contract needs
   full graph capture authority for the current 24-image stack.
3. **`capture_source` cannot bind tonight.** It pins current master equal to source (`:551-554`) and then calls
   `_ci`; since no current-master push satisfies `_ci`, it fails closed.
4. **`capture_equivalent_master_source` cannot bind tonight either.** It first requires an equivalent historical
   source that satisfies `_ci`; because there is no qualifying catalog-era historical CI row, the ancestor and
   byte-equal backend/CI/context/catalog check (`:452-472`) is moot as a rescue. The Oct-era capture authority is
   already known foreign/23-name and not current 24-name evidence.
5. **The prerequisite is debt repair, not new ceremony tooling.** The immediate repo/CI blocker is the persistent
   full-validation red family feeding `ci-gate` and `full-validation-aggregation`, especially
   `compose-e2e`, `local-ci-parity`, `backend-go (role)`,
   `backend-go-integration (space)`, `backend-go-integration-pr (space)`,
   `role-space-receipt-integration`, `space-native-installer-integration`, and
   `a1-attachment-restart-proof`. Until those members flip green, the aggregators cannot flip green, and
   `_ci` cannot bind a fresh LAN baseline source.

## Consequences — honest, not implied by the code path

- **The LAN node ceremony cannot start tonight** on a fresh 24-name baseline route: Stage B cannot write or bind a
  fresh authority because the repo capture primitive fails closed before any node state is touched.
- **The STG rollout chain stays HOLD** behind the LAN baseline closure. The reviewed baseline successor route is
  blocked one step earlier than the prior LAN-successor audit: no capture source is satisfiable under current
  CI debt.
- **Our UIV3 merges are not the cause of these persistent reds.** The listed reds appear in November-11/12 full-graph
  runs before the current UIV3 merge era, and they are documented Space/Role proof debt plus full-validation
  propagation debt. The UIV3 merges are relevant only as future qualifying code-touching pushes if the CI debt is
  repaired.

## Verdict

**Blocked.** For the 24-name catalog era beginning 2026-10-17, there are **zero green full-graph master push runs**
that satisfy the source-authority `_ci` contract. Therefore both LAN baseline capture paths fail closed:

- `capture_source` cannot bind current master;
- `capture_equivalent_master_source` cannot bind a historical equivalent because no historical catalog-era run
  satisfies `_ci`.

The next prerequisite is CI debt repair for the full-validation family, not running the LAN ceremony and hoping it
binds. The TT continuation card is `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1`.

## Deliberately not done

- No LAN node probe, sudo run, or capture attempt; the repo-side contract already fails closed, so no node ceremony
  was attempted.
- No new TT card created in this audit; the prerequisite is recorded on the existing LAN capture/green-push card.
- No CI re-dispatch or product-code changes in this docs PR.
