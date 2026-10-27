# ExecPlan: protected Notification primary read (BE-118)

## Purpose and context

Resolve an existing payer primary profile through User's actual read-only
`ResolvePrimaryProfileIDs` authority. The canonical store filters `is_primary`
and `deleted_at`; frozen primary remains primary. No list ordering, profile
provisioning or payer impersonation is allowed. Sources: User/Notification service
docs, subscription lifecycle convergence ExecPlan and DATA_MODEL/DATA_STORES.

The durable predecessor is PR936 at `4a281f191`. This successor is isolated on
`codex/anton-146-notification-primary-read`; that reviewed version remains frozen.

## Scope and decisions

Add a separate `notification-primary` capability and mTLS User listener at
`:9098`, exposing only the existing primary read RPC. Its exact Notification
workload principal binds audience User, method, request ID and complete request
hash with shared replay protection. Leave existing presence configuration and
documented Auth primary-read behavior intact. The ordinary primary read rejects
non-Auth legacy callers; Notification uses no internal-header fallback.

Notification's existing configured signer creates a narrowly scoped primary
client with a separate required mTLS endpoint. No new signing credentials or
provider/worker activation. The future BE-118 dispatch worker is its consumer.

## Milestones and validation

- [x] RED: unknown/missing/wrong caller, request mismatch, ordinary Notification
  header denial, closed method registration and Auth compatibility.
- [x] Implement User protected handler/runtime and bounded canonical account IDs.
- [x] RED/GREEN Notification signed client, exact response recipient validation,
  nil/missing/malformed/unavailable/deadline failure handling and no fallback.
- [x] Real local TLS/replay and PostgreSQL primary/nonprimary/deleted/frozen proof.
- [ ] Affected tests, vet, pinned lint, independent review and exact-head custody.

Use focused Go tests in pkg/socialprincipal, User grpcsvc and Notification
presence. Ordinary bounded tests are authorized; coordinate broader suites and
Graphify separately. No full reminder delivery or acceptance claim follows from
this prerequisite.

## Progress and follow-ups

WT74 lease readback matches `f8791e7efe308fd9fae27855d899eede`; clean source base
`4a281f191`. No other live User writer is known. Coordinate shared paths with
Subscription authority owner147 and ephemeral transport/client owner148; no
Subscription/proto/Realtime edits belong to this stage.

Observed RED: new capability was absent; the ordinary Notification legacy header
incorrectly reached the store. GREEN: both focused tests pass after separation.
User primary domain/Auth compatibility and real PostgreSQL lookup tests pass
with race (12.187s). The real mTLS/HTTPS-JWKS/shared replay fixture passes for the
new capability alongside preserved Space capabilities (5.802s; explicit miniredis,
not a claim of real Redis server acceptance). Notification's real TLS signed RPC,
wrong server identity, metadata/recipient failure and deadline tests pass with
race (3.570s). User main-package short tests pass (12.976s).
Complete scoped compatibility packages pass: socialprincipal short (18.199s),
User grpcsvc short (6.530s), Notification presence (4.966s). Scoped vet and
repository-pinned lint pass for all three changed modules (zero issues).
Independent source/security reviewer144 accepted exact-head review. Broad
Graphify refresh awaits coordinated serialization; no freshness claim is made.

Full BE-118 still needs genuine Subscription current authority, unique durable
in-app insertion plus atomic hint intent, policy-aware push worker and complete
privacy/restore/quarantine behavior. Pending channel work is not delivered work.
