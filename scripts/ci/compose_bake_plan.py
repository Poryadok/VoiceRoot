#!/usr/bin/env python3
"""Prepare a fresh, validated Bake plan for the disposable compose-e2e stack.

Only BuildKit layers are cached. The plan is regenerated from the current
checkout; the caller loads every image before running all existing smoke tests.

Docker documents native Compose input and per-image GHA scopes at:
https://docs.docker.com/build/bake/compose-file/
https://docs.docker.com/build/cache/backends/gha/
The official bake-action supplies GHA runtime authentication. Export is optional;
its transfer timeout is 30s per target, plus cache preparation time. Cache import
never selects a prebuilt image.
"""
import argparse
import copy
import json
from pathlib import Path
import re
import subprocess
import tempfile

FILES = ("docker-compose.yml", "docker-compose.voice-space-media.yml",
         "docker-compose.phase0.yml")
BUILD_KEYS = {"context", "dockerfile", "args", "target", "platforms"}
TARGET_KEYS = {"context", "dockerfile", "args", "target", "platforms", "tags"}
ROOT = Path(__file__).resolve().parents[2]


def frontend_reader(name, target):
    """Share PR frontend layers only for the exact matching local build recipe."""
    recipes = {
        "web": ("frontend", {"VOICE_API_BASE_URL": "http://127.0.0.1:18080",
                              "VOICE_LIVEKIT_URL": "ws://127.0.0.1:7880"}),
        "developer-portal": ("developer-portal", {
            "VITE_VOICE_API_BASE": "http://127.0.0.1:18080",
            "VITE_OAUTH_CLIENT_ID": "voice-developer-portal",
            "VITE_OAUTH_DISABLED": "false"}),
    }
    if name not in recipes:
        return None
    directory, args = recipes[name]
    if (Path(target["context"]).resolve() == ROOT / "src" / directory
            and target["dockerfile"] == "Dockerfile"
            and target.get("args") == args
            and not target.get("platforms") and not target.get("target")):
        return f"type=gha,scope={name}"
    return None


def build_document(config):
    """Strip runtime credentials and retain only supported current build inputs."""
    project = config.get("name", "")
    if not re.fullmatch(r"[a-z0-9][a-z0-9_-]*", project):
        raise ValueError("project_identity")
    services = {}
    for name, service in config["services"].items():
        if "build" not in service:
            continue
        if service.get("pull_policy", "missing") not in ("missing", "if_not_present", "never", "build"):
            raise ValueError("unsupported_pull_policy")
        if not re.fullmatch(r"[a-z0-9][a-z0-9_-]*", name):
            raise ValueError("target_identity")
        build = copy.deepcopy(service["build"])
        if not isinstance(build, dict) or set(build) - BUILD_KEYS:
            raise ValueError("unsupported_build")
        if not build.get("context") or not Path(build["context"]).is_absolute():
            raise ValueError("context_identity")
        build.setdefault("dockerfile", "Dockerfile")
        platforms = build.get("platforms", [])
        platform = service.get("platform")
        if platform:
            if platforms and platforms != [platform]:
                raise ValueError("platform_identity")
            build["platforms"] = [platform]
        if len(build.get("platforms", [])) > 1:
            raise ValueError("unsupported_multi_platform_load")
        services[name] = {"build": build,
                          "image": service.get("image") or f"{project}-{name}"}
    if not services:
        raise ValueError("missing_targets")
    tags = [service["image"] for service in services.values()]
    if len(set(tags)) != len(tags):
        raise ValueError("duplicate_image")
    return {"services": services}


def configure_plan(document, native):
    """Reject dropped/changed inputs before adding isolated layer cache scopes."""
    services = document["services"]
    targets = native.get("target", {})
    if set(targets) != set(services):
        raise ValueError("target_inventory")
    result = {"group": {"default": {"targets": sorted(services)}}, "target": {}}
    for name, service in services.items():
        target = copy.deepcopy(targets[name])
        if set(target) - TARGET_KEYS:
            raise ValueError("unsupported_native_target")
        expected = copy.deepcopy(service["build"])
        expected["tags"] = [service["image"]]
        # Native Bake omits empty optional fields.
        expected = {key: value for key, value in expected.items() if value != {} and value != []}
        if target != expected:
            raise ValueError("build_identity")
        scope = f"compose-e2e-{name}"
        target["cache-from"] = [f"type=gha,scope={scope}"]
        reader = frontend_reader(name, target)
        if reader:
            target["cache-from"].append(reader)
        dockerfile = Path(target["dockerfile"]).as_posix()
        if dockerfile in (f"{name}/Dockerfile", f"src/backend/{name}/Dockerfile"):
            target["cache-from"].append(f"type=gha,scope=backend-{name}")
        target["cache-to"] = [f"type=gha,scope={scope},mode=max,timeout=30s,ignore-error=true"]
        result["target"][name] = target
    return result


def docker_json(args):
    # Compose config may contain credentials; never echo its output or errors.
    result = subprocess.run(["docker", *args], capture_output=True, timeout=90)
    if result.returncode:
        raise ValueError("docker_plan_command")
    return json.loads(result.stdout)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    # A failed preparation must not leave an older plan eligible for the action.
    args.output.unlink(missing_ok=True)
    compose_files = [item for path in FILES for item in ("-f", path)]
    try:
        config = docker_json(["compose", *compose_files, "--profile", "app",
                              "config", "--format", "json"])
        document = build_document(config)
        images = subprocess.run(["docker", "compose", *compose_files, "--profile", "app",
                                 "config", "--images"], capture_output=True, text=True, timeout=90)
        if images.returncode or not {s["image"] for s in document["services"].values()} <= set(images.stdout.splitlines()):
            raise ValueError("compose_image_identity")
        # JSON is YAML. A build-only Compose file avoids passing runtime secrets
        # to Bake and avoids version-specific multi-file network merge behavior.
        with tempfile.TemporaryDirectory(prefix="voice-compose-bake-") as temp:
            source = Path(temp) / "compose.yaml"
            source.write_text(json.dumps(document), encoding="utf-8")
            source.chmod(0o600)
            native = docker_json(["buildx", "bake", "-f", str(source), "--print"])
        plan = configure_plan(document, native)
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps(plan, indent=2) + "\n", encoding="utf-8")
        args.output.chmod(0o600)
    except (ValueError, KeyError, TypeError, OSError, subprocess.TimeoutExpired):
        raise SystemExit("COMPOSE_BAKE_PLAN=FAIL")
    print(f"COMPOSE_BAKE_PLAN=PASS targets={len(plan['target'])}")


if __name__ == "__main__":
    main()
