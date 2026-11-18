# Audit history

Durable verdicts of audit passes (TT tracker ↔ GitHub reality ↔ docs registries).
One file per audit pass, authored by the agent that ran it. Files are verdicts,
not execution records: the executable evidence (comments, checkpoints, PR links,
releases) lives in TT task history and must be cited here, never paraphrased as
"claimed true" without a receipt.

## Convention

- One file per pass: `YYYY-MM-DD-<agent-label>.md`, where `<agent-label>` is the
  agent/nick that produced the verdict (e.g. `2026-11-18-tt-fleet-audit.md`).
- Multiple passes on the same day: separate files, do not merge or overwrite.
- Required sections:
  1. **Scope and method** — what was reconciled, what tools/read paths were used.
  2. **Ground truth** — verified facts with anchors (commit SHA, PR number,
     check rollup snapshot, observation id, TT revision).
  3. **Findings** — discrepancies found, each with evidence.
  4. **Actions taken** — exact receipts (counts by TT command kind, card codes);
     distinguish accepted vs failed attempts, retries noted.
  5. **Verdict** — what state the system is in after the pass, honestly.
  6. **Deliberately not done** — blockers, out-of-scope items, report-only items.
- Honesty rules: no fabricated evidence; actor/nick labels in TT are not verified
  identities; staleness conclusions must name their evidence (recorded SHA vs
  merged ancestry, session liveness).
- If a file corrects a number from a previous statement, keep a
  **Self-corrections** section.

## Consumers

Every audit pass is a product artifact, not a transient report. The contract:

1. The audit pass **must** write its verdict into `docs/audits/YYYY-MM-DD-<label>.md` and add one
   Index row here; verdicts are committed to `master` (docs-only PR), never left only in a session.
2. For every actionable finding the audit pass **must** create one TT card
   (`tt task create`, status `available`): title = the finding, description carries the finding
   evidence + **decision** (what to do) + **priority**, and `evidence = docs/audits/<file>` path.
   Not every finding is fixed immediately; the card is the durable unit of work.
3. Scheduled audit tasks (fleet audit 12h, nightly full-master CI gate) run this contract themselves
   (prompt-level mandate); their receipts cite the audit doc path and card codes.
4. Red CI job classes must be registered in `docs/todo/ci.md` (class + evidence + retry policy),
   so "documented debt" enumeration stays verifiable across runs.

## Index

| Date | Agent | Scope | File |
|------|-------|-------|------|
| 2026-11-18 | Codex-PR-Merge (OpenChamber session, maintenance wrap-up) | durable handoff: DEV delivered / STG pending, blockers (PR #1167 vs owned-enrollment infra red, GHCR 2/24), fence grammar, exact next moves | [2026-11-18-stg-reconcile-handoff.md](2026-11-18-stg-reconcile-handoff.md) |
| 2026-11-17 | TT-Fleet-Audit-20261117 (OpenChamber session, model qwen3.8-flash-next-coder; write-actor label Codex-PR-Merge) | TT cards ↔ merged PRs/ancestry (tip 44bd3c76), ci-gate debt registry, stale custody reconciliation with stage advances | [2026-11-17-tt-fleet-audit.md](2026-11-17-tt-fleet-audit.md) |
| 2026-11-18 | TT-Fleet-Audit-20261118 (OpenChamber session, model qwen3.8-flash-next-coder; gate-actor label Codex-PR-Merge) | TT cards ↔ merged PRs/ancestry, ci-gate debt registry, resources | [2026-11-18-tt-fleet-audit.md](2026-11-18-tt-fleet-audit.md) |
| 2026-11-18 | CI-Gate-Full-Audit-20261118 (OpenChamber session, model qwen3.8-flash-next-coder-iq1_m) | full-master CI dispatch run #6034 at head d4af0992: job totals, red-family classification vs docs registries, staging-images-push, GHCR 23+1 head-tag gate | [2026-11-18-ci-full-gate-audit.md](2026-11-18-ci-full-gate-audit.md) |
| 2026-11-18 | TT-Fleet-Reverify-20261118 (OpenChamber session, model qwen3.8-flash-next-coder) | read-only re-verification: head vs pin/STG target, GHCR gate 2/24, kubectl 33/51 drift-0, claim ccba8020 gen12 rev41, CI push/promote skipped; zero deltas | [2026-11-18-tt-fleet-audit-reverify.md](2026-11-18-tt-fleet-audit-reverify.md) |
