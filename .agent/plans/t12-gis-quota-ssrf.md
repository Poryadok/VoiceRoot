# ExecPlan: T12 GIS quotas, suspension, diagnostics, provenance, and callback safety

## Purpose

Complete T12 on `codex/game-sdk-federation-docs` by enforcing one application-wide quota across its environments, adding operator-controlled suspension, exposing safe owner diagnostics and provenance audit, rejecting cross-app/environment identifiers, and registering installation callback URLs safely for the future T15 command dispatcher.

## Context

- Canonical task: `docs/testing/game-integrations-exec-plan.md`, T12 (`D: T11`), and G07/Q12 row.
- Canonical behavior: `docs/microservices/game-integration-service.md` (G06/Q11 bootstrap, credential checks, policy source and audit); `docs/architecture/game-integration-api.md` sections 5–6 (idempotency, `429 RATE_LIMITED`, `Retry-After`), section T03/T06 command contract (future callback is installation-scoped), and portal registry/status contract near line 785.
- Canonical outbound URL policy: `docs/features/game-bot-interactions.md`, section 5, lines 354–358: HTTPS, allowed ports, canonical ASCII callback path, reject private/metadata destinations and DNS rebinding, and do not follow auth-bearing redirects.
- Acceptance: `docs/testing/game-integrations-acceptance.md`, OPS02, expects tenant quota isolation, `429`, SSRF defense, and failure isolation.
- Current implementation base: `codex/game-sdk-federation-docs` at `7c50ec7aed36f6c5b6ce5bfab7dce7fd25d95ba4` (T11 PR #503). Current GIS has application/environment/credential/policy registry routes and PostgreSQL store. Application and environment lifecycle enums already admit `suspended`; there is no suspension route, installation callback field, diagnostics route, quota store, or provider evidence distinction yet.
- Outbound-consumer trace: there is no GIS dispatcher in source. `docs/architecture/game-integration-api.md` defines future GIS command POST to a callback per installation; T15 owns the dispatcher. Existing `src/backend/bot/.../bots.webhook_url` and Bot webhook consumer are bot-owned and are not the T12 game installation surface.
- T12 will add only the installation callback registration/storage seam and reusable safe dialer contract required by T15. It will not implement command delivery or claim live provider proof.
- Constraints: main checkout remains `master` with pre-existing untracked `ideas/`; worktree branch `codex/t12-gis-quota-ssrf`; no staging deployment or Realtime/Compose execution on Windows. PostgreSQL acceptance must run in hosted CI or an approved non-Windows environment.

## Scope

- In: app-wide quota across all environments; application suspension/restore; owner diagnostics and immutable provenance events; exact app/env/installation ownership checks; installation callback registration, strict URL admission and a safe outbound HTTP transport primitive; provider assertion and operator app admission remain distinct.
- Out: GIS command delivery, live Google/provider proof, production admission workflow, staging deployment, and changes to the separate Bot-owned webhook API.
- T15 dependency: command dispatcher must use the T12 safe transport for every callback attempt, revalidate DNS at dial time, reject redirects, and stop when app/environment/installation is suspended or revoked.
- T08/Q12 capacity and restore targets remain proposed until measured by T08/T93; the T12 request-rate contract is an enforcement default, not a capacity claim.

## Selected T12 contract defaults

- Quota: 120 developer installation/callback registrations per application per UTC minute, shared across that app's sandbox and production environments. This quota covers only the mutable installation registration route introduced by T12; read/diagnostics routes, existing application/environment bootstrap and policy/credential routes, operator admission/suspension actions, and future T15 callback deliveries do not consume this bucket. Operator routes remain restricted to configured operator accounts and idempotency keys; this contract adds no separate operator quota. Sequence each registration as: authenticate bearer; resolve the trusted actor and application owner; admit against the shared app limiter; validate environment/installation scope and callback destination; write domain state. Missing/invalid bearer produces no store, quota, or audit call because no trusted actor exists. A request denied after authentication and app resolution creates exactly one sanitized denial audit record (actor/app/env/operation/result/provenance only), which is intentional evidence rather than a partial domain write. A foreign-owner denial occurs before quota admission and changes no quota or domain rows. Once the authenticated owner is admitted by the limiter, a later cross-environment or unsafe-destination denial consumes that attempt and records the denial audit, but creates/updates no installation or credential rows. The 121st registration returns HTTP 429, code `RATE_LIMITED`, and integer `Retry-After` equal to the positive seconds until the next UTC-minute boundary (1–60 inclusive); repeated quota denials coalesce into one sanitized audit row per app and UTC minute keyed by `quota_window_start`, with a denial counter to prevent audit amplification. A rate-limited request does not create domain rows. Do not multiply the allowance by creating environments.
- Suspension: only a configured operator account may suspend or restore an application. Both transitions require an idempotency key, are serialized with app/environment changes, and append an audit record in the same transaction. An owner or non-operator request to change suspension returns 403 `OPERATOR_REQUIRED`; a transition blocked by the current lifecycle state returns 409 `APPLICATION_STATE_CONFLICT`. A suspended application makes existing service credentials and Auth policy unavailable with 503 `APP_SUSPENDED`; 503 is used because all application services are temporarily unavailable and clients may retry after operator restoration. Restoring the app does not reactivate revoked/expired credentials or retired/suspended environments.
- Callback URL: HTTPS only, port 443 only, no userinfo/query/fragment, canonical ASCII path using the Game API contract's literal unreserved segments separated by `/`, with no percent escape, empty, `.` or `..` segment. The source requires an allowed port but names no numeric port set, so T12 chooses 443 as the conventional HTTPS port and narrowest compatible default. Resolve at registration and reject if any returned IP is non-global or special-use. At connection, resolve again, validate every answer, and pin the actual TCP dial to one approved answer while retaining the original callback hostname for TLS certificate verification. Disable redirects (3xx is a terminal response), so no redirect target can receive a signed request.
- Diagnostics/audit: the read-only owner route is `GET /api/v1/game-integrations/applications/{app_id}/diagnostics`. It returns app status/revision; exact environment IDs/kinds/statuses; installation IDs and callback validation/admission status; app-wide quota used/limit/reset; provider values copied from developer policy as `developer_asserted`; application lifecycle approval provenance as `operator_approved`; and independent provider evidence status `not_verified` unless a future provider verifier stores accepted evidence. It returns at most 20 recent sanitized audit events with actor kind/ID, app/env/installation IDs, operation, result, source, reason code, timestamp, and quota-minute counter where relevant. Diagnostics never echo the raw callback URL, credentials/digests, OAuth subject/assertion, provider proofs, request payloads, authorization/signature headers, or secrets. `developer_asserted`, `operator_approved`, and `provider_verified`/`provider_admitted` remain separate values; T12 only records the first two and must never promote either to provider proof.

## Acceptance test map (red → green)

1. **Installation-registration quota and tenant isolation:** PostgreSQL-backed tests create one app with valid sandbox and production environments plus a second app, then prove 120 registration attempts are shared across the first app's environments while the second app independently receives 120; each app's 121st registration returns a typed rate-limit error that HTTP maps to 429 and exact integer Retry-After. A fixed clock at 12:35:05 UTC yields 55 seconds until 12:36:00; advancing the clock across that boundary admits the next request; no quota counter is reset through SQL. Repeated 429s coalesce into one sanitized event per app/minute keyed by `quota_window_start`; after refill in the next minute, the test asserts a second event bucket for the same app. Foreign-owner denial is audited once with sanitized actor/app/env/operation/result/provenance but does not change quota or domain rows. A request by the authenticated app owner that is admitted by the limiter and then fails cross-environment scope validation consumes quota and writes one sanitized denial audit only; it creates no installation or credential rows. Reads, existing registry mutation routes, operator actions, and T15 deliveries are explicitly outside this selected quota scope.
2. **Suspension lifecycle:** HTTP + PostgreSQL tests prove only allowlisted operator can suspend/restore; owner/self requests deny; retries are idempotent; same transaction records actor/source/target/result; suspension immediately denies existing credentials and Auth policy resolution; restore does not resurrect revoked credentials or suspended env; failed/audit-write transitions leave no partial status change.
3. **Diagnostics and provenance:** owner GET route returns app/env/install IDs and statuses, callback validation state without the callback URL, quota usage/limit/reset, recent safe audit events, developer-entered `google` only as `developer_asserted`, application admission only as `operator_approved`, and provider status `not_verified`. Tests create a credential and callback so their secret/digest/URL can be checked absent from serialized output. Foreign owners cannot read the app. The audit projection preserves actor/source/result/target IDs/reason/timestamp, but excludes sensitive payloads and proof.
4. **Scope isolation:** missing/invalid bearer yields no store/quota/audit call. A foreign owner using an otherwise correct app+environment receives one sanitized denial audit event but no quota or domain write. An authenticated app owner presenting a cross-app environment or installation ID is charged only if the app limiter admitted the attempt, then receives one sanitized denial audit event and no installation/credential write. Duplicate/idempotent retries obey the same scope sequence.
5. **SSRF callback registration and dial:** table tests reject representative addresses from each documented private, loopback, link-local, multicast, unspecified, metadata, CGNAT, documentation, benchmarking, transition and reserved IPv4/IPv6 range; empty and failed DNS, plus mixed public/private answers, reject. Safe transport tests issue an actual TLS request through an injected dialer, prove DNS is resolved again at connection, the socket dials an approved resolved IP, a certificate for the original hostname passes while a certificate for another hostname fails, and an actual redirect response causes no second request or dial. Invalid destinations cause zero dial attempts. PostgreSQL registry tests prove registration persists under the exact app/env/installation and the DB-loaded URL passed directly to the reusable client is re-resolved and pinned. A denied registration after owner/app admission consumes quota and creates one sanitized denial audit event, with no installation or credential row. The reusable client is a future T15 dependency; T12 does not wire or claim command delivery.
6. **Hosted integration:** run focused Go registry/httpapi tests and PostgreSQL acceptance via hosted CI; run scoped vet/lint and diff checks. No Windows Realtime/Compose checks.

## Milestones

- [x] Revalidate exact T12 text, target head and source/spec URL consumer.
- [x] Land this ExecPlan and test map before production edits.
- [x] Add failing registry/HTTP acceptance cases for quota, suspension, diagnostics/provenance, ID isolation, callback URL SSRF and safe dial behavior.
- [x] Implement minimal registry schema, routes and safe transport; make all acceptance cases pass without partial secret/proof writes.
- [x] Update API, service, acceptance and T12 progress docs; identify T15 as the only dispatcher consumer.
- [x] Run focused local GIS registry/PostgreSQL, HTTP handler, callback transport and diff checks.
- [ ] Run hosted required checks, final exact-head review and feature-branch PR lifecycle.

## Detailed Steps

1. Add migration(s) for app quota windows/state, installation callback registration, audit provenance/source/result/installation target, and any provider provenance needed for diagnostics.
2. Add API handlers and registry methods using transactions and scope checks before any persistent mutation. Keep suspension operator-only using the existing configured operator identity boundary.
3. Add a dedicated callback URL parser/resolver and HTTP transport with injected resolver/dialer for deterministic tests. Make the transport impossible to follow redirects and ensure actual dialing is pinned to an approved DNS result.
4. Implement each red test first, then minimum code for that behavior, focused green check, and only then refactor.
5. Update `docs/architecture/game-integration-api.md`, `docs/microservices/game-integration-service.md`, `docs/testing/game-integrations-acceptance.md`, and T12 progress here. Record callback dispatch explicitly as T15 work.
6. Review the exact diff for secret/proof leakage, cross-app/env queries, quota transaction behavior, safe dialing, unrelated files, and migration ownership.

## Validation

- `rtk go test ./internal/registry ./internal/httpapi` from `src/backend/gameintegration` for focused unit/handler regression tests.
- Hosted PostgreSQL integration acceptance for quotas, suspension, audit, callback registration and cross-scope denial; Windows testcontainers is not accepted as evidence.
- `rtk go vet ./...` and repository-defined GIS lint/checks; `rtk git diff --check`.
- `graphify update .` after Go source edits.
- Hosted required CI checks on the exact PR SHA; all staging jobs must be skipped.
- No test here constitutes real provider proof or command delivery acceptance.

## Progress

- [x] Checked out exact current feature target SHA in leased worktree and verified clean state.
- [x] Read project instructions, T12 task, GIS service contract, API callback contract, acceptance OPS02, and repository data/verification guidance.
- [x] Traced current callback URL owner and identified missing GIS installation registration/dispatcher on this base.
- [x] Callback, registry and API tests completed red-to-green, including migration upgrade, 120/121 quota boundaries, quota-window coalescing and suspension replay.
- [x] Implementation and docs; focused GIS PostgreSQL suites pass locally with Docker.
- [x] Independent exact-diff implementation, security and migration review found no remaining code finding; its documentation wording finding is corrected below.
- [ ] Hosted required checks, final exact-head review and merge.

Latest T12 review fixes: ownership and environment authorization precedes
suspension disclosure; pending callback environments are denied; the first
quota insertion counts once; idempotent suspension retries restore the stored
status/revision/timestamp snapshot. Audit result uses the stable `success` or
`denied` value; migration 000002 backfills legacy operator sandbox approvals as
`operator_approved` and leaves other legacy audit rows at `system` / `success`.

## Decisions

- Use an app-wide UTC-minute quota because T12 says app-scoped and environments are separately namespaced; per-environment quotas would multiply capacity by adding environments. The 120/minute limit is a selected enforcement default, not a measured capacity result.
- Restrict callback registration to canonical HTTPS on 443 because the existing canonical docs require HTTPS, allowed ports, canonical paths and SSRF defense but do not give the numeric port allowlist; 443 is the smallest conventional default.
- Attach the callback URL to an installation record because the API contract says each command is POSTed to the callback URL of the specific installation; an environment-wide URL would violate that ownership boundary.
- Do not implement the dispatcher in T12: the API contract says command delivery is future GIS work and T15 is its named consumer. T12 supplies the safe URL admission and dial seam T15 must call.
- Do not treat operator approval of an application as Google/provider proof. Only T13's independent provider proof verifier can make that claim.

## Risks And Follow-Ups

- T15 must integrate the safe transport at each outbound command callback and add delivery/retry tests; a helper alone does not establish dispatch readiness.
- This T12 default does not close T08/T93 measured capacity, backup restore, RPO/RTO or live-provider proof.
- Real public DNS behavior is environment-dependent. Unit tests inject deterministic DNS; hosted acceptance should also exercise a controlled resolver/network harness without connecting to a live third-party provider.
