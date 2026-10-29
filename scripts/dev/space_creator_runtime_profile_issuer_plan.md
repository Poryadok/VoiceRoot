# ExecPlan: Space Creator runtime-profile identities

## Purpose

Issue only absent fixed Creator identity Secrets from the existing DEV media CA and transfer a private, operation-bound packet to the guarded Creator consumer.

## Context

- `docs/DEPLOYMENT.md` and `reset_credentials.TLS_FAMILIES` define the media client and server identities.
- `space_p3_issuer.py` demonstrates root-owned CA FD/key matching; `space_p3_issuer_export.py` demonstrates sealed memfd and verified peer custody.
- The five identities are Space and Gateway media clients, Auth and Role media servers, and Gateway principal signing. Existing valid Secrets are preserved; incomplete or invalid objects hold.
- The existing live `voice-media-ca` and its matching protected private key are required; their current custody is unknown. The issuer does not create or rotate a CA.

## Scope

Only new `space_creator_runtime_profile_issuer*` files. The Creator phase owner supplies the protected preflight, current cluster observations, packet receiver and create-only effects. No cluster writes, privileged execution, or real credentials are used in development.

## Milestones and steps

- [x] Agree fixed Secret names and TLS SAN/EKU with Creator consumer.
- [ ] Bind a source-owned root guard to a complete, fresh missing-subset preflight before key generation.
- [ ] Validate the closed packet and generated cryptographic material against the retained media CA.
- [ ] Export once via sealed memfd to the verified operation-specific peer; lost outcome holds.
- [ ] Exercise immutable Linux fixtures and obtain independent security review.

## Validation

Linux tests must cover each subset, unknown members, existing/partial objects, mismatched CA/key, wrong SAN/EKU or signer, binding drift, wrong peer, and uncertain handoff. Synthetic fixtures do not establish live CA custody or deployment readiness.

## Progress and decisions

The five-object catalog is source-backed. The source-owned root-readable guarded preflight has not yet been identified; caller-supplied lists or booleans cannot substitute for it. Continue independent packet and crypto work while its owner supplies the fixed authority seam.

## Risks and follow-ups

Actual issuer CA private key, current live trust and target absence are unknown and must be verified by the real guarded operator before issuance. The consumer owns all cluster effects and protects valid existing leaves from replacement.
