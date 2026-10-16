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
- [x] Review, document outcome, commit/push and open PR.

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
fixed via subprocess --patch JSON argument plus a regression test. Actual three
frontend digest rollouts subsequently passed and the lock was released.

## Final outcome

Implemented and live. All27 dev Deployments and3 StatefulSets ready after actual
frontend digest rollouts; original staging workloads and TTenrec remain ready.
30 focused Python tests pass (26dev+4storage). Independent reviewers found no
remaining blockers in the tooling. PR: https://github.com/Poryadok/VoiceRoot/pull/810.
Known limitations remain: public Go HTTP2 response stalls (HTTP1 dev workaround),
failed bidirectional RTP test, test-only email, local frontend image custody, no
full host reboot/off-host backup test. Shared-host storage move and TTenrec
container-restart persistence are verified independently of application audio.

## Continued acceptance audit

2026-10-16: prior turn classified progress: live environment, storage migration,
actual updater rollouts and PR exist. Completion still unproven while audio
acceptance fails. Revalidated30 healthy controllers and /home backing mounts.
Browser loaded login, signed in with a disposable dev fixture account, opened
its DM and visibly sent/read a message. PCM diagnostics showed both sources
processing ~48kHz; one subscription received RTP, but server closed both peers
with NEGOTIATE_FAILED. DM identity filtering matches issued JWT profile IDs.

LiveKit was inherited from staging1.8.4 (2025-03-02), while Compose test infra
uses latest. Testing dev-only official stable1.13.10 (2026-10-14), digest
sha256:89ca9ce0791e18d6e03bf6e2c49ceb27e1c754eae8bd50c989386c1a5963791a;
old image retained for exact rollback. No staging image/configuration changes.

Acceptance result: original unchanged RTP assertions pass with signaling routed
directly to dev NodePort30272 on BOTH LiveKit1.13.10(7.176s) and1.8.4(5.942s).
Restored original1.8.4 digest. PublicCF endpoint still fails negotiation; alternate
api-dev/livekit CF route also failed and its two temporary dev objects removed.
This rules out a required server upgrade and proves bidirectional media path.

Proceed with ownhostname livekit-dev HTTPS origin, not shared staging SNI: adding
a cert to livekit.comrade.click would affect staging TLS selection too. Cloudflare
DNS access is missing (browserloginrequired, no configured envvar); user asked
where API/CLI access is configured or to toggle onlylivekit-dev record DNS-only.
Preparing all certificate/routes before waiting on that one external DNS action.
Certbot pinnedf70ad0adbb7e117f0fe42a63c553f28ea451edabc0148757b6efcd9735acaa20,
client/k3s probe passed. Raw operator diagnostics are private and may contain
Secret data after a malformed kubectl metadata query; no such data in source/PR.

## Current completion audit

- [x] Independent dev namespace/routes, all27Deployments+3StatefulSets healthy.
- [x] Fresh stores/PVs/keys and observed dev-to-staging Postgres denial.
- [x] All container storage on/home, persistent bind dependencies, TTenrec forced
  restart preserves full SQLite content; existing staging workloads retained.
- [x] Digest-only app updater tested with real frontend rollouts and lock release.
- [x] Browser login, DM send/read visibly verified; screenshot in ignored
  tmp/dev-environment/browser-smoke.jpg.
- [x] Original bidirectional audio test passes direct signaling on existing1.8.4.
- [x] Independent trusted dev TLS origin issued; curl resolve HTTP200/TLSverify0;
  unchanged RTP test with actual WSS hostname+test-only DNS override passes6.372s.
- [x] Daily root renewal timer installed+enabled; root systemd-analyze verify and
  actual renewal/no-op service passed Resultsuccess/Exit0. Certificate expires
  2027-01-14; state root0700/home/voice-dev/tls, protected operator copy on/home.
- [x]37dev+4storage focused Python tests pass, independent TLS review no blockers.
- [x] Five explicit dev A95.31.10.177 DNS-only records; no provider access in
  current browser/env, user question pending. Never modify wildcard/staging DNS.
- [x] Original public RTP test without test DNS override after resolver change.
- [x] Implementation PR CI completed successfully at9edb13f6d; final documentation
  update requires only documentation validation, not repeated backend tests.

Only new dev TLS SNI is published; shared staging hostname design was rejected.
Temporary CF path experiment removed, LiveKit upgrade reverted. Existing TLS
readiness does not prove public DNS has switched. Goal remains active.

Five-host expansion succeeded: independent dev certificate covers dev, api-dev,
admin-dev, developers-dev and livekit-dev.comrade.click; expires2027-01-14.
All ten fixed dev HTTP01/HTTPS routes use owned resources. The existing timer
uses the expanded lineage. Independent review approved failure preservation,
ownership/CAS and renewal compatibility. Public API still stalls through the
Cloudflare route even with HTTP1; DNS-only is needed for all five dev names.
User question updated with the exact five A records; wildcard/staging untouched.

Five-host origin acceptance: all five certificates validate (curl TLSverify0),
web/admin/portal/LiveKit HTTP200; API root404 expected, health tested separately.
Original full authenticated bidirectional RTP scenario with actual HTTPS API
and WSS names, test-local DNS overrides only, passed7.477s. No assertion or TLS
verification weakened. Normal public DNS acceptance still pending provider change.
Current /var82GiB free, /home195GiB free after disposable media tool caches.

## Completed public acceptance (supersedes earlier pending outcomes)

2026-10-16: operator created the five explicit DNS-only A records. Authoritative
Cloudflare answers return95.31.10.177; prior recursive caches briefly retained
Cloudflare addresses. First normal-DNS attempt failed at registration while
that stale answer was observed. After expiration, original full authenticated
RTP test passed7.760s with actual public API/WSS names, default Go HTTP client,
normal DNS, no --add-host/--resolve or TLS bypass. All five HTTPS endpoints pass
normal certificate validation and HTTP200 (API /health). All implementation CI
checks at9edb13f6d are SUCCESS or intentionally SKIPPED. Final docs record these
observations; runtime/source implementation remains the same tested revision.

Read-only current storage audit again confirmed original PV/PVC identities,
all six exact /home bind roots, retained rollback copies and SQLite integrity.
The historical cold-content equality verifier now reports tracker content drift:
the live tracker has continued operating. Both tables still have their original
row counts (graph1, projects2); no historical baseline was overwritten or equality
claim fabricated. The forced-restart equality proof remains the migration-time
evidence, and current integrity passes independently.

Remaining operational limitations are shared-host failure domain, test-mail
fixture, locally imported frontend images, TTenrec Delete reclaim policy and no
full host-reboot/off-host disaster recovery proof. These do not block the requested
independent dev installation and storage relocation.
