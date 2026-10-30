#!/usr/bin/env python3
"""Opt-in real Docker proof of current Compose inputs and cache invalidation.

Uses its own Buildx builder and scratch image; no application or shared
Compose project is started. GHA transport is replaced with the local BuildKit
transport so this proof needs no GitHub credential.
"""
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import time
import uuid
import sys

sys.dont_write_bytecode = True


def command(*args):
    return subprocess.run(list(args), check=True, capture_output=True, text=True)


def main():
    spec = importlib.util.spec_from_file_location(
        "cache", Path(__file__).with_name("compose_build_cache.py"))
    cache = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(cache)
    identity = "marusya113-cache-" + uuid.uuid4().hex[:12]
    image = identity + ":proof"
    created = False
    try:
        command("docker", "buildx", "create", "--name", identity,
                "--driver", "docker-container")
        created = True
        with tempfile.TemporaryDirectory(prefix="voice-compose-cache-proof-") as temp:
            root = Path(temp)
            (root / "Dockerfile").write_text(
                'FROM scratch\nARG SOURCE\nLABEL proof.source=$SOURCE\nCOPY payload /payload\n',
                encoding="utf-8")
            source = root / "payload"
            source.write_text("first", encoding="utf-8")
            compose_file = root / "compose.json"
            compose_file.write_text(json.dumps({"services": {"proof": {
                "image": image, "build": {"context": str(root),
                                         "args": {"SOURCE": "current-input"}}}}}),
                encoding="utf-8")
            compose = ["docker", "compose", "-p", identity, "-f", str(compose_file)]
            old_cache = None
            ids = []
            for phase in ("cold", "warm", "source_mutation"):
                if phase != "cold":
                    command("docker", "buildx", "rm", identity)
                    command("docker", "buildx", "create", "--name", identity,
                            "--driver", "docker-container")
                if phase == "source_mutation":
                    source.write_text("second", encoding="utf-8")
                new_cache = root / ("cache-" + phase)
                def local_override(config, scope):
                    build = {"cache_to": [f"type=local,dest={new_cache},mode=max"]}
                    if old_cache:
                        build["cache_from"] = [f"type=local,src={old_cache}"]
                    return {"services": {"proof": {"build": build}}}
                cache.cache_override = local_override
                log = root / ("log-" + phase)
                def run(args, **kwargs):
                    if args[-3:] == ["config", "--format", "json"]:
                        return subprocess.run(args, **kwargs)
                    with log.open("w", encoding="utf-8") as output:
                        return subprocess.run(args, stdout=output, stderr=output,
                                              text=True, check=False)
                started = time.monotonic()
                status = cache.build(compose, "voice-proof", identity, temp, run=run)
                if status:
                    raise RuntimeError(log.read_text(encoding="utf-8"))
                elapsed = time.monotonic() - started
                inspected = json.loads(command("docker", "image", "inspect", image).stdout)[0]
                assert inspected["Config"]["Labels"]["proof.source"] == "current-input"
                current = inspected["Id"]
                ids.append(current)
                content = log.read_text(encoding="utf-8")
                if phase == "warm":
                    assert "importing cache manifest from local" in content, content
                    assert "CACHED" in content, content
                    assert ids[0] == ids[1], ids
                if phase == "source_mutation":
                    assert ids[1] != ids[2], ids
                print(json.dumps({"phase": phase, "elapsed_seconds": round(elapsed, 3),
                                  "image": current, "exit": status}), flush=True)
                old_cache = new_cache
    finally:
        subprocess.run(["docker", "image", "rm", image], capture_output=True, check=False)
        if created:
            command("docker", "buildx", "rm", identity)


if __name__ == "__main__":
    main()
