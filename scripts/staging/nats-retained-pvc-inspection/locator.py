"""Bounded candidate directory metadata. Never a census or preservation proof."""
import grp
import hashlib
import json
import os
from contextlib import ExitStack
from pathlib import Path
import re
import signal
import socket
import stat
import sys
import tempfile
import time

ROOTS = (
    ("/var/lib/rancher/k3s/storage", "LOCAL_PATH"),
    ("/var/lib/kubelet/pods", "POD_CANDIDATE"),
    ("/var/lib/rancher/k3s/agent/kubelet/pods", "POD_CANDIDATE"),
)
KNOWN = "pvc-52c42e20-c7e6-4182-b5d3-ecaa6e1f9855_voice-staging_voice-nats-jsdata"
UUID = r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"
POD = re.compile(UUID, re.ASCII)
LOCAL = re.compile(r"pvc-" + UUID + r"_voice-staging_[a-z0-9-]{0,80}nats[a-z0-9-]{0,80}", re.ASCII)
MAX_ENTRIES = 4096
MAX_CANDIDATES = 128
FLAGS = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_NOATIME | os.O_NONBLOCK | os.O_CLOEXEC
OUTPUT = Path("/var/lib/voice-nats-preservation")


class Unsafe(Exception):
    pass


def digest(value):
    return hashlib.sha256(os.fsencode(value)).hexdigest()


def fingerprint(s):
    return (s.st_dev, s.st_ino, s.st_mode, s.st_uid, s.st_gid,
            s.st_size, s.st_mtime_ns, s.st_ctime_ns, s.st_atime_ns, s.st_nlink)


def metadata(s):
    return {"device": s.st_dev, "inode": s.st_ino, "mode": stat.S_IMODE(s.st_mode),
            "uid": s.st_uid, "gid": s.st_gid, "bytes": s.st_size,
            "mtime_ns": s.st_mtime_ns, "ctime_ns": s.st_ctime_ns}


def checkpoint(deadline):
    if time.monotonic() >= deadline:
        raise Unsafe("TIME_BOUND")


def opened(stack, parent, name, checks, path, mounts=(), trusted=False):
    if path in mounts:
        raise Unsafe("NESTED_MOUNT")
    before = os.stat(name, dir_fd=parent, follow_symlinks=False)
    if not stat.S_ISDIR(before.st_mode):
        raise Unsafe("DIRECTORY_REQUIRED")
    if trusted and (before.st_uid != 0 or before.st_mode & 0o022):
        raise Unsafe("UNTRUSTED_ROOT")
    fd = os.open(name, FLAGS, dir_fd=parent)
    stack.callback(os.close, fd)
    if fingerprint(before) != fingerprint(os.fstat(fd)):
        raise Unsafe("DIRECTORY_CHANGED")
    checks.append((parent, name, fd, fingerprint(before)))
    return fd


def verify(checks):
    for parent, name, fd, expected in checks:
        if (fingerprint(os.fstat(fd)) != expected
                or fingerprint(os.stat(name, dir_fd=parent, follow_symlinks=False)) != expected):
            raise Unsafe("DIRECTORY_CHANGED")


def root_fd(stack, root, checks):
    if not root.is_absolute() or ".." in root.parts:
        raise Unsafe("ROOT_PATH_INVALID")
    fd = os.open("/", FLAGS)
    stack.callback(os.close, fd)
    for part in root.parts[1:]:
        fd = opened(stack, fd, part, checks, "", trusted=True)
    return fd


def scan(root, kind, mounts=(), deadline=None):
    root = Path(root)
    deadline = time.monotonic() + 15 if deadline is None else deadline
    if kind not in ("LOCAL_PATH", "POD_CANDIDATE"):
        raise Unsafe("KIND_INVALID")
    rows = []
    checks = []
    with ExitStack() as stack:
        try:
            fd = root_fd(stack, root, checks)
        except FileNotFoundError:
            verify(checks)
            return {"root_sha256": digest(str(root)), "kind": kind,
                    "status": "CANDIDATE_ROOT_ABSENT", "candidates": []}
        with os.scandir(fd) as entries:
            for index, entry in enumerate(entries):
                checkpoint(deadline)
                if index >= MAX_ENTRIES:
                    raise Unsafe("ENTRY_BOUND")
                name = entry.name
                if not (LOCAL if kind == "LOCAL_PATH" else POD).fullmatch(name):
                    continue
                if len(rows) >= MAX_CANDIDATES:
                    raise Unsafe("CANDIDATE_BOUND")
                with ExitStack() as candidate_stack:
                    candidate_checks = []
                    path = root / name
                    current = opened(candidate_stack, fd, name, candidate_checks, str(path), mounts)
                    row = {"locator_sha256": digest(str(path)), "kind": kind,
                           "known_inspected_pv": kind == "LOCAL_PATH" and name == KNOWN}
                    if kind == "LOCAL_PATH":
                        row["relative_name"] = name  # Strict allowlisted locator grammar only.
                    else:
                        row["pod_uid"] = name  # Canonical UUID; no arbitrary names.
                    if kind == "POD_CANDIDATE":
                        try:
                            for part in ("volumes", "kubernetes.io~empty-dir", "jsdata"):
                                checkpoint(deadline)
                                path /= part
                                current = opened(candidate_stack, current, part, candidate_checks, str(path), mounts)
                        except FileNotFoundError:
                            verify(candidate_checks)
                            continue
                        row["locator_sha256"] = digest(str(path))
                    row["directory"] = metadata(os.fstat(current))
                    row["jetstream_directory"] = False
                    try:
                        js = opened(candidate_stack, current, "jetstream", candidate_checks,
                                    str(path / "jetstream"), mounts)
                    except FileNotFoundError:
                        pass
                    else:
                        row["jetstream_directory"] = True
                        row["jetstream_metadata"] = metadata(os.fstat(js))
                    row["status"] = "DIRECTORY_METADATA_ONLY"
                    verify(candidate_checks)
                    rows.append(row)
        verify(checks)
    rows.sort(key=lambda row: row["locator_sha256"])
    return {"root_sha256": digest(str(root)), "kind": kind, "status": "ENUMERATED",
            "candidates": rows}


def mount_targets():
    # Kernel metadata only; no source contents or Kubernetes credentials.
    fd = os.open("/proc/self/mountinfo", os.O_RDONLY | os.O_CLOEXEC | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        raw = bytearray()
        while chunk := os.read(fd, min(65536, 1048577 - len(raw))):
            raw.extend(chunk)
            if len(raw) > 1048576:
                raise Unsafe("MOUNTINFO_BOUND")
    finally:
        os.close(fd)
    result = set()
    for line in raw.decode("utf-8", "strict").splitlines():
        parts = line.split()
        if len(parts) < 10 or "-" not in parts:
            raise Unsafe("MOUNTINFO_INVALID")
        target = re.sub(r"\\([0-7]{3})", lambda match: chr(int(match[1], 8)), parts[4])
        if not target.startswith("/"):
            raise Unsafe("MOUNTINFO_INVALID")
        result.add(target)
    return result


def publish(report):
    with ExitStack() as stack:
        checks = []
        root_fd(stack, OUTPUT.parent, checks)
        try:
            OUTPUT.mkdir(mode=0o700)
        except FileExistsError:
            pass
        root_fd(stack, OUTPUT, checks)
        directory = Path(tempfile.mkdtemp(prefix="historical-locators-", dir=OUTPUT))
        group = grp.getgrnam("pmd").gr_gid
        target = directory / "locators.json"
        fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, "w", encoding="utf-8") as stream:
            json.dump(report, stream, sort_keys=True, indent=2)
            stream.write("\n")
            stream.flush()
            os.fsync(stream.fileno())
        os.chown(target, 0, group)
        os.chmod(target, 0o440)
        os.chown(directory, 0, group)
        os.chmod(directory, 0o750)
        os.chown(OUTPUT, 0, group)
        os.chmod(OUTPUT, 0o750)
    return target


def main():
    if sys.platform != "linux" or os.geteuid() != 0 or socket.gethostname() != "pmdebook":
        raise Unsafe("ROOT_LINUX_NODE_REQUIRED")
    os.umask(0o077)
    signal.signal(signal.SIGALRM, lambda *_: (_ for _ in ()).throw(Unsafe("TIME_BOUND")))
    signal.alarm(20)
    deadline = time.monotonic() + 15
    mounts = mount_targets()
    observations = [scan(Path(root), kind, mounts, deadline) for root, kind in ROOTS]
    if mounts != mount_targets():
        raise Unsafe("MOUNTS_CHANGED")
    # All input roots are explicit candidates, not authenticated historical roots.
    source = Path(__file__)
    with ExitStack() as stack:
        checks = []
        parent = root_fd(stack, source.parent, checks)
        fd = os.open(source.name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NOATIME | os.O_NONBLOCK | os.O_CLOEXEC, dir_fd=parent)
        stack.callback(os.close, fd)
        info = os.fstat(fd)
        if not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o077 or not 0 < info.st_size <= 65536:
            raise Unsafe("PRIVATE_CODE_REQUIRED")
        code = os.read(fd, 65537)
        if len(code) != info.st_size or fingerprint(info) != fingerprint(os.fstat(fd)):
            raise Unsafe("PRIVATE_CODE_CHANGED")
    report = {"status": "BOUNDED_CANDIDATE_LOCATORS", "canonical_census": False,
              "preservation_pass": False, "historical_attribution_verified": False,
              "historical_completeness": "UNKNOWN", "creation_time_verified": False,
              "source_content_reads": 0, "node": "pmdebook",
              "code_sha256": hashlib.sha256(code).hexdigest(),
              "observed_unix_ns": time.time_ns(), "roots": observations,
              "candidate_root_paths": [root for root, _ in ROOTS],
              "bounds": {"entries_per_root": MAX_ENTRIES, "candidates_per_root": MAX_CANDIDATES,
                         "max_exact_child_depth": 5, "runtime_seconds": 20}}
    print(str(publish(report)))
    signal.alarm(0)


if __name__ == "__main__":
    try:
        main()
    except Unsafe as exc:
        print(json.dumps({"status": "FAILED", "reason": str(exc)}), file=sys.stderr)
        sys.exit(1)
    except Exception:
        print('{"status":"FAILED","reason":"METADATA_UNCERTAIN"}', file=sys.stderr)
        sys.exit(1)
