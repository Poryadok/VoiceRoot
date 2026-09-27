#!/usr/bin/env python3
"""Verify the probe Job manifest is rendered without expanding its pod script."""

import os
from pathlib import Path
import subprocess
import tempfile

import yaml


ROOT = Path(__file__).resolve().parents[2]
workflow = yaml.safe_load((ROOT / ".github/workflows/staging-deploy.yml").read_text(encoding="utf-8"))
steps = workflow["jobs"]["nats-search-jetstream-probe"]["steps"]
run = next(step["run"] for step in steps if step.get("name", "").startswith("Probe Search JetStream"))
lines = run.splitlines()
start = next(i for i, line in enumerate(lines) if "kctl create -f -" in line)
end = next(i for i in range(start + 1, len(lines)) if lines[i] == "EOF")
create = "\n".join(lines[start : end + 1])

with tempfile.TemporaryDirectory() as directory:
    capture = Path(directory) / "job.yaml"
    script = "\n".join(
        [
            "set -euo pipefail",
            'kctl() { [ "$1" = create ] && [ "$2" = -f ] && [ "$3" = - ]; cat > "$PROBE_CAPTURE"; }',
            'fail_probe() { printf "FAIL %s\\n" "$1" >&2; exit 1; }',
            create,
        ]
    )
    env = os.environ.copy()
    env.update(
        PROBE_JOB_NAME="voice-nats-search-probe-123-1",
        PROBE_CAPTURE=str(capture),
    )
    bash = Path(os.environ["PROGRAMFILES"]) / "Git/bin/bash.exe" if os.name == "nt" else "bash"
    result = subprocess.run([str(bash), "-c", script], env=env, text=True, capture_output=True)
    assert result.returncode == 0, f"stdout={result.stdout!r} stderr={result.stderr!r}"
    manifest = yaml.safe_load(capture.read_text(encoding="utf-8"))

assert manifest["metadata"]["name"] == "voice-nats-search-probe-123-1"
assert manifest["metadata"]["namespace"] == "voice-staging"
pod_script = manifest["spec"]["template"]["spec"]["containers"][0]["args"][0]
assert 'tmp="$(mktemp -d)"' in pod_script
assert 'trap \'rm -rf "$tmp"\' EXIT' in pod_script
assert "'$JS.API.CONSUMER.INFO.message_events.search-indexer-message-v1'" in pod_script
assert '"$response"' in pod_script
assert '"$category"' in pod_script
print("nats-search-probe-workflow: PASS")
