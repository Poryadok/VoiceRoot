#!/usr/bin/env python3
"""Keep the mail secret preflight ahead of staging workflow mutations."""

from pathlib import Path
import re


workflow = Path(__file__).resolve().parents[2] / ".github/workflows/staging-deploy.yml"
source = workflow.read_text(encoding="utf-8")
steps = re.split(r"(?=^      - name: )", source, flags=re.MULTILINE)
named_steps = {}
for step in steps:
    match = re.match(r"^      - name: ([^\r\n]+)", step)
    if match:
        named_steps[match.group(1)] = step

preflight = "Preflight staging Auth email credential"
mutations = (
    "Ensure MinIO credentials",
    "Restore principal secrets",
    "Restore NATS activation secrets",
    "Ensure User search cursor key",
    "Apply staging manifests",
)
assert preflight in named_steps, "staging mail credential preflight step missing"
assert all(name in named_steps for name in mutations), "expected staging mutation step missing"
assert all(
    source.index(named_steps[preflight]) < source.index(named_steps[name])
    for name in mutations
), "staging mail credential preflight must precede every mutation step"

step = named_steps[preflight]
assert "STAGING_APP_SECRETS_YAML_B64: ${{ secrets.STAGING_APP_SECRETS_YAML }}" in step
assert "VOICE_K8S_NAMESPACE: ${{ vars.VOICE_K8S_NAMESPACE }}" in step
assert "run: bash scripts/staging/preflight-resend-key.sh" in step
assert "set -x" not in step
assert "echo" not in step
assert "STAGING_APP_SECRETS_YAML_B64: ${{ secrets.STAGING_APP_SECRETS_YAML }}" in named_steps[
    "Apply staging manifests"
], "preflight and apply must receive the same secure YAML artifact"

assert "validate_app_secret_only:" in source, "read-only dispatch input missing"
assert "inputs.validate_app_secret_only != true" in source, "validation dispatch must skip deploy job"
assert "\n  validate-app-secret:\n" in source, "read-only validation job missing"
validation = source.split("\n  validate-app-secret:\n", 1)[1]
assert "runs-on: ubuntu-latest" in validation, "validation must use an isolated hosted runner"
assert "environment: staging" in validation, "validation must read the staging Environment secret"
assert "STAGING_APP_SECRETS_YAML_B64: ${{ secrets.STAGING_APP_SECRETS_YAML }}" in validation
assert "bash scripts/staging/preflight-resend-key.sh" in validation
assert "configure-kubectl-ci.sh" not in validation, "validation must not load cluster credentials"
assert not re.search(r"kubectl (apply|patch|delete|rollout|create secret)", validation)
print("Staging mail preflight workflow ordering passed.")
