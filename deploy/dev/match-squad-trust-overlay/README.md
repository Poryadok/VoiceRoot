# DEV MatchSquad trust overlay

This overlay belongs only to the `voice-dev` namespace and binds the MatchSquad runtime contract from PR #667 to the DEV host and PKI issued for this flow. It is a strategic-merge patch set for the existing DEV Deployments and Services; it does not copy the staging manifests or change the canonical staging configuration. The issuer creates the MatchSquad Secrets and public-only `voice-match-squad-ca` ConfigMap separately. Apply this overlay only after the exact source images, namespace, migrations and credentials have passed the separately authorized DEV activation preflight.

```sh
python3 -m unittest discover -s scripts/dev -p 'match_squad_runtime_overlay_test.py'
kubectl create --dry-run=client -f deploy/dev/match-squad-trust-overlay/ -o yaml >/dev/null

kubectl --namespace voice-dev patch deployment voice-matchmaking --type=strategic --patch-file deploy/dev/match-squad-trust-overlay/voice-matchmaking.runtime.patch.yaml
kubectl --namespace voice-dev patch deployment voice-chat --type=strategic --patch-file deploy/dev/match-squad-trust-overlay/voice-chat.runtime.patch.yaml
kubectl --namespace voice-dev patch deployment voice-voice --type=strategic --patch-file deploy/dev/match-squad-trust-overlay/voice-voice.runtime.patch.yaml
kubectl --namespace voice-dev patch deployment voice-gateway --type=strategic --patch-file deploy/dev/match-squad-trust-overlay/voice-gateway.patch.yaml
kubectl --namespace voice-dev patch service voice-matchmaking --type=strategic --patch-file deploy/dev/match-squad-trust-overlay/voice-matchmaking.service.patch.yaml
kubectl --namespace voice-dev patch service voice-chat --type=strategic --patch-file deploy/dev/match-squad-trust-overlay/voice-chat.service.patch.yaml
kubectl --namespace voice-dev patch service voice-voice --type=strategic --patch-file deploy/dev/match-squad-trust-overlay/voice-voice.service.patch.yaml
```

The Deployment patches provide the existing MatchSquad listeners and bind their `9092`, `9094`, `9095`, and Matchmaking JWKS `8443` ports through the corresponding Services. They add only the existing contract's Secret references and read-only mounts. Matchmaking uses `https://voice-matchmaking.voice-dev.svc:8443/internal/v1/principal/jwks.json`; Gateway's public JWKS host is `api-dev.comrade.click`. Matchmaking outbound Chat/Voice trust, Chat 9092 and Voice 9094's Matchmaking client trust, and Gateway's outbound Voice 9095 trust use `voice-match-squad-ca`. Voice 9095 still authenticates the Gateway media client with `voice-media-ca`; Gateway keeps its existing media client certificate and key. The Matchmaking JWKS trust bundle and Voice member's outbound JWKS trust remain unchanged. The application JWT principal, audience, RPC/request binding, and replay checks remain the authorization boundary.

Do not apply these patches to staging or production. This patch set does not run database migrations, provision application credentials, install GameIntegration, select images, or prove a MatchFound-to-Voice admission flow. Those are separate ordered activation steps with their own exact-source and custody checks. Do not patch before the ConfigMap and all referenced Secrets exist and have been checked by public metadata only. A partial sequence is resumed by inspecting resource versions and applying only the remaining exact files; never reapply migrations or alter the issuer journal/volume to recover it. After rollout, verify ordinary Deployment readiness, all named Service endpoints, Matchmaking's verified JWKS readiness, and the real application flow before accepting activation.
