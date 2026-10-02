# Federation Service

The master authority foundation serves the HTTPS v1 contract in
[`federation-service.md`](../../../docs/microservices/federation-service.md)
and [`federation-authority-v1.md`](../../../docs/architecture/federation-authority-v1.md).
It is intentionally not advertised through Gateway. Legacy S2S gRPC remains
unimplemented. The production node controller and maintained node SFU now
consume and enforce signed complete authority in owned local tests; production
owning-service policy projection, node bundle and qualified capacity gates
remain open, so the game merge/staging hold remains in force.

`/health`, `/ready`, and `/metrics` listen on `:8080`. The separate mTLS API
listens on `:9443`; it does not share the health listener. Startup requires a
provisioned `federation_db` URL, server TLS certificate/key, client CA, Ed25519
signing seed file, key ID, issuer, environment, and operator certificate pins.
Startup applies the embedded, advisory-locked schema migration; `/ready` checks
the database and migration version 5. Canonical resource routes require the exact
Space/node placement. Voice rooms persist an explicit immutable RTC room binding
which cannot be reused by another Space/resource on the same node. Migration 5
preserves legacy evidence; missing canonical rooms need a new owner-supplied
generation. Runtime database credentials must belong
to a dedicated role with access limited to `federation_db`.

Kubernetes expects the `voice-federation-authority` Secret described in the
staging and production deployment docs. Create the database and dedicated
database role before enabling the Deployment. The process migrates tables at
startup. Do not put private signing seeds or TLS keys in ConfigMaps or source
control. The standard Compose `app` profile does not start Federation. An
operator may start the separate `federation` profile after setting all runtime
values and mounting local certificates plus the signing seed from
`FEDERATION_LOCAL_SECRET_DIR`; startup fails closed when required settings or
files are absent. Do not use local Compose credentials in staging or production.

Build/run the node consumer separately using
[`node authority`](../../../docker/voice-node/authority/README.md). Full root
PostgreSQL/API tests cover exact placement, room identity/collision, migration
from schema 4, paged signed policy, revision/lease replay, mTLS controller
consumption, suspension and Q11 clean-start audit. The node media fixture proves
actual process death and watchdog enforcement; it uses a controlled signer.

A separate pinned master Voice mTLS role can discover an unambiguous current
media route and mint a private signed media credential. The Voice client then
exchanges only that credential at the registered node's HTTPS `node-media`
edge; the node signs the LiveKit JWT with its local secret. Route/application/
binding/installation/session scope and publish permission are checked at master,
exchange and SFU. See the authority contract for opt-in configuration and the
explicit not-hosted result required for hosted fallback. Root PostgreSQL/mTLS
tests and a separate production exchange/controller real-media fixture cover
these mechanisms; combined owning-service projection acceptance remains open.
