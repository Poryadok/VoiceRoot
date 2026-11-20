# DEV ClickHouse OOM-class re-observation 2026-11-20 — read-only DEV ceremony

- **Agent:** OpenChamber session (model qwen3.8-flash-next-coder-iq1_m, working dir
  `D:\Git\Voice`, branch `master`), TT write-actor label **Codex-PR-Merge**, pass label
  **DEV-CHOOM-Reobservation-20261120**.
- **Scope:** honest re-observation of the DEV ClickHouse OOM-class resolution state on the live
  DEV cluster (`voice-dev` namespace on the LAN node `voice-staging` / `pmdebook`), under a fresh
  bounded DEV resource claim, plus the reconciliation of the two `dev_test` CH OOM cards
  (`OPS-CLICKHOUSE-OOM-20261021`, `OPS-CLICKHOUSE-OOM-RECOVERY-450`) against that claim state and
  the acceptance gate. No product code touched, no cluster mutation, no card status change.
- **Method:** `tt --url http://192.168.0.109:30990 --project voice` (`resource show/acquire/observe/release`,
  `task show/comments/comment`), LAN probes as **`pmd` over ssh alias `voice-staging`**
  (`python3 /tmp/nodeprobe*.py` → `kubectl` JSON read-back, `kubectl exec` read-only,
  `kubectl logs`), `gh pr view 1165` for merge timestamp/SHA, `git rev-parse origin/master`.

## Ground truth (verified 2026-11-20T02:54Z–03:12Z)

- `origin/master` tip during the pass: **`aab06d19098ebc0f381f991751205fa44b771092`** (docs PR #1175
  merge; unchanged through the pass).
- DEV resource: **`acquire` at r302→r303** — claim `cec179ac-561d-4332-b788-8410c8f2db85`, window
  `da69f949-d16b-498f-b863-96c21220b7e5`, generation 81, operation 38, phase `prepare`, tasks =
  both CH OOM cards (2026-11-20T02:54:37Z). **`observe` r303→r304** with the fresh evidence
  (03:10:00Z). **`release` r304→r305 `--safety safe --outcome PASS`** (03:12:17Z); DEV now `free`
  at rev 305, gen 81.
- LAN read-back 2026-11-20T03:04Z–03:08Z as `pmd` (uid 1000), namespace `voice-dev`, **no writes**:
  - ConfigMap `voice-clickhouse-memory-policy-v2`: uid `a472c3a4-07cd-491c-9d09-4e36eb4b890f`,
    created `2026-11-17T08:16:59Z`, `resourceVersion` 4371975, body 274 B, sha256
    `940e598ecbbb7afe1a4da5695f085c06161b937b52107d5d8c6252c640807c2d`,
    `max_server_memory_usage_to_ram_ratio` **0.9**.
  - Mounted proof: inside the RUNNING pod `voice-clickhouse-0` the file
    `/etc/clickhouse-server/config.d/90-voice-dev-memory.xml` has sha256
    `940e598e…` — byte-identical to that v2 ConfigMap body (the live pod serves v2, not v1).
  - Pod `voice-clickhouse-0`: uid `5df07cc4-6131-4074-b224-449b6a668f06`, created
    `2026-11-17T08:17:03Z`, `resourceVersion` 4372010, **1/1 Running, `restartCount` 0**, no
    `lastTerminationInfo` recorded, age 2d18h.
  - StatefulSet `voice-clickhouse`: uid `25a6ec5b-d3a9-4549-9c56-10e3ca35e8e7`, created
    `2026-11-17T06:35:22Z`, **1/1 READY**, `replicas` 1, `currentRevision`
    `voice-clickhouse-78cc88cc64`.
  - cgroup inside the pod: `memory.max` **6442450944 B** (= 6.0 GiB exactly), `memory.current`
    **1813389312 B** (1.69 GiB); policy ceiling `0.9 × 6 GiB = 5.53 GiB` sits inside the cgroup,
    ~4.6 GiB headroom, no OOM pressure.
  - `kubectl logs voice-clickhouse-0 --tail=2000`: 287 lines available, **0 hits** for the OOM
    class (`code 241|total memory|OOMKilled|Memory exceeded|out of memory`).
  - Superseded ConfigMap `voice-clickhouse-memory-policy` (v1, ratio **0.8**, uid
    `44ed39bb-e164-492e-a4dd-72f9482aa35f`, `resourceVersion` 4371915, created
    `2026-10-21T09:59:52Z`) still present in `voice-dev` but **not mounted**.
- Node kubectl route (read-back 2026-11-20, corrected form): `/usr/local/bin/kubectl` is a symlink
  to `/usr/local/bin/k3s` (multi-call k3s v1.34.3+k3s1); `--config` is **not** a kubectl flag;
  `/etc/rancher/k3s/k3s.yaml` mode **0644** root-owned world-readable → `pmd` and root resolve the
  **same** single k3s instance (`/var/lib/rancher/k3s`, systemd `k3s.service` `User=root`);
  the NATS marker `voice-nats-generation` requires `-n voice-staging`.

## Findings

1. **The CH OOM class is still resolved in the live DEV cluster** — the v2 0.9-ratio policy is
   byte-mounted in the running pod, restartCount 0, 6 GiB cgroup with 1.69 GiB used, 0 OOM-class
   log hits. Evidence as above (LAN read-back, 2026-11-20). This is a re-proving of the
   2026-11-17 delivery, not a new delivery.
2. **The recorded `next_action` on both cards is unexecutable as written** — `task test PASS on
   DEV-CHFIX-62B6FCA-20261117-V1; accept; done`. The acceptance gate requires the referenced
   deployment to hold a **current** DEV claim; DEV deployment `DEV-CHFIX-62B6FCA-20261117-V1`/`-V2`
   claims were already released (`949a47f2-903e-461a-9a43-aae50c2b55f3` at r299, `4a053888…` at
   r302). Both cards are `dev_test` with **owner empty** (custody released by
   `TT-Fleet-Audit-20261118`), so the verbs that would rewrite `next_action` refuse: `task
   checkpoint` → `conflict / claim task first`, `task claim` → `conflict / claim requires pending
   task`. The honest owed work is recorded in card comments instead (c18, c43).
3. **Ordering caveat:** the cluster-side policy application predates the source-PR merge — v2
   ConfigMap created `2026-11-17T08:16:59Z`, StatefulSet `2026-11-17T06:35:22Z`, while PR #1165
   (policy source) merged `2026-11-17T19:00:34Z` at **`265975b8c89edb8781ece739ea330b37f4870a04`**,
   i.e. **after** product pin `62b6fca83b606c4f458f190d20a7e8e02610168c`. So the live DEV cluster
   state is not a read-back of the current `master` tip; any future DEV deployment record bound to
   a fresh claim must be re-pinned to the current tip, and creating it requires a real re-rollout
   (cluster mutation, claim-gated).
4. **Docs gap:** `docs/STAGING_SERVER.md` said only "kubectl: Usually `sudo kubectl`" and gave no
   working LAN-user invocation; the failing first attempt (`--config …toml`) is not a real k3s
   flag. Closed by this PR (k3s multi-call route, world-readable `k3s.yaml`, explicit `-n`
   requirement, single-instance fact, root-only preservation tree).

## Actions taken (receipts)

- TT resource commands: **3** (`resource acquire` DEV r302→r303; `resource observe` r303→r304;
  `resource release` r304→r305 `safe`/`PASS`), actor Codex-PR-Merge, all rc 0.
- TT `task comment`: **2** (`c18` on `OPS-CLICKHOUSE-OOM-20261021` 2026-11-20T03:11:17Z,
  `c43` on `OPS-CLICKHOUSE-OOM-RECOVERY-450` 03:11:18Z) carrying the full fresh receipt block.
- TT card state changes: **0** (no `task test`/`accept`/`recheck`/`status`/`reopen`, no
  `deployment create`, no `resource` claim left held).
- Refused attempts (noted, not forced): `task checkpoint` ×2 (rc 1, `claim task first`), `task
  claim` ×2 (rc 1, `claim requires pending task`).
- LAN: 3 probe scripts run as `pmd` (`nodeprobe1/2/3`), read-only `kubectl get/exec/logs` only;
  `/var/lib/voice-nats-preservation` and `voice-staging` deployments **not** touched.
- Docs PR: `docs/STAGING_SERVER.md` k3s route rows + `docs/audits/2026-11-20-dev-ch-oom-reobservation.md`
  + `docs/audits/README.md` index row (this file).

## Verdict

The DEV ClickHouse OOM-class resolution is **re-proven live** at 2026-11-20 in the DEV cluster;
the DEV resource is **free** at revision 305, generation 81, with no claim held. Both CH OOM cards
remain **`dev_test`**, tester `TT-DevTester-20261116`, `dev_deployment`
`DEV-CHFIX-62B6FCA-20261117-V1`, epoch `A1`, and their `next_action` cannot run against a released
claim. Docs gap on the node kubectl route closed. STAGING remains untouched and its root ceremony
remains human-gated (see `OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1` comments c1–c3).

## Deliberately not done

- **No new DEV deployment record** (`DEV-CHFIX-…-V3` or similar): `deployment create` records a real
  rollout, and no DEV policy re-rollout happened in this pass; inventing a record would be
  fabricated evidence.
- **No cluster mutation**: no `kubectl apply`/`patch`/`delete`, no ConfigMap repoint, no restart —
  the pass is read-only, so the v2 policy was re-observed, not re-applied.
- **No card status change** and no `task test`/`accept`: the acceptance gate needs a deployment
  record holding a current DEV claim; that ceremony (re-rollout → `deployment create` under a fresh
  claim → tester `test PASS` → `accept`) is owed and named in comments c18/c43.
- **STG package work** (85-card `stg_ready_for_deploy` set under claim `b73c1c98…`, LAN root
  `--install --upgrade-idle-installed-cohort-v1`): still blocked on the user's euid=0 step.
- `GI-T13`, `OPS-STAGING-INITIAL-RECONCILE-144` (other owners) — report-only, not touched.

## Self-corrections

- Comment `c2` on `OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1` (2026-11-20T02:32Z) phrased the marker
  blocker as "the default namespace is a different cluster view where the marker is NotFound".
  Corrected in **c3** (2026-11-20T03:17Z): there is **one** k3s instance on the node; a query
  without `-n` resolves in the `default` **namespace** where that ConfigMap does not exist
  (`NotFound`). All other c2 receipts stand.
