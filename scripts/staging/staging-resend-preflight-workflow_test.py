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
print("Staging mail preflight workflow ordering passed.")
