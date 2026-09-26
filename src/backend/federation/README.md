# Federation Service

The master authority foundation serves the HTTPS v1 contract in
[`federation-service.md`](../../../docs/microservices/federation-service.md)
and [`federation-authority-v1.md`](../../../docs/architecture/federation-authority-v1.md).
It is intentionally not advertised through Gateway. Legacy S2S gRPC remains
unimplemented; federation stays deferred until a Voice Node/media verifier
consumes and enforces the signed authority.

`/health`, `/ready`, and `/metrics` listen on `:8080`. The separate mTLS API
listens on `:9443`; it does not share the health listener. Startup requires a
provisioned `federation_db` URL, server TLS certificate/key, client CA, Ed25519
signing seed file, key ID, issuer, environment, and operator certificate pins.
Startup applies the embedded, advisory-locked schema migration; `/ready` checks
the database and migration version. Runtime database credentials must belong
to a dedicated role with access limited to `federation_db`.

Kubernetes expects the `voice-federation-authority` Secret described in the
staging and production deployment docs. Create the database and dedicated
database role before enabling the Deployment. The process migrates tables at
startup. Do not put private signing seeds or TLS keys in ConfigMaps or source
control. Compose does not start Federation.
