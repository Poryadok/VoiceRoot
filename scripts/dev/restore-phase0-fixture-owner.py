#!/usr/bin/env python3
"""Restore only enumerated Phase0 fixture paths chowned for nonroot services."""

import os
import subprocess
import sys
from pathlib import Path


def supported_root(root: Path, runner_temp: Path | None, declared_root: Path | None) -> bool:
    if declared_root is None or declared_root != root:
        return False
    if root.name == "phase0-fixture":
        parents = (
            "voice-a1-flutter-profile-handoff.",
            "voice-a1-multi-account-proof.",
            "voice-file-restart-proof.",
        )
        return any(root.parent.name.startswith(prefix)
                   and len(root.parent.name) == len(prefix) + 8
                   and root.parent.name[len(prefix):].isalnum()
                   for prefix in parents)
    if root.name == "credentials":
        return root.parent.name == "phase0-t31-ci-fixture"
    if root.name == "voice-space-media-phase0" and runner_temp is not None:
        return root.parent == runner_temp
    return False


def restore(destination: Path) -> None:
    if os.name != "posix":
        raise RuntimeError("Phase0 fixture ownership can only be restored on POSIX")
    if destination.is_symlink():
        raise RuntimeError("fixture destination must not be a symlink")
    root = destination.resolve(strict=True)
    fixture_value = os.environ.get("PHASE0_FIXTURE_DIR")
    declared_root = Path(fixture_value).resolve(strict=True) if fixture_value else None
    runner_temp_value = os.environ.get("RUNNER_TEMP")
    runner_temp = Path(runner_temp_value).resolve(strict=True) if runner_temp_value else None
    if not supported_root(root, runner_temp, declared_root):
        raise RuntimeError("fixture destination is outside the supported disposable paths")
    signers = tuple(root / name for name in ("matchmaking", "chat", "space"))
    if any(path.is_symlink() for path in (root / "tls", root / "ca", *signers)):
        raise RuntimeError("fixture directories must not be symlinks")
    owned = [directory / name for directory in signers for name in ("current.pem", "next.pem")]
    owned += [root / "tls/chat-client.crt", root / "tls/chat-client.key",
             root / "tls/chat-jwks.crt", root / "tls/chat-jwks.key",
             root / "tls/chat-match-squad-grpc.crt", root / "tls/chat-match-squad-grpc.key",
             root / "tls/voice-match-squad-grpc.crt", root / "tls/voice-match-squad-grpc.key",
             root / "tls/voice-match-squad-member-grpc.crt",
             root / "tls/voice-match-squad-member-grpc.key",
             root / "tls/matchmaking-client.crt", root / "tls/matchmaking-client.key",
             root / "tls/matchmaking-complete-grpc.crt", root / "tls/matchmaking-complete-grpc.key",
             root / "tls/matchmaking-jwks.crt", root / "tls/matchmaking-jwks.key",
             root / "tls/space-lifecycle.crt", root / "tls/space-lifecycle.key",
             root / "tls/space-client.crt", root / "tls/space-client.key",
             root / "ca/ca.crt"]
    uid, gid = os.getuid(), os.getgid()
    # Restore traversal before inspecting 0700 service-owned signer children.
    # Never recurse: only the two generated rotation keys belong to cleanup.
    for directory in signers:
        if directory.is_dir():
            if os.geteuid() == 0:
                os.chown(directory, uid, gid)
            else:
                result = subprocess.run(["sudo", "-n", "chown", f"{uid}:{gid}", str(directory)],
                                        capture_output=True, timeout=30, check=False)
                if result.returncode:
                    raise RuntimeError("could not restore Phase0 signer directory ownership")
    if any(path.is_symlink() for path in owned):
        raise RuntimeError("fixture ownership path must not be a symlink")
    paths = [path for path in owned if path.exists()]
    if os.geteuid() == 0:
        for path in paths:
            os.chown(path, uid, gid)
    elif paths:
        result = subprocess.run(["sudo", "-n", "chown", f"{uid}:{gid}", *(str(path) for path in paths)],
                                capture_output=True, timeout=30, check=False)
        if result.returncode:
            raise RuntimeError("could not restore Phase0 fixture ownership")
    for directory in (root, root / "tls", root / "ca", *signers):
        if directory.is_dir():
            directory.chmod(0o700)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("usage: restore-phase0-fixture-owner.py FIXTURE_DIR")
    try:
        restore(Path(sys.argv[1]))
    except (OSError, RuntimeError, subprocess.SubprocessError) as exc:
        print(f"Phase0 fixture owner restoration failed: {exc}", file=sys.stderr)
        raise SystemExit(1)
