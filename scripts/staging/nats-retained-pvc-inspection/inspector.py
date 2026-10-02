"""Pinned original-PV file inventory. No payload output and no preservation PASS."""
import datetime
from contextlib import contextmanager
import grp
import hashlib
import json
import os
import posixpath
import re
from pathlib import Path
import socket
import stat
import subprocess
import sys
import tempfile

NAMESPACE = "voice-staging"
PVC = "voice-nats-jsdata"
PV = "pvc-52c42e20-c7e6-4182-b5d3-ecaa6e1f9855"
NODE = "pmdebook"
SOURCE = Path("/var/lib/rancher/k3s/storage") / (PV + "_voice-staging_voice-nats-jsdata")
OUTPUT = Path("/var/lib/voice-nats-preservation")
CONFIG = Path("/etc/rancher/k3s/k3s.yaml")
K3S = Path("/usr/local/bin/k3s")
ENV = {"PATH": "/usr/sbin:/usr/bin:/sbin:/bin", "LANG": "C.UTF-8"}


class Unsafe(Exception):
    pass


def require_root():
    if os.geteuid() != 0 or sys.platform != "linux":
        raise Unsafe("ROOT_LINUX_REQUIRED")


def fingerprint(s):
    return (s.st_dev, s.st_ino, s.st_mode, s.st_uid, s.st_gid,
            s.st_size, s.st_mtime_ns, s.st_ctime_ns, s.st_atime_ns, s.st_nlink)


def open_read(path, directory=False, dir_fd=None):
    flags = os.O_RDONLY | os.O_NOFOLLOW | os.O_NOATIME | os.O_CLOEXEC
    if directory:
        flags |= os.O_DIRECTORY
    return os.open(path, flags, dir_fd=dir_fd)


def trusted_path(path, root_owned=False):
    path = Path(path)
    if not path.is_absolute() or ".." in path.parts:
        raise Unsafe("PATH_INVALID")
    current = Path("/")
    for part in path.parts[1:]:
        current /= part
        s = current.lstat()
        if stat.S_ISLNK(s.st_mode):
            raise Unsafe("SYMLINK")
        if root_owned and (s.st_uid != 0 or s.st_mode & 0o022):
            raise Unsafe("UNTRUSTED_ROOT_PATH")


def inventory(root):
    """FD-relative walk; no symlinks, specials or credential contents are read."""
    trusted_path(root)
    rows, census = [], []
    def walk(fd, relative):
        before = os.fstat(fd)
        names = sorted(os.listdir(fd))
        for name in names:
            s = os.stat(name, dir_fd=fd, follow_symlinks=False)
            rel = relative / name
            census.append((str(rel), fingerprint(s)))
            if stat.S_ISDIR(s.st_mode):
                child = open_read(name, True, fd)
                try:
                    if fingerprint(os.fstat(child)) != fingerprint(s):
                        raise Unsafe("DIRECTORY_CHANGED")
                    walk(child, rel)
                finally:
                    os.close(child)
            elif not stat.S_ISREG(s.st_mode):
                raise Unsafe("SYMLINK_OR_SPECIAL")
            elif selected(relative, name):
                if s.st_nlink != 1:
                    raise Unsafe("HARDLINK")
                file_fd = open_read(name, dir_fd=fd)
                try:
                    if fingerprint(os.fstat(file_fd)) != fingerprint(s):
                        raise Unsafe("FILE_CHANGED")
                    digest = hashlib.sha256()
                    while chunk := os.read(file_fd, 1024 * 1024):
                        digest.update(chunk)
                    if fingerprint(os.fstat(file_fd)) != fingerprint(s) or fingerprint(
                            os.stat(name, dir_fd=fd, follow_symlinks=False)) != fingerprint(s):
                        raise Unsafe("FILE_CHANGED")
                    rows.append({"path": str(rel), "size": s.st_size,
                                 "mtime_ns": s.st_mtime_ns, "ctime_ns": s.st_ctime_ns,
                                 "inode": s.st_ino, "device": s.st_dev,
                                 "sha256": digest.hexdigest()})
                finally:
                    os.close(file_fd)
        if names != sorted(os.listdir(fd)) or fingerprint(os.fstat(fd)) != fingerprint(before):
            raise Unsafe("DIRECTORY_CHANGED")
    fd = open_read(root, True)
    try:
        walk(fd, Path())
    finally:
        os.close(fd)
    # Re-stat every encountered entry without following links; ancestors were checked.
    for relative, expected in census:
        if fingerprint((Path(root) / relative).lstat()) != expected:
            raise Unsafe("TREE_CHANGED")
    if not rows:
        raise Unsafe("NO_JETSTREAM_METADATA")
    return rows


def selected(parent, name):
    parts = parent.parts
    if len(parts) < 4 or parts[0] != "jetstream" or parts[2] != "streams":
        return False
    if len(parts) == 4:
        return name in ("meta.inf", "meta.sum")
    if len(parts) == 5 and parts[4] == "msgs":
        return name.endswith(".blk") and name[:-4].isdigit()
    return (len(parts) == 6 and parts[4] == "obs"
            and name in ("meta.inf", "meta.sum", "o.dat"))


def inside(path, source):
    return path == str(source) or path.startswith(str(source).rstrip("/") + "/")


def no_open_handles(source, proc=Path("/proc")):
    # Any open handle is rejected, including read-only ones; maps catch closed-fd mmap.
    device, coordinate, _ = source_mount(source)
    for process in proc.iterdir():
        if not process.name.isdigit() or int(process.name) == os.getpid():
            continue
        with process_mount_view(process) as mounts:
            check_process_handles(process, mounts, device, coordinate)


def check_process_handles(process, mounts, device, coordinate):
    def touches(path):
        if not path.startswith("/"):
            return False
        candidates = [r for r in mounts if inside(path, r[2])]
        if not candidates:
            raise Unsafe("HANDLE_MAPPING_UNCERTAIN")
        longest = max(len(r[2]) for r in candidates)
        matches = [r for r in candidates if len(r[2]) == longest]
        if len(matches) != 1:
            raise Unsafe("HANDLE_MAPPING_UNCERTAIN")
        dev, root, target = matches[0]
        if not root.startswith("/"):
            if dev == device:
                raise Unsafe("HANDLE_MAPPING_UNCERTAIN")
            return False  # Recognized nsfs objects have no filesystem subtree.
        physical = posixpath.normpath(posixpath.join(root, posixpath.relpath(path, target)))
        return dev == device and inside(physical, coordinate)
    for fd in (process / "fd").iterdir():
        if touches(os.readlink(fd).removesuffix(" (deleted)")):
            raise Unsafe("OPEN_SOURCE_HANDLE")
    for line in (process / "maps").read_text().splitlines():
        parts = line.split(maxsplit=5)
        if len(parts) == 6 and touches(parts[5].removesuffix(" (deleted)")):
            raise Unsafe("MAPPED_SOURCE")


def mount_records(path):
    return parse_mount_records(path.read_text())


def parse_mount_records(text):
    result = []
    for line in text.splitlines():
        fields = line.split()
        if len(fields) < 10 or "-" not in fields or not re.fullmatch(r"\d+:\d+", fields[2]):
            raise Unsafe("MOUNTINFO_UNCERTAIN")
        separator = fields.index("-", 6)
        if len(fields) != separator + 4:
            raise Unsafe("MOUNTINFO_UNCERTAIN")
        decoded = []
        for index, field in enumerate((fields[3], fields[4])):
            value = re.sub(r"\\([0-7]{3})", lambda match: chr(int(match[1], 8)), field)
            if (index == 0 and fields[separator + 1] == "nsfs"
                    and fields[2].startswith("0:") and re.fullmatch(r"net:\[\d+\]", value)):
                decoded.append(value)
                continue  # Kernel network namespace object, not a filesystem root.
            if not value.startswith("/") or posixpath.normpath(value) != value:
                raise Unsafe("MOUNTINFO_UNCERTAIN")
            decoded.append(value)
        result.append((fields[2], *decoded))
    if not result:
        raise Unsafe("MOUNTINFO_UNCERTAIN")
    return result


def process_identity(process):
    directory = process.stat()
    fields = (process / "stat").read_text().rsplit(")", 1)[-1].split()
    if len(fields) < 20 or not fields[19].isdigit():
        raise Unsafe("PROCESS_IDENTITY_UNCERTAIN")
    namespace = (process / "ns/mnt").stat()
    root = (process / "root").stat()
    return (directory.st_dev, directory.st_ino, int(fields[19]),
            namespace.st_dev, namespace.st_ino, root.st_dev, root.st_ino)


@contextmanager
def pinned_process(process):
    fds = []
    try:
        before = process_identity(process)
        for path, flags, expected in (
            (process, os.O_DIRECTORY | os.O_NOFOLLOW, before[:2]),
            (process / "ns/mnt", 0, before[3:5]),
            (process / "root", os.O_DIRECTORY, before[5:7]),
        ):
            fd = os.open(path, os.O_RDONLY | os.O_CLOEXEC | flags)
            fds.append(fd)
            s = os.fstat(fd)
            if (s.st_dev, s.st_ino) != expected:
                raise Unsafe("PROCESS_IDENTITY_CHANGED")
        if process_identity(process) != before:
            raise Unsafe("PROCESS_IDENTITY_CHANGED")
        try:
            yield before
        finally:
            if process_identity(process) != before:
                raise Unsafe("PROCESS_IDENTITY_CHANGED")
    except Exception as error:
        if process.name.isdigit():
            error.pid = int(process.name)
        raise
    finally:
        for fd in reversed(fds):
            os.close(fd)


@contextmanager
def process_mount_view(process):
    raw = (process / "mountinfo").read_bytes()
    if raw:
        yield parse_mount_records(raw.decode())
        return
    # EMPTY can be chroot filtering, never permission to skip a live process.
    caller_path, init_path = Path("/proc") / str(os.getpid()), Path("/proc/1")
    with pinned_process(process) as target, pinned_process(caller_path) as caller, pinned_process(init_path) as init:
        host_root = Path("/").stat()
        if target[3:5] != caller[3:5] or init[3:7] != caller[3:7]:
            raise Unsafe("EMPTY_NAMESPACE_UNPROVEN")
        if caller[5:7] != (host_root.st_dev, host_root.st_ino):
            raise Unsafe("HOST_ROOT_UNPROVEN")
        host_raw = (caller_path / "mountinfo").read_bytes()
        init_raw = (init_path / "mountinfo").read_bytes()
        if init_raw != host_raw:
            raise Unsafe("EMPTY_NAMESPACE_UNPROVEN")
        records = parse_mount_records(host_raw.decode())
        roots = [record for record in records if record[2] == "/"]
        if (len(roots) != 1 or roots[0][1] != "/" or roots[0][0] !=
                str(os.major(host_root.st_dev)) + ":" + str(os.minor(host_root.st_dev))):
            raise Unsafe("HOST_ROOT_UNPROVEN")
        snapshots = ((process, target, raw), (caller_path, caller, host_raw), (init_path, init, init_raw))
        def recheck():
            for path, identity, table in snapshots:
                # Re-stat live proc references: pinned old FDs alone cannot detect
                # a surviving process that changes root or mount namespace.
                if process_identity(path) != identity:
                    raise Unsafe("PROCESS_IDENTITY_CHANGED")
                if (path / "mountinfo").read_bytes() != table:
                    raise Unsafe("MOUNT_VIEW_CHANGED")
            fresh_root = Path("/").stat()
            if (fresh_root.st_dev, fresh_root.st_ino) != caller[5:7]:
                raise Unsafe("HOST_ROOT_UNPROVEN")
        recheck()
        # FD/maps paths are rendered by the kernel relative to the current reader's
        # root (d_path), so use caller coordinates unchanged, not target chroot paths.
        try:
            yield records
        finally:
            recheck()


def source_mount(source):
    candidates = [r for r in mount_records(Path("/proc/self/mountinfo")) if inside(str(source), r[2])]
    if not candidates:
        raise Unsafe("SOURCE_MAPPING_UNCERTAIN")
    longest = max(len(r[2]) for r in candidates)
    matches = [r for r in candidates if len(r[2]) == longest]
    if len(matches) != 1:
        raise Unsafe("SOURCE_MAPPING_UNCERTAIN")
    backing = matches[0]
    device, root, target = backing
    if not root.startswith("/"):
        raise Unsafe("SOURCE_MAPPING_UNCERTAIN")
    if target == str(source) or inside(target, source):
        raise Unsafe("SOURCE_MOUNTED")
    s = source.stat()
    if device != str(os.major(s.st_dev)) + ":" + str(os.minor(s.st_dev)):
        raise Unsafe("SOURCE_DEVICE_CHANGED")
    coordinate = posixpath.normpath(posixpath.join(root, posixpath.relpath(str(source), target)))
    return device, coordinate, backing


def no_mounts(source):
    # mountinfo roots are relative to their filesystem, not the host's /.
    device, coordinate, backing = source_mount(source)
    for process in Path("/proc").iterdir():
        if not process.name.isdigit():
            continue
        with process_mount_view(process) as records:
            for record in records:
                dev, root, target = record
                if inside(target, source):
                    raise Unsafe("SOURCE_MOUNTED")
                if record == backing:
                    continue  # Canonical host backing mount is expected.
                if dev == device:
                    if not root.startswith("/"):
                        raise Unsafe("SOURCE_MAPPING_UNCERTAIN")
                    if inside(root, coordinate) or inside(coordinate, root):
                        raise Unsafe("SOURCE_MOUNTED")


def validate_kube(pvc, pv, pods):
    try:
        if (pvc["metadata"]["name"] != PVC or pvc["metadata"]["namespace"] != NAMESPACE
                or pvc["status"]["phase"] != "Bound" or pvc["spec"]["volumeName"] != PV
                or pv["metadata"]["name"] != PV or pv["status"]["phase"] != "Bound"
                or pv["spec"]["claimRef"]["name"] != PVC
                or pv["spec"]["claimRef"]["namespace"] != NAMESPACE
                or pv["spec"]["claimRef"]["uid"] != pvc["metadata"]["uid"]
                or pv["spec"]["local"]["path"] != str(SOURCE)):
            raise Unsafe("PV_PROVENANCE")
        terms = pv["spec"]["nodeAffinity"]["required"]["nodeSelectorTerms"]
        if terms != [{"matchExpressions": [{"key": "kubernetes.io/hostname", "operator": "In", "values": [NODE]}]}]:
            raise Unsafe("NODE_AFFINITY")
        for pod in pods["items"]:
            if pod["metadata"]["namespace"] == NAMESPACE:
                for volume in pod["spec"].get("volumes", []):
                    if volume.get("persistentVolumeClaim", {}).get("claimName") == PVC:
                        raise Unsafe("POD_MOUNT")
    except (KeyError, TypeError, AttributeError):
        raise Unsafe("KUBERNETES_UNCERTAIN") from None


def kube_state():
    results = []
    for args in (("pvc", PVC, "-n", NAMESPACE), ("pv", PV), ("pods", "--all-namespaces")):
        result = subprocess.run([str(K3S), "kubectl", "--kubeconfig", str(CONFIG),
                                 "get", *args, "-o", "json", "--request-timeout=15s"],
                                env=ENV, capture_output=True, timeout=20, check=True)
        results.append(json.loads(result.stdout))
    validate_kube(*results)
    return results


def main():
    require_root()
    if socket.gethostname() != NODE:
        raise Unsafe("WRONG_NODE")
    for path in (CONFIG, K3S):
        trusted_path(path, root_owned=True)
    trusted_path(SOURCE)
    trusted_path(SOURCE.parent, root_owned=True)
    # Parent creation uses a root-controlled /var/lib; existing parent must be trusted.
    trusted_path(OUTPUT.parent, root_owned=True)
    if not OUTPUT.exists():
        OUTPUT.mkdir(mode=0o700)
    trusted_path(OUTPUT, root_owned=True)
    output = Path(tempfile.mkdtemp(prefix="original-pv-", dir=OUTPUT))
    initial = kube_state()
    no_mounts(SOURCE)
    no_open_handles(SOURCE)
    rows = inventory(SOURCE)
    no_open_handles(SOURCE)
    no_mounts(SOURCE)
    final = kube_state()
    if initial[0] != final[0] or initial[1] != final[1]:
        raise Unsafe("KUBERNETES_CHANGED")
    if rows != inventory(SOURCE):
        raise Unsafe("SOURCE_CHANGED")
    no_open_handles(SOURCE)
    no_mounts(SOURCE)
    last = kube_state()
    if initial[0] != last[0] or initial[1] != last[1]:
        raise Unsafe("KUBERNETES_CHANGED")
    report = {"status": "READONLY_FILE_INVENTORY_COMPLETE", "preservation_pass": False,
              "canonical_census": False, "pv": PV, "pvc": PVC, "node": NODE,
              "source": str(SOURCE), "time_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
              "pvc_uid": initial[0]["metadata"]["uid"], "pv_uid": initial[1]["metadata"]["uid"],
              "files": rows}
    target = output / "metadata.json"
    fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as stream:
        json.dump(report, stream, indent=2, ensure_ascii=True)
        stream.write("\n")
        stream.flush()
        os.fsync(stream.fileno())
    group = grp.getgrnam("pmd").gr_gid
    os.chown(target, 0, group)
    os.chmod(target, 0o440)
    os.chown(output, 0, group)
    os.chmod(output, 0o750)
    os.chown(OUTPUT, 0, group)
    os.chmod(OUTPUT, 0o750)
    print(str(target))


PUBLIC_FAILURE_CODES = frozenset("""
ROOT_LINUX_REQUIRED PATH_INVALID SYMLINK UNTRUSTED_ROOT_PATH DIRECTORY_CHANGED
SYMLINK_OR_SPECIAL HARDLINK FILE_CHANGED TREE_CHANGED NO_JETSTREAM_METADATA
HANDLE_MAPPING_UNCERTAIN OPEN_SOURCE_HANDLE MAPPED_SOURCE MOUNTINFO_UNCERTAIN
PROCESS_IDENTITY_UNCERTAIN PROCESS_IDENTITY_CHANGED EMPTY_NAMESPACE_UNPROVEN
HOST_ROOT_UNPROVEN MOUNT_VIEW_CHANGED SOURCE_MAPPING_UNCERTAIN SOURCE_MOUNTED
SOURCE_DEVICE_CHANGED PV_PROVENANCE NODE_AFFINITY POD_MOUNT KUBERNETES_UNCERTAIN
WRONG_NODE KUBERNETES_CHANGED SOURCE_CHANGED
""".split())
PUBLIC_FUNCTIONS = frozenset("""
require_root fingerprint open_read trusted_path inventory walk selected inside
no_open_handles check_process_handles touches mount_records parse_mount_records
process_identity pinned_process process_mount_view recheck source_mount no_mounts
validate_kube kube_state main
""".split())


def safe_failure(error):
    """Only pinned public labels and bounded numeric context cross the CLI."""
    category = "Unsafe" if isinstance(error, Unsafe) else "OSError" if isinstance(error, OSError) else "Exception"
    result = {"status": "INSPECTION_FAILED_NO_ACCEPTANCE", "class": category, "code": "UNEXPECTED_FAILURE"}
    if (isinstance(error, Unsafe) and len(error.args) == 1
            and type(error.args[0]) is str and error.args[0] in PUBLIC_FAILURE_CODES):
        result["code"] = error.args[0]
    frame = error.__traceback__
    while frame is not None:
        code = frame.tb_frame.f_code
        if code.co_filename == __file__ and code.co_name in PUBLIC_FUNCTIONS:
            result["function"] = code.co_name
        frame = frame.tb_next
    for key, maximum in (("pid", 2**31 - 1), ("errno", 4095)):
        value = getattr(error, key, None)
        if type(value) is int and 0 < value <= maximum:
            result[key] = value
    return result


def run_cli():
    try:
        main()
    except Exception as error:
        print(json.dumps(safe_failure(error), sort_keys=True), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(run_cli())
