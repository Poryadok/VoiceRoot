# Production Kubernetes manifests (skeleton)

Mirror [`deploy/staging/`](../staging/) when ready for first production cutover. Current skeleton:

| File | Status |
|------|--------|
| `namespace.yaml` | `voice-prod` |
| `services.yaml` | Bot service only (bots (docs/features/bots.md) skeleton) |
| `configmap-app.yaml` | Not yet — copy from staging + prod FQDNs |
| `gateway-deployment.yaml` | Not yet |
| `infra.yaml` | Not yet |

## Deploy

Workflow **[Production deploy](../../.github/workflows/prod-deploy.yml)** — manual only, `environment: production` (approval), **required** `image_tag` (git SHA or semver; no `latest` default).

GitHub setup:

| Item | Purpose |
|------|---------|
| Environment `production` | Required reviewers / wait timer |
| Secret `PROD_KUBECONFIG` | base64 kubeconfig |
| Variables `VOICE_PROD_K8S_NAMESPACE`, `VOICE_PROD_GATEWAY_INGRESS_HOST`, `VOICE_PROD_DEVELOPER_PORTAL_INGRESS_HOST`, `VOICE_PROD_IMAGE_PULL_SECRET` | Prod FQDNs and namespace |

```bash
export VOICE_IMAGE_REGISTRY=ghcr.io/your-org/voiceroot
export VOICE_IMAGE_TAG=v1.0.0   # or git SHA
export VOICE_K8S_NAMESPACE=voice-prod
bash scripts/prod/render-and-apply-prod.sh
```

See [DEPLOYMENT.md](../../docs/DEPLOYMENT.md) for tag policy and approval flow.
