#!/usr/bin/env python3
"""Run BE-260 in a fresh, capped Compose project from a Windows Docker host.

Only orchestration runs on Windows. The Gateway Go test is compiled in Linux
and executed in a Linux Docker CLI container with the Docker socket attached.
"""

import hashlib
import json
import os
from datetime import datetime
from pathlib import Path
import secrets
import shutil
import socket
import subprocess
import sys
import time
import urllib.request


ROOT = Path(__file__).resolve().parents[2]
RUNS = ROOT / "tmp" / "be260-story-chain" / "runs"
PREFIX = "voice-story-ttl-delete-"
SERVICES = ("postgres", "story", "file", "gateway")


def call(args, *, env=None, timeout=120, capture=False):
    result = subprocess.run(args, cwd=ROOT, env=env, text=True,
                            stdout=subprocess.PIPE if capture else None,
                            stderr=subprocess.PIPE if capture else None,
                            timeout=timeout, check=False)
    if result.returncode:
        detail = (result.stderr or result.stdout or "").strip()[-4000:]
        raise RuntimeError(f"command failed ({result.returncode}): {args!r}\n{detail}")
    return result.stdout.strip() if capture else ""


def docker(*args, **kwargs):
    return call(["docker", *args], **kwargs)


def clean_head():
    head = call(["git", "rev-parse", "HEAD"], capture=True)
    tree = call(["git", "log", "-1", "--format=%T"], capture=True)
    if call(["git", "status", "--porcelain", "--untracked-files=all"], capture=True):
        raise RuntimeError("source tree is dirty; freeze a commit before live proof")
    return head, tree


def free_ports(start):
    for port in range(start, start + 17):
        with socket.socket() as probe:
            probe.bind(("127.0.0.1", port))


def owned_rows(project, started):
    ids = docker("ps", "-aq", "--filter", f"label=com.docker.compose.project={project}", capture=True).split()
    rows = {}
    for container in ids:
        row = json.loads(docker("inspect", container, capture=True))[0]
        labels = row["Config"].get("Labels") or {}
        if labels.get("com.docker.compose.project") != project:
            raise RuntimeError("foreign container label in private project")
        created = datetime.fromisoformat(row["Created"].replace("Z", "+00:00"))
        if created.timestamp() < started:
            raise RuntimeError("preexisting container in private project")
        service = labels.get("com.docker.compose.service")
        if not service or service in rows:
            raise RuntimeError("ambiguous private service identity")
        rows[service] = {"id": row["Id"], "image": row["Image"], "created": row["Created"]}
    return rows


def main():
    if os.name != "nt":
        raise RuntimeError("this launcher is for Windows Docker hosts")
    if os.environ.get("COMPOSE_PROJECT_NAME"):
        raise RuntimeError("caller-supplied Compose project is forbidden")
    head, tree = clean_head()
    proof_id = f"{int(time.time())}-{secrets.token_hex(5)}"
    project = PREFIX + proof_id
    started = time.time_ns()
    started_seconds = started / 1_000_000_000
    run_dir = RUNS / proof_id
    run_dir.mkdir(parents=True, exist_ok=False)
    fixture = run_dir / "phase0-fixture"
    env_file = run_dir / "compose.env"
    limits = run_dir / "limits.yml"
    binary = run_dir / "gateway.test"
    selected = os.environ.get("VOICE_STORY_TTL_DELETE_PORT_BASE")
    candidates = [int(selected)] if selected else [26000 + secrets.randbelow(30000) for _ in range(32)]
    base_port = None
    for candidate in candidates:
        if not 20000 <= candidate <= 65000:
            raise RuntimeError("private port base must be 20000..65000")
        try:
            free_ports(candidate)
        except OSError:
            if selected:
                raise
        else:
            base_port = candidate
            break
    if base_port is None:
        raise RuntimeError("no free private 17-port range")
    if docker("ps", "-aq", "--filter", f"label=com.docker.compose.project={project}", capture=True):
        raise RuntimeError("private Compose project collision")

    env = os.environ.copy()
    ports = ["POSTGRES_PORT", "REDIS_PORT", "NATS_PORT", "NATS_HTTP_PORT",
             "CLICKHOUSE_HTTP_PORT", "LIVEKIT_PORT", "LIVEKIT_RTC_TCP_PORT",
             "LIVEKIT_RTC_UDP_PORT", "CLAMAV_PORT", "MINIO_PORT",
             "MINIO_CONSOLE_PORT", "NOTIFICATION_DEBUG_PORT", "GATEWAY_PORT",
             "WEB_PORT", "DEVELOPER_PORTAL_PORT", "ADMIN_PORT", "VERIFICATION_STUB_PORT"]
    env.update({name: str(base_port + index) for index, name in enumerate(ports)})
    env.update(COMPOSE_PROJECT_NAME=project, STORY_TTL_DEV="30s",
               PHASE0_FIXTURE_DIR=fixture.as_posix(), COMPOSE_PARALLEL_LIMIT="2",
               MINIO_ROOT_USER="voice-minio", MINIO_ROOT_PASSWORD="voice-minio-dev",
               FILE_R2_ENDPOINT=f"http://host.docker.internal:{base_port+9}",
               FILE_R2_REGION="us-east-1", FILE_R2_ACCESS_KEY_ID="voice-minio",
               FILE_R2_SECRET_ACCESS_KEY="voice-minio-dev",
               FILE_R2_BUCKET=f"voice-story-ttl-{proof_id}")
    env_file.write_text(f"PHASE0_FIXTURE_DIR={fixture.as_posix()}\n", encoding="utf-8")
    compose = ["docker", "compose", "--env-file", str(env_file), "--project-name", project,
               "--project-directory", str(ROOT), "-f", str(ROOT / "docker-compose.yml"),
               "-f", str(ROOT / "docker-compose.voice-space-media.yml")]

    # Compile the frozen test before starting any Compose resource.
    docker("run", "--rm", "--cpus=2", "--memory=4g",
           "-v", f"{ROOT}:/workspace:ro", "-v", f"{run_dir}:/out",
           "-w", "/workspace/src/backend/gateway", "-e", "CGO_ENABLED=0",
           "-e", "GOFLAGS=-mod=readonly", "golang:1.26.5-bookworm",
           "go", "test", "-c", "-o", "/out/gateway.test", ".", timeout=900)
    test_sha = hashlib.sha256(binary.read_bytes()).hexdigest()
    call([sys.executable, str(ROOT / "scripts/dev/generate-phase0-fixture.py"), str(fixture)], timeout=900)
    services = call([*compose, "--profile", "app", "config", "--services"], env=env, capture=True).splitlines()
    lines = ["services:"]
    for service in services:
        if service in ("clickhouse", "auth", "clamav"):
            memory, cpu = "1536m", "1.0"
        elif service in ("postgres", "minio", "livekit"):
            memory, cpu = "768m", "0.75"
        else:
            memory, cpu = "384m", "0.5"
        lines.extend((f"  {service}:", f"    mem_limit: {memory}", f"    cpus: \"{cpu}\""))
    limits.write_text("\n".join(lines) + "\n", encoding="utf-8")
    compose.extend(("-f", str(limits), "--profile", "app"))
    call([*compose, "config", "--quiet"], env=env)
    config_sha = hashlib.sha256(call([*compose, "config"], env=env, capture=True).encode()).hexdigest()
    receipt = {"project": project, "source_head": head, "source_tree": tree,
               "test_binary_sha256": test_sha, "compose_config_sha256": config_sha,
               "started_unix_nano": started, "run_dir": str(run_dir), "containers": {}}
    receipt_path = run_dir / "receipt.json"
    receipt_path.write_text(json.dumps(receipt, indent=2), encoding="utf-8")
    launched = False
    try:
        launched = True
        call([*compose, "up", "-d", "--build"], env=env, timeout=1800)
        deadline = time.monotonic() + 240
        while time.monotonic() < deadline:
            try:
                with urllib.request.urlopen(f"http://127.0.0.1:{base_port+12}/health", timeout=3) as response:
                    if response.status == 200:
                        break
            except Exception:
                time.sleep(2)
        else:
            raise RuntimeError("private Gateway did not become healthy")
        rows = owned_rows(project, started_seconds)
        if not all(service in rows for service in SERVICES):
            raise RuntimeError("private Story/File/Gateway/Postgres containers missing")
        receipt["containers"] = rows
        receipt_path.write_text(json.dumps(receipt, indent=2), encoding="utf-8")
        test_env = {
            "COMPOSE_PROJECT_NAME": project, "STORY_TTL_DEV": "30s", "VOICE_RUN_LIVE_COMPOSE": "true",
            "VOICE_STORY_TTL_DELETE_LIVE_TEST": "true", "VOICE_STORY_TTL_DELETE_PROOF_ID": proof_id,
            "VOICE_STORY_TTL_DELETE_STARTED_UNIX_NANO": str(started),
            "VOICE_STORY_TTL_DELETE_SOURCE_HEAD": head,
            "VOICE_STORY_TTL_DELETE_TEST_SHA256": test_sha,
            "VOICE_STORY_TTL_DELETE_STORY_IMAGE_ID": rows["story"]["image"],
            "VOICE_STORY_TTL_DELETE_FILE_IMAGE_ID": rows["file"]["image"],
            "VOICE_STORY_TTL_DELETE_GATEWAY_IMAGE_ID": rows["gateway"]["image"],
            "VOICE_API_BASE_URL": f"http://127.0.0.1:{base_port+12}",
            "VOICE_AUTH_MAIL_STUB_URL": f"http://127.0.0.1:{base_port+16}",
            "MINIO_PORT": str(base_port+9),
        }
        args = ["run", "--rm", "--network", "host", "--cpus=2", "--memory=1g",
                "-v", "/var/run/docker.sock:/var/run/docker.sock:rw",
                "-v", f"{binary}:/gateway.test:ro"]
        for key, value in test_env.items():
            args.extend(("-e", f"{key}={value}"))
        docker(*args, "docker:27-cli", "/gateway.test", "-test.v",
               "-test.run=^TestComposeStoryTTLDeleteFile_live$", timeout=600)
        receipt["result"] = "PASS"
    finally:
        if launched:
            rows = owned_rows(project, started_seconds)
            if rows:
                receipt["containers"] = rows
            receipt_path.write_text(json.dumps(receipt, indent=2), encoding="utf-8")
            call([*compose, "down", "--remove-orphans", "--volumes"], env=env, timeout=300)
        if fixture.exists():
            if run_dir.resolve().parent != RUNS.resolve() or fixture.resolve().parent != run_dir.resolve():
                raise RuntimeError("refusing fixture cleanup outside private run directory")
            shutil.rmtree(fixture)
    print(json.dumps({"project": project, "source_head": head, "source_tree": tree,
                      "test_binary_sha256": test_sha, "result": receipt.get("result", "FAIL"),
                      "receipt": str(receipt_path)}))


if __name__ == "__main__":
    main()
