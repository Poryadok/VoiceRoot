# Node authority controller

Build the production command and its dependency closure:

```powershell
rtk proxy python scripts/federation/build-node-controller.py
```

The image runs as UID/GID `10001`. Start `/usr/local/bin/node-authority --config
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

Production owner projection and media-grant issuance, persisted online boot and
permanent fences, bundle deployment/rotation automation, and qualified worker
load remain open. A local controller/media test does not complete T73–T78 or
capacity acceptance. See the [ExecPlan](../../../docs/testing/game-integrations-exec-plan.md).
