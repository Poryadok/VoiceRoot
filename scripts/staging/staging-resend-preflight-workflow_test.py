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

# Normal release uses root-captured canonical authority, not runner-supplied
# plaintext Secret replacement. Preserve mail validation before every mutation.
prepare = "Prepare cold backup and isolated proof using installed root bridge"
apply = "Apply exact frozen target while all consumers remain paused"
assert prepare in named_steps and apply in named_steps
assert source.index(named_steps[prepare]) < source.index(named_steps[apply])
assert "set -x" not in named_steps[prepare]

import base64, copy, sys
root = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(root / "scripts/staging/nats-known-baseline"))
sys.path.insert(0, str(root / "scripts/staging/nats-rollout-preservation"))
import source_plan, source_plan_test, nonnats_plan
transaction = (root / "scripts/staging/nats-rollout-preservation/transaction.py").read_text()
assert transaction.index("nonnats_runtime.preflight(") < transaction.index("stage.fence()")
for mode in ("full", "app-only", "images-only"):
    kube, params = source_plan_test.fixture() if mode == "images-only" else source_plan_test.SourceTests().canonical_fixture(mode)
    original = copy.deepcopy(kube.objects)
    plan = source_plan.compile_plan(root, params, source_plan_test.decoder, kube)
    mail = next(row for row in plan["checks"] if row["id"] == "auth-mail")
    assert any(obj["name"] == "voice-app-secrets" and obj["desired"]["predicates"]["auth_mail"] for obj in mail["objects"])
    assert kube.objects == original, "mail preflight must not mutate Secret authority"
    kube.objects[("Secret", "voice-app-secrets")]["data"]["AUTH_RESEND_FROM"] = base64.b64encode(b"invalid-sender").decode()
    try:
        source_plan.compile_plan(root, params, source_plan_test.decoder, kube)
    except nonnats_plan.PlanError:
        pass
    else:
        raise AssertionError("invalid live mail authority passed canonical preflight")

# Unsupported observability changes are rejected explicitly before the fence;
# neither manual nor reusable deployment silently drops requested reconciliation.
for mode in ("full", "app-only"):
    kube, params = source_plan_test.SourceTests().canonical_fixture(mode)
    params["apply_observability"] = True
    try:
        source_plan.compile_plan(root, params, source_plan_test.decoder, kube)
    except nonnats_plan.PlanError as error:
        assert "unsupported_enabled_observability" in str(error)
    else:
        raise AssertionError("requested unsupported observability was silently accepted")

assert "validate_app_secret_only:" in source, "read-only dispatch input missing"
assert "inputs.validate_app_secret_only != true" in source, "validation dispatch must skip deploy job"
assert "\n  validate-app-secret:\n" in source, "read-only validation job missing"
validation = source.split("\n  validate-app-secret:\n", 1)[1].split("\n  mail-only:\n", 1)[0]
assert "runs-on: ubuntu-latest" in validation, "validation must use an isolated hosted runner"
assert "environment: staging" in validation, "validation must read the staging Environment secret"
assert "STAGING_APP_SECRETS_YAML_B64: ${{ secrets.STAGING_APP_SECRETS_YAML }}" in validation
assert "STAGING_SECRET_OFFLINE_PARSE: '1'" in validation
assert "python3 -m pip install PyYAML==6.0.3" in validation
assert "mail-only-patch.py" in validation and "--yaml-check" in validation
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
assert "uses: actions/checkout@" not in mail_job
assert "STAGING_SOURCE_SHA: ${{ github.sha }}" in mail_job
assert "run: *download_staging_source" in mail_job
assert "STAGING_APP_SECRETS_YAML_B64: ${{ secrets.STAGING_APP_SECRETS_YAML }}" in mail_job
assert "bash scripts/staging/mail-only-resend.sh" in mail_job
assert "set -x" not in mail_job
assert "render-and-apply.sh" not in mail_job
assert "ensure-minio-credentials.sh" not in mail_job
print("Staging mail preflight workflow ordering passed.")
