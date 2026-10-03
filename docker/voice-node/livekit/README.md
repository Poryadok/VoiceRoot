# Voice Node SFU implementation candidate

This dedicated build pins LiveKit v1.8.4 and LiveKit protocol v1.34.0 to the
archive SHA-256 values in `scripts/federation/build-sfu.py`. `prepare.py` refuses
upstream source-anchor drift. Federation's canonical protocol, node cache and
media-authority packages are copied into upstream; no second policy algorithm
is maintained in the adapter. Build contexts are fresh, minimal, and retained
under `tmp/sfu-enforcement-20261002` with their input manifests.

```powershell
rtk proxy python scripts/federation/build-sfu.py
rtk proxy python scripts/federation/build-media-fixture.py
```

The private `voice_media_grant` claim crosses LiveKit's internal StartSession
relay and never enters participant metadata/attributes. The selected SFU checks
the signed explicit room and identity before room creation/admission and again
before join/resume. Active sessions retain a private verified admission keyed
by the concrete participant SID. An independent in-process watchdog enforces
the current signed per-Space policy/lease. JWT refresh copies the original
admission credential without renewing it.

Startup requires `VOICE_NODE_ID`, `VOICE_SFU_TRUST_FILE`,
`VOICE_SFU_AUTHORITY_DIR` and `VOICE_SFU_BOOT_REQUEST_DIR`. The configured node ID is its canonical registered
UUID. Trust contains the master issuer, environment, node ID and pinned Ed25519
public keys. The authority directory carries atomically replaced signed complete
manifest/page/lease bundles. SFU reads it without write access; failed updates
cannot replace valid complete authority. Missing trust/authority is fatal with
a nonzero process exit. Clock rollback or expiry closes admission/media.

Mount the separate boot request directory read/write for UID/GID 10001; use an
absolute existing non-symlink directory with mode 0750 shared with the controller
and node-media. Exclude it from backups. The SFU generates its own boot UUID and
writes a one-second request containing no credential. Before activating saved
authority, it requires a fresh master-signed lease naming that UUID. An old
Bundle cannot activate a restarted SFU even if its signature and time remain
valid. See the [controller configuration](../authority/README.md) for all paths,
permissions and fail-closed cleanup requirements.

The opt-in `docker-compose.federation-media.yml` overlay keeps the existing
hosted LiveKit service intact. Use it with the existing owned Compose project:

1. Run `sfu-media-acceptance` once with entrypoint `/usr/local/bin/fixture-init`.
2. Start `federated-sfu` and wait for its health check.
3. Run `sfu-media-acceptance` with its default command.

The initializer keeps ephemeral test credentials in the owned volume. Both
signalling and bidirectional Opus RTP run between containers with no published
ports. `udp_port: 7882` uses a shared UDP listener for multiple participants.
The default test covers two Spaces, revocation, active admission-credential
expiry, independent lease expiry, and stale bearer rejection. The additional
`TestSFUEnforcesControllerProcessDeathForRealMedia_live` runs the production
[node controller](../authority/README.md) as a separate UID/GID 10001 process
over verified mTLS, then kills and reaps it while the controlled signer advances
and the SFU stays healthy. Files and ACK counts stop changing; real media expires
within five seconds and a fresh bearer cannot reconnect. The Linux fixture also
runs `TestNodeMediaRestartRequiresFreshBootLease_live`: it kills/reaps and restarts
the actual HTTPS edge, proves the unchanged old lease is still valid when access
is denied, then obtains a fresh signed boot lease and proves admission resumes.
These use a controlled signer; the combined owning-service-to-master media path,
remote purge participation, full bundle backup/restore and qualified
capacity/2× load remain open. See
[ExecPlan](../../../docs/testing/game-integrations-exec-plan.md) and
[federation contract](../../../docs/architecture/game-federation.md).
