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
- [x] Validate the closed packet and generated cryptographic material against a fixture media CA.
- [x] Exercise one-shot sealed memfd export to a verified operation-specific fixture peer; lost outcome holds.
- [ ] Exercise immutable Linux fixtures and obtain independent security review.

## Validation

Linux tests must cover each subset, unknown members, existing/partial objects, mismatched CA/key, wrong SAN/EKU or signer, binding drift, wrong peer, and uncertain handoff. Synthetic fixtures do not establish live CA custody or deployment readiness.

## Progress and decisions

The five-object catalog is source-backed. The draft root stage exchanges two fixed authenticated preflight messages with the consumer and invokes the independently owned root metadata observer before and after generation. The observer and consumer integration are not yet reviewed or proven together. No caller-supplied list or boolean is an authority. Linux fixture tests cover crypto, one-shot FD transfer, missing READY and post-generation drift; external metadata and guards are mocked in stage tests.

## Risks and follow-ups

Actual issuer CA private key, current live trust and target absence are unknown and must be verified by the real guarded operator before issuance. The consumer owns all cluster effects and protects valid existing leaves from replacement.
