# ExecPlan: current Creator ingress

## Purpose and context

Restore the reviewed typed Creator authentication boundary onto b8a71768 for
`A1-SPACE-CREATOR-COMPOSE-UNAVAILABLE` (TT c78 author offer, c79 ACCEPT).
The canonical target remains migration 34 -> 45. Sources are Space Service API,
Architecture Requirements Phase-0 principals, DATA_MODEL account/profile rules,
TESTING and CONTRIBUTING. CodeGraph/graphify are excluded by explicit user order.

## Scope and ownership

342 owns this isolated transport stage in Treehouse38, lease
9fb3f9a1332912c273a45632613c86ad. Port only reviewed 072817 Creator capability,
ingress and Auth floor checker; extend current interceptor/runtime for Creator
only. Do not copy admission dispatch, native SQL, Role receiver or main/config
activation. Native186 and Role183 retain their source ownership.

## Milestones and detailed steps

- [x] Run current ordinary/protected Creator boundary tests before implementation.
- [x] Restore the selected capability/checker and add dedicated runtime options.
- [x] Run package race tests and vet in owned Linux Docker without host ports.
- [x] Exercise real TLS/JWKS transport with explicitly controlled callbacks.
- [ ] Freeze source/proof and request independent review through the dispatcher.

## Validation

`go test -race ./internal/lifecycleprincipal ./internal/creatorauthority -count=1`
and matching `go vet`, in golang:1.26-bookworm. Negative cases include ordinary
listener, absent callback, raw metadata, wrong body/RPC/audience, replay, revoked
floor, unavailable floor and expiry after ingress. Existing lifecycle tests must
retain their unlisted-method denial independently of the new Creator route.

Executed immutable archive `cf446ddbffff75c139d9d0356fc8614c8beb951e3c67415abd6c6b67d30a5a01`:
both baseline boundary tests RED (ordinary handler reached, protected method
unlisted), final race GREEN lifecycleprincipal 4.831s / creatorauthority 1.224s,
including real TLS/JWKS Creator transport 1.72s; affected-package vet and full
Space `go build ./...` exit 0. Raw logs/source manifest are in
`tmp/swarm/rudeus342-current-ingress/` in the main checkout, for TT attachment.
Docker image was `sha256:dd2d88d0c7034f9e48bb74156ea562e66d3064971aed54ccbb23554637580f1c`.

## Decisions

Creator uses explicit dedicated epoch/replay callbacks; legacy Redis callbacks
cannot activate it. No main, environment, migration or canonical Compose change
occurs in this stage. A controlled transport callback is component evidence only.

## Risks and follow-ups

Full public POST remains blocked until accepted native45 affected-writer closure
and qualified actual Role OwnerBootstrap inputs are assembled. This source stage
does not authorize activation, unknown baseline sealing, production identity,
Invite authority, or claim full CI/A1 success. See native186 c203 reconciliation.
