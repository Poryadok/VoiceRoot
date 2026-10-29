#!/usr/bin/env python3
"""Build current Compose images with per-service content-addressed GHA caches.

Only build cache fields enter the disposable override. Resolved runtime config,
private fixture paths and proof state are never persisted or sent to the cache.
BuildKit checks the current Dockerfile/context/args against cached layer digests;
this helper never restores containers, volumes or test results.
"""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time


def cache_override(config, scope):
    if not re.fullmatch(r"[a-z0-9][a-z0-9_-]{0,95}", scope):
        raise ValueError("invalid build cache scope")
    services = {}
    for name, service in config["services"].items():
        if "build" not in service:
            continue
        if not re.fullmatch(r"[a-zA-Z0-9][a-zA-Z0-9_.-]*", name):
            raise ValueError("invalid Compose build service")
        entry = f"type=gha,version=2,scope={scope}-{name}"
        services[name] = {"build": {"cache_from": [entry],
                                    "cache_to": [entry + ",mode=max,ignore-error=true"]}}
    return {"services": services}


def build(compose, scope, builder, temp_dir=None, *, run=subprocess.run):
    if not builder:
        raise ValueError("explicit isolated Buildx builder required")
    resolved = run([*compose, "config", "--format", "json"],
                   stdout=subprocess.PIPE, text=True, check=False)
    if resolved.returncode:
        return resolved.returncode
    override = cache_override(json.loads(resolved.stdout), scope)
    if not override["services"]:
        raise ValueError("Compose configuration has no build services")
    fd, path = tempfile.mkstemp(prefix="voice-compose-build-cache-", suffix=".json",
                                dir=temp_dir)
    os.close(fd)
    try:
        Path(path).write_text(json.dumps(override), encoding="utf-8")
        started = time.monotonic()
        result = run([*compose, "-f", path, "build", "--builder", builder], check=False)
        print(f"compose_preparation_phase=image_build elapsed_seconds={time.monotonic()-started:.3f} "
              f"exit={result.returncode}", flush=True)
        return result.returncode
    finally:
        Path(path).unlink()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--scope", required=True)
    parser.add_argument("--builder", required=True)
    parser.add_argument("compose", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    command = args.compose
    if command[:1] == ["--"]:
        command = command[1:]
    if command[:2] != ["docker", "compose"]:
        parser.error("expected -- docker compose followed by Compose options")
    return build(command, args.scope, args.builder, os.environ.get("RUNNER_TEMP"))


if __name__ == "__main__":
    raise SystemExit(main())
