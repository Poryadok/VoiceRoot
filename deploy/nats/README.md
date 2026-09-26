# NATS JWT credential contract

This directory defines the Kubernetes JWT activation contract. It does not
change `docker-compose.yml`; Compose remains outside this rollout.

`secret-contract.example.yaml` is a placeholder-only Kubernetes interface for
both `voice-staging` and `voice-prod`: render `__VOICE_NAMESPACE__` to the
target namespace in the secret manager, never in source control. Values use
`stringData` and are UTF-8 text; Kubernetes encodes them to `data` on write.
The exact required keys, TLS SAN, and staging delivery bundle are specified in
[`docs/DEPLOYMENT.md`](../../docs/DEPLOYMENT.md#nats-jwt-and-leaf-activation).

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

The reviewed policy is [`acl-intent.yaml`](acl-intent.yaml). It enumerates all
19 services and the Job-only bootstrap identity with exact event, consumer,
pull and ACK permissions. Service users cannot create, update or delete
JetStream state. Reply inbox subscriptions use scoped service prefixes, never
`_INBOX.>` or `$JS.API.>`. The disposable fixture command above is for proof
only; its signing seeds are discarded.

For staging or production issuance, use the separate Linux-only issuer with a
fresh absolute destination and external TLS certificate/key/CA PEM files. The
certificate must chain to that CA and contain an exact `voice-nats` DNS SAN.
Run this on an isolated trusted Linux host/runner with a protected parent
directory. The shared staging `pmd` host is not an issuance location because
its UID also holds SSH and Kubernetes credentials; do not copy output or
signing seeds into `/home/pmd`. Prefer an ephemeral private filesystem,
transfer the `secrets.json` restore bundle through the approved secret-manager
stdin path, and separately back up the three signing seeds there:

```bash
cd src/backend/pkg
go run ./cmd/nats-jwt-issuer --namespace voice-staging \
  --tls-cert /secure/voice-nats.crt --tls-key /secure/voice-nats.key \
  --tls-ca /secure/ca.crt ../../../deploy/nats/acl-intent.yaml \
  /secure/fresh-nats-issue
```

The destination must not exist. Output files are mode `0600`, directories
`0700`. `secrets.json` is the staging restore contract: a Kubernetes `List`
with exactly four Opaque Secrets in the target namespace, using base64 `data`
and the keys listed in `secret-contract.example.yaml`. Individual Secret YAML
manifests are also emitted. `signing-seeds/{operator,app-account,system-account}.seed`
must be backed up separately in the secret manager for rotation; they are not
part of the Kubernetes List. Never commit or print the destination. The issuer
fails if the destination exists or if Linux permissions, ACL, or TLS validation
fail. Staging restore transports only gzip+base64 of `secrets.json` via secret
stdin; the hosted proof reports its byte count against the 48 KiB limit.

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
four bootstrap Jobs own 40 durables. A preexisting durable with a different
filter, delivery subject, policy, group, or push/pull mode fails bootstrap and
requires an explicit migration before the application is started. Auth's old
random-target `auth_subscription_tier` cannot overlap the new no-group push
binding during a rolling start. Staging migration is an additional hard gate:
the existing anonymous `$G` account was observed with 566 consumers, while
the issued APP account limit is 512. Inventory and classify the legacy
dynamic consumers and prove a state-preserving migration of the existing
eight stream messages before account cutover. Do not silently raise the cap,
discard consumer state, or recreate the hub on empty storage. Rotate a service
credential by replacing its external Secret key and restarting only that
service's sidecar; rotate
account/operator material through its separate rehearsed broker rollout.

### NATS 2.12.12 non-JetStream leaf compatibility

The staging per-service `$G` leaf config sets an empty `default_js_domain`
mapping as a temporary NATS 2.12.12 compatibility bridge. This prevents that
version's automatic `$JS.API.>` deny from blocking ordinary JetStream API
request/reply traffic across a non-JetStream leaf. The isolated NATS 2.12.12
two-server regression test verifies the map is required on the leaf; adding it
to the APP hub alone does not restore the route. This does not enable JetStream
or storage on the leaf and does not widen the Search user JWT permissions; the
exact API publish and scoped inbox subscribe ACL remains the authorization
boundary. Staging hub and leaf images pin the official multi-platform
`nats:2.12.12-alpine` manifest digest
`sha256:2ca98656a279b2d88cfdf2b8c3f0d5d7f3941ae9dc2ab12ebaa92d83e0f4ccdb`.

NATS marks `default_js_domain` as temporary backwards-compatibility behavior.
Before upgrading the NATS server, migrate to a named JetStream domain and the
matching `$JS.<domain>.API` request subjects, with ACLs and consumer clients
updated and tested together. Do not remove the leaf mapping or enable leaf
JetStream as a workaround. This bridge is limited to staging's
currently verified NATS 2.12.12 runtime; production configuration is unchanged.

Auth's fixed subscription-tier durable uses a five-delivery budget with an
increasing server backoff. Persistent processing or unsupported-payload errors
are copied with their original bytes, SHA-256, source sequence, and reason to
the centrally provisioned `subscription_auth_quarantine` stream (P400D) before
the source delivery is ACKed. If quarantine publication fails, Auth NAKs the
source and logs an operator-visible error; the source remains retained for
recovery and JetStream emits its max-deliver advisory when the retry budget is
exhausted.
