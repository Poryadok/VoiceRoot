# Independent existing JWKS helper source review

Reviewer `/root/role_admission`: **source CLEAR**, final author runtime tests pending at this review checkpoint. Baseline WT35 `3a2b8e19b0f8fe7a6104b753340f62864e039326`. No reviewer tests, cluster operation, deployment, Auth172 or stopped Auth/Chat runtime.

Scope: helper-only additions in source_plan.py, runtime_stage.py, bridge_root.py, compiler.py and associated helper tests. Unrelated release-policy transport, installer, Bot migration and other concurrent hunks are excluded. Config wiring has a separate review at `independent-config-review.md`.

| File under scripts/staging/nats-rollout-preservation | SHA-256 at reviewed checkpoint |
|---|---|
| source_plan.py | c93bfad0aeba8ecfbc7957f7d20b641bd7b0a8ba79014fade267896c4987270c |
| runtime_stage.py | 83103adc1a21e6cd5dd6f8d85bbb3268770eb4c5f22e09903bcdac22d1cfda12 |
| bridge_root.py | efa249bc5e1135e7267918a1daee0ec28d0bde25d0e17f23a37049d378224792 |
| compiler.py | ff8be2c0e93f685f356f85bc7b14213d6c9048f97f94b440e75d80c91a39ec1f |
| source_plan_test.py | 3f18f6cf2ad2790f010ea1a6b549e1213d386e32513c93cf2dad34eb4bb09a64 |
| runtime_stage_test.py | b75d77fbb41f106c74e393464c422b91da0c9777774ccf0e64b8b7d3cfab385e |
| compiler_test.py | e7b40c8bf34e70a40f4dbea9ca06583992f4e5ab454feae58c37d3b1b2f640cc |
| bridge_composition_test.py | 10de1bd9e3f998b9c3eaa417d424f42f8c2ae210bcba828dceb347a4c0525858 |

The exact existing `voice-p3-principal-jwks` helper is selected only from the reviewed Deployment/Service/publisher-ConfigMap triple in approved source for full/app-only mode. Exact container name, no init/env/command/args, fixed read-only mounts, exact three volumes and selectors prevent treating arbitrary helper names or images as app enrollment. Missing installed helper fails capture; no first-install path is introduced. Running-image identity uses existing Deployment UID → ReplicaSet → Pod provenance and immutable observed nginx digest, not an approved app-image substitution. Previous target/context image equality is required on the existing continuation path.

Original finding CLOSED: validating Deployment/Service without constraining nginx data could admit a route that exposed the mounted TLS key. The final common document validator now requires only `default.conf` data and exact reviewed content SHA-256 `6afc7058fb4fbef8ca8458bd642db1f83c4e79ef9bd9de63ff0d57408468fd08`; independently decoded source matches. It serves `/realtime.json` from `/srv/jwks`, TLS1.2/1.3, denies other locations. Source selection, application plan and compiler target path all enforce the triple. Negative tests change root to `/etc/voice/tls`, broaden location, and attempt the invalid config through explicitly enrolled compiler input.

External `voice-p3-principal-public` must already exist, be immutable, have exactly nonempty bounded text `realtime.json` and no binary data. Its semantic hash is preserved through the existing nonnats private binding/check; this is **not** cryptographic proof that the JSON matches a private signer generation. Provisioning owns that relation; real Chat JWT verification remains the operational gate. Pod readiness also is not a substituted successful JWKS/authentication proof.

The helper joins original snapshots, fencing/draining and strict old-image inventory. After HUB readiness, existing finish/release callbacks execute before the helper's guarded scale/readiness, and helper readiness precedes business app start. Completed finish-ledger helper work is reobserved without repeating scale. The helper is removed from the later generic frontend extra loop; original template/image restoration and strict closed-state checks remain in force. Failure before readiness blocks the subsequent Chat start. Added tests retain call-order failure assertion and completed-ledger no-scale assertion.

No generic nginx exemption, arbitrary source permission, weakened original snapshot, or runtime preservation PASS is granted by this review. Final focused/full author tests and any subsequent source deltas require their own evidence before completion.
