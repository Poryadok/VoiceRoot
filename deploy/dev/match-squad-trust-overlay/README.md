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

The Deployment patches provide the existing MatchSquad listeners and bind their `9092`, `9094`, `9095`, and Matchmaking JWKS `8443` ports through the corresponding Services. They add only the existing contract's Secret references and read-only mounts. Matchmaking uses `https://voice-matchmaking.voice-dev.svc:8443/internal/v1/principal/jwks.json`; Gateway's public JWKS host is `api-dev.comrade.click`. Matchmaking outbound Chat/Voice trust, Chat 9092 and Voice 9094's Matchmaking client trust, and Gateway's outbound Voice 9095 trust use `voice-match-squad-ca`. Voice 9095 still authenticates the Gateway media client with `voice-media-ca`; Gateway keeps its existing media client certificate and key. The Matchmaking JWKS trust bundle and Voice member's outbound JWKS trust remain unchanged. The application JWT principal, audience, RPC/request binding, and replay checks remain the authorization boundary.

Do not apply this overlay to staging or production. It does not run migrations, provision credentials, install GameIntegration, select images, or prove a MatchFound-to-Voice admission flow. Those are separate ordered activation steps with exact-source and custody checks. Do not plan before the ConfigMap and referenced Secrets exist and have been checked by public metadata only.

The applicator renders the seven reviewed patch files against live JSON and records the exact source commit, namespace UID, object UIDs/resourceVersions, source patch digests, and desired-render digests in a mode-0600 plan. It refuses an existing plan path. Apply re-reads every object before the first mutation, verifies every source and render binding, and sends one atomic JSON Patch per object with UID and resourceVersion `test` operations. The fixed order publishes all three Services first, then rolls out Matchmaking, Chat, Voice, and Gateway. It admits only the four source-qualified image digests and the exact additive fields in this patch set. A conflict, drift, failed request, failed rollout, or uncertain response stops the sequence at that object; the successful prefix is preserved. Do not retry an old plan, roll back, recreate objects, or manually issue strategic patches after a partial result. Preserve the plan and object receipts for a fresh reviewed recovery decision.

The tool changes only listener Service ports and Deployment template environment, ports, mounts, and volumes. Existing objects and unrelated fields remain intact. Voice 9095 continues to trust `voice-media-ca` for the Gateway client; the MatchSquad CA is used for the separately reviewed MatchSquad TLS paths. After rollout, verify ordinary Deployment readiness, all named Service endpoints, Matchmaking's verified JWKS readiness, and the real application flow before accepting activation.
