# NATS JWT credential contract

This directory defines the Kubernetes JWT activation contract. It does not
change `docker-compose.yml`; Compose remains outside this rollout.

`secret-contract.example.yaml` is a placeholder-only Kubernetes interface for
both `voice-staging` and `voice-prod`: render `__VOICE_NAMESPACE__` to the
target namespace in the secret manager, never in source control. Values use
`stringData` and are UTF-8 text; Kubernetes encodes them to `data` on write.

The non-root renderer init container receives `voice-nats-operator/operator.jwt`,
`account.jwt`, `system-account.jwt`, `account.public`, and
`system-account.public`; after signature/claim validation it writes the distinct
`APP` and `SYS` MEMORY resolver preload to a memory-only config file. The main
hub mounts only `operator.jwt` at `/etc/nats/jwt/`, the rendered config, and the
TLS Secret `voice-nats-hub-tls` (`tls.crt`, `tls.key`, `ca.crt`). Its leaf
listener is Service port 7422. It must mount one
`voice-nats-service-credentials/<service>.creds` entry into each matching pod
at `/var/run/nats/creds/<service>.creds`, read-only with `defaultMode: 0400`.
No pod receives the shared Secret wholesale and no app pod receives an operator
or account seed. The account and operator signing seeds are secret-manager-only
rotation material, deliberately absent from this manifest.

For disposable Compose or staging rehearsal, generate fresh fixtures from the
pkg Go module:

```powershell
cd src/backend/pkg
go run ./cmd/nats-jwt-fixture C:\temp\voice-nats-acl.yaml C:\temp\voice-nats-fixture
```

The ACL manifest is mandatory: it must enumerate every service plus the
Job-only bootstrap identity and use exact Core and JetStream subjects. The
generator rejects wildcards, `$JS.API.>`, duplicate subjects, and incomplete
service sets. It emits a disposable operator JWT, account JWT, distinct
service and bootstrap `.creds` files, and seed-free `acl-intent.yaml`; it never
prints private material. The final activation supplies the reviewed manifest
from the canonical publisher/consumer topology rather than granting a default
per-service namespace.

`bootstrap.creds` is a separate, short-lived provisioning-Job credential in
`voice-nats-bootstrap-credentials`. It
must be stored and mounted separately from `voice-nats-service-credentials`;
no application pod or leaf sidecar may receive it.

## Per-service leaf topology

[`leaf-sidecar.template.yaml`](leaf-sidecar.template.yaml) documents the
selected rendering contract used by staging and production. Compose remains
outside this rollout. Each rendered workload
adds one `nats-leaf` sidecar with an app listener restricted to `127.0.0.1:4222`;
the matching application is configured only for that loopback endpoint and has
no route or credential that can reach the hub directly.

The sidecar mounts exactly one `<service>.creds` Secret key at mode `0400` and
uses it for an outbound TLS leaf connection. Its CA and expected hub server name
are explicit. Operator/account JWTs mount only in the central NATS workload,
under `/etc/nats/jwt/`, consistent with the Secret contract above; no app or
sidecar gets them, and no seed is represented in the template.

The hub alone owns fixed, centrally pre-provisioned JetStream consumers. The
activation review must supply exact per-service publish, subscribe, consumer,
ACK and stream-owner grants, prove denial for neighboring Core and JetStream
subjects, and prohibit both dynamic consumer creation and `$JS.API.>`. Rotate a
service credential by replacing its external Secret key and restarting only that
service's sidecar; rotate account/operator material through its separate
rehearsed broker rollout.
