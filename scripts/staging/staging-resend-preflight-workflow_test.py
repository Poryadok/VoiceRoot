#!/usr/bin/env python3
"""Keep the mail secret preflight ahead of staging workflow mutations."""

from pathlib import Path
import importlib.util
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
validation = source.split("\n  validate-app-secret:\n", 1)[1].split("\n  mail-only:\n", 1)[0]
assert "runs-on: ubuntu-latest" in validation, "validation must use an isolated hosted runner"
assert "environment: staging" in validation, "validation must read the staging Environment secret"
assert "STAGING_APP_SECRETS_YAML_B64: ${{ secrets.STAGING_APP_SECRETS_YAML }}" in validation
assert "STAGING_SECRET_OFFLINE_PARSE: '1'" in validation
assert "python3 -m pip install PyYAML==6.0.3" in validation
assert "bash scripts/staging/preflight-resend-key.sh" in validation
assert "configure-kubectl-ci.sh" not in validation, "validation must not load cluster credentials"
assert "setup-kubectl" not in validation, "validation must parse YAML without kubectl"
assert "STAGING_KUBECONFIG" not in validation
assert "STAGING_SSH_PRIVATE_KEY" not in validation
assert "kubectl" not in validation
assert "set -x" not in validation
assert not re.search(r"kubectl (apply|patch|delete|rollout|create secret)", validation)

root = Path(__file__).resolve().parents[2]
checker_path = root / "scripts/staging/check-resend-key.py"
spec = importlib.util.spec_from_file_location("staging_secret_checker", checker_path)
assert spec and spec.loader
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)
referenced = set()
for path in (root / "deploy/staging").glob("*.yaml"):
    if path.name == "secret.example.yaml":
        continue
    manifest = path.read_text(encoding="utf-8")
    for match in re.finditer(r"secretKeyRef:\s*\{([^{}]*)\}", manifest):
        fields = match.group(1)
        if not re.search(r"\bname:\s*voice-app-secrets\b", fields):
            continue
        if re.search(r"\boptional:\s*true\b", fields):
            continue
        key = re.search(r"\bkey:\s*([A-Z][A-Z0-9_]*)\b", fields)
        assert key, f"unrecognized app Secret reference in {path.name}"
        referenced.add(key.group(1))
    for match in re.finditer(
        r"secretKeyRef:\s*\n\s*name:\s*voice-app-secrets\s*\n\s*key:\s*([A-Z][A-Z0-9_]*)"
        r"(?:\s*\n\s*optional:\s*(true|false))?",
        manifest,
    ):
        if match.group(2) != "true":
            referenced.add(match.group(1))
expected = referenced | {"AUTH_JWT_PRIVATE_KEY", "AUTH_RESEND_API_KEY"}
assert checker.REQUIRED_KEYS == expected, (
    f"app Secret preflight/ref mismatch: missing {sorted(expected - checker.REQUIRED_KEYS)}, "
    f"stale {sorted(checker.REQUIRED_KEYS - expected)}"
)

assert "mail_only:" in source, "mail-only dispatch input missing"
assert "inputs.mail_only != true" in source, "mail-only dispatch must skip other jobs"
assert "\n  mail-only:\n" in source, "mail-only job missing"
mail_job = source.split("\n  mail-only:\n", 1)[1]
assert "github.event_name == 'workflow_dispatch' && inputs.mail_only == true" in mail_job
assert "environment: staging" in mail_job
assert "STAGING_APP_SECRETS_YAML_B64: ${{ secrets.STAGING_APP_SECRETS_YAML }}" in mail_job
assert "bash scripts/staging/mail-only-resend.sh" in mail_job
assert "set -x" not in mail_job
assert "render-and-apply.sh" not in mail_job
assert "ensure-minio-credentials.sh" not in mail_job
print("Staging mail preflight workflow ordering passed.")
