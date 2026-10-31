"""Materialize and verify the captured native fixture before importing it."""

import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat
import sys


_SHA256 = re.compile(r'[a-f0-9]{64}\Z')
_ROOTS = frozenset({'nats-rollout-preservation', 'nats-known-baseline'})
_CAPTURE_SOURCE = Path('/bundle/capture')
_EXPECTED_CAPTURED_NAMES_SHA256 = '42a4a01270ac4e2328faa81c32dd363d19b8c4d8fe383d8b7aa46bbf97af2998'


def _materialize_capture(base):
    base = Path(base)
    source = _CAPTURE_SOURCE
    code = base / 'scripts/staging'
    if code.exists() or code.is_symlink():
        raise RuntimeError('captured_code_destination_exists')
    manifest_path = source / 'capture-manifest.json'
    if manifest_path.is_symlink() or not manifest_path.is_file():
        raise RuntimeError('captured_code_manifest_missing')
    manifest_bytes = manifest_path.read_bytes()
    manifest = json.loads(manifest_bytes)
    if not isinstance(manifest, dict) or len(manifest) != 89:
        raise RuntimeError('captured_code_manifest_invalid')
    if any(not isinstance(relative, str) for relative in manifest):
        raise RuntimeError('captured_code_manifest_invalid')
    names = json.dumps(sorted(manifest), separators=(',', ':')).encode()
    if hashlib.sha256(names).hexdigest() != _EXPECTED_CAPTURED_NAMES_SHA256:
        raise RuntimeError('captured_code_manifest_invalid')
    required = {
        'nats-rollout-preservation/contract_fixture_entry.py',
        'nats-rollout-preservation/root_cli.py',
        'nats-known-baseline/kernel',
    }
    if not required.issubset(manifest):
        raise RuntimeError('captured_code_manifest_invalid')

    code.mkdir(parents=True, mode=0o755)
    os.chown(code.parent.parent, 0, 0)
    os.chmod(code.parent.parent, 0o755)
    os.chown(code.parent, 0, 0)
    os.chmod(code.parent, 0o755)
    os.chown(code, 0, 0)
    os.chmod(code, 0o755)
    for relative, wanted in sorted(manifest.items()):
        if not isinstance(relative, str) or not isinstance(wanted, str):
            raise RuntimeError('captured_code_manifest_invalid')
        path = PurePosixPath(relative)
        if (not isinstance(relative, str) or path.is_absolute() or
                '..' in path.parts or not path.parts or path.parts[0] not in _ROOTS or
                not _SHA256.fullmatch(wanted)):
            raise RuntimeError('captured_code_manifest_invalid')
        source_path = source.joinpath(*path.parts)
        if source_path.is_symlink() or not source_path.is_file():
            raise RuntimeError('captured_code_source_invalid')
        content = source_path.read_bytes()
        if hashlib.sha256(content).hexdigest() != wanted:
            raise RuntimeError('captured_code_source_changed')
        target = code.joinpath(*path.parts)
        target.parent.mkdir(parents=True, exist_ok=True, mode=0o755)
        for directory in (target.parent, *target.parent.parents):
            if directory == code.parent.parent:
                break
            os.chown(directory, 0, 0)
            os.chmod(directory, 0o755)
        with target.open('xb') as output:
            output.write(content)
        os.chown(target, 0, 0)
        os.chmod(target, 0o555 if relative == 'nats-known-baseline/kernel' else 0o444)

    copied_manifest = code / 'capture-manifest.json'
    with copied_manifest.open('xb') as output:
        output.write(manifest_bytes)
    os.chown(copied_manifest, 0, 0)
    os.chmod(copied_manifest, 0o444)
    sys.path[:0] = [str(code / 'nats-rollout-preservation'), str(code / 'nats-known-baseline')]
    import root_cli
    root_cli.code_binding(code)
    kernel = code / 'nats-known-baseline/kernel'
    info = kernel.stat(follow_symlinks=False)
    if (not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or
            info.st_mode & 0o222):
        raise RuntimeError('captured_kernel_custody_invalid')
    return code


def inner(base):
    base = Path(base)
    _materialize_capture(base)
    from nats_contract_integration import inner as run_fixture
    return run_fixture(str(base))


def inner_a3(base):
    base=Path(base)
    _materialize_capture(base)
    from nats_a3_roster_integration import inner as run_fixture
    return run_fixture(str(base))


def inner_a3_existing(base):
    base=Path(base)
    _materialize_capture(base)
    from nats_a3_roster_integration import inner_existing as run_fixture
    return run_fixture(str(base))
