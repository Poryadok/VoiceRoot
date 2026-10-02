"""Exact original-PV stored observations; never canonical census/preservation PASS."""
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import struct
import sys
import tempfile
import types

REPORT = Path("/var/lib/voice-nats-preservation/original-pv-sckcykyd/metadata.json")
REPORT_SHA = "12d2b844a3af68362abe3614c4119eb677fb243eb0c2a982137bd9dfc910c2c6"
REPORT_BYTES = 61916
REPORT_GID = None  # Resolved from existing pmd group on the root runtime.
INSPECTOR_SHA = "81fe14bf85f1beb95f78ac7a2da9ebd8183472fc3be89bc4f5148386f4d7c475"
FILE_COUNT = 165

MAX_BYTES = 2 * 1024 * 1024
MAX_ENTRIES = 4096
MAX_NUMBER = 2**63 - 1


class Unsupported(Exception):
    pass


def unsupported(reason):
    return {"status": "UNSUPPORTED_" + reason}


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise Unsupported()
        result[key] = value
    return result


ENUMS = {
    "retention": {"limits", "interest", "workqueue"},
    "storage": {"file", "memory"}, "discard": {"old", "new"},
    "compression": {"none", "s2"}, "ack_policy": {"none", "all", "explicit"},
    "deliver_policy": {"all", "last", "new", "by_start_sequence", "by_start_time", "last_per_subject"},
    "replay_policy": {"instant", "original"},
}
NUMERIC = frozenset("""max_consumers max_msgs max_bytes max_age max_msgs_per_subject
max_msg_size num_replicas duplicate_window ack_wait max_deliver rate_limit_bps
max_ack_pending max_waiting max_batch max_expires max_bytes inactive_threshold opt_start_seq""".split())
BOOLEANS = frozenset("""sealed deny_delete deny_purge allow_rollup_hdrs no_ack
discard_new_per_subject flow_control headers_only memory_storage direct mirror_direct
allow_direct""".split())
PRIVATE_FIELDS = frozenset("""Created Name name durable_name description metadata
subjects filter_subject filter_subjects deliver_subject deliver_group opt_start_time
backoff placement mirror sources republish subject_transform consumer_limits
template_owner allow_msg_ttl subject_delete_marker_ttl allow_msg_schedules
allow_atomic persist_mode""".split())


def project_meta(raw, kind):
    """Project known primitives only; arbitrary strings/nested metadata stay private."""
    try:
        if len(raw) > MAX_BYTES:
            raise Unsupported()
        data = json.loads(raw, object_pairs_hook=unique_object)
        if type(data) is not dict or kind not in {"stream", "consumer"}:
            raise Unsupported()
        fields = {}
        for key, value in data.items():
            if key not in ENUMS and key not in NUMERIC and key not in BOOLEANS and key not in PRIVATE_FIELDS:
                raise Unsupported()
            if key in ENUMS:
                if type(value) is not str or value not in ENUMS[key]:
                    raise Unsupported()
                fields[key] = value
            elif key in NUMERIC:
                if type(value) is not int or not -1 <= value <= MAX_NUMBER:
                    raise Unsupported()
                fields[key] = value
            elif key in BOOLEANS:
                if type(value) is not bool:
                    raise Unsupported()
                fields[key] = value
        counts = {}
        for key in ("subjects", "filter_subjects", "backoff"):
            if key in data:
                if type(data[key]) is not list or len(data[key]) > MAX_ENTRIES:
                    raise Unsupported()
                if key != "backoff" and any(type(value) is not str for value in data[key]):
                    raise Unsupported()
                counts[key.replace("subjects", "subject") + "_count"] = len(data[key])
        if "subjects_count" in counts:
            counts["subject_count"] = counts.pop("subjects_count")
        if "subject_count" not in counts and "subjects" in data:
            counts["subject_count"] = len(data["subjects"])
        return {"status": "JSON_PRIMITIVES_COMPATIBLE", "fields": fields, **counts,
                "writer_version_verified": False, "native_checksum_verified": False}
    except (Unsupported, ValueError, TypeError, UnicodeError, RecursionError):
        return unsupported("JSON_SCHEMA_OR_ENCRYPTION")


def consumer_state(raw):
    if not raw:
        return {"status": "EMPTY_UNVERSIONED", "writer_version_verified": False}
    try:
        if len(raw) > MAX_BYTES or len(raw) < 2 or raw[0] != 22 or raw[1] not in (1, 2):
            raise Unsupported()
        version, offset = raw[1], 2
        def uint():
            nonlocal offset
            value = 0
            for i in range(10):
                if offset >= len(raw):
                    raise Unsupported()
                byte = raw[offset]
                offset += 1
                if i == 9 and byte > 1:
                    raise Unsupported()
                value |= (byte & 127) << (7 * i)
                if byte < 128:
                    if i and byte == 0:
                        raise Unsupported()
                    return value
            raise Unsupported()
        def signed():
            value = uint()
            return -(value // 2) - 1 if value & 1 else value // 2
        ac, ast, dc, ds = (uint() for _ in range(4))
        if version == 1:
            dc += max(ac - 1, 0)
            ds += max(ast - 1, 0)
        if any(n > MAX_NUMBER for n in (ac, ast, dc, ds)) or ac > dc or ast > ds:
            raise Unsupported()
        pending = uint()
        if pending > MAX_ENTRIES:
            raise Unsupported()
        seen = set()
        if pending:
            mintime = signed()
            for _ in range(pending):
                seq = uint() + ast
                delivery = uint() + ac if version == 2 else 0
                delta = signed()
                timestamp = (mintime + delta if version == 1 else mintime - delta) * 10**9
                if (seq == 0 or seq > MAX_NUMBER or seq in seen or delivery > MAX_NUMBER
                        or not -2**63 <= timestamp <= MAX_NUMBER):
                    raise Unsupported()
                seen.add(seq)
        redelivered = uint()
        if redelivered > MAX_ENTRIES:
            raise Unsupported()
        seen = set()
        for _ in range(redelivered):
            delta, count = uint(), uint()
            seq = delta + ast
            if not delta or not count or seq > MAX_NUMBER or count > MAX_NUMBER or seq in seen:
                raise Unsupported()
            seen.add(seq)
        if offset != len(raw):
            raise Unsupported()
        return {"status": "CONSUMER_STATE_LAYOUT_COMPATIBLE", "layout_version": version,
                "ack_floor_consumer": ac, "ack_floor_stream": ast,
                "delivered_consumer": dc, "delivered_stream": ds,
                "pending_count": pending, "redelivered_count": redelivered,
                "writer_version_verified": False}
    except (Unsupported, ValueError, IndexError):
        return unsupported("STATE_FORMAT_OR_ENCRYPTION")


def block_header(raw):
    # .blk has no identifying magic. Even valid-looking headers do not prove
    # plaintext, writer version, checksum, live count, deletion map or index state.
    if raw.startswith(b"cmp"):
        return unsupported("COMPRESSED_BLOCK")
    if not raw:
        return {"status": "EMPTY_BLOCK_UNVERSIONED", "canonical_count": False}
    if len(raw) < 30 or len(raw) > MAX_BYTES:
        return unsupported("BLOCK_FORMAT_OR_ENCRYPTION")
    length, sequence, timestamp, subject_size = struct.unpack_from("<IQQH", raw)
    headers = bool(length & (1 << 31))
    length &= (1 << 31) - 1
    if length < 30 + subject_size + (4 if headers else 0) or length > len(raw) or length >= 1 << 30:
        return unsupported("BLOCK_FORMAT_OR_ENCRYPTION")
    return {"status": "HEADER_IDENTIFICATION_ONLY", "canonical_count": False, "deletion_semantics_unknown": True,
            "writer_version_verified": False, "native_checksum_verified": False}


def open_regular(guard, path, dir_fd=None):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NOATIME | os.O_NONBLOCK | os.O_CLOEXEC,
                 dir_fd=dir_fd)
    if not stat.S_ISREG(os.fstat(fd).st_mode):
        os.close(fd)
        raise guard.Unsafe("SOURCE_ROW_CHANGED")
    return fd


def protected_report(guard, path=REPORT):
    guard.trusted_path(path, root_owned=True)
    fd = open_regular(guard, path)
    try:
        before = os.fstat(fd)
        gid = REPORT_GID if REPORT_GID is not None else guard.grp.getgrnam("pmd").gr_gid
        if (not stat.S_ISREG(before.st_mode) or before.st_uid != 0 or before.st_gid != gid
                or stat.S_IMODE(before.st_mode) != 0o440 or before.st_nlink != 1
                or before.st_size != REPORT_BYTES):
            raise guard.Unsafe("REPORT_AUTHORITY")
        raw = os.read(fd, REPORT_BYTES + 1)
        if (len(raw) != REPORT_BYTES or hashlib.sha256(raw).hexdigest() != REPORT_SHA
                or guard.fingerprint(os.fstat(fd)) != guard.fingerprint(before)
                or guard.fingerprint(path.lstat()) != guard.fingerprint(before)):
            raise guard.Unsafe("REPORT_CHANGED")
        return raw
    finally:
        os.close(fd)


def manifest(raw, guard):
    try:
        value = json.loads(raw, object_pairs_hook=unique_object)
        if (set(value) != {"status", "preservation_pass", "canonical_census", "pv", "pvc", "node", "source",
                           "time_utc", "pvc_uid", "pv_uid", "files"}
                or value["status"] != "READONLY_FILE_INVENTORY_COMPLETE"
                or value["preservation_pass"] is not False or value["canonical_census"] is not False
                or value["pv"] != guard.PV or value["pvc"] != guard.PVC or value["node"] != guard.NODE
                or value["source"] != str(guard.SOURCE) or type(value["files"]) is not list
                or len(value["files"]) != FILE_COUNT):
            raise Unsupported()
        seen, total = set(), 0
        for row in value["files"]:
            if (type(row) is not dict or set(row) != {"path", "size", "mtime_ns", "ctime_ns", "inode", "device", "sha256"}
                    or type(row["path"]) is not str or not re.fullmatch(
                        r"jetstream/[A-Z0-9]+/streams/[A-Za-z0-9_-]+/(meta\.(inf|sum)|msgs/[0-9]+\.blk|obs/[A-Za-z0-9_-]+/(meta\.(inf|sum)|o\.dat))", row["path"])
                    or row["path"] in seen or type(row["sha256"]) is not str
                    or not re.fullmatch(r"[0-9a-f]{64}", row["sha256"])):
                raise Unsupported()
            for key in ("size", "mtime_ns", "ctime_ns", "inode", "device"):
                if type(row[key]) is not int or not 0 <= row[key] <= MAX_NUMBER:
                    raise Unsupported()
            if row["size"] > MAX_BYTES:
                raise Unsupported()
            seen.add(row["path"])
            total += row["size"]
        if total > MAX_BYTES:
            raise Unsupported()
        return value
    except (Unsupported, ValueError, TypeError, KeyError, UnicodeError, RecursionError):
        raise guard.Unsafe("REPORT_SCHEMA") from None


def checked_read(guard, root_fd, row):
    """Every path component is FD-relative and checked without following links."""
    components = Path(row["path"]).parts
    if not components or any(part in {".", ".."} for part in components) or Path(row["path"]).is_absolute():
        raise guard.Unsafe("SOURCE_ROW_INVALID")
    parent = os.dup(root_fd)
    fd = None
    try:
        for component in components[:-1]:
            child = guard.open_read(component, True, parent)
            os.close(parent)
            parent = child
        fd = open_regular(guard, components[-1], dir_fd=parent)
        before = os.fstat(fd)
        expected = tuple(row[key] for key in ("device", "inode", "size", "mtime_ns", "ctime_ns"))
        actual = (before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns, before.st_ctime_ns)
        if (not stat.S_ISREG(before.st_mode) or before.st_nlink != 1 or actual != expected
                or before.st_size > MAX_BYTES):
            raise guard.Unsafe("SOURCE_ROW_CHANGED")
        chunks, count = [], 0
        while chunk := os.read(fd, min(65536, before.st_size + 1 - count)):
            chunks.append(chunk)
            count += len(chunk)
            if count > before.st_size:
                raise guard.Unsafe("SOURCE_ROW_CHANGED")
        raw = b"".join(chunks)
        if (count != before.st_size or hashlib.sha256(raw).hexdigest() != row["sha256"]
                or guard.fingerprint(os.fstat(fd)) != guard.fingerprint(before)
                or guard.fingerprint(os.stat(components[-1], dir_fd=parent, follow_symlinks=False)) != guard.fingerprint(before)
                or guard.fingerprint(fresh_row_stat(guard, root_fd, components)) != guard.fingerprint(before)):
            raise guard.Unsafe("SOURCE_ROW_CHANGED")
        return raw
    finally:
        if fd is not None:
            os.close(fd)
        os.close(parent)


def fresh_row_stat(guard, root_fd, components):
    parent = os.dup(root_fd)
    try:
        for component in components[:-1]:
            child = guard.open_read(component, True, parent)
            os.close(parent)
            parent = child
        return os.stat(components[-1], dir_fd=parent, follow_symlinks=False)
    finally:
        os.close(parent)


def bounded_names(guard, fd, already):
    names = []
    with os.scandir(fd) as iterator:
        for entry in iterator:
            if already + len(names) >= MAX_ENTRIES:
                raise guard.Unsafe("SOURCE_BOUND")
            names.append(entry.name)
    return sorted(names)


def source_census(guard):
    """Bounded metadata-only census; never open unselected or oversized bytes."""
    found, selected, entries = [], {}, 0
    def walk(fd, relative, depth):
        nonlocal entries
        if depth > 12:
            raise guard.Unsafe("SOURCE_BOUND")
        before = os.fstat(fd)
        names = bounded_names(guard, fd, entries)
        for name in names:
            entries += 1
            if entries > MAX_ENTRIES:
                raise guard.Unsafe("SOURCE_BOUND")
            s = os.stat(name, dir_fd=fd, follow_symlinks=False)
            path = relative / name
            found.append((str(path), guard.fingerprint(s)))
            if stat.S_ISDIR(s.st_mode):
                child = guard.open_read(name, True, fd)
                try:
                    if guard.fingerprint(os.fstat(child)) != guard.fingerprint(s):
                        raise guard.Unsafe("SOURCE_ROW_CHANGED")
                    walk(child, path, depth + 1)
                finally:
                    os.close(child)
            elif not stat.S_ISREG(s.st_mode):
                raise guard.Unsafe("SOURCE_ROW_CHANGED")
            elif guard.selected(relative, name):
                if s.st_nlink != 1 or s.st_size > MAX_BYTES or len(selected) >= FILE_COUNT:
                    raise guard.Unsafe("SOURCE_BOUND")
                selected[str(path)] = (s.st_dev, s.st_ino, s.st_size, s.st_mtime_ns, s.st_ctime_ns)
        if bounded_names(guard, fd, 0) != names or guard.fingerprint(os.fstat(fd)) != guard.fingerprint(before):
            raise guard.Unsafe("SOURCE_ROW_CHANGED")
    fd = guard.open_read(guard.SOURCE, True)
    try:
        walk(fd, Path(), 0)
    finally:
        os.close(fd)
    return found, selected


def observations(rows, read):
    output = []
    for index, row in enumerate(rows):
        raw = read(row)
        path = Path(row["path"])
        if path.name == "meta.inf":
            projected = project_meta(raw, "consumer" if "obs" in path.parts else "stream")
        elif path.name == "meta.sum":
            projected = {"status": "NOT_VERIFIED_HIGHWAYHASH64" if re.fullmatch(b"[0-9a-f]{16}", raw)
                         else "UNSUPPORTED_NATIVE_SUM", "native_checksum_verified": False}
        elif path.name == "o.dat":
            projected = consumer_state(raw)
        else:
            projected = block_header(raw)
        output.append({"row_index": index, "path_sha256": hashlib.sha256(row["path"].encode()).hexdigest(),
                       "bytes": row["size"], "sha256": row["sha256"], "observation": projected})
    return output


def load_guard():
    path = Path(__file__).with_name("inspector.py")
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC)
    try:
        s = os.fstat(fd)
        if not stat.S_ISREG(s.st_mode) or s.st_uid != 0 or stat.S_IMODE(s.st_mode) != 0o600 or s.st_size > MAX_BYTES:
            raise Unsupported()
        raw = os.read(fd, MAX_BYTES + 1)
        if len(raw) != s.st_size or hashlib.sha256(raw).hexdigest() != INSPECTOR_SHA:
            raise Unsupported()
    finally:
        os.close(fd)
    module = types.ModuleType("pinned_retained_guard")
    module.__file__ = str(path)
    exec(compile(raw, str(path), "exec"), module.__dict__)
    module.trusted_path(path.parent, root_owned=True)
    return module


def main():
    guard = load_guard()
    guard.require_root()
    if len(sys.argv) != 1 or guard.socket.gethostname() != guard.NODE:
        raise guard.Unsafe("ROOT_NODE_NO_ARGS")
    for path in (guard.CONFIG, guard.K3S, guard.SOURCE.parent, guard.OUTPUT):
        guard.trusted_path(path, root_owned=True)
    guard.trusted_path(guard.SOURCE)
    authority = protected_report(guard)
    accepted = manifest(authority, guard)
    initial = guard.kube_state()
    if accepted["pvc_uid"] != initial[0]["metadata"]["uid"] or accepted["pv_uid"] != initial[1]["metadata"]["uid"]:
        raise guard.Unsafe("REPORT_AUTHORITY")
    guard.no_mounts(guard.SOURCE)
    guard.no_open_handles(guard.SOURCE)
    initial_census, selected = source_census(guard)
    expected = {row["path"]: tuple(row[key] for key in ("device", "inode", "size", "mtime_ns", "ctime_ns"))
                for row in accepted["files"]}
    if selected != expected:
        raise guard.Unsafe("SOURCE_INVENTORY_CHANGED")
    source_before = guard.SOURCE.stat()
    fd = guard.open_read(guard.SOURCE, True)
    try:
        if guard.fingerprint(os.fstat(fd)) != guard.fingerprint(source_before):
            raise guard.Unsafe("SOURCE_ROW_CHANGED")
        result = observations(accepted["files"], lambda row: checked_read(guard, fd, row))
        if (guard.fingerprint(os.fstat(fd)) != guard.fingerprint(source_before)
                or guard.fingerprint(guard.SOURCE.stat()) != guard.fingerprint(source_before)):
            raise guard.Unsafe("SOURCE_ROW_CHANGED")
    finally:
        os.close(fd)
    guard.no_open_handles(guard.SOURCE)
    guard.no_mounts(guard.SOURCE)
    if source_census(guard)[0] != initial_census or protected_report(guard) != authority:
        raise guard.Unsafe("SOURCE_INVENTORY_CHANGED")
    fd = guard.open_read(guard.SOURCE, True)
    try:
        for row in accepted["files"]:
            checked_read(guard, fd, row)  # Fresh bounded byte hash match after guards.
    finally:
        os.close(fd)
    guard.no_open_handles(guard.SOURCE)
    guard.no_mounts(guard.SOURCE)
    if source_census(guard)[0] != initial_census:
        raise guard.Unsafe("SOURCE_INVENTORY_CHANGED")
    final = guard.kube_state()
    if initial[0] != final[0] or initial[1] != final[1]:
        raise guard.Unsafe("KUBERNETES_CHANGED")
    report = {"status": "STORED_LAYOUT_OBSERVATIONS_COMPLETE", "canonical_census": False,
              "preservation_pass": False, "writer_version_verified": False, "inventory_sha256": REPORT_SHA,
              "inspector_sha256": INSPECTOR_SHA, "selected_files": FILE_COUNT,
              "logical_message_count": None, "observations": result}
    os.umask(0o077)
    private = Path(tempfile.mkdtemp(prefix="stored-observations-", dir=guard.OUTPUT))
    target = private / "observations.json"
    fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, "w") as output:
        json.dump(report, output, sort_keys=True)
        output.write("\n")
        output.flush()
        os.fsync(output.fileno())
    gid = guard.grp.getgrnam("pmd").gr_gid
    os.chown(target, 0, gid); os.chmod(target, 0o440)
    os.chown(private, 0, gid); os.chmod(private, 0o750)
    print(str(target))


PUBLIC_CODES = frozenset("""REPORT_AUTHORITY REPORT_CHANGED REPORT_SCHEMA SOURCE_ROW_INVALID
SOURCE_ROW_CHANGED SOURCE_BOUND ROOT_NODE_NO_ARGS SOURCE_INVENTORY_CHANGED KUBERNETES_CHANGED
ROOT_LINUX_REQUIRED PATH_INVALID SYMLINK UNTRUSTED_ROOT_PATH SOURCE_MOUNTED OPEN_SOURCE_HANDLE
MAPPED_SOURCE MOUNTINFO_UNCERTAIN PROCESS_SCAN_UNSTABLE PROCESS_IDENTITY_CHANGED
PROCESS_IDENTITY_UNCERTAIN EMPTY_NAMESPACE_UNPROVEN HOST_ROOT_UNPROVEN MOUNT_VIEW_CHANGED
SOURCE_MAPPING_UNCERTAIN SOURCE_DEVICE_CHANGED PV_PROVENANCE NODE_AFFINITY POD_MOUNT
KUBERNETES_UNCERTAIN HANDLE_MAPPING_UNCERTAIN""".split())


def run_cli():
    try:
        main()
    except Exception as error:
        result = {"status": "STORED_OBSERVATIONS_FAILED_NO_ACCEPTANCE", "code": "UNEXPECTED_FAILURE"}
        if len(error.args) == 1 and type(error.args[0]) is str and error.args[0] in PUBLIC_CODES:
            result["code"] = error.args[0]
        for key, limit in (("pid", 2**31 - 1), ("errno", 4095)):
            value = getattr(error, key, None)
            if type(value) is int and 0 < value <= limit:
                result[key] = value
        print(json.dumps(result, sort_keys=True), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(run_cli())
