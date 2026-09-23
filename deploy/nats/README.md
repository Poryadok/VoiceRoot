# NATS JWT credential contract

This directory establishes assets only. It does not change `docker-compose.yml`,
the NATS server arguments, app environment, streams, consumers, or NetworkPolicy.
The broker therefore remains anonymous until the dedicated JWT activation PR.

`secret-contract.example.yaml` is a placeholder-only Kubernetes interface for
both `voice-staging` and `voice-prod`: render `__VOICE_NAMESPACE__` to the
target namespace in the secret manager, never in source control. Values use
`stringData` and are UTF-8 text; Kubernetes encodes them to `data` on write.

The activation leaf must mount `voice-nats-operator/operator.jwt` and
`account.jwt` into the NATS pod at `/etc/nats/jwt/` read-only. It must mount one
`voice-nats-service-credentials/<service>.creds` entry into each matching pod
at `/var/run/nats/creds/<service>.creds`, read-only with `defaultMode: 0400`.
No pod receives the shared Secret wholesale and no app pod receives an operator
or account seed. The account and operator signing seeds are secret-manager-only
rotation material, deliberately absent from this manifest.

For disposable Compose or staging rehearsal, generate fresh fixtures from the
pkg Go module:

```powershell
cd src/backend/pkg
go run ./cmd/nats-jwt-fixture C:\temp\voice-nats-fixture
```

The generator emits a disposable operator JWT, account JWT, distinct service
`.creds` files, and `acl-intent.yaml`; it never prints private material. Its
default ACL intent is deliberately narrow and non-activatable: every service
has only its own `voice.<service>.>` namespace and no `$JS.API.>` permission.
The activation PR must replace this placeholder intent with documented exact
publish, subscribe, JetStream consumer, ACK, and stream-ownership grants.
