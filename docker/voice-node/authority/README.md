# Node authority controller

Build the production command and its dependency closure:

```powershell
rtk proxy python scripts/federation/build-node-controller.py
```

The image runs as UID/GID `10001`. Start `/usr/local/bin/voice-node-authority --config
/etc/voice-node/controller.json`. The strict JSON configuration names an HTTPS
master root URL, pinned public trust, a node bearer credential file, CA and client
certificate/key files, the shared authority directory, canonical Space UUIDs and
an optional `interval_milliseconds` (default 100, allowed 50–500):

```json
{
  "master_url": "https://master.example.test",
  "trust_file": "/etc/voice-node/trust.json",
  "credential_file": "/run/secrets/node-credential",
  "ca_cert_file": "/etc/voice-node/ca.pem",
  "client_cert_file": "/run/secrets/node-cert.pem",
  "client_key_file": "/run/secrets/node-key.pem",
  "authority_directory": "/var/lib/voice-node/authority",
  "spaces": ["11111111-1111-4111-8111-111111111111"],
  "interval_milliseconds": 100
}
```

Trust uses the SFU's `issuer`, `environment`, `node_id` and `keys` schema. Keys
are pinned Ed25519 public keys encoded with unpadded base64url. The bearer is a
canonical 32-byte unpadded base64url credential issued to the registered node.
Only the controller reads its bearer and client key; the SFU mounts authority
read-only and needs public trust. Give the controller ownership of the output
directory and the SFU group read access. Private master signing keys never belong
on either node process. Configuration and credentials load at startup; rotation
currently requires restarting the controller with the new credential files.

Each Space refreshes independently over verified mTLS with redirects forbidden.
The controller verifies the manifest, every page and the complete digest before
requesting an applied-revision lease. Process-local generation, authority epoch
and revision floors reject fresh-signed rollback before ACK. Exact signed
manifest/pages/lease bundles replace `SpaceUUID.json` using a synced temporary
file and atomic rename. Fetch and verification failures leave the last file;
an error syncing the directory after rename can leave a complete replacement
whose persistence is uncertain. Neither outcome extends signed authority expiry.
Transition logs contain only Space UUID and availability.

The independent SFU watchdog remains active if this controller hangs or dies.
The owned Linux media acceptance runs this production executable as UID/GID
10001 against a controlled mTLS signer. Four real RTP peers exercise two Spaces;
revocation ejects one pair while the other continues. The test kills and reaps
the controller, proves the signer still advances, authority files and ACK counts
stop changing, the SFU stays healthy, the second pair expires within five seconds,
and a still-valid admission credential cannot reconnect. Run
`TestSFUEnforcesControllerProcessDeathForRealMedia_live` through the opt-in media
Compose fixture after building its client and SFU images.

Production owning-service policy projection, persisted online boot and
permanent fences, bundle deployment/rotation automation, and qualified worker
load remain open. A local controller/media test does not complete T73–T78 or
capacity acceptance. See the [ExecPlan](../../../docs/testing/game-integrations-exec-plan.md).

## Node-local Voice media edge

The same image includes `/usr/local/bin/node-media`. Run it as a separate
container/process, overriding the entrypoint and passing `--config` to this
strict JSON file (all paths below are node-local; no master private key):

```json
{
  "listen_address": ":8443",
  "trust_file": "/etc/voice-node/trust.json",
  "authority_directory": "/var/lib/voice-node/authority",
  "credentials_file": "/run/secrets/livekit.json",
  "tls_cert_file": "/run/secrets/media.crt",
  "tls_key_file": "/run/secrets/media.key",
  "media_url": "wss://media.node.example.test"
}
```

`livekit.json` contains only `api_key` and `api_secret` for this node's SFU.
Mount that file and the HTTPS server key read-only for UID/GID 10001. Mount the
controller's authority directory read-only; its same complete signed Bundles
are rechecked every 100ms. The edge needs pinned master public trust, its own
HTTPS key and its local SFU secret. It does not need the controller's bearer,
node client key or a master user token. The edge speaks TLS 1.3 end-to-end;
forwarded TLS headers do not enable plain HTTP. Its registered canonical root
endpoint serves `POST /v1/media/token` with the narrow master-signed credential,
explicit room and profile. Wrong/stale scope is denied and responses are no-store.

The master Voice configuration is separate. `VOICE_FEDERATED_MEDIA_CONFIG`
names strict JSON with `master_url`, `master_ca_file`, `client_cert_file`,
`client_key_file`, `node_ca_file`, and `trust_file`. Its trust JSON contains
`issuer`, `environment`, and `keys` (unpadded base64url Ed25519 public keys).
Provision the distinct master Voice certificate pin in
`FEDERATION_MEDIA_ISSUER_CERT_SHA256`. Compose mounts these master-only files
from `FEDERATION_MASTER_VOICE_SECRET_DIR` at `/run/voice/federation-media:ro`.
An absent configuration leaves hosted baseline active; an incomplete configured
runtime fails startup. Federation-enabled deployments must configure this path
before exposing mapped rooms. Rotation of files currently requires restart.

The real media fixture starts the unmodified node-media child as UID/GID 10001,
exchanges each admitted credential over verified HTTPS and uses its actual JWT
at the SFU. Only its internal test media URL is substituted for the returned
public WSS URL; clean-host public TLS bundle acceptance remains open. After
controller death the exchange stays alive but refuses new tokens, independently
of the SFU expiry watchdog.
