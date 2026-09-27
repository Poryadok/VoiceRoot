#!/usr/bin/env python3
"""Staging User/File S3 credentials must come from the MinIO Secret."""

from pathlib import Path
import re


root = Path(__file__).resolve().parents[2]
manifest = (root / "deploy/staging/services.yaml").read_text(encoding="utf-8")
documents = re.split(r"(?m)^---\s*$", manifest)
env_pattern = re.compile(
    r"(?m)^\s*- \{name: ([A-Z0-9_]+), valueFrom: "
    r"\{secretKeyRef: \{name: ([a-z0-9-]+), key: ([A-Z0-9_]+)\}\}\}$"
)


def deployment(name: str) -> dict[str, tuple[str, str]]:
    matches = [
        document
        for document in documents
        if re.search(r"(?m)^kind: Deployment$", document)
        and re.search(rf"(?m)^  name: {re.escape(name)}$", document)
    ]
    assert len(matches) == 1, f"expected one {name} Deployment"
    refs = env_pattern.findall(matches[0])
    assert len(refs) == len({env for env, _, _ in refs}), f"duplicate env in {name}"
    return {env: (secret, key) for env, secret, key in refs}


for service, prefix in (("voice-user", "USER"), ("voice-file", "FILE")):
    refs = deployment(service)
    expected = {
        f"{prefix}_R2_ACCESS_KEY_ID": ("voice-minio-credentials", "MINIO_ROOT_USER"),
        f"{prefix}_R2_SECRET_ACCESS_KEY": ("voice-minio-credentials", "MINIO_ROOT_PASSWORD"),
        f"{prefix}_R2_ENDPOINT": ("voice-app-secrets", f"{prefix}_R2_ENDPOINT"),
        f"{prefix}_R2_REGION": ("voice-app-secrets", f"{prefix}_R2_REGION"),
        f"{prefix}_R2_BUCKET": ("voice-app-secrets", f"{prefix}_R2_BUCKET"),
    }
    for env, source in expected.items():
        assert refs.get(env) == source, f"{service} {env} has the wrong Secret source"

example = (root / "deploy/staging/secret.example.yaml").read_text(encoding="utf-8")
app_secret = re.split(r"(?m)^---\s*$", example)[0]
for prefix in ("USER", "FILE"):
    for suffix in ("ACCESS_KEY_ID", "SECRET_ACCESS_KEY"):
        assert not re.search(
            rf"(?m)^  {prefix}_R2_{suffix}:", app_secret
        ), "staging app Secret template must not request duplicated MinIO credentials"

for script_name in (
    "ensure-app-secrets.sh",
    "patch-app-secrets-database-urls.sh",
):
    script = (root / "scripts/staging" / script_name).read_text(encoding="utf-8")
    for prefix in ("USER", "FILE"):
        for suffix in ("ACCESS_KEY_ID", "SECRET_ACCESS_KEY"):
            assert f"{prefix}_R2_{suffix}" not in script, (
                f"{script_name} still writes duplicated {prefix} MinIO credentials"
            )

print("Staging User/File MinIO env contract passed.")
