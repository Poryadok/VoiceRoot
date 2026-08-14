# ADR 004: Helm / GitOps deferred

## Status

Accepted (2026-08-13)

## Context

Staging/prod deploy uses bash scripts on the CI runner (`scripts/staging/*`, `scripts/prod/*`) with Kustomize-style YAML in `deploy/`. [ci.md](../todo/ci.md) lists Helm/Kustomize + GitOps as future work.

## Decision

**Continue script-driven ordered rollout** for v1. No Helm chart or ArgoCD/Flux repo in this monorepo.

- Traefik may be installed via Helm on the cluster host; Voice app manifests remain plain YAML.
- Selective deploy (`deploy-changed.sh`, `stack.lock`) stays the merge gate.

## Consequences

- Infra evolution (Helm/GitOps) tracked in infra repo or a later ADR when secrets and promotion flow are ready.
- Docs reference this ADR instead of implying GitOps is imminent.
