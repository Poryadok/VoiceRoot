"""Historical actual-row contract RED through the production-shared J guard.

This is a fixture, not a producer/resume API or production admission. Setup or
provenance failure has exit2 and can never stand in for the assertion RED(exit1).
Execution requires independent CODE review and the caller's explicit test stage.
"""
import hashlib
import errno
import importlib.util
import json
import os
from pathlib import Path, PurePosixPath
import re
import shlex
import shutil
import stat
import subprocess
import sys
import tempfile
import tarfile
import zipfile

OPAQUE_UTILITY_PREFIX = b'F13_OPAQUE_UTILITY_OBSERVATION_v1 '
OPAQUE_UTILITY_NAMESPACE = b'F13_OPAQUE_UTILITY_'

ROOT = Path(__file__).resolve().parents[2]
FIXTURE = ROOT / 'scripts/ci/testdata/f13-exact-j-547'
ARCHIVE_SHA = 'ba6a775f6c38664d8603ac8080abca5c5e464d865b2e94b297127b6217549bd1'
ARCHIVE_BYTES = 92933183
SOURCE = '547df286d3e8c5846b09885a6c21d0610b42d7f5'
TARGET = 'c6a7876239ddbf70ac1d084c64a1e3f95eeda846'
BASE = 'ea845cc695b3cde2a13ba9a97345dd78795833e0'
TRACE_SHA = '08259a181aca2a185a3310aa2f745f663fc6a0baddc458418136b02da02e4fe8'
MAP_SHA = 'f088e093d271ea39b5394be0dd217d1d17f99782b8f7349d39be5eac0dde9e45'
LEDGER_SHA = '974d3d65066541c254645d87bb0939e66e30341dd59ea47e5310755c45b1ed17'
PENDING_SHA = 'd54ddd7340b0de4b1e9aed91286ccc1e5148bad7b8f832af898b452245da552a'
OBSERVED = Path('/tmp/F13-author-547')
J_CAPTURE = '/out/actual/observed/fixed3071-J-ledger.json'
J_ORIGINAL_RECORD = '/tmp/F13-author-547/fixed3071-J-ledger.json'
J_RECORD_BYTES = 109549632
PATH = 'src/backend/space/internal/grpcsvc/audit.go'
OPERATION = 'F5/3071ddad390443de4119c796cdad9899ff08f7fd/2'
OUTER_SOURCE_HASHES = {
    'scripts/ci/f13_f54_history.py': 'ead5eb6b3c0548f5b7d64ed332929cafdb3adbe1860e517ab8fe46a9837db27f',
    'scripts/ci/f13_f54_history_test.py': '6f3f21e921a3b68e2263940b169cf8a4b99ed2f8ff84b4042a215ef433495c08',
    'scripts/ci/voice-db-runtime-provisioning-contract_test.sh': 'e2c4d941f08e89737ecfd4298d7eac638f574998c054961413112edf7a14e4d2',
    '.agent/execplans/native-p3-master-source.md': 'e341dd23c6c1833e2a33cd7e306cff4d618c9772b9b97453c66accd809403afc',
}
HARNESS_SOURCE_PATHS = list(OUTER_SOURCE_HASHES) + [
    'scripts/ci/f13_exact_j_historical_red_test.py', 'scripts/ci/f13_guarded_merge_test.py',
    'scripts/ci/f13_guarded_correction_test.py', 'Makefile',
    'scripts/ci/testdata/f13-exact-j-547/source-unit.json',
    'scripts/ci/testdata/f13-exact-j-547/git-2.39.5-data.tar.gz',
    'scripts/ci/testdata/f13-exact-j-547/git-2.39.5-data-manifest.json',
]
GIT_ARCHIVE_SHA = '6326cc23ae91825a592ae6b4c70e34b40566661128b357cc6642404f7ed87462'
GIT_MANIFEST_SHA = '2450a593c024b8ce6d03ed97c5db6c60b397b90ec5c57d3e8d5c5a9287bf41ff'
GIT_COMMANDS = ('cat-file', 'checkout', 'clone', 'config', 'diff', 'diff-tree', 'hash-object',
                'index-pack', 'ls-tree', 'merge-base', 'merge-tree', 'rev-list', 'rev-parse', 'show', 'version')
UTILITY_DIRS = ('/usr/local/bin', '/usr/bin', '/bin')
UTILITY_DIAGNOSTIC_NAMES = frozenset(('python', 'python3', 'python3.11', 'python3.12', 'python3.13',
    'bash', 'sh', 'env', 'sudo', 'unshare', 'mount', 'umount', 'chroot', 'cat', 'cut', 'tr', 'sed',
    'awk', 'grep', 'sort', 'head', 'tail', 'wc', 'sha256sum', 'readlink', 'realpath', 'find', 'xargs',
    'dirname', 'basename', 'printf', 'stat', 'id', 'groups', 'cp', 'mkdir', 'git', 'git-index-pack', 'git-rev-list'))
UTILITY_DIAGNOSTIC_ROOTS = ('/usr/bin', '/usr/local/bin', '/bin')
UTILITY_DIAGNOSTIC_BYTES = 16384
UTILITY_DIAGNOSTIC_ENTRIES = 32


class SetupFailure(Exception):
    pass


def require(value, reason):
    if not value:
        raise SetupFailure(reason)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def run(args, **kwargs):
    p = subprocess.run(args, capture_output=True, **kwargs)
    require(p.returncode == 0, 'command:' + repr(args) + ':' + p.stderr.decode(errors='replace')[-2000:])
    return p.stdout


def pinned_manifest():
    raw = (FIXTURE / 'git-2.39.5-data-manifest.json').read_bytes()
    require(sha(raw) == GIT_MANIFEST_SHA, 'pinned_Git_manifest')
    manifest = json.loads(raw)
    require(len(manifest['files']) == 5, 'pinned_Git_five_files')
    return {item['path']: item for item in manifest['files']}


def expected_utilities():
    utilities = {}
    for directory in UTILITY_DIRS:
        source = Path(directory)
        if source.is_dir():
            for path in sorted(source.iterdir()):
                if path.name != 'git' and not path.name.startswith('git-') and path.name not in utilities:
                    if path.is_file() and os.access(path, os.X_OK):
                        utilities[path.name] = str(path)
    return utilities


def utility_diagnostic_token(value, name=False):
    if not isinstance(value, str):
        return {'status': 'invalid_type'}
    if name and value in UTILITY_DIAGNOSTIC_NAMES:
        return {'name': value}
    if not name:
        for root in UTILITY_DIAGNOSTIC_ROOTS:
            relative = value[len(root) + 1:] if value.startswith(root + '/') else ''
            if relative in UTILITY_DIAGNOSTIC_NAMES:
                return {'root': root, 'relative': relative}
    raw = value.encode('utf-8', errors='surrogatepass')
    return {'sha256': sha(raw), 'utf8_bytes': len(raw)}


def utility_diagnostic_access(value):
    if not isinstance(value, str) or not value.startswith('/') or '\0' in value or len(value) > 4096:
        return {'status': 'not_inspected'}
    try:
        path = Path(value)
        return dict(status='inspected', is_symlink=path.is_symlink(), is_file=path.is_file(),
                    executable=os.access(path, os.X_OK), resolved=utility_diagnostic_token(str(path.resolve())))
    except Exception:
        # Exception text may contain an arbitrary pathname; never serialize it.
        return {'status': 'metadata_unavailable'}


def utility_view_diagnostic(base, actual, expected, entry_count, entry_names=()):
    """Output-only observation of cached predicate inputs, never authority."""
    encode = lambda value: json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode()
    expected_keys = set(expected) if expected is not None else set()
    missing = expected_keys - set(actual) if expected is not None else set()
    extra = set(actual) - expected_keys if expected is not None else set()
    changed = {key for key in expected_keys & set(actual) if expected[key] != actual[key]}
    unmatched_slots = set(entry_names) - set(actual)
    groups = os.getgroups()
    numeric_groups = sorted(value for value in groups if type(value) is int and 0 <= value <= 4294967295)
    report = dict(kind='F13_UTILITY_VIEW_DIAGNOSTIC_v1', status='mismatch',
                  expected_status='evaluated' if expected is not None else 'not_evaluated_short_circuit',
                  counts=dict(expected=len(expected) if expected is not None else None, actual=len(actual),
                              entries=entry_count, non_symlink=entry_count - len(actual),
                              missing=len(missing) if expected is not None else None,
                              extra=len(extra) if expected is not None else None,
                              changed=len(changed) if expected is not None else None),
                  mappings=dict(actual_sha256=sha(encode(actual)),
                                expected_sha256=sha(encode(expected)) if expected is not None else None),
                  uid=os.getuid(), gid=os.getgid(), groups=numeric_groups[:16],
                  groups_total=len(groups), groups_invalid=len(groups) - len(numeric_groups),
                  entries=[], truncated=False, omitted=0)
    differences = sorted(missing | extra | changed | unmatched_slots)
    for key in differences[:UTILITY_DIAGNOSTIC_ENTRIES]:
        item = dict(tool=utility_diagnostic_token(key, name=True),
                    difference='missing' if key in missing else 'extra' if key in extra else
                    'changed' if key in changed else 'not_in_actual_symlink_snapshot')
        for label, mapping in [('expected', expected), ('actual', actual)]:
            if mapping is not None and key in mapping:
                item[label] = dict(target=utility_diagnostic_token(mapping[key]), access=utility_diagnostic_access(mapping[key]))
        # Name is only used for a direct child metadata lookup when it is a
        # valid filesystem basename. It is never interpolated into output.
        if isinstance(key, str) and key not in ('', '.', '..') and '/' not in key and '\0' not in key and len(key) <= 255:
            try:
                slot = base / 'util' / key
                item['slot'] = dict(is_symlink=slot.is_symlink(), is_file=slot.is_file(),
                                    executable=os.access(slot, os.X_OK))
            except Exception:
                item['slot'] = {'status': 'metadata_unavailable'}
        candidate = dict(report, entries=report['entries'] + [item])
        if len(encode(candidate)) > UTILITY_DIAGNOSTIC_BYTES - 256:
            break
        report['entries'].append(item)
    report['omitted'] = len(differences) - len(report['entries'])
    report['truncated'] = report['omitted'] > 0
    require(len(encode(report)) < UTILITY_DIAGNOSTIC_BYTES, 'utility_diagnostic_bound')
    return report


def emit_utility_view_diagnostic(base, actual, expected, entry_count, entry_names=()):
    try:
        value = utility_view_diagnostic(base, actual, expected, entry_count, entry_names)
        line = json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True)
    except Exception:
        line = '{"kind":"F13_UTILITY_VIEW_DIAGNOSTIC_v1","status":"diagnostic_unavailable"}'
    try:
        sys.stderr.write(line + '\n')
    except Exception:
        pass  # Output failure must not mask the original require/SETUP2 below.

    # Only the original source-checked inner invocation carries this binding.
    # Unknown names/paths remain opaque here; new disclosure belongs to outer.
    try:
        identity = json.loads(os.environ['F13_HARNESS_IDENTITY'])
        raw = encode_opaque_observation(opaque_utility_observation(
            base, actual, expected, entry_count, entry_names, identity))
        sys.stderr.write(raw.decode('ascii'))
    except Exception:
        pass

    emit_paired_utility_observation(base, actual, expected, entry_count, entry_names)


def diagnostic_json(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode('ascii')


def diagnostic_units(value):
    """Count metadata/entry/mapping/closure/token objects, not just tool rows."""
    if isinstance(value, dict):
        return 1 + sum(diagnostic_units(item) for item in value.values())
    if isinstance(value, list):
        return sum(diagnostic_units(item) for item in value)
    return 0


def opaque_utility_token(value):
    raw = value.encode('utf-8', errors='surrogatepass')
    return dict(sha256=sha(raw), utf8_bytes=len(raw))


def public_catalog_from_lists(lists):
    """Pure bounded parser; filesystem authority is checked by the reader."""
    paths, total, count = set(), 0, 0
    for raw in lists:
        count += 1
        total += len(raw)
        if count > 4096 or len(raw) > 1048576 or total > 33554432 or b'\0' in raw:
            raise ValueError('catalog_bound')
        text = raw.decode('utf-8')
        if raw and not raw.endswith(b'\n'):
            raise ValueError('catalog_truncated')
        for path in text.split('\n'):
            if re.fullmatch(r'/(?:usr/bin|bin|usr/sbin|sbin)/[A-Za-z0-9][A-Za-z0-9+_.-]{0,95}', path):
                paths.add(path)
        if len(paths) > 8192:
            raise ValueError('catalog_bound')
    result = tuple(sorted(paths))
    if len(('\n'.join(result) + ('\n' if result else '')).encode('ascii')) > 131072:
        raise ValueError('catalog_bound')
    return result


_PUBLIC_CONTEXT_FAILURE_REASONS = (
    'catalog_open_failed', 'catalog_permissions_or_acl_failed',
    'catalog_directory_limit', 'catalog_list_open_failed',
    'catalog_list_trust_failed', 'catalog_list_bytes_or_changed',
    'catalog_canonical_failed', 'catalog_changed',
    'catalog_descriptor_exhausted', 'context_snapshot_failed',
    'context_snapshot_limit', 'context_build_failed',
    'context_catalog_unavailable',
)


def record_public_context_failure(failure, reason, preserve=False):
    """Private optional observation only; never inspect exception text or paths."""
    try:
        if type(failure) is not dict or type(reason) is not str or reason not in _PUBLIC_CONTEXT_FAILURE_REASONS:
            return
        old = failure.get('reason')
        if preserve and type(old) is str and old in _PUBLIC_CONTEXT_FAILURE_REASONS:
            return
        failure['reason'] = reason
    except Exception:
        pass  # Recording cannot change an original result or exception.


def emit_public_context_failure(status, failure):
    """Only a fixed source-site observation after the existing owned mismatch gate."""
    try:
        if type(status) is not str or status != 'context_unavailable' or type(failure) is not dict:
            return
        reason = failure.get('reason')
        if type(reason) is not str or reason not in _PUBLIC_CONTEXT_FAILURE_REASONS:
            return
        sys.stderr.buffer.write(diagnostic_json(dict(kind='F13_PUBLIC_UTILITY_CONTEXT_FAILURE_v1', reason=reason)) + b'\n')
    except Exception:
        pass  # Optional output never changes the original guard result.


def read_public_utility_catalog(failure=None):
    """Observed installed-package catalog; no caller path or exception output."""
    descriptors, snapshots, files = [], [], []
    phase = 'catalog_open_failed'
    def checked(fd, directory):
        info = os.fstat(fd)
        if info.st_uid != 0 or info.st_mode & 0o022 or not (stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode)):
            raise ValueError('catalog_permissions')
        # Unknown ACL semantics/errors fail closed; do not infer absent ACL.
        for attribute in ('system.posix_acl_access', 'system.posix_acl_default') if directory else ('system.posix_acl_access',):
            try:
                os.getxattr(fd, attribute)
            except OSError as error:
                if error.errno != errno.ENODATA:
                    raise
            else:
                raise ValueError('catalog_acl')
        return (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid,
                info.st_size, info.st_mtime_ns, info.st_ctime_ns)
    try:
        flags = os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC
        fd = os.open('/', flags | os.O_DIRECTORY)
        descriptors.append(fd)
        phase = 'catalog_permissions_or_acl_failed'
        snapshots.append(checked(fd, True))
        for name in ('var', 'lib', 'dpkg', 'info'):
            phase = 'catalog_open_failed'
            fd = os.open(name, flags | os.O_DIRECTORY, dir_fd=fd)
            descriptors.append(fd)
            phase = 'catalog_permissions_or_acl_failed'
            snapshots.append(checked(fd, True))
        phase = 'catalog_open_failed'
        names = []
        with os.scandir(fd) as entries:
            for entry in entries:
                names.append(entry.name)
                if len(names) > 4096:
                    phase = 'catalog_directory_limit'
                    raise ValueError('catalog_bound')
        def lists():
            nonlocal phase
            total = 0
            for name in sorted(names):
                if not name.endswith('.list'):
                    continue
                # NONBLOCK avoids blocking a raced FIFO before fstat rejection.
                phase = 'catalog_list_open_failed'
                item = os.open(name, flags | os.O_NONBLOCK, dir_fd=fd)
                files.append((item, None))
                phase = 'catalog_list_trust_failed'
                before = checked(item, False)
                files[-1] = (item, before)
                phase = 'catalog_list_bytes_or_changed'
                chunks, size = [], 0
                while size <= 1048576 and total + size <= 33554432:
                    chunk = os.read(item, min(65536, 1048577 - size, 33554433 - total - size))
                    if not chunk:
                        break
                    chunks.append(chunk)
                    size += len(chunk)
                total += size
                if size > 1048576 or total > 33554432 or before != checked(item, False):
                    raise ValueError('catalog_bound_or_changed')
                phase = 'catalog_canonical_failed'
                yield b''.join(chunks)
        phase = 'catalog_canonical_failed'
        result = public_catalog_from_lists(lists())
        phase = 'catalog_changed'
        if any(before != checked(item, True) for item, before in zip(descriptors, snapshots)):
            raise ValueError('catalog_changed')
        if any(before != checked(item, False) for item, before in files):
            raise ValueError('catalog_changed')
        return result
    except Exception as error:
        try:
            if phase in ('catalog_open_failed', 'catalog_list_open_failed') and isinstance(error, OSError) and error.errno in (errno.EMFILE, errno.ENFILE):
                phase = 'catalog_descriptor_exhausted'
        except Exception:
            pass
        record_public_context_failure(failure, phase)
        return None
    finally:
        for fd, _ in reversed(files):
            os.close(fd)
        for fd in reversed(descriptors):
            os.close(fd)


def public_diagnostic_context(catalog, actual, identity):
    """Private same-run outer record, never populated from candidate fields."""
    canonical = ('\n'.join(catalog) + ('\n' if catalog else '')).encode('ascii')
    if len(catalog) > 8192 or len(canonical) > 131072 or len(actual) > 8192 or len(diagnostic_json(actual)) > 2097152:
        raise ValueError('snapshot_bound')
    if tuple(sorted(set(catalog))) != tuple(catalog) or public_catalog_from_lists([canonical]) != tuple(catalog):
        raise ValueError('catalog_schema')
    paths, names = {}, {}
    def add(index, text):
        token = opaque_utility_token(text)
        key = (token['sha256'], token['utf8_bytes'])
        index.setdefault(key, set()).add(text)
    for path in catalog:
        add(paths, path)
        add(names, path.rsplit('/', 1)[1])
    # Preserve only the already-reviewed fixed allowlist's exact disclosures.
    for name in UTILITY_DIAGNOSTIC_NAMES:
        add(names, name)
        for root in UTILITY_DIAGNOSTIC_ROOTS:
            add(paths, root + '/' + name)
    return dict(nonce=os.urandom(32).hex(), source_sha256=sha(diagnostic_json(identity)),
                catalog_sha256=sha(canonical), catalog_bytes=len(canonical), catalog_count=len(catalog),
                actual_sha256=sha(diagnostic_json(actual)), actual_count=len(actual), paths=paths, names=names)


def prepare_public_diagnostic_context(root, identity, failure=None):
    phase = 'context_catalog_unavailable'
    try:
        catalog = read_public_utility_catalog() if failure is None else read_public_utility_catalog(failure)
        if catalog is None:
            record_public_context_failure(failure, 'context_catalog_unavailable', preserve=True)
            return None
        phase = 'context_snapshot_failed'
        base = root / 'historical-git' / 'util'
        actual = {}
        encoded_bytes = 2
        for count, path in enumerate(base.iterdir(), 1):
            if count > 8192:
                record_public_context_failure(failure, 'context_snapshot_limit')
                return None
            if path.is_symlink():
                target = str(path.readlink())
                encoded_bytes += len(diagnostic_json({path.name: target})) - 2 + bool(actual)
                if encoded_bytes > 2097152:
                    record_public_context_failure(failure, 'context_snapshot_limit')
                    return None
                actual[path.name] = target
        phase = 'context_build_failed'
        return public_diagnostic_context(catalog, actual, identity)
    except Exception:
        record_public_context_failure(failure, phase)
        return None


PAIRED_UTILITY_PREFIX = b'F13_PAIRED_UTILITY_OBSERVATION_v1 '
PAIRED_UTILITY_NAMESPACE = b'F13_PAIRED_UTILITY_'


def paired_credentials():
    groups = sorted(os.getgroups())
    if len(groups) > 32:
        raise ValueError('credential_bound')
    return sha(diagnostic_json([os.getuid(), os.getgid(), groups]))


def paired_referent_observation(value):
    """Subsequent metadata only; no raw paths/errors or predicate substitution."""
    current, visited = value, set()
    result = dict(state='not_inspected', error='NONE', path=None, metadata_sha256=None, executable=None)
    for _ in range(8):
        if type(current) is not str or not current.startswith('/') or len(current) > 4096 or '\0' in current:
            return result
        result['path'] = opaque_utility_token(current)
        if current in visited:
            return dict(result, state='loop')
        visited.add(current)
        try:
            info = os.lstat(current)
            if stat.S_ISLNK(info.st_mode):
                target = os.readlink(current)
                current = target if target.startswith('/') else os.path.dirname(current) + '/' + target
                continue
            metadata = [info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid,
                        info.st_size, info.st_mtime_ns, info.st_ctime_ns]
            return dict(result, state='regular' if stat.S_ISREG(info.st_mode) else 'non_regular',
                        metadata_sha256=sha(diagnostic_json(metadata)), executable=os.access(current, os.X_OK))
        except OSError as error:
            codes = {errno.ENOENT: ('missing', 'ENOENT'), errno.EACCES: ('denied', 'EACCES'),
                     errno.EPERM: ('denied', 'EPERM'), errno.ENOTDIR: ('not_directory', 'ENOTDIR'),
                     errno.ELOOP: ('loop', 'ELOOP'), errno.EIO: ('io_error', 'EIO')}
            state, code = codes.get(error.errno, ('unavailable', 'OTHER'))
            return dict(result, state=state, error=code)
        except Exception:
            return dict(result, state='unavailable', error='OTHER')
    return dict(result, state='limit')


def prepare_paired_utility_context(root, identity):
    """Private same-run POST snapshot, independent of optional public catalog."""
    try:
        actual, probes, encoded = {}, {}, 2
        entry_count = 0
        for entry_count, path in enumerate((root / 'historical-git' / 'util').iterdir(), 1):
            if entry_count > 8192:
                return None
            if path.is_symlink():
                target = str(path.readlink())
                encoded += len(diagnostic_json({path.name: target})) - 2 + bool(actual)
                if encoded > 2097152:
                    return None
                actual[path.name] = target
        probe_bytes = 2
        for name, target in actual.items():
            probe = paired_referent_observation(target)
            probe_bytes += len(diagnostic_json({name: probe})) - 2 + bool(probes)
            if encoded + probe_bytes > 2097152:
                return None
            probes[name] = probe
        result = dict(actual=actual, probes=probes, actual_sha256=sha(diagnostic_json(actual)),
                      actual_count=len(actual), entry_count=entry_count,
                      source_sha256=sha(diagnostic_json(identity)), credential_sha256=paired_credentials())
        return result if len(diagnostic_json(result)) <= 2097152 else None
    except Exception:
        return None


def paired_utility_observation(base, actual, expected, entry_count, entry_names, identity):
    if len(actual) > 8192 or (expected is not None and len(expected) > 8192) or len(diagnostic_json(actual)) > 2097152:
        raise ValueError('snapshot_bound')
    original = opaque_utility_observation(base, actual, expected, entry_count, entry_names, identity)
    index = {}
    for name in actual:
        token = opaque_utility_token(name)
        index.setdefault((token['sha256'], token['utf8_bytes']), []).append(name)
    result = {key: original[key] for key in ('source_sha256', 'actual_sha256', 'actual_count', 'entry_count')}
    result.update(credential_sha256=paired_credentials(), entries=[], omitted=original['omitted'] + len(original['entries']))
    for old in original['entries']:
        names = index.get((old['tool']['sha256'], old['tool']['utf8_bytes']), [])
        if len(names) != 1 or 'actual' not in old:
            continue
        name = names[0]
        entry = dict(tool=old['tool'], target=old['actual']['target'], difference=old['difference'],
                     probe=paired_referent_observation(actual[name]))
        trial = dict(result, entries=result['entries'] + [entry], omitted=result['omitted'] - 1)
        if diagnostic_units(trial) > 32 or len(PAIRED_UTILITY_PREFIX) + len(diagnostic_json(trial)) + 1 > 16384:
            break
        result = trial
    return result


def encode_paired_utility_observation(value):
    raw = PAIRED_UTILITY_PREFIX + diagnostic_json(value) + b'\n'
    if len(raw) > 16384 or diagnostic_units(value) > 32:
        raise ValueError('paired_bound')
    return raw


def emit_paired_utility_observation(base, actual, expected, entry_count, entry_names):
    try:
        identity = json.loads(os.environ['F13_HARNESS_IDENTITY'])
        value = paired_utility_observation(base, actual, expected, entry_count, entry_names, identity)
        sys.stderr.write(encode_paired_utility_observation(value).decode('ascii'))
    except Exception:
        pass  # Optional opaque output cannot change original failure semantics.


def resolve_owned_paired_output(stdout, stderr, context):
    """Only owning result streams; remove this NEW namespace before old resolver."""
    candidates = []
    def suppress(raw):
        kept = []
        for line in raw.splitlines(keepends=True):
            if line.startswith(PAIRED_UTILITY_NAMESPACE):
                candidates.append(line)
            else:
                kept.append(line)
        return b''.join(kept)
    stdout, stderr = suppress(stdout), suppress(stderr)
    try:
        if context is None or len(candidates) != 1 or coarse_public_diagnostic_status(stdout, stderr, None, None) is None:
            return stdout, stderr, None
        raw = candidates[0]
        if not raw.startswith(PAIRED_UTILITY_PREFIX) or not raw.endswith(b'\n') or len(raw) > 16384:
            raise ValueError('framing')
        def unique(items):
            result = {}
            for key, value in items:
                if key in result:
                    raise ValueError('duplicate')
                result[key] = value
            return result
        value = json.loads(raw[len(PAIRED_UTILITY_PREFIX):].decode('ascii'), object_pairs_hook=unique)
        if encode_paired_utility_observation(value) != raw:
            raise ValueError('canonical')
        def exact(obj, keys):
            if type(obj) is not dict or set(obj) != set(keys):
                raise ValueError('schema')
        def number(n, limit):
            if type(n) is not int or not 0 <= n <= limit:
                raise ValueError('number')
        def digest(text):
            if type(text) is not str or not re.fullmatch('[0-9a-f]{64}', text):
                raise ValueError('digest')
        def token(item):
            exact(item, ('sha256', 'utf8_bytes'))
            digest(item['sha256'])
            number(item['utf8_bytes'], 16777216)
            return item['sha256'], item['utf8_bytes']
        def probe(item):
            exact(item, ('state', 'error', 'path', 'metadata_sha256', 'executable'))
            if item['state'] not in ('not_inspected', 'regular', 'non_regular', 'missing', 'denied', 'not_directory', 'loop', 'io_error', 'unavailable', 'limit'):
                raise ValueError('state')
            if item['error'] not in ('NONE', 'ENOENT', 'EACCES', 'EPERM', 'ENOTDIR', 'ELOOP', 'EIO', 'OTHER'):
                raise ValueError('errno')
            if item['path'] is not None:
                token(item['path'])
            if item['metadata_sha256'] is not None:
                digest(item['metadata_sha256'])
            if item['executable'] is not None and type(item['executable']) is not bool:
                raise ValueError('access')
            allowed = {'not_inspected': ('NONE',), 'regular': ('NONE',), 'non_regular': ('NONE',),
                       'missing': ('ENOENT',), 'denied': ('EACCES', 'EPERM'), 'not_directory': ('ENOTDIR',),
                       'loop': ('NONE', 'ELOOP'), 'io_error': ('EIO',), 'unavailable': ('OTHER',), 'limit': ('NONE',)}
            if item['error'] not in allowed[item['state']]:
                raise ValueError('state_errno')
            if item['state'] in ('regular', 'non_regular'):
                if item['metadata_sha256'] is None or type(item['executable']) is not bool or item['path'] is None:
                    raise ValueError('metadata')
            elif item['metadata_sha256'] is not None or item['executable'] is not None:
                raise ValueError('unavailable_metadata')
            if item['state'] != 'not_inspected' and item['path'] is None:
                raise ValueError('path')
        exact(value, ('source_sha256', 'actual_sha256', 'actual_count', 'entry_count', 'credential_sha256', 'entries', 'omitted'))
        for key in ('source_sha256', 'actual_sha256'):
            digest(value[key])
            if value[key] != context[key]:
                raise ValueError('binding')
        digest(value['credential_sha256'])
        for key in ('actual_count', 'entry_count'):
            number(value[key], 8192)
            if value[key] != context[key]:
                raise ValueError('binding')
        number(value['omitted'], 8192)
        if type(value['entries']) is not list:
            raise ValueError('entries')
        index = {}
        for name in context['actual']:
            key = token(opaque_utility_token(name))
            index.setdefault(key, []).append(name)
        entries, seen = [], set()
        for entry in value['entries']:
            exact(entry, ('tool', 'target', 'difference', 'probe'))
            key = token(entry['tool'])
            names = index.get(key, [])
            if key in seen or len(names) != 1:
                raise ValueError('slot')
            seen.add(key)
            name = names[0]
            if token(entry['target']) != token(opaque_utility_token(context['actual'][name])):
                raise ValueError('target')
            if entry['difference'] not in ('extra', 'changed'):
                raise ValueError('difference')
            probe(entry['probe'])
            outer = context['probes'][name]
            probe(outer)
            entries.append(dict(tool=entry['tool'], target=entry['target'], difference=entry['difference'],
                                outer=outer, inner=entry['probe']))
        receipt = dict(kind='F13_PAIRED_UTILITY_CONTEXT_RECEIPT_v1', source_sha256=context['source_sha256'],
                       post_materialization_actual_sha256=context['actual_sha256'], actual_count=context['actual_count'],
                       observation_sha256=sha(raw), outer_observation='POST_MATERIALIZATION_METADATA',
                       inner_observation='SUBSEQUENT_MISMATCH_METADATA', credentials_equal=context['credential_sha256'] == value['credential_sha256'],
                       entries=entries, omitted=value['omitted'])
        while receipt['entries'] and (diagnostic_units(receipt) > 32 or len(diagnostic_json(receipt)) + 1 > 16384):
            receipt['entries'].pop()
            receipt['omitted'] += 1
        if diagnostic_units(receipt) > 32 or len(diagnostic_json(receipt)) + 1 > 16384:
            raise ValueError('receipt_bound')
        return stdout, stderr, receipt
    except Exception:
        return stdout, stderr, None


def opaque_utility_closure(value):
    edges, visited, current = [], set(), value
    for _ in range(8):
        if current in visited:
            return dict(edges=edges, status='loop')
        visited.add(current)
        if not current.startswith('/') or len(current) > 4096 or '\0' in current:
            return dict(edges=edges, status='not_inspected')
        try:
            info = os.lstat(current)
            if not stat.S_ISLNK(info.st_mode):
                return dict(edges=edges, status='complete')
            target = os.readlink(current)
        except OSError:
            return dict(edges=edges, status='unavailable')
        edges.append(opaque_utility_token(target))
        current = target if target.startswith('/') else os.path.dirname(current) + '/' + target
    return dict(edges=edges, status='limit')


def opaque_utility_observation(base, actual, expected, entry_count, entry_names, identity):
    keys = set(expected) if expected is not None else set()
    missing = keys - set(actual) if expected is not None else set()
    extra = set(actual) - keys if expected is not None else set()
    changed = {key for key in keys & set(actual) if expected[key] != actual[key]}
    unmatched = set(entry_names) - set(actual)
    differences = sorted(missing | extra | changed | unmatched)
    result = dict(source_sha256=sha(diagnostic_json(identity)), actual_sha256=sha(diagnostic_json(actual)),
                  actual_count=len(actual), entry_count=entry_count,
                  expected_status='evaluated' if expected is not None else 'not_evaluated_short_circuit',
                  entries=[], omitted=len(differences))
    units = 1  # Root metadata is included in the global structural budget.
    for key in differences:
        item = dict(tool=opaque_utility_token(key), difference='missing' if key in missing else
                    'extra' if key in extra else 'changed' if key in changed else 'not_in_actual_symlink_snapshot')
        for label, mapping in (('actual', actual), ('expected', expected)):
            if mapping is not None and key in mapping:
                closure = opaque_utility_closure(mapping[key])
                item[label] = dict(target=opaque_utility_token(mapping[key]), closure=closure)
        size = diagnostic_units(item)
        if units + size > 32:
            break
        trial = dict(result, entries=result['entries'] + [item], omitted=result['omitted'] - 1)
        if len(OPAQUE_UTILITY_PREFIX) + len(diagnostic_json(trial)) + 1 > 16384:
            break
        result = trial
        units += size
    return result


def encode_opaque_observation(value):
    raw = OPAQUE_UTILITY_PREFIX + diagnostic_json(value) + b'\n'
    if len(raw) > 16384:
        raise ValueError('observation_bound')
    return raw


def resolve_owned_utility_output(stdout, stderr, context):
    """Called only on the owning invocation's captured result, before forwarding."""
    candidates = []
    def suppress(raw):
        kept = []
        for line in raw.splitlines(keepends=True):
            if line.startswith(OPAQUE_UTILITY_NAMESPACE):
                candidates.append(line)
            else:
                kept.append(line)
        return b''.join(kept)
    stdout, stderr = suppress(stdout), suppress(stderr)
    try:
        if context is None or len(candidates) != 1:
            return stdout, stderr, None
        raw = candidates[0]
        if not raw.startswith(OPAQUE_UTILITY_PREFIX) or not raw.endswith(b'\n') or len(raw) > 16384:
            raise ValueError('framing')
        def pairs(items):
            result = {}
            for key, value in items:
                if key in result:
                    raise ValueError('duplicate_key')
                result[key] = value
            return result
        value = json.loads(raw[len(OPAQUE_UTILITY_PREFIX):].decode('ascii'), object_pairs_hook=pairs)
        if encode_opaque_observation(value) != raw:
            raise ValueError('noncanonical_observation')
        if diagnostic_units(value) > 32:
            raise ValueError('entry_bound')
        def exact(obj, keys):
            if not isinstance(obj, dict) or set(obj) != set(keys):
                raise ValueError('schema')
        def number(number, limit):
            if type(number) is not int or not 0 <= number <= limit:
                raise ValueError('number')
        def digest(text):
            if not isinstance(text, str) or not re.fullmatch('[0-9a-f]{64}', text):
                raise ValueError('digest')
        exact(value, ('source_sha256', 'actual_sha256', 'actual_count', 'entry_count', 'expected_status', 'entries', 'omitted'))
        for key in ('source_sha256', 'actual_sha256'):
            digest(value[key])
            if value[key] != context[key]:
                raise ValueError('binding')
        for key in ('actual_count', 'entry_count', 'omitted'):
            number(value[key], 8192)
        if value['actual_count'] != context['actual_count'] or value['entry_count'] < value['actual_count']:
            raise ValueError('binding')
        if value['expected_status'] not in ('evaluated', 'not_evaluated_short_circuit') or not isinstance(value['entries'], list):
            raise ValueError('schema')
        units = 0
        def token(item, name=False):
            nonlocal units
            exact(item, ('sha256', 'utf8_bytes'))
            digest(item['sha256'])
            number(item['utf8_bytes'], 16777216)
            units += 1
            if units > 32:
                raise ValueError('entry_bound')
            result = dict(item)
            matches = context['names' if name else 'paths'].get((item['sha256'], item['utf8_bytes']), set())
            if len(matches) == 1:
                result['basename' if name else 'path'] = next(iter(matches))
                result['qualification'] = 'OUTER_CATALOG_RESOLUTION_OF_CHILD_TOKEN'
            return result
        entries = []
        for entry in value['entries']:
            if not isinstance(entry, dict) or not {'tool', 'difference'} <= set(entry) or not set(entry) <= {'tool', 'difference', 'actual', 'expected'}:
                raise ValueError('schema')
            if entry['difference'] not in ('missing', 'extra', 'changed', 'not_in_actual_symlink_snapshot'):
                raise ValueError('difference')
            result = dict(tool=token(entry['tool'], True), difference=entry['difference'])
            for label in ('actual', 'expected'):
                if label in entry:
                    data = entry[label]
                    exact(data, ('target', 'closure'))
                    closure = data['closure']
                    exact(closure, ('edges', 'status'))
                    if closure['status'] not in ('loop', 'complete', 'not_inspected', 'unavailable', 'limit') or not isinstance(closure['edges'], list) or len(closure['edges']) > 8:
                        raise ValueError('closure')
                    result[label] = dict(target=token(data['target']), closure=dict(
                        edges=[token(edge) for edge in closure['edges']], status=closure['status'],
                        observation='subsequent_metadata_not_predicate_snapshot'))
            entries.append(result)
        receipt = dict(kind='F13_PUBLIC_UTILITY_OUTER_RESOLUTION_RECEIPT_v1', origin='owned_outer_child',
                       nonce=context['nonce'], source_sha256=context['source_sha256'],
                       catalog_sha256=context['catalog_sha256'], catalog_bytes=context['catalog_bytes'], catalog_count=context['catalog_count'],
                       post_materialization_actual_sha256=context['actual_sha256'], actual_count=context['actual_count'],
                       observation_sha256=sha(raw), expected_status=value['expected_status'], entry_count=value['entry_count'],
                       entries=entries, omitted=value['omitted'])
        while (len(diagnostic_json(receipt)) + 1 > 16384 or diagnostic_units(receipt) > 32) and receipt['entries']:
            receipt['entries'].pop()
            receipt['omitted'] += 1
        if len(diagnostic_json(receipt)) + 1 > 16384 or diagnostic_units(receipt) > 32:
            raise ValueError('receipt_bound')
        return stdout, stderr, receipt
    except Exception:
        return stdout, stderr, None


def coarse_public_diagnostic_status(stdout, stderr, context, public_receipt):
    """Fixed outer observation only; never infer mismatch from an absent receipt."""
    try:
        def unique_fields(items):
            result = {}
            for key, value in items:
                if key in result:
                    raise ValueError('duplicate_field')
                result[key] = value
            return result
        mismatch = False
        for stream in (stdout, stderr):
            for line in stream.splitlines():
                if len(line) > 16384:
                    continue
                try:
                    value = json.loads(line.decode('ascii'), object_pairs_hook=unique_fields)
                except (ValueError, UnicodeError):
                    continue
                if isinstance(value, dict) and value.get('kind') == 'F13_UTILITY_VIEW_DIAGNOSTIC_v1' and value.get('status') == 'mismatch':
                    mismatch = True
        if not mismatch:
            return None
        if context is None:
            return 'context_unavailable'
        if public_receipt is None:
            return 'candidate_rejected_or_not_selected'
        if (not isinstance(public_receipt, dict) or
                public_receipt.get('kind') != 'F13_PUBLIC_UTILITY_OUTER_RESOLUTION_RECEIPT_v1' or
                not isinstance(public_receipt.get('entries'), list) or
                diagnostic_units(public_receipt) > 32 or len(diagnostic_json(public_receipt)) + 1 > 16384):
            return None
        def qualified(value):
            if isinstance(value, dict):
                if (value.get('qualification') == 'OUTER_CATALOG_RESOLUTION_OF_CHILD_TOKEN' and
                        any(isinstance(value.get(key), str) for key in ('basename', 'path'))):
                    return True
                return any(qualified(item) for item in value.values())
            if isinstance(value, list):
                return any(qualified(item) for item in value)
            return False
        return None if qualified(public_receipt['entries']) else 'catalog_match_failed'
    except Exception:
        return None


def emit_coarse_public_diagnostic_status(status):
    try:
        if type(status) is str and status in ('context_unavailable', 'candidate_rejected_or_not_selected', 'catalog_match_failed'):
            sys.stderr.buffer.write(diagnostic_json(dict(kind='F13_PUBLIC_UTILITY_DIAGNOSTIC_STATUS_v1', status=status)) + b'\n')
    except Exception:
        pass  # Optional fixed metadata cannot mask the original guard result.


def verify_pinned_git(root, runtime=True):
    base = root / 'historical-git'
    require(not base.is_symlink(), 'pinned_Git_root_path')
    for name, item in pinned_manifest().items():
        path = base / name
        require(base.resolve() in path.resolve().parents and not path.is_symlink(), 'pinned_Git_path')
        current = path
        while current != root:
            require(not current.is_symlink(), 'pinned_Git_path')
            current = current.parent
        require(path.is_file() and stat.S_IMODE(path.stat().st_mode) == 0o555, 'pinned_Git_mode')
        raw = path.read_bytes()
        require(len(raw) == item['bytes'] and sha(raw) == item['sha256'] and raw[:4] == b'\x7fELF', 'pinned_Git_bytes')
    if runtime:
        require(all(not (base / name).is_symlink() and stat.S_IMODE((base / name).stat().st_mode) == 0o555
                    for name in ('bin', 'lib', 'launch', 'exec', 'util', 'templates')), 'pinned_Git_runtime_directories')
        require((base / 'bin/git-elf').read_bytes() == (base / 'bin/git').read_bytes() and
                not (base / 'bin/git-elf').is_symlink(), 'pinned_Git_loader_target')
        for folder, names in [('launch', {'git'}), ('exec', {'git', 'git-index-pack', 'git-rev-list'})]:
            require({p.name for p in (base / folder).iterdir()} == names, 'pinned_Git_closed_execpath')
            for name in names:
                path = base / folder / name
                require(not path.is_symlink() and path.read_bytes() == git_launcher().encode() and
                        stat.S_IMODE(path.stat().st_mode) == 0o555, 'pinned_Git_launcher_bytes')
        actual = {p.name: str(p.readlink()) for p in (base / 'util').iterdir() if p.is_symlink()}
        utility_entries = list((base / 'util').iterdir())
        entry_count = len(utility_entries)
        expected = None
        valid_view = entry_count == len(actual) and actual == (expected := expected_utilities())
        if not valid_view:
            emit_utility_view_diagnostic(base, actual, expected, entry_count, [p.name for p in utility_entries])
        require(valid_view, 'pinned_Git_private_utility_view')
        require(not any((base / 'templates').iterdir()), 'pinned_Git_empty_templates')
    return base


def git_launcher():
    # The real immutable ELF handles ALL commands, including --version. No output
    # or status is synthesized. --argv0 preserves Git's own dashed dispatch.
    commands = '|'.join(GIT_COMMANDS)
    return '''#!/bin/sh
set -eu
here=${0%/*}; base=${here%/*}; name=${0##*/}
reject() { printf '%s\\n' 'F13_PINNED_GIT_SETUP_FAILURE' >&2; exit 125; }
case "$here" in "$base/launch"|"$base/exec") ;; *) reject ;; esac
[ "${GIT_EXEC_PATH-}" = "$base/exec" ] || reject
[ "${GIT_TEMPLATE_DIR-}" = "$base/templates" ] || reject
rest=${PATH-}
while :; do
 case "$rest" in "$base/exec:"*) rest=${rest#"$base/exec:"} ;; *) break ;; esac
done
[ "$rest" = "$base/launch:$base/util" ] || reject
[ "${GIT_CONFIG_NOSYSTEM-}" = 1 ] || reject
[ "${GIT_CONFIG_GLOBAL-}" = /dev/null ] || reject
[ "${GIT_ALLOW_PROTOCOL-}" = file ] || reject
[ -z "${LD_PRELOAD-}${LD_LIBRARY_PATH-}${LD_AUDIT-}" ] || reject
/usr/bin/env | while IFS='=' read -r key value; do
 case "$key" in
  GIT_EXEC_PATH|GIT_TEMPLATE_DIR|GIT_CONFIG_NOSYSTEM|GIT_CONFIG_GLOBAL|GIT_ALLOW_PROTOCOL) ;;
  GIT_DIR) [ "$value" = "${base%/*}/repo/.git" ] || exit 125 ;;
  GIT_PREFIX) [ -z "$value" ] || exit 125 ;;
  GIT_NO_REPLACE_OBJECTS) [ "$value" = 1 ] || exit 125 ;;
  GIT_*|LD_*) exit 125 ;;
 esac
done || reject
case "$name" in
 git) command=; skip=0
  for arg do
   if [ "$skip" = 1 ]; then skip=0; continue; fi
   case "$arg" in
    -C) skip=1 ;;
    --no-replace-objects) ;;
    --version) command=version; break ;;
    -*) reject ;;
    *) command=$arg; break ;;
   esac
  done
  [ "$skip" = 0 ] || reject ;;
 git-index-pack) command=index-pack ;;
 git-rev-list) command=rev-list ;;
 *) reject ;;
esac
case "$command" in ''' + commands + ''') ;; *) reject ;; esac
if [ "$command" = clone ]; then
 [ "$#" = 4 ] && [ "$1" = clone ] && [ "$2" = --no-checkout ] &&
 [ "$3" = "${base%/*}/out/prelaunch/private-target.bundle" ] && [ "$4" = "${base%/*}/repo" ] || reject
fi
if [ "$command" = checkout ]; then
 [ "$#" = 5 ] && [ "$1" = -C ] && [ "$2" = "${base%/*}/repo" ] &&
 [ "$3" = checkout ] && [ "$4" = --detach ] && [ "$5" = c6a7876239ddbf70ac1d084c64a1e3f95eeda846 ] || reject
fi
exec "$base/lib/ld-linux-x86-64.so.2" --library-path "$base/lib" --argv0 "$name" "$base/bin/git-elf" "$@"
'''


def materialize_pinned_git(root):
    root = root.resolve()
    base = root / 'historical-git'
    require(not base.exists() and not base.is_symlink(), 'pinned_Git_no_overwrite')
    archive = FIXTURE / 'git-2.39.5-data.tar.gz'
    require(archive.stat().st_size == 3082106 and sha(archive.read_bytes()) == GIT_ARCHIVE_SHA, 'pinned_Git_archive')
    expected = pinned_manifest()
    with tarfile.open(archive, 'r:gz') as tar:
        members = tar.getmembers()
        require(len(members) == 5 and {m.name for m in members} == set(expected), 'pinned_Git_exact_members')
        # Validate every member and byte before creating ANY executable.
        files = {}
        for member in members:
            item = expected[member.name]
            require(member.isfile() and not member.issym() and not member.islnk() and
                    member.mode == 0o555 and member.size == item['bytes'], 'pinned_Git_member_type_mode_size')
            require(not PurePosixPath(member.name).is_absolute() and '..' not in PurePosixPath(member.name).parts,
                    'pinned_Git_member_path')
            raw = tar.extractfile(member).read()
            require(len(raw) == item['bytes'] and sha(raw) == item['sha256'] and raw[:4] == b'\x7fELF', 'pinned_Git_member_SHA')
            files[member.name] = raw
    for name, raw in files.items():
        path = base / name
        path.parent.mkdir(parents=True, exist_ok=True)
        with path.open('xb') as stream:
            stream.write(raw)
        path.chmod(0o555)
    verify_pinned_git(root, runtime=False)
    # Retain the raw binary at its reviewed name for verification; invoke a
    # separate exact-byte hardlink via the transparent loader launcher.
    os.link(base / 'bin/git', base / 'bin/git-elf')
    for folder in ('exec', 'util', 'templates'):
        (base / folder).mkdir()
    launcher = git_launcher().encode()
    # bin/git remains the immutable manifest-bound ELF. Public entrypoints live
    # in a dedicated bin view, so no review-bound binary is overwritten.
    (base / 'launch').mkdir()
    for folder, names in [('launch', ['git']), ('exec', ['git', 'git-index-pack', 'git-rev-list'])]:
        for name in names:
            path = base / folder / name
            path.write_bytes(launcher)
            path.chmod(0o555)
    # Launcher location is one level below base just like exec. This private
    # launch directory is the sole normal Git PATH entry.
    for name, target in expected_utilities().items():
        (base / 'util' / name).symlink_to(target)
    for directory in sorted(base.rglob('*'), reverse=True):
        if directory.is_dir():
            directory.chmod(0o555)
    base.chmod(0o555)
    return base


def pinned_git_environment(root):
    base = verify_pinned_git(root)
    require(not any(key.startswith(('LD_', 'GIT_')) and key not in
                    ('GIT_EXEC_PATH', 'GIT_TEMPLATE_DIR', 'GIT_CONFIG_NOSYSTEM', 'GIT_CONFIG_GLOBAL', 'GIT_ALLOW_PROTOCOL')
                    for key in os.environ), 'pinned_Git_inherited_override')
    desired = dict(PATH=str(base / 'launch') + ':' + str(base / 'util'), GIT_EXEC_PATH=str(base / 'exec'),
                   GIT_TEMPLATE_DIR=str(base / 'templates'), GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL='/dev/null', GIT_ALLOW_PROTOCOL='file')
    require(all(key not in os.environ or os.environ[key] == value for key, value in desired.items() if key != 'PATH'),
            'pinned_Git_context_override')
    env = dict(os.environ, **desired)
    return env


def verify_checkout_inputs(repo, target):
    values = run(['git', '-C', str(repo), 'config', '--null', '--list']).split(b'\0')
    allowed = ('core.repositoryformatversion', 'core.filemode', 'core.bare', 'core.logallrefupdates',
               'remote.origin.url', 'remote.origin.fetch')
    for record in filter(None, values):
        key = record.split(b'\n', 1)[0].decode()
        require(key in allowed or re.fullmatch(r'branch\.[A-Za-z0-9_./-]+\.(remote|merge)', key), 'pinned_Git_unsafe_config:' + key)
    hooks = repo / '.git/hooks'
    require(not hooks.exists() or not any(hooks.iterdir()), 'pinned_Git_hooks_absent')
    require(not (repo / '.git/objects/info/alternates').exists() and
            not (repo / '.git/info/attributes').exists(), 'pinned_Git_no_unbound_child_inputs')
    names = run(['git', '-C', str(repo), 'ls-tree', '-rz', '--name-only', target]).split(b'\0')
    for raw_name in filter(None, names):
        name = raw_name.decode()
        if PurePosixPath(name).name == '.gitattributes':
            attributes = run(['git', '-C', str(repo), 'show', target + ':' + name]).decode()
            require(not any(re.search(r'(?:^|\s)(?:filter|diff|merge|working-tree-encoding)=', line)
                            for line in attributes.splitlines() if not line.lstrip().startswith('#')),
                    'pinned_Git_unbound_attribute_driver:' + name)


def capability_differences(captured, actual, path='$'):
    """Diagnostic paths only; never adjusts a captured authority value."""
    if type(captured) is not type(actual):
        return [path]
    if isinstance(captured, dict):
        differences = []
        for key in sorted(set(captured) | set(actual)):
            if key not in captured or key not in actual:
                differences.append(path + '.' + key)
            else:
                differences.extend(capability_differences(captured[key], actual[key], path + '.' + key))
        return differences
    if isinstance(captured, list):
        if len(captured) != len(actual):
            return [path]
        return [difference for i, (left, right) in enumerate(zip(captured, actual))
                for difference in capability_differences(left, right, path + '[' + str(i) + ']')]
    return [] if captured == actual else [path]


def materialize_J_record_alias(root, logical):
    """Lossless replay of one captured file at its original consumer pathname.

    This creates no ready value or qualification; the original J reconstruction
    below still must independently equal this captured raw transport.
    """
    require(logical.get(J_CAPTURE) == (LEDGER_SHA, J_RECORD_BYTES), 'J_record_declared_binding')
    root = root.resolve()
    source, target = root / J_CAPTURE.lstrip('/'), root / J_ORIGINAL_RECORD.lstrip('/')
    for path in (source, target):
        require(root in path.resolve().parents, 'J_record_private_path')
        current = path
        while current != root:
            require(not current.is_symlink(), 'J_record_no_symlink')
            current = current.parent
    require(source.is_file(), 'J_record_captured_source_required')
    require(not target.exists(), 'J_record_no_overwrite')
    raw = source.read_bytes()
    require(len(raw) == J_RECORD_BYTES and sha(raw) == LEDGER_SHA, 'J_record_raw_binding')
    target.parent.mkdir(parents=True, exist_ok=True)
    with target.open('xb') as stream:
        stream.write(raw)


def materialize(root):
    """Strict lossless private-root extraction; never writes a host logical path."""
    raw = (FIXTURE / 'historical-inputs.zip').read_bytes()
    require(len(raw) == ARCHIVE_BYTES and sha(raw) == ARCHIVE_SHA, 'immutable_archive')
    with zipfile.ZipFile(FIXTURE / 'historical-inputs.zip') as archive:
        manifest_raw = archive.read('transport-manifest.json')
    require(len(manifest_raw) == 54004, 'fixed_archive_manifest_bytes')
    m = json.loads(manifest_raw)
    require(m['unique_objects'] == 227 and m['uncompressed_object_bytes'] == 400559753, 'fixed_extract_bounds')
    objects = {}
    logical = {}
    for item in m['logical_files']:
        name, digest, size = item['logical_path'], item['sha256'], item['bytes']
        p = PurePosixPath(name)
        require(p.is_absolute() and '..' not in p.parts and str(p) == name, 'literal_logical_path')
        require(name.startswith(('/out/', '/inputs/', '/unit/', '/fixed3071-unit/', '/original/', '/tmp/F13-author-547/')), 'closed_logical_roots')
        require(re.fullmatch('[0-9a-f]{64}', digest) and type(size) is int and size >= 0, 'object_shape')
        require(name not in logical or logical[name] == (digest, size), 'conflicting_alias')
        require(digest not in objects or objects[digest] == size, 'conflicting_object')
        logical[name] = (digest, size)
        objects[digest] = size
    require(len(objects) == 227 and sum(objects.values()) == 400559753 and len(logical) == 246, 'complete_fixed_manifest')
    with zipfile.ZipFile(FIXTURE / 'historical-inputs.zip') as z:
        names = ['objects/' + x for x in objects] + ['transport-manifest.json']
        require(len(z.infolist()) == len(names) and set(z.namelist()) == set(names), 'closed_archive_entries')
        for info in z.infolist():
            require(not stat.S_ISLNK(info.external_attr >> 16) and not info.is_dir(), 'no_symlink_or_directory')
            expected = len(manifest_raw) if info.filename == 'transport-manifest.json' else objects[info.filename[8:]]
            require(info.file_size == expected, 'bounded_uncompressed_entry')
        require(z.read('transport-manifest.json') == manifest_raw, 'archive_manifest_exact')
        for name, (digest, size) in logical.items():
            dest = root / name.lstrip('/')
            require(root.resolve() in dest.resolve().parents and not dest.exists(), 'private_no_overwrite')
            data = z.read('objects/' + digest)
            require(len(data) == size and sha(data) == digest, 'raw_object_binding')
            dest.parent.mkdir(parents=True, exist_ok=True)
            dest.write_bytes(data)
    for item in m['expected_absences']:
        require(not (root / item['logical_path'].lstrip('/')).exists(), 'keep_actual_absence')
    materialize_J_record_alias(root, logical)
    materialize_pinned_git(root)
    return m


def verify_private_devices():
    """Validate only the fixed sealed kernel endpoints; never retain RNG bytes."""
    try:
        require(sys.platform == 'linux' and ROOT == Path('/harness'), 'private_device_namespace')
        require(os.geteuid() != 0 and os.getuid() == os.geteuid() and
                os.getgid() == os.getegid(), 'private_device_identity')

        def bounded(path, limit):
            with Path(path).open('rb') as stream:
                raw = stream.read(limit + 1)
            require(len(raw) <= limit, 'private_device_metadata_bound')
            return raw.decode('ascii')

        mounts = bounded('/proc/self/mountinfo', 131072).splitlines()
        require(len(mounts) <= 4096, 'private_device_mount_bound')
        selected = []
        for line in mounts:
            fields = line.split()
            require(len(fields) >= 10 and fields.count('-') == 1, 'private_device_mount_shape')
            separator = fields.index('-')
            require(separator >= 6 and len(fields) == separator + 4, 'private_device_mount_shape')
            if fields[4] == '/dev':
                selected.append((fields, separator))
        require(len(selected) == 1, 'private_device_exact_mount')
        fields, separator = selected[0]
        mount_flags = set(fields[5].split(','))
        fs_flags = set(fields[separator + 3].split(','))
        require(fields[3] == '/' and fields[separator + 1] == 'tmpfs' and
                fields[separator + 2] == 'f13-private-dev' and
                {'ro', 'nosuid', 'noexec'} <= mount_flags and 'ro' in fs_flags and
                not {'rw', 'nodev'} & (mount_flags | fs_flags), 'private_device_mount_flags')
        vfs = os.statvfs('/dev')
        require(vfs.f_flag & os.ST_RDONLY and vfs.f_flag & os.ST_NOSUID and
                vfs.f_flag & os.ST_NOEXEC and not vfs.f_flag & os.ST_NODEV and
                0 < vfs.f_blocks * vfs.f_frsize <= 65536 and
                0 < vfs.f_files <= 8, 'private_device_filesystem')
        directory = os.lstat('/dev')
        require(stat.S_ISDIR(directory.st_mode) and directory.st_uid == 0 and
                directory.st_gid == 0 and stat.S_IMODE(directory.st_mode) == 0o555,
                'private_device_directory')
        names = set()
        with os.scandir('/dev') as entries:
            for index, entry in enumerate(entries):
                require(index < 2, 'private_device_entry_bound')
                names.add(entry.name)
        require(names == {'null', 'urandom'}, 'private_device_entries')
        for path in ('/dev', '/dev/null', '/dev/urandom'):
            for attribute in ('system.posix_acl_access', 'system.posix_acl_default'):
                try:
                    os.getxattr(path, attribute, follow_symlinks=False)
                except OSError as error:
                    require(error.errno in (errno.ENODATA, errno.ENOTSUP, errno.EOPNOTSUPP),
                            'private_device_acl_inspection')
                else:
                    raise SetupFailure('private_device_unexpected_acl')
        capabilities = {}
        for line in bounded('/proc/self/status', 16384).splitlines():
            key, separator, value = line.partition(':')
            if key in ('CapEff', 'CapPrm', 'CapInh', 'CapAmb'):
                value = value.strip()
                require(separator and key not in capabilities and
                        re.fullmatch('[0-9a-fA-F]{1,16}', value), 'private_device_capability_shape')
                capabilities[key] = int(value, 16)
        require(set(capabilities) == {'CapEff', 'CapPrm', 'CapInh', 'CapAmb'} and
                not any(capabilities.values()), 'private_device_capabilities')
        node = os.lstat('/dev/urandom')
        require(stat.S_ISCHR(node.st_mode) and node.st_rdev == os.makedev(1, 9) and
                node.st_uid == 0 and node.st_gid == 0 and
                stat.S_IMODE(node.st_mode) == 0o444, 'private_device_entropy_node')
        null = os.lstat('/dev/null')
        require(stat.S_ISCHR(null.st_mode) and null.st_rdev == os.makedev(1, 3) and
                null.st_uid == 0 and null.st_gid == 0 and
                stat.S_IMODE(null.st_mode) == 0o666, 'private_device_null_node')
        flags = os.O_NOFOLLOW | os.O_CLOEXEC
        descriptor = os.open('/dev/urandom', os.O_RDONLY | flags)
        try:
            opened = os.fstat(descriptor)
            attributes = ('st_dev', 'st_ino', 'st_mode', 'st_rdev', 'st_uid', 'st_gid')
            require(all(getattr(opened, key) == getattr(node, key) for key in attributes),
                    'private_device_open_identity')
            require(len(os.read(descriptor, 32)) == 32, 'private_device_entropy_read')
        finally:
            os.close(descriptor)
        for access in (os.O_WRONLY, os.O_RDWR):
            try:
                descriptor = os.open('/dev/urandom', access | flags)
            except OSError as error:
                require(error.errno in (errno.EACCES, errno.EPERM, errno.EROFS),
                        'private_device_write_denial')
            else:
                os.close(descriptor)
                raise SetupFailure('private_device_writable_entropy')
        descriptor = os.open('/dev/null', os.O_RDWR | flags)
        try:
            opened = os.fstat(descriptor)
            require(all(getattr(opened, key) == getattr(null, key) for key in attributes),
                    'private_device_null_identity')
            require(os.read(descriptor, 1) == b'' and os.write(descriptor, b'') == 0,
                    'private_device_null_semantics')
        finally:
            os.close(descriptor)
    except SetupFailure:
        raise
    except Exception:
        raise SetupFailure('private_device_inspection') from None


def prepare():
    """Pure-Python fixture construction in the final ordinary-user chroot."""
    require(sys.platform == 'linux' and ROOT == Path('/harness'), 'private_namespace_required')
    require(os.geteuid() != 0, 'ordinary_runner_uid_required')
    harness_identity = json.loads(os.environ['F13_HARNESS_IDENTITY'])
    require(re.fullmatch('[0-9a-f]{40}', harness_identity['commit']), 'explicit_frozen_testhead')
    require(set(harness_identity['files']) == set(HARNESS_SOURCE_PATHS), 'complete_testhead_source_binding')
    require(all(sha((ROOT / name).read_bytes()) == digest for name, digest in harness_identity['files'].items()), 'mounted_testhead_same_bytes')
    materialize(Path('/'))
    return 0


def namespace_main():
    """Mounts exist only in the isolated child's namespace; no Docker/service."""
    require(sys.platform == 'linux', 'Linux_only_no_Windows_test')
    require(os.geteuid() != 0, 'ordinary_runner_uid_required')
    require(shutil.which('sudo') and shutil.which('unshare') and shutil.which('mount'), 'namespace_tools_required')
    head = run(['git', '-C', str(ROOT), 'rev-parse', 'HEAD']).decode().strip()
    source_bindings = {}
    for name in HARNESS_SOURCE_PATHS:
        data = (ROOT / name).read_bytes()
        require(run(['git', '-C', str(ROOT), 'show', head + ':' + name]) == data, 'immutable_testhead_source:' + name)
        source_bindings[name] = sha(data)
    harness_identity = json.dumps(dict(commit=head, files=source_bindings), sort_keys=True)
    with tempfile.TemporaryDirectory(prefix='f13-exact-j-') as folder:
        root = Path(folder) / 'root'
        root.mkdir()
        # Deferred preparation occurs only after the final fixed roots are mounted.
        # No owning outer POST snapshot exists; optional disclosure stays unavailable.
        paired_context = None
        context_failure = {}
        diagnostic_context = None
        for name in ('repo', 'harness', 'tmp', 'dev', 'proc'):
            (root / name).mkdir(exist_ok=True)
        # The runner's already-installed toolchain is mounted read-only. No pull,
        # package installation, host network or mutation of host root directories.
        mounts = []
        for name in ('usr', 'bin', 'lib', 'lib64', 'opt', 'etc'):
            source = Path('/') / name
            if source.exists():
                (root / name).mkdir(exist_ok=True)
                mounts.append((str(source), str(root / name)))
        mounts.append((str(ROOT), str(root / 'harness')))
        script = 'set -eu\nmount --make-rprivate /\n'
        for source, target in mounts:
            script += 'mount --bind ' + shlex.quote(source) + ' ' + shlex.quote(target) + '\n'
            script += 'mount -o remount,bind,ro ' + shlex.quote(target) + '\n'
        devices = shlex.quote(str(root / 'dev'))
        entropy = shlex.quote(str(root / 'dev/urandom'))
        null = shlex.quote(str(root / 'dev/null'))
        # New bounded private filesystem: root DAC, not RO alone, denies device writes.
        script += '/usr/bin/mount -t tmpfs -o size=64k,nr_inodes=8,mode=0555,uid=0,gid=0,nosuid,noexec,dev f13-private-dev ' + devices + '\n'
        script += '/usr/bin/mknod -m 0444 ' + entropy + ' c 1 9\n'
        script += '/usr/bin/chown 0:0 ' + entropy + '\n'
        script += ': > ' + null + '\n'
        script += 'mount --bind /dev/null ' + null + '\n'
        script += '/usr/bin/mount -o remount,ro,nosuid,noexec,dev ' + devices + '\n'
        script += 'mount -t proc proc ' + shlex.quote(str(root / 'proc')) + '\n'
        executable = '/harness/scripts/ci/f13_exact_j_historical_red_test.py'
        invocation = 'chroot --userspec=' + str(os.getuid()) + ':' + str(os.getgid()) + ' ' + shlex.quote(str(root))
        invocation += ' /usr/bin/env -i PATH=/historical-git/launch:/historical-git/util GIT_EXEC_PATH=/historical-git/exec GIT_TEMPLATE_DIR=/historical-git/templates GIT_ALLOW_PROTOCOL=file HOME=/tmp PYTHONDONTWRITEBYTECODE=1 GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null '
        invocation += 'F13_HARNESS_IDENTITY=' + shlex.quote(harness_identity) + ' '
        invocation += shlex.quote(sys.executable) + ' -B ' + executable
        script += invocation + ' --prepare\n'
        toolchain = shlex.quote(str(root / 'historical-git'))
        script += 'test -d ' + toolchain + '\ntest ! -L ' + toolchain + '\n'
        script += 'mount --bind ' + toolchain + ' ' + toolchain + '\n'
        script += 'mount -o remount,bind,ro ' + toolchain + '\n'
        script += 'exec ' + invocation + ' --inside\n'
        # User/mount/network/pid isolation is fixture infrastructure only. There
        # is no listener, published port, engine start, DEV or STAGING operation.
        result = subprocess.run(['sudo', '-n', 'unshare', '--mount', '--net', '--pid', '--fork', 'bash', '-c', script], capture_output=True)
        paired_stdout, paired_stderr, paired_receipt = resolve_owned_paired_output(result.stdout, result.stderr, paired_context)
        stdout, stderr, public_receipt = resolve_owned_utility_output(paired_stdout, paired_stderr, diagnostic_context)
        sys.stdout.buffer.write(stdout)
        sys.stderr.buffer.write(stderr)
        if public_receipt is not None:
            try:
                sys.stderr.buffer.write(diagnostic_json(public_receipt) + b'\n')
            except Exception:
                pass  # Optional output never changes the original guard result.
        emit_coarse_public_diagnostic_status(coarse_public_diagnostic_status(stdout, stderr, diagnostic_context, public_receipt))
        emit_public_context_failure(coarse_public_diagnostic_status(stdout, stderr, diagnostic_context, public_receipt), context_failure)
        if paired_receipt is not None:
            try:
                sys.stderr.buffer.write(diagnostic_json(paired_receipt) + b'\n')
            except Exception:
                pass  # Optional opaque receipt cannot change original guard acceptance.
        receipts = []
        for line in stdout.splitlines():
            try:
                value = json.loads(line)
            except ValueError:
                continue
            if isinstance(value, dict) and value.get('kind') == 'FAITHFUL_HISTORICAL_SHARED_GUARD':
                receipts.append(value)
        # sudo/unshare/chroot exit1 is not the contract assertion RED.
        require(len(receipts) == 1 and result.returncode in (0, 1) and
            receipts[0]['guard_observed'] == result.returncode and receipts[0]['contract_expected'] == 0,
            'namespace_must_return_actual_guard_receipt_not_setup_exit')
        return result.returncode


def historical_module():
    spec = importlib.util.spec_from_file_location('historical547', '/repo/scripts/ci/f13_f54_history.py')
    h = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(h)
    return h


def inside():
    require(sys.platform == 'linux' and ROOT == Path('/harness'), 'private_namespace_required')
    harness_identity = json.loads(os.environ['F13_HARNESS_IDENTITY'])
    require(re.fullmatch('[0-9a-f]{40}', harness_identity['commit']), 'explicit_frozen_testhead')
    require(set(harness_identity['files']) == set(HARNESS_SOURCE_PATHS), 'complete_testhead_source_binding')
    require(all(sha((ROOT / name).read_bytes()) == digest for name, digest in harness_identity['files'].items()), 'mounted_testhead_same_bytes')
    repo = Path('/repo')
    # Validate the complete read-only pinned closure before the first inner Git.
    pinned_git_environment(Path('/'))
    bundle = '/out/prelaunch/private-target.bundle'
    run(['git', 'clone', '--no-checkout', bundle, str(repo)])
    verify_checkout_inputs(repo, TARGET)
    run(['git', '-C', str(repo), 'checkout', '--detach', TARGET])
    git = lambda *args: run(['git', '--no-replace-objects', '-C', str(repo), *args])
    require(git('rev-list', '--parents', '-n', '1', 'HEAD').decode().split() == [TARGET, BASE, SOURCE], 'historical_actual_parents')
    for name, digest in OUTER_SOURCE_HASHES.items():
        require(sha((repo / name).read_bytes()) == digest and (repo / name).read_bytes() == git('show', SOURCE + ':' + name), 'historical_source:' + name)
    unit_raw = (FIXTURE / 'source-unit.json').read_bytes()
    require(sha(unit_raw) == 'e8fce2a60ae0975ddcaee47db3cf36a51e57744194f5885bb8953ee2406a8f02', 'c69_factual_source_unit')
    unit = json.loads(unit_raw)
    require(unit['path'] == PATH and len(unit['versions']) == 12 and len(unit['edges']) == 6, 'whole_exact_J_source_unit')
    for version in unit['versions'].values():
        require(git('ls-tree', '-z', version['ref'], '--', PATH).decode() == version['tree_record'], 'actual_source_mode_type_oid')
        blob = git('show', version['ref'] + ':' + PATH)
        require(len(blob) == version['bytes'] and sha(blob) == version['sha256'], 'actual_source_content')
        require(git('rev-list', '--parents', '-n', '1', version['ref']).decode().split()[1:] == version['ordered_parents'], 'actual_source_ordered_parents')
    for edge in unit['edges'].values():
        require(sha(git('diff', '--raw', '-z', '--no-abbrev', '--no-renames', edge['from'], edge['to'], '--', PATH)) == edge['raw_sha256'], 'actual_full_source_edge')
        require(sha(git('diff', '--no-ext-diff', '--no-renames', '--abbrev=9', edge['from'], edge['to'], '--', PATH)) == edge['patch_sha256'], 'actual_full_source_patch')
    h = historical_module()
    trace = OBSERVED / 'trace.jsonl'
    raw = trace.read_bytes()
    require(sha(raw) == TRACE_SHA, 'actual_full_failed_trace')
    events = [json.loads(line) for line in raw.splitlines()]
    require(len(events) == 10648 and sha(b''.join(raw.splitlines(keepends=True)[:10645])) == PENDING_SHA, 'actual_pending_prefix')
    pending = events[10644]
    require(pending['kind'] == 'row_before' and pending['operation'] == OPERATION and pending['payload']['path'] == PATH, 'actual_same_predicate_pending')
    context = next(e['payload']['spec']['gate_context'] for e in events if e['kind'] == 'manual_F_begin')
    os.environ.update(context['event'])
    # No authority is supplied by a self-consistent captured pathlist/hash.
    expected = h.manual_authority_expected(repo, context)
    h.validate_manual_authority(context, expected)
    captured = json.loads((OBSERVED / 'fixed3071-J-ledger.json').read_bytes())
    require(sha((OBSERVED / 'fixed3071-J-ledger.json').read_bytes()) == LEDGER_SHA, 'captured_ledger_raw')
    prefix = captured['prefix']
    cap_raw = (OBSERVED / 'fixed3071-structural-capabilities.json').read_bytes()
    cap = json.loads(cap_raw)
    actual_cap = h.prepare_3071_author_capabilities(repo, cap['directory'], cap['native_path'])
    differences = capability_differences(cap, actual_cap)
    if differences:
        print(json.dumps(dict(kind='F13_HISTORICAL_CAPABILITY_MISMATCH', paths=differences[:64],
                              total=len(differences), captured_sha256=sha(cap_raw), captured_bytes=len(cap_raw),
                              actual_sha256=sha(h.canonical(actual_cap)), actual_bytes=len(h.canonical(actual_cap)))))
    require(cap == actual_cap and not differences, 'fixed3071_complete_current_capability_binding')
    checked = h.validate_trace(repo, trace, prefix['sha256'], J_checkpoint=OPERATION, sealed_prefix=prefix)
    reconstructed = h.build_3071_J_ready(checked['J_checkpoint'], prefix)
    require(h.canonical(reconstructed) == h.canonical(captured), 'independent_complete_J_not_trusted_import')
    # Fold the complete original failed trace to prove all prior transitions and
    # the exact final map, not just the old J checkpoint or its selected row.
    terminal = h.validate_trace(repo, trace, TRACE_SHA)
    require(terminal['failures'] and terminal['master_wrapper'] is None, 'old_failure_not_success')
    folded = terminal['ledger']['F5']
    map_raw = ''.join(row + '\n' for row in folded['rows']).encode()
    require(sha(map_raw) == MAP_SHA and h.file_digest(map_raw) == pending['payload']['map_before'], 'full_actual_pending_map')
    source_map = OBSERVED / 'f13-nested-map-e6757e8ac26032ac4bad7f4356e91fffbf0725ba-2'
    require(source_map.read_bytes() == map_raw, 'full_map_bytes_not_selected_rows')
    child = next(e['frame'] for e in events if e['kind'] == 'frame_enter' and e['payload']['parent_operation'] == OPERATION)
    nested_rows = [r.split('\t') for r in reconstructed['ledger'][child]['rows'] if r.split('\t')[0] == PATH]
    require(len(nested_rows) == 1, 'actual_child_owner_not_incoming_commit_guess')
    nested = nested_rows[0]
    incoming = pending['payload']['incoming']['raw'].split('\t')
    prior = pending['payload']['prior']['raw'].split('\t')
    require(pending['payload']['prior']['sha256'] == 'bdcd1df8603081f14a80d5627616466f7ede36439915a193e62345d177af8ee1' and
        pending['payload']['incoming']['sha256'] == '51a6d356845a23ab3b4f1e7d362075a284d55aea9d3fb907e7d0ec00a62fb13d', 'c69_literal_actual_eight_field_rows')
    require([r for r in folded['rows'] if r.split('\t')[0] == PATH] == [pending['payload']['prior']['raw']], 'complete_prior_rows')
    require(incoming[4:6] == nested[4:6] and incoming[1] == nested[1], 'original_child_flags_source')
    frame = next(x['context'] for x in reconstructed['frame_ancestry'] if x['frame'] == 'F5')
    op = reconstructed['operation_binding']
    require(op['classification'] == 'merge' and frame['partition_source_history'] == 'false' and
        frame['allow_direct_followup'] == 'true' and frame['base'] == h.AUTHOR_HISTORICAL_SCOPE['base'] and
        frame['target'] == h.AUTHOR_HISTORICAL_SCOPE['target'], 'actual_outer_F5_merge_partition')
    outer = next(x['context'] for x in reconstructed['frame_ancestry'] if x['frame'] == 'F2')
    require(frame['parent'] == 'F2' and frame['parent_operation'] == 'F2/e6757e8ac26032ac4bad7f4356e91fffbf0725ba/2' and
        outer['base'] == BASE and outer['target'] == SOURCE and outer['parent'] is None and
        outer['allow_direct_followup'] == 'true' and outer['partition_source_history'] == 'true', 'actual_F5_parent_not_child_frame')
    parent_operation = next(e for e in events if e['kind'] == 'operation_begin' and e['operation'] == frame['parent_operation'])
    require(parent_operation['frame'] == frame['parent'] and parent_operation['payload']['source_head'] == frame['target'] and
        parent_operation['payload']['boundary'] == frame['base'], 'actual_parent_operation_source_boundary_link')
    require(h.classify(reconstructed['classification']['unapproved_delta']['raw'].splitlines()) == reconstructed['classification']['trigger'], 'actual_full_classifier_recomputed')
    require(PATH not in json.loads(expected['files'][context['values']['native_p3_binding']]['raw'])['paths'], 'non_native_audit_actual_membership')
    require(not expected['values']['game_checkpoint_aggregate_active'] == 'true' or PATH not in expected['files'][expected['values']['game_checkpoint_aggregate_paths']]['raw'].splitlines(), 'non_Game_audit_actual_membership')
    entry = lambda commit: git('ls-tree', commit, '--', PATH).decode().strip()
    require(entry(op['source_head']) == entry(op['commit']), 'ordinary_merge_shortcut_actual_Git')
    # This permits the original ordinary_guarded_merge=false and outcome
    # unverified defaults; no fabricated merge qualification is injected.
    values = dict(expected['values'], **context['values'])
    require(all(values[k] == v for k, v in expected['values'].items()), 'same_independent_authority_values')
    values.update(repo='/repo', path=PATH, output=str(source_map), base=frame['base'], target=frame['target'],
        commit=op['commit'], first_parent=op['first_parent'], source_base=op['boundary'], source_head=op['source_head'],
        nested_owner_commit=nested[2], nested_base=nested[4], nested_head=nested[5], source_entry=entry(op['source_head']),
        content_base=incoming[6], source_base_candidates=','.join(op['candidates']), trigger=incoming[1],
        allow_direct_followup=frame['allow_direct_followup'], is_direct='false', ordinary_guarded_merge='false',
        source_lineage='false', guarded_correction_allowed='false', role_duplicate_allowed='false', space_audit_allowed='false',
        exact_unit_supersession='false', native_supersession_status='1', f13_f54_operation=OPERATION,
        f13_f54_J_operation=OPERATION, f13_f54_J_ledger=str(OBSERVED / 'fixed3071-J-ledger.json'),
        f13_f54_J_ledger_sha=LEDGER_SHA,
        f13_f54_capability_file=str(OBSERVED / 'f54-exact-source-capabilities.json'),
        f13_f54_3071_capability_file=str(OBSERVED / 'fixed3071-structural-capabilities.json'),
        f13_f54_observer='/repo/scripts/ci/f13_f54_history.py', f13_f54_trace_file=str(trace),
        diagnostic_file='/tmp/shared-guard-unexpected-failure')
    M_operations = [e['operation'] for e in events[:10645] if e['kind'] == 'M_ledger_ready']
    # Missing or surprising dynamic state is not silently replaced with a
    # convenient empty M flag. The original M receipt supplies the identity.
    require(len(M_operations) == 1 and M_operations[0] in reconstructed['inherited_M_ledgers'], 'actual_retained_M_dynamic_identity')
    M_binding = next(e['payload']['ledger'] for e in events if e['kind'] == 'M_ledger_ready')
    require(M_binding == h.raw_binding(M_binding['raw']) and
        json.loads(M_binding['raw']) == reconstructed['inherited_M_ledgers'][M_operations[0]], 'actual_reconstructed_M_raw_record')
    M_path = OBSERVED / 'f54-actual-M-ledger.json'
    require(not M_path.exists(), 'no_unbound_M_file_overwrite')
    # Lossless materialization of already validated raw event transport, not a
    # newly assembled/trusted ledger or producer operation.
    M_path.write_bytes(M_binding['raw'].encode())
    values.update(f13_f54_M_operation=M_operations[0],
        f13_f54_M_ledger=str(M_path), f13_f54_M_ledger_sha=M_binding['sha256'])
    new_source = (ROOT / 'scripts/ci/voice-db-runtime-provisioning-contract_test.sh').read_text()
    definitions = h.original_policy_functions(new_source)
    definitions += '\n' + (repo / 'scripts/ci/voice-r22-runtime-scope.sh').read_text()
    # A private function frame supplies independently reconstructed dynamic
    # scope to the SAME two functions used by the real collector. It is not a
    # live collector frame and cannot append/restart any producer operation.
    script = 'set -uo pipefail\n' + definitions + '\nfixture_call() {\n'
    for function in ['f13_evaluate_guarded_owner_qualifiers', 'f13_guarded_owner_transition_allowed',
        'f13_path_is_guarded', 'game_checkpoint_duplicate_owner_supersession_allowed',
        'f13_guarded_correction_qualified', 'native_p3_path_bound', 'native_p3_duplicate_owner_qualified',
        'f13_role_conditional_qualified', 'f13_space_audit_qualified', 'f13_f54_exact_supersession']:
        script += 'declare -F ' + function + ' >/dev/null || return 92\n'
    for key, value in values.items():
        require(re.fullmatch('[A-Za-z_][A-Za-z0-9_]*', key), 'closed_variable_name')
        script += 'local ' + key + '=' + shlex.quote(value) + '\n'
    script += 'local -A owners owner_triggers owner_content_bases owner_source_heads owner_source_entries guarded_entries guarded_entry_commits checkpoint_merge_outcomes\n'
    for key, value in dict(owners=prior[2],owner_triggers=prior[1],owner_content_bases=prior[6],owner_source_heads=prior[5],owner_source_entries=entry(prior[5]),guarded_entries=entry(op['source_head']),guarded_entry_commits=nested[2],checkpoint_merge_outcomes='unverified').items():
        script += key + '[' + shlex.quote(PATH) + ']=' + shlex.quote(value) + '\n'
    script += 'f13_path_is_guarded "$path" || return 90\n'
    script += 'f13_evaluate_guarded_owner_qualifiers "$repo" "$path" "$output" || return 91\n'
    script += 'status=0; f13_guarded_owner_transition_allowed "$guarded_correction_allowed" "$role_duplicate_allowed" "$space_audit_allowed" "$exact_unit_supersession" "$allow_direct_followup" "$is_direct" "$incremental_merge" || status=$?\n'
    script += 'printf "F13_SHARED_GUARD_STATUS=%s\\n" "$status"\n}\nfixture_call\n'
    before = {p: sha((OBSERVED / p).read_bytes()) for p in ['trace.jsonl', 'fixed3071-J-ledger.json', source_map.name]}
    before_files = {str(p):sha(p.read_bytes()) for p in OBSERVED.rglob('*') if p.is_file()}
    result = subprocess.run(['bash'], input=script.encode(), capture_output=True)
    require(result.returncode == 0 and not Path(values['diagnostic_file']).exists(), 'shared_dispatch_setup:' + result.stderr.decode(errors='replace')[-2000:])
    require(all(sha((OBSERVED / name).read_bytes()) == digest for name, digest in before.items()), 'no_trace_ledger_map_write')
    require(before_files == {str(p):sha(p.read_bytes()) for p in OBSERVED.rglob('*') if p.is_file()}, 'unexpected_native_or_other_side_effect_is_setup_failure')
    matches = re.findall(rb'^F13_SHARED_GUARD_STATUS=([01])$', result.stdout, re.M)
    require(len(matches) == 1, 'explicit_shared_guard_status')
    observed = int(matches[0])
    print(json.dumps(dict(kind='FAITHFUL_HISTORICAL_SHARED_GUARD',historical_source=SOURCE,target=TARGET,event_base=BASE,
        pending_sequence=10645,pending_prefix_sha256=PENDING_SHA,map_sha256=MAP_SHA,ledger_sha256=LEDGER_SHA,
        guard_observed=observed,contract_expected=0,source_hashes={p:sha((ROOT/p).read_bytes())for p in OUTER_SOURCE_HASHES},
        harness_identity=harness_identity,archive_sha256=ARCHIVE_SHA,
        producer_executed=False,full_F13='NOT_RUN',admission=False)))
    if observed != 0:
        print('ASSERTION RED: accepted exact-J contract expects qualified transition; unchanged shared production guard refused', file=sys.stderr)
        return 1
    return 0


def main():
    arguments = sys.argv[1:]
    require(arguments in ([], ['--prepare'], ['--inside']), 'explicit_harness_mode')
    if arguments == ['--prepare']:
        verify_private_devices()
        return prepare()
    if arguments == ['--inside']:
        verify_private_devices()
        return inside()
    return namespace_main()


if __name__ == '__main__':
    try:
        raise SystemExit(main())
    except Exception as error:
        print('F13_HISTORICAL_SETUP_FAILURE: ' + type(error).__name__ + ': ' + str(error), file=sys.stderr)
        raise SystemExit(2)
