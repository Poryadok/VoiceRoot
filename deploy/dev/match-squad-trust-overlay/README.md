# DEV MatchSquad trust overlay

This overlay belongs only to the `voice-dev` namespace and depends on the MatchSquad runtime contract from PR #667. The issuer first creates six immutable Secrets and the public-only `voice-match-squad-ca` ConfigMap. After the PR #667 images and source are verified in DEV under their separate activation grant, apply these four strategic patches to mount the new CA at `/etc/voice/match-squad/ca/ca.crt`.

```sh
kubectl --namespace voice-dev patch deployment voice-matchmaking --type=strategic --patch-file deploy/dev/match-squad-trust-overlay/voice-matchmaking.patch.yaml
kubectl --namespace voice-dev patch deployment voice-chat --type=strategic --patch-file deploy/dev/match-squad-trust-overlay/voice-chat.patch.yaml
kubectl --namespace voice-dev patch deployment voice-voice --type=strategic --patch-file deploy/dev/match-squad-trust-overlay/voice-voice.patch.yaml
kubectl --namespace voice-dev patch deployment voice-gateway --type=strategic --patch-file deploy/dev/match-squad-trust-overlay/voice-gateway.patch.yaml
```

The patches change Matchmaking's outbound Chat/Voice CA, Chat 9092 and Voice 9094's Matchmaking client CA, and Gateway's outbound Voice 9095 CA. The Voice 9095 member client-auth CA remains `voice-media-ca`; JWKS CA mounts remain unchanged. The application JWT principal, audience, RPC/request binding, and replay checks remain the authorization boundary.

Do not apply these patches to staging or production. Do not apply them before the new ConfigMap exists and the MatchSquad runtime image/source is proven in DEV. A partial patch sequence is resumed by applying the remaining exact patch files; do not edit the old media-PKI journal or issuer volume to recover it.
