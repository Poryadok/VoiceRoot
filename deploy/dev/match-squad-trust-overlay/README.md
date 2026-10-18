# DEV MatchSquad trust overlay

This overlay belongs only to the `voice-dev` namespace and binds the MatchSquad runtime contract from PR #667 to the DEV host and PKI issued for this flow. It is a strategic-merge patch set for the existing DEV Deployments and Services; it does not copy the staging manifests or change the canonical staging configuration. The issuer creates the MatchSquad Secrets and public-only `voice-match-squad-ca` ConfigMap separately. Apply this overlay only after the exact source images, namespace, migrations and credentials have passed the separately authorized DEV activation preflight.

```sh
python3 -m unittest discover -s scripts/dev -p 'match_squad_runtime_overlay_test.py'
python3 -B -m unittest scripts.dev.match_squad_overlay_test

# First update all four workloads to the source-qualified linux/amd64 image
# digests from the readiness packet. Then build an immutable local CAS plan.
python3 -B scripts/dev/match_squad_overlay.py \
  --root . --source-sha "$(git rev-parse HEAD)" \
  --plan-file /tmp/match-squad-overlay-plan.json
sha256sum /tmp/match-squad-overlay-plan.json

# Review the plan hash and exact target list before the separately granted apply.
python3 -B scripts/dev/match_squad_overlay.py \
  --root . --source-sha "$(git rev-parse HEAD)" \
  --plan-file /tmp/match-squad-overlay-plan.json \
  --plan-sha256 'PASTE_THE_REVIEWED_64_HEX_DIGEST_HERE' --apply
```

The Deployment patches provide the existing MatchSquad listeners and bind their `9092`, `9094`, `9095`, and Matchmaking JWKS `8443` ports through the corresponding Services. They add only the existing contract's Secret references and read-only mounts. Matchmaking's pod-level `fsGroup: 65532` lets its nonroot image read the group-readable `0440` signing-key projection. Kubernetes applies that group to every Secret volume in the pod: the current `0400` NATS credential becomes group-readable by that group too, but its read-only volume remains mounted only in the `nats-leaf` sidecar; the signing-key volume remains mounted only in the Matchmaking container. The focused overlay contract pins these Secret modes, key names, and container mount assignments. Matchmaking uses `https://voice-matchmaking.voice-dev.svc:8443/internal/v1/principal/jwks.json`; Chat and Voice configure that issuer directly with their own `*_JWKS_URL` and CA settings. The Matchmaking JWKS publisher remains separate from its principal verifier: `S2S_JWKS_URLS_JSON` now contains only the `gateway` issuer at `https://api-dev.comrade.click/.well-known/voice-principal-jwks.json`, which enables Gateway-originated CompleteMatch while leaving the Space issuer absent. The Gateway host uses the existing system trust roots. Matchmaking's CompleteMatch listener uses its own server identity (`voice-matchmaking-complete-server-tls`) and a dedicated client-CA bundle (`voice-matchmaking-complete-client-ca`); it must not reuse the broader MatchSquad client root. Gateway presents its distinct `voice-gateway-matchmaking-complete-client-tls` client identity and verifies the Matchmaking server through `voice-match-squad-ca`. The Matchmaking replay client keeps the existing `REDIS_ADDR` and sets an explicitly empty password for the current DEV Redis without an auth requirement; Redis replay checks remain enabled. Matchmaking outbound Chat/Voice trust, Chat 9092 and Voice 9094's Matchmaking client trust, and Gateway's outbound Voice 9095 trust use `voice-match-squad-ca`. Voice 9095 still authenticates the Gateway media client with `voice-media-ca`; Gateway keeps its existing media client certificate and key. The Matchmaking JWKS trust bundle and Voice member's outbound JWKS trust remain unchanged. The application JWT principal, audience, RPC/request binding, and replay checks remain the authorization boundary.

Named Service ports and their matching Deployment container ports use names no longer than Kubernetes' 15-character limit. Each Service `targetPort` matches the container port name; the listener numbers and TLS trust paths are unchanged.

Do not apply this overlay to staging or production. It does not run migrations, provision credentials, install GameIntegration, select images, or prove a MatchFound-to-Voice admission flow. Those are separate ordered activation steps with exact-source and custody checks. Before planning, the public CA ConfigMaps and referenced Secrets must exist and be checked by public metadata only. This DEV patch assumes the current no-auth Redis configuration and explicitly supplies an empty password; if that Redis authentication policy changes, resolve the service-specific password/ACL contract before rollout. Do not reuse another MatchSquad replay credential or PKI journal for CompleteMatch.

The applicator renders the seven reviewed patch files against live JSON and records the exact source commit, namespace UID, object UIDs/resourceVersions, source patch digests, and the post-patch projection digests in a mode-0600 plan. The projection is predicted by applying the exact generated JSON Patch operations to a copy of each live object, so it follows API list order instead of local strategic-render order. Existing list entries must retain their identity, value, and relative order. New entries append to an existing list; when the parent list field is absent, one `add` operation creates that field with the rendered list. It refuses an existing plan path. Apply re-reads every object before the first mutation, verifies every source and render binding, then submits a server-side dry-run of every effective patch and compares each returned projection with the frozen desired hash. Any dry-run rejection or mismatch stops with zero writes. Only after all dry-runs pass does it send one atomic JSON Patch per changed object with UID and resourceVersion `test` operations. An already-satisfied target can be skipped only when a fresh read matches its planned UID, resourceVersion, full object hash, and desired projection; it receives no patch. This allows a reviewed plan to continue after a confirmed partial prefix without replaying that target. The fixed order publishes all three Services first, then rolls out Matchmaking, Chat, Voice, and Gateway. It admits only the four source-qualified image digests and the exact reviewed fields in this patch set. In normal seven-target mode, an otherwise-unready Matchmaking Deployment is plan-eligible only through the narrowly pinned listener-removal path described below. All other unready Deployments remain blocked. A conflict, drift, failed request, failed rollout, or uncertain response stops the sequence at that object; the successful prefix is preserved. Stop records distinguish `completed_prefix` (targets whose API state is verified, including Deployment rollout readiness) from `applied_prefix` (the confirmed-persistence lower bound for objects changed during that invocation). If a request times out or loses its response, the named failed target's persistence is unknown; fresh readback is required before planning again. Do not retry an old plan, roll back, recreate objects, or manually issue strategic patches after a partial result. Preserve the plan and object receipts for a fresh reviewed recovery decision.

The normal seven-target plan changes only listener Service ports and the reviewed Deployment template environment, ports, mounts, and volumes. It replaces the known optional Matchmaking `S2S_JWKS_URLS_JSON` entry with the exact gateway-only issuer map; all other existing entries and unrelated fields remain intact. Voice 9095 continues to trust `voice-media-ca` for the Gateway client; the MatchSquad CA is used for the separately reviewed MatchSquad TLS paths. After rollout, verify ordinary Deployment readiness, all named Service endpoints, Matchmaking's verified JWKS readiness, and the real application flow before accepting activation.

The fsGroup recovery commands are separate one-target operations, not special cases of the seven-target overlay plan. Matchmaking recovery is available only when its Deployment matches the pinned recovery template and image. Chat recovery is available only for the exact reviewed post-prefix projection: the current generation is observed, its requested replica is updated but unready, the source-qualified Chat image is active, and all Secret volumes and mounts match the captured boundary. Create a fresh plan with the matching recovery flag, review its single target and digest, then apply that same plan with the same flag. Each fixed patch adds only `spec.template.spec.securityContext.fsGroup: 65532` and waits for that Deployment's readiness. Do not combine a recovery plan with the normal overlay plan; after recovery succeeds, create a fresh normal seven-target plan from the new live resource versions. For example, Chat recovery is:

```sh
python3 -B scripts/dev/match_squad_overlay.py --root . \
  --source-sha 'MERGED_SOURCE_SHA' --plan-file '/private/chat-recovery-plan.json' \
  --recover-chat-fsgroup
python3 -B scripts/dev/match_squad_overlay.py --root . \
  --source-sha 'MERGED_SOURCE_SHA' --plan-file '/private/chat-recovery-plan.json' \
  --plan-sha256 'REVIEWED_PLAN_DIGEST' --recover-chat-fsgroup --apply
```

Chat's nonroot image reads its TLS Secret projection from files with mode `0440`; the pod-level fsGroup supplies the matching group access while preserving the Secret mode and read-only mount. Kubernetes applies fsGroup to every Secret volume in the pod. The existing `0400` NATS credential therefore becomes group-readable by the pod group, but that volume remains mounted only in the `nats-leaf` sidecar; Chat's server-key Secret remains mounted only in the Chat container. The recovery guard pins the complete supported volume-source set (five direct Secrets and the two existing ConfigMaps), each mount path, read-only state, and container assignment. Projected Secrets, unknown volume sources, or extra mounts are rejected. Do not broaden these mounts or change Secret modes.

For Matchmaking recovery, use the separate flag and matching source-qualified template as shown below:

```sh
python3 -B scripts/dev/match_squad_overlay.py --root . \
  --source-sha "$(git rev-parse HEAD)" --plan-file /tmp/matchmaking-fsgroup-plan.json \
  --recover-matchmaking-fsgroup
sha256sum /tmp/matchmaking-fsgroup-plan.json
python3 -B scripts/dev/match_squad_overlay.py --root . \
  --source-sha "$(git rev-parse HEAD)" --plan-file /tmp/matchmaking-fsgroup-plan.json \
  --plan-sha256 'REVIEWED_PLAN_DIGEST' --recover-matchmaking-fsgroup --apply
```

In normal seven-target mode, an otherwise-unready Matchmaking Deployment can use only the listener-removal path: the current generation must be observed, every requested replica updated, and exactly one `S2S_JWKS_URLS_JSON` entry present; the patch removes only that entry. The separate Matchmaking fsGroup recovery command above is limited to its exact source-qualified image and captured template SHA-256 `39a62305c3fdb01824b47bce9dce6d7533c5010c91673cbfbfbf876d97995339`, with the current generation observed, all requested replicas updated, and the S2S entry absent. Chat has its own one-target recovery mode, limited to the exact post-prefix runtime projection and current unready Chat Deployment. Both modes reject an ordinarily ready or drifted target and may add only `spec.template.spec.securityContext.fsGroup: 65532`; other unready Deployments remain blocked.
