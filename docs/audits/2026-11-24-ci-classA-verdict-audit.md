# Audit 2026-11-24 — class-A CI verdict + fleet snapshot (OpenChamber autonomous session)

Cadence: full audit due 12h after `2026-11-23-openchamber-fleet-inventory-audit.md` (overdue ~8h at start, closed by this entry).

## Scope and evidence base

- Repo snapshot from fresh worktree at `origin/master` tip `f9732dd7fe939369773c0285b6c1f44307a1271e` (PR #1210 docs merge).
- Hosted checks read-back via `gh pr checks 1207` / job logs saved in the class-A harness (`log-voice-new`, `log-authjava-new`, `log-cigate-new`, `log-ownedenr-new`, `log-voice-r2`).
- Main checkout `D:\Git\Voice` is intentionally stale (per fleet rule: never commit/push there; it lags `master` until LAN-ceremony follow-up).

## class-A (CI debt) — PR #1207 status at head

- Head: `1d1c54683636115490e2e55df9055800cd40ef18` = commit #5 `ci: class-3 fix intents batch SELECT column drift + drop orphaned single-read helpers` on lane branch `codex/ci-space-ban-attver-normalization-20261123` (commits: `6dd4506e`/`85eef226` class-A attver normalization, class-3 batching follow-ons, `3a0f6aa9` batching landing, `1d1c5468` fix).
- Root cause of the class-3 reds at `3a0f6aa9` (role, role-space-receipt, nsi, grpcsvc, ci-gate chain): the batch member-authority loader SELECTed `membership_generation` on `space_member_admission_intents`, whose column is `generation` (`000039_native_membership_authority.up.sql:46`); column-not-found aborted every cohort snapshot read including an ACTIVE kind-1 head. Fixed at `1d1c5468` (one batched SELECT per relation preserved; per-entry validation cores shared with single-read routes; 5s/2s observation leases untouched).
- Same commit cleared golangci debt (ST1021 doc-comment re-anchor; deleted orphaned `loadEffectiveHead`/`retirePendingMemberAdmission`, zero callers verified).
- Full-graph runs at the fixed head: run 45459880334 (first attempt), then a full-run rerun (proxy flakes), third attempt queued. First attempt reds: voice shard, `chat-auth-java-integration`, `ci-gate`, plus `owned-enrollment` (separate run 45459880284).

## Hosted red attribution (this audit's core finding)

1. `backend-go-integration-pr (voice)`: failed **before any test ran**, `go: can't load test package` — proxy.golang.org read failures for `containerd/containerd@v1.7.18` (attempt 1) and `github.com/livekit/protocol@v1.46.0` (rerun, job 138704568857). Infra transport flake class, not product; shard code compiles green locally (`GOPROXY=off go build ./internal/store ./internal/grpcsvc` EXIT=0).
2. `chat-auth-java-integration`: "Go integration matrix evidence rejected: required integration cases did not complete successfully" in the same proxy-starved window; **passed on the rerun** → transient, confirmed.
3. `ci-gate`: pure aggregator — its JSON mirrors exactly the two shards above (L1287 `backend-go-integration-pr: failure`, L1339 `chat-auth-java-integration: failure`); flips green when they do.
4. `owned-enrollment` (`target8 Go image identity mismatch`, `scripts/ci/staging_story_target8_helper.py:86`): matches the **registered live-infra debt** verbatim (`docs/todo/ci.md:173/174/281`: fails on every PR head since ≥2026-11-02, zero successful runs in a 1000-run window, pins verified current via registry manifests, master stays green; TT card `CI-OWNED-ENROLLMENT-TARGET8`; merge precedent: docs-only PR #1192 merged `--merge --admin` under this same condition). The NATS enrollment body itself PASSED (`owned_nats_enrollment=PASS verifier=PASS signature_refusal=PASS broker_auth=PASS production_refusal=PASS`). Not a class-A blocker; no code change owed in this PR (authorized re-pin routes live in the TT card).

## Local verification snapshot (class-A lane)

- Fixed head builds green: `GOPROXY=off go build ./internal/store ./internal/grpcsvc` EXIT=0, gofmt clean (only pre-existing untouched-dirty `internal\s2s\privacy.go`).
- Full store suite post-fix: first attempt reds in `bot_installation_scope`/`bot_message_authority`; isolated re-runs pass 2 of 3; `-count=3` sample shows expiry deltas 6.0/8.4/11.3/12.6s vs the 5s lease = Windows-docker slowness noise band (same code path was green on the pre-fix 992s local run; batch intents SELECT is conditional on ACTIVE kind-1 cohorts and unchanged for these tests). Hosted Linux store jobs green at the head (space shard 2m36s, integration-pr (space) 11m25s, space-native-installer) are authoritative for this class.
- Gate evidence (class-3 consumer, `TestAccountBanOver100Cohort50` on mirror `_f`): pre-cut 5.4677969s FAIL → post-cut 4.9469441s PASS (`error=<nil>`); fresh local consumer mirrors share the container PG heap (4.95–5.81s noise band) → hosted nsi consumer is the authoritative <5s gate.
- class-A pins harness re-run skipped with recorded rationale: Go-only changes cannot move db-init SQL digests (PG16 `178743ba…`/PG18 `685105e0…` stay).

## Fleet snapshot

- Open deliverable: PR #1207 unmerged awaiting full green at head (only `owned-enrollment` infra-class red + shard flakes block; merge will use the registered precedent if green-at-head is unreachable due to that infra class).
- LAN capture legality + owner Stage B–C: HOLD until class-A merged + first all-green full-graph master push; `lan-ceremony-owner-20261121.txt` scripts to be refreshed at the new tip (user's personal sudo steps on the ssh node; assistant never runs sudo there).
- STG rollout `:d4af0992…` + 84-card acceptance + fence-claim release (`b73c1c98-1b13-43f9-a5db-9b046f71de6f`, gen 14, rev 57): HOLD until LAN Stage C closes; dispatch surface = `staging-deploy.yml` workflow-dispatch with `image_tag`/`changed_services` inputs (create-repository-event form 422s, abandoned today per recorded HOLD decision).
- TT tracker: down (connection timeouts, repeated probes); class-A evidence items 1–10 staged locally for posting on recovery to `CI-SPACE-BAN-AUTHORITY-FIXTURE-DRIFT`.
- Cleanup owed after PR resolves: class-A harness run-3 containers/volumes + r2 image `classa-cdbi:r2-6dd4506e` + `classa-mirror-pg16` container + pinned migrate bin + `disc-original-20261124` worktree removal.

## TT handling

- Cards referenced: `CI-SPACE-BAN-AUTHORITY-FIXTURE-DRIFT` (class-A lane carrier, evidence staged), `CI-OWNED-ENROLLMENT-TARGET8` (infra debt carrier, already `available`). Both actionable on tracker recovery; no status change possible while TT is down (connection_error).
