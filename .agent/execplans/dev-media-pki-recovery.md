# ExecPlan: DEV media JWKS PKI recovery

## Purpose

Provide a reviewable DEV-only recovery path that issues distinct server TLS
leaves for `voice-voice` and `voice-gameintegration`, moves Role outbound JWKS
trust to an additive bundle, and rotates the exposed DEV Voice leaf without
changing inbound mTLS trust or Staging.

## Context

- Docs: `scripts/dev/README.md`, `docs/DEV_SERVER.md`, `.agent/AGENTS.md`.
- Code: `scripts/dev/environment.py` renders the DEV Role; the issuer and
  operator are `scripts/dev/media-pki-issuer/issuer.py` and
  `scripts/dev/media_pki.py`.
- Current state: the existing `voice-media-ca` Secret is public-only; GIS TLS
  is absent. Existing issuer custody is not recoverable from the operator's
  accessible archive. DEV Voice leaf key exposure requires controlled leaf
  replacement; no Secret payload containing a private key may be queried.
- Constraints: issue only in a dedicated UID 10001 container with a Docker
  named volume, no host bind/socket, no host admin changes, no Staging actions,
  and no Kubernetes write before independent review.

## Scope

- In: fixed DEV issuer, name-constrained CA, separate 90-day leaves, additive
  JWKS trust projection, UID/resourceVersion guarded Secret/Deployment writes,
  receipt and pre-leaf rollback, operator documentation and focused tests.
- Out: Staging, Space/Gateway leaves, Role product authorization changes,
  plaintext key backup/export, deployment execution before review, and host
  package/security changes.
- Follow-up: an approved encrypted offline backup process is needed for
  long-lived CA custody; this implementation intentionally has no plaintext
  key export path.

## Milestones

- [x] Add a non-root, network-isolated issuer image and fixed DEV issuer.
- [x] Add the DEV operator flow with additive trust, create-only GIS TLS,
  Voice leaf rotation, resource identity pins, and pre-leaf rollback.
- [x] Separate Role outbound JWKS trust from inbound client CA in DEV rendering.
- [x] Document scope, custody, rollback boundary, and exact focused checks.
- [ ] Independent review and Linux-focused test execution.
- [ ] After review, operator-approved DEV issuance and runtime GIS/Voice checks.

## Detailed Steps

1. Build the pinned-base issuer image, record its immutable local image ID, and
   initialize the named volume once with `media_pki.py prepare`.
2. Review the image and tool source. Run focused Linux tests and verify the
   generated chains reject `voice-space` and Gateway hostnames.
3. Run the reviewed `apply` only in `voice-dev`. The tool preserves the old
   public bundle and inbound CA, updates Role outbound trust, creates GIS TLS
   only when absent, rotates Voice TLS, and restarts Role/Voice.
4. Validate the GIS installer and Voice JWKS at runtime. Staging is a separate
   decision with its own review.

## Validation

- `python3 -m unittest scripts.dev.media_pki_test scripts.dev.environment_test`
  (run from repository root on Linux; not executed in this Windows checkout).
- `git diff --check` (completed).
- Reviewed source-level checks prove namespace and resource names are fixed,
  TLS key payload reads are absent, trust update is additive, and rollback
  refuses after leaf cutover starts.
- Runtime issuer build, initialization, Kubernetes writes, and service checks
  remain pending independent review and an authorized Linux DEV operator run.

## Progress

- [x] Source, tests, and docs prepared in treehouse lease 7 on branch
  `codex/dev-media-pki`.
- [ ] Independent review before any issuer initialization or cluster mutation.

## Decisions

- Use a separate `jwks-ca.crt` key and projection so Role's inbound
  `ROLE_PRINCIPAL_CLIENT_CA_FILE` remains on the existing `ca.crt`.
- Keep the CA key in the dedicated container volume; stream generated leaf
  material from that volume through process memory directly to kubectl stdin.
- Preserve existing trust roots during overlap. Do not restore the exposed old
  leaf key after cutover; repair forward under the additive trust root.

## Risks And Follow-Ups

- The Docker named volume persists issuer custody but the tool does not provide
  encrypted offline backup; that process must be approved before long-lived use.
- The issuer enforces negative hostname chain checks with OpenSSL at issuance;
  the focused runtime integration and service rollout checks are still pending.
- Recovery is intentionally DEV-only; no Staging resource or secret is touched.
