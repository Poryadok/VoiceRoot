# Shared-host dev

`voice-dev` on pmdebook is for WIP; `voice-staging` holds a deliberately selected
build for testing. Same physical host remains a common failure domain.

## Current installation

- Web `https://dev.comrade.click`, API `https://api-dev.comrade.click`, admin
  `https://admin-dev.comrade.click`, portal `https://developers-dev.comrade.click`,
  LiveKit signaling `wss://livekit-dev.comrade.click`.
- Dedicated PostgreSQL, Redis, MinIO, ClickHouse, NATS and LiveKit. Static local
  PVs use `/home/voice-dev/data/{postgres,minio,clickhouse,nats}` and `Retain`.
  These local directories have no filesystem quota; PVC sizes are scheduling
  declarations. Monitor `df -h /home` for actual capacity.
- New DB/S3/JWT/NATS/principal/media credentials; no staging data or private keys
  copied. Private issuance archive `/home/voice-dev/issuance/generation1`, bootstrap
  state `/home/voice-dev/bootstrap` are root-only. Do not print their contents.
- Namespace ResourceQuota and LimitRange; NetworkPolicy isolates other namespaces
  and private address ranges. Traefik and DNS are admitted explicitly. LiveKit
  public UDP is separately scoped. LiveKit alone also admits LAN client ICE
  candidates in 192.168.0.0/24 on UDP1024-65535; other private ranges remain
  blocked. Other dev pods do not receive that exception.
- Dev mail uses an internal Resend protocol fixture, not real email delivery.
  Retrieve test OTPs with a loopback-only port forward:
  `kubectl -n voice-dev port-forward svc/voice-dev-mail 14180:4180` then
  `GET http://127.0.0.1:14180/emails/latest?to=<test-email>`.
- Federation, external billing/push activation and production rollout are outside
  this bootstrap. Admin access still requires the application's documented roles.

## Updating WIP

Build and push images first, or import them into this single node's k3s
containerd. For local imports also register the repository@sha256 digest alias
with k3s ctr --namespace k8s.io images tag <imported-tag> <repository@sha256:digest>
so CRI can resolve it without an unpublished registry pull. Use an exact registry manifest digest. Frontends must be rebuilt
with dev API and WS URLs; reusing a staging frontend image targets staging.
The first installation's three frontend images were built from
`e0d875ba612517cd7aac3b184a2deff41cb0fec3` and imported locally, not pushed to GHCR.
They remain dependent on this node's image cache until published.

On a host with this repository and the intended kubeconfig:

```sh
python3 scripts/dev/update_images.py --image user=ghcr.io/poryadok/voiceroot/user@sha256:<64hex>
python3 scripts/dev/update_images.py --image user=ghcr.io/poryadok/voiceroot/user@sha256:<64hex> --apply
```

Repeat `--image` for multiple services. First command previews with server
dry-run; second updates only main app images and waits for rollout. Only the
owned `voice-dev` namespace is admitted. UID/resourceVersion CAS guards against
concurrent edits; an atomic ConfigMap lock serializes this tool. A failed or
uncertain mutation retains `voice-dev-image-update-lock`. Inspect the Deployment,
rollout and lock UID before manually resolving it; do not blindly delete it.
The updater never resets stores, applies migrations or changes configuration.
Schema-changing builds need separately reviewed dev migrations from the same
source before image updates. Migrations use `migrate up` / Flyway `migrate`;
Flyway repair is not automatic.

## Fresh installation only

`environment.py` renders captured staging workload/configuration identities into
paused dev resources and rejects foreign namespaces, Secrets, host access and
unresolved Secret references. `credentials.py` uses the Linux NATS issuer under
its dedicated UID and generates independent keys; see its CLI help. Public CA
inputs and the JDK truststore must match the selected Auth image.

`bootstrap.py prepare --secrets <fresh-secret-list> --revision <40hex>` captures
source configuration/images and prepares root-only JSON under
`/home/voice-dev/bootstrap`. It reads staging configuration, never staging Secret
payloads. `start` exclusively creates `voice-dev`, provisions fresh directories,
starts stores, bootstraps central NATS streams before consumers, runs SQL
migrations, then apps and routes. It refuses nonempty data directories or an
existing namespace. This is host-specific operational tooling, not an unattended
installer for arbitrary clusters. Bootstrap configuration input must be reviewed
against the matching source revision; captured live images can be mixed revisions.

Recovery actions (`activate`, `central-bootstrap`, `repair-fresh-nats-claim`,
`rotate-postgres`, `configure-routes`) are operator tools scoped to the owned dev
namespace. `activate` can apply captured app configuration; do not use it for
routine updates. Never use bootstrap to reset an existing dev installation.
Captured state includes secrets and must remain root-only.

## Checks

```sh
python3 -m unittest discover -s scripts/dev -p '*_test.py'
kubectl -n voice-dev get deploy,sts,pvc
curl --fail https://api-dev.comrade.click/health
```

Deployment verification results and remaining media routing limitations are
recorded in `.agent/execplans/dev-environment.md`. HTTP health alone is not an
authenticated application or audio test.

Current edge limitation: Go HTTP/2 clients can receive headers then stall reading
Cloudflare response bodies. The public authenticated DM/WS scenario passed with
GODEBUG=http2client=0 (68.560s), and direct Gateway passed (41.881s). Dev Voice
uses the same HTTP/1.1 workaround for HTTPS Gateway JWKS; TLS validation remains
enabled. Remove the workaround only after a public HTTP/2 regression proof.

Voice smoke: real calls/tokens, LiveKit signaling, ICE/DTLS and publishing both
microphone tracks succeeded after the LAN media exception. The bidirectional
RTP test still timed out waiting for remote audio. Audio is not signed off.
