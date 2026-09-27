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

with tempfile.TemporaryDirectory() as directory:
    bin_dir = Path(directory)
    nats = bin_dir / "nats"
    nats.write_text(
        '#!/bin/sh\nprintf "%s" "$FAKE_NATS_RESPONSE"\n'
        'printf "%s" "$FAKE_NATS_STDERR" >&2\nexit "$FAKE_NATS_STATUS"\n',
        encoding="utf-8",
    )
    jq = bin_dir / "jq"
    jq.write_text(
        """#!/usr/bin/env python3
import json
from pathlib import Path
import sys

expression = " ".join(sys.argv[1:-1])
try:
    data = json.loads(Path(sys.argv[-1]).read_text(encoding="utf-8"))
except (OSError, ValueError):
    raise SystemExit(4)
error = data.get("error") or {}
if "durable_name" in expression:
    raise SystemExit(0 if not error and data.get("config", {}).get("durable_name") == "search-indexer-message-v1" else 1)
if ".error.description" in expression:
    print(error.get("description", ""))
elif ".error.err_code" in expression:
    print(error.get("err_code", 0))
elif ".error.code" in expression:
    print(error.get("code", 0))
else:
    print(json.dumps(data))
""",
        encoding="utf-8",
    )
    nats.chmod(0o755)
    jq.chmod(0o755)
    sh = Path(os.environ["PROGRAMFILES"]) / "Git/bin/sh.exe" if os.name == "nt" else "sh"

    def probe(response: str, stderr: str = "", status: int = 0) -> str:
        env = os.environ.copy()
        env.update(
            FAKE_NATS_RESPONSE=response,
            FAKE_NATS_STDERR=stderr,
            FAKE_NATS_STATUS=str(status),
            PATH=f"{bin_dir}{os.pathsep}{env['PATH']}",
        )
        result = subprocess.run([str(sh), "-ceu", pod_script], env=env, text=True, capture_output=True)
        assert "secret-sentinel" not in result.stdout + result.stderr
        return result.stdout.strip()

    assert probe('{"config":{"durable_name":"search-indexer-message-v1"}}') == "NATS_SEARCH_JS_INFO=PASS"
    assert probe('{"error":{"code":403,"err_code":10037,"description":"authorization violation secret-sentinel"}}') == (
        "NATS_SEARCH_JS_INFO=FAIL PERMISSION_DENIED CODE_403 ERR_10037"
    )
    assert probe("", "authentication failed secret-sentinel", 1) == (
        "NATS_SEARCH_JS_INFO=FAIL AUTHENTICATION_FAILED CODE_0 ERR_0"
    )
    assert probe("not-json secret-sentinel") == "NATS_SEARCH_JS_INFO=FAIL INVALID_RESPONSE CODE_0 ERR_0"
    assert probe('{"error":{"code":500,"err_code":123,"description":"secret-sentinel"}}') == (
        "NATS_SEARCH_JS_INFO=FAIL OTHER CODE_500 ERR_123"
    )

outer_start = next(i for i in range(end + 1, len(lines)) if lines[i] == 'case "$result" in')
outer_end = next(i for i in range(outer_start + 1, len(lines)) if lines[i] == "esac")
outer_case = "\n".join(lines[outer_start : outer_end + 1])
outer_script = "\n".join(
    [
        "set -euo pipefail",
        'fail_probe() { printf "STAGE_%s\\n" "$1"; exit 1; }',
        "conditions=Failed",
        'result="$1"',
        outer_case,
    ]
)
valid = subprocess.run(
    [str(bash), "-c", outer_script, "--", "NATS_SEARCH_JS_INFO=FAIL OTHER CODE_500 ERR_123"],
    text=True,
    capture_output=True,
)
assert valid.returncode == 1 and valid.stdout.strip() == "NATS_SEARCH_JS_INFO=FAIL OTHER CODE_500 ERR_123"
invalid = subprocess.run(
    [str(bash), "-c", outer_script, "--", "NATS_SEARCH_JS_INFO=FAIL OTHER CODE_x ERR_0 secret-sentinel"],
    text=True,
    capture_output=True,
)
assert invalid.returncode == 1 and invalid.stdout.strip() == "STAGE_JOB_RESULT"
print("nats-search-probe-workflow: PASS")
