# ExecPlan: independent shared-host dev environment

## Purpose

Run an independently addressable Voice dev instance on the existing pmdebook
k3s host, so WIP can be exercised without replacing the staging instance.

## Context

Sources: user explicitly authorized a same-host dev namespace; docs/DEPLOYMENT.md,
docs/STAGING_SERVER.md, docs/OPERATIONS.md, docs/DATA_STORES.md,
docs/TESTING.md, docs/CONTRIBUTING.md and deploy/staging/media-principal.md.
Staging currently has healthy workloads, independently evolved live manifests,
version-bound NATS preservation transactions and many principal secrets.
The host has 31 GiB RAM (~20 GiB available), 22 GiB available under /var,
and a 110-pod node ceiling. SSH alias voice-staging and user kubectl work.
The user changes the previous no-shared-dev default; record the new environment
in deployment docs. Do not change product features or enable Federation.

## Scope

In: voice-dev bootstrap/deploy tooling, independent credentials and volumes,
resource limits, cross-namespace isolation, dev routes, repeatable WIP updates,
live smoke and preservation evidence for staging.
Out: production release, acceptance of staging as stable, destructive cleanup,
copying staging data/private keys, changing host firewall or other projects.

## Milestones

- [x] Confirm live resource, pod and ingress capacity and safe bootstrap inputs.
- [x] Add isolation/target validation tests and run them red.
- [x] Implement independently generated bootstrap and dev deployment.
- [x] Deploy and prove namespace/data/credentials/network isolation.
- [x] Verify frontend API target and authenticated basic flow via direct Gateway.
- [ ] Review, document outcome, commit/push and open PR.

## Detailed Steps

1. Capture safe inventory, image references, pod count, source config structure,
   available DNS and reusable credential generators. Never print secrets.
2. Define dev-only target validation, namespace/network/port/storage invariants
   and red tests before implementation. Preserve existing staging deployment
   safety guards. Reject references to staging/prod in dev output.
3. Generate new dev trust roots, keys, NATS credentials, DB/S3 credentials.
   Create dedicated stores and migration jobs from the matching source.
4. Deploy within a resource budget; use dedicated addresses and prove compiled
   frontend config uses dev. Keep all staging writes forbidden in the tooling.
5. Run unit/contract tests, server dry-run and actual endpoint/basic auth smoke;
   compare staging workload identities before/after. Obtain independent review.
6. Update docs, run graphify update, commit and push promptly, create PR.

## Validation

- Dev renderer tests: namespace rejection, no shared volumes/host ports/secrets,
  no external staging DSNs/routes, bounded resources and network isolation.
- Live: independent PVC UIDs, healthy dependencies/app, API and frontend target,
  auth/basic smoke and unchanged staging Deployment specs/UIDs.
- Relevant Python/shell checks; no whole-product readiness claim.

## Progress

- [x] Read project/deployment/testing rules and confirmed SSH/kubectl access.
- [x] Synced main master and leased isolated treehouse worktree.
- [x] Bootstrap design and source inspection.

## Decisions

- Owner: root writes implementation/tests/docs. Auditor dev_bootstrap_audit first reviewed read-only, then owned credentials
  and image updater in isolated worktree4; inherited model/effort, focused context,
  checkpoint was reusable issuance. storage_review independently audited root
  worktree and updater. Root is the only live infrastructure writer.
- Preserve staging and production; only voice-dev is an admitted mutation target.
- Generate independent credentials; do not clone live Secret payloads.

## Risks And Follow-Ups

- Shared host remains a common failure domain; /var and pod capacity are bounded.
- Flutter/admin/portal baked API URLs need explicit verification or rebuild.
- Fresh bootstrap is distinct from the existing populated staging preservation
  path; it must not weaken or bypass that path for staging.
- DNS/provider access and RTC routing may require external configuration; report
  exact missing dependencies rather than claim a working public endpoint.

## Deployment evidence (2026-10-16)

All 27 Deployments and 3 StatefulSets ready. Four independent static Retain PVs
Bound on /home; new credentials issued under dedicated Linux issuer UID. No staging
Secret payload copied. All 15 Go DB migrations and Auth Flyway succeeded, canonical
NATS bootstrap jobs completed in dependency order. Dev web/admin/portal built from
e0d875ba612517cd7aac3b184a2deff41cb0fec3 and locally imported. Compiled Flutter JS
contains dev API/RTC URLs and no staging API URL. Public page/API health HTTP200.
In-namespace PG connectivity succeeds; staging PG connection is denied. Resource
requests ~3 GiB memory/960m CPU; quota limits20Gi memory. Image updater apply/no-op
exercised successfully including lock creation/release. All original staging apps
remain ready; repository STAGING_DEPLOY_ENABLED is false.

Independent review fixed namespace-adoption race via atomic create, broad private
UDP exception, automatic Flyway repair, stale PVC binder annotations, completed-state
storage verification, and mutable commit-tag admission in image updater.
Dev mail is an internal fixture; initial missing endpoint env was found by auth
smoke and corrected. One initially logged dev PG credential was rotated before
application exposure; raw migration logs remain root-only. Issuance archive
contains the historical invalid key.

Remaining validation: authenticated DM/WS scenario in progress. Public TCP31081
is reachable; bidirectional RTP audio not yet proven. Linux media test cross-build
requires CGO opus libraries absent on Windows. No whole-product release sign-off.

Authenticated direct-Gateway DM/WS test passed (41.881s): two email accounts,
OTP verification, DM creation, two messages/WS delivery, refresh/reconnect, REST
history and read state. Public-route variant consistently fails reading the
second OTP response despite origin HTTP200; boundary diagnosis remains active.

Public authenticated DM/WS passed with HTTP/1.1 (68.560s). Diagnostic overlay
showed HTTP/2 response headers200 then empty body until45s client deadline. Voice
JWKS fetches also timed out; GODEBUG=http2client=0 restores validated HTTPS key
fetch and real call tokens. This dev-only workaround preserves TLS validation.

Linux CGO media test executed in a disposable Docker tool container. Initial
ICE failed because real LAN clients advertised192.168.0.109 and private UDP was
blocked. Added deliberate LiveKit-only UDP1024-65535 egress to LAN192.168.0.0/24;
all other dev pods/private ranges stay isolated. ICE/DTLS connected, both
microphone tracks published, but caller received no remote audio before timeout.
Bidirectional RTP is a failed check, not a passed feature; investigate separately
from storage correctness. Public TCP31081 reachable.

Final focused checks:26 dev Python tests,4 storage tests, independent bounded
review with no blocking findings before the final LAN/HTTP1 runtime adjustments.
Image updater actual-change smoke caught kubectl --patch-file=- unsupported;
fix and real rollout validation pending before commit.
