#!/usr/bin/env python3
"""Restore only Phase0 MatchSquad fixture paths chowned for nonroot services."""

import os
import subprocess
import sys
from pathlib import Path


def restore(destination: Path) -> None:
    if os.name != "posix":
        raise RuntimeError("Phase0 fixture ownership can only be restored on POSIX")
    if destination.is_symlink():
        raise RuntimeError("fixture destination must not be a symlink")
    root = destination.resolve(strict=True)
    if root.name not in {"phase0-fixture", "credentials", "voice-space-media-phase0"}:
        raise RuntimeError("fixture destination is outside the supported disposable paths")
    matchmaking = root / "matchmaking"
    if any(path.is_symlink() for path in (root / "tls", root / "ca", matchmaking)):
        raise RuntimeError("fixture directories must not be symlinks")
    owned = [matchmaking / "current.pem", matchmaking / "next.pem",
             root / "tls/chat-match-squad-grpc.crt", root / "tls/chat-match-squad-grpc.key",
             root / "tls/voice-match-squad-grpc.crt", root / "tls/voice-match-squad-grpc.key",
             root / "tls/voice-match-squad-member-grpc.crt",
             root / "tls/voice-match-squad-member-grpc.key",
             root / "tls/matchmaking-client.crt", root / "tls/matchmaking-client.key",
             root / "ca/ca.crt"]
    if matchmaking.is_symlink():
        raise RuntimeError("fixture ownership path must not be a symlink")
    uid, gid = os.getuid(), os.getgid()
    if matchmaking.is_dir():
        if os.geteuid() == 0:
            os.chown(matchmaking, uid, gid)
        else:
            result = subprocess.run(["sudo", "-n", "chown", f"{uid}:{gid}", str(matchmaking)],
                                    capture_output=True, timeout=30, check=False)
            if result.returncode:
                raise RuntimeError("could not restore MatchSquad signer directory ownership")
    paths = [path for path in owned if path.exists()]
    if any(path.is_symlink() for path in paths):
        raise RuntimeError("fixture ownership path must not be a symlink")
    if os.geteuid() == 0:
        for path in paths:
            os.chown(path, uid, gid)
    elif paths:
        result = subprocess.run(["sudo", "-n", "chown", f"{uid}:{gid}", *(str(path) for path in paths)],
                                capture_output=True, timeout=30, check=False)
        if result.returncode:
            raise RuntimeError("could not restore Phase0 fixture ownership")
    for directory in (root, root / "tls", root / "ca", matchmaking):
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
