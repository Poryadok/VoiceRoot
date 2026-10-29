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
- [x] Bind the draft stage to the source-owned root guard and two fresh missing-subset preflight exchanges.
- [x] Validate the closed packet and generated cryptographic material against a fixture media CA.
- [x] Exercise one-shot sealed memfd export to a verified operation-specific fixture peer; lost outcome holds.
- [x] Pin the independently reviewed root bundle payload and exercise immutable Linux fixtures.

## Validation

Linux tests must cover each subset, unknown members, existing/partial objects, mismatched CA/key, wrong SAN/EKU or signer, binding drift, wrong peer, and uncertain handoff. Synthetic fixtures do not establish live CA custody or deployment readiness.

## Progress and decisions

The five-object catalog is source-backed. The root stage exchanges two fixed authenticated preflight messages with the consumer and invokes the independently reviewed root metadata observer before and after generation. A root-owned global one-shot reservation prevents a second operation from silently repeating an uncertain issuance; reconciliation is required. The exporter waits for a bounded, exact acknowledgement after the receiver validates the sealed packet; a lost acknowledgement remains uncertain. A joint fixture exercises the real protected phase, socket exchange, root CA/key check and actual leaf creation; Kubernetes and TT remain fixture inputs. The root bundle source is pinned to independently reviewed payload commit `7a2d8245868ce65a67dd1e2943302f752e8b1c6c`. No caller-supplied list or boolean is an authority.

## Risks and follow-ups

Actual issuer CA private key, current live trust and target absence are unknown and must be verified by the real guarded operator before issuance. The consumer owns all cluster effects and protects valid existing leaves from replacement.
