#!/usr/bin/env python3
"""Verify the probe Job manifest is rendered without expanding its pod script."""

import os
import json
from pathlib import Path
import shutil
import subprocess
import tempfile

import yaml


ROOT = Path(__file__).resolve().parents[2]
workflow = yaml.safe_load((ROOT / ".github/workflows/staging-deploy.yml").read_text(encoding="utf-8"))
steps = workflow["jobs"]["nats-search-jetstream-probe"]["steps"]
run = next(step["run"] for step in steps if step.get("name", "").startswith("Probe Search JetStream"))
assert 'capture("^voice-nats-service-credentials(?<g>-r[0-9]{8}[a-z0-9]{0,8})$")' in run
assert '"voice-nats-hub-tls" + $g' in run
lines = run.splitlines()
deployment_probe_start = next(i for i, line in enumerate(lines) if "kctl get deployment voice-search" in line)
jq_start = next(i for i in range(deployment_probe_start, len(lines)) if "jq -e '" in lines[i])
jq_end = next(i for i in range(jq_start + 1, len(lines)) if "|| fail_probe SEARCH_LEAF_DEPLOYMENT" in lines[i])
jq_filter = "\n".join(line.strip() for line in lines[jq_start + 1 : jq_end])

# GitHub's Ubuntu runner includes jq. Exercise the exact workflow predicate
# with legacy refs, a matching generated pair, and a mismatched pair.
jq_binary = shutil.which("jq")
if jq_binary:
    leaf_pod = {
        "spec": {
            "template": {
                "spec": {
                    "containers": [
                        {
                            "name": "nats-leaf",
                            "image": "nats:2.12.12-alpine@sha256:2ca98656a279b2d88cfdf2b8c3f0d5d7f3941ae9dc2ab12ebaa92d83e0f4ccdb",
                            "args": ["-c", "/etc/nats/leaf.conf"],
                            "env": [{"name": "NATS_CREDS", "value": "/var/run/nats/creds/search.creds"}],
                            "volumeMounts": [
                                {"mountPath": "/var/run/nats/creds/search.creds", "subPath": "search.creds", "readOnly": True},
                                {"mountPath": "/etc/nats/leaf.conf", "subPath": "leaf.conf", "readOnly": True},
                                {"mountPath": "/etc/nats/tls/ca.crt", "subPath": "ca.crt", "readOnly": True},
                            ],
                        }
                    ],
                    "volumes": [
                        {"name": "nats-service-creds", "secret": {"secretName": "voice-nats-service-credentials", "items": [{"key": "search.creds", "path": "search.creds"}]}},
                        {"name": "nats-hub-tls", "secret": {"secretName": "voice-nats-hub-tls", "items": [{"key": "ca.crt", "path": "ca.crt"}]}},
                        {"name": "nats-leaf-config", "configMap": {"name": "voice-nats-leaf-config"}},
                    ],
                }
            }
        }
    }

    def check_deployment_refs(service_secret: str, tls_secret: str) -> bool:
        pod = json.loads(json.dumps(leaf_pod))
        volumes = pod["spec"]["template"]["spec"]["volumes"]
        volumes[0]["secret"]["secretName"] = service_secret
        volumes[1]["secret"]["secretName"] = tls_secret
        result = subprocess.run(
            [jq_binary, "-e", jq_filter], input=json.dumps(pod), text=True, capture_output=True
        )
        return result.returncode == 0

    assert check_deployment_refs("voice-nats-service-credentials", "voice-nats-hub-tls")
    assert check_deployment_refs("voice-nats-service-credentials-r20260930a3", "voice-nats-hub-tls-r20260930a3")
    assert not check_deployment_refs("voice-nats-service-credentials-r20260930a3", "voice-nats-hub-tls-r20260929a1")

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
    probe_script = pod_script.replace("/var/run/nats/creds/search.creds", "./credentials")
    assert probe_script != pod_script
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
    for name, status_var in (("getent", "FAKE_DNS_STATUS"), ("nc", "FAKE_TCP_STATUS")):
        command = bin_dir / name
        command.write_text(f'#!/bin/sh\nexit "${status_var}"\n', encoding="utf-8")
        command.chmod(0o755)
    credentials = bin_dir / "credentials"
    sh = Path(os.environ["PROGRAMFILES"]) / "Git/bin/sh.exe" if os.name == "nt" else "sh"

    def probe(response: str, stderr: str = "", status: int = 0, dns_status: int = 0, tcp_status: int = 0, creds_present: bool = True) -> str:
        if creds_present:
            credentials.touch()
        else:
            credentials.unlink(missing_ok=True)
        env = os.environ.copy()
        env.update(
            FAKE_NATS_RESPONSE=response,
            FAKE_NATS_STDERR=stderr,
            FAKE_NATS_STATUS=str(status),
            FAKE_DNS_STATUS=str(dns_status),
            FAKE_TCP_STATUS=str(tcp_status),
            PATH=f"{bin_dir}{os.pathsep}{env['PATH']}",
        )
        result = subprocess.run([str(sh), "-ceu", probe_script], cwd=bin_dir, env=env, text=True, capture_output=True)
        assert "secret-sentinel" not in result.stdout + result.stderr
        return result.stdout.strip()

    assert probe("", creds_present=False) == "NATS_SEARCH_JS_INFO=FAIL CREDS_UNREADABLE CODE_0 ERR_0"
    assert probe("", dns_status=1) == "NATS_SEARCH_JS_INFO=FAIL DNS_FAILURE CODE_0 ERR_0"
    assert probe("", tcp_status=1) == "NATS_SEARCH_JS_INFO=FAIL TCP_FAILURE CODE_0 ERR_0"
    assert probe('{"config":{"durable_name":"search-indexer-message-v1"}}') == "NATS_SEARCH_JS_INFO=PASS"
    assert probe('{"error":{"code":403,"err_code":10037,"description":"authorization violation secret-sentinel"}}') == (
        "NATS_SEARCH_JS_INFO=FAIL PERMISSION_DENIED CODE_403 ERR_10037"
    )
    assert probe("", "authentication failed secret-sentinel", 1) == (
        "NATS_SEARCH_JS_INFO=FAIL AUTHENTICATION_FAILED CODE_0 ERR_0"
    )
    assert probe("", "no servers available secret-sentinel", 1) == (
        "NATS_SEARCH_JS_INFO=FAIL CONNECT_ERROR CODE_0 ERR_0"
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
