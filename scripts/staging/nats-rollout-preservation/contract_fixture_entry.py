"""Materialize and verify the captured native fixture before importing it."""

import hashlib
import importlib.abc
import importlib.util
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat
import subprocess
import sys


_SHA256 = re.compile(r'[a-f0-9]{64}\Z')
_ROOTS = frozenset({'nats-rollout-preservation', 'nats-known-baseline'})
_CAPTURE_SOURCE = Path('/bundle/capture')
_EXPECTED_CAPTURED_NAMES_SHA256 = '9e08bb7df3c0dcdb9f4bf6287895eff08a4451062ca41199b79ca131da7465e4'


def _pairs(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise RuntimeError('captured_code_manifest_invalid')
        result[key] = value
    return result


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
    manifest = json.loads(manifest_bytes, object_pairs_hook=_pairs)
    if not isinstance(manifest, dict) or len(manifest) != 99:
        raise RuntimeError('captured_code_manifest_invalid')
    if manifest_bytes != json.dumps(manifest, sort_keys=True).encode():
        raise RuntimeError('captured_code_manifest_invalid')
    if any(not isinstance(relative, str) for relative in manifest):
        raise RuntimeError('captured_code_manifest_invalid')
    names = json.dumps(sorted(manifest), separators=(',', ':')).encode()
    if hashlib.sha256(names).hexdigest() != _EXPECTED_CAPTURED_NAMES_SHA256:
        raise RuntimeError('captured_code_manifest_invalid')
    required = {
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
                '..' in path.parts or not path.parts or
                (path.parts[0] not in _ROOTS and relative not in
                 ('configure-kubectl-ci.sh', 'mail-only-patch.py')) or
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
    modules = {}
    for relative in manifest:
        path = PurePosixPath(relative)
        if len(path.parts) == 2 and path.suffix == '.py':
            name = path.stem
            if name in modules or name in sys.modules:
                raise RuntimeError('captured_module_collision')
            modules[name] = code.joinpath(*path.parts)

    class CapturedFinder(importlib.abc.MetaPathFinder):
        def find_spec(self, fullname, path=None, target=None):
            if fullname in modules:
                return importlib.util.spec_from_file_location(fullname, modules[fullname])
            return None

    sys.meta_path.insert(0, CapturedFinder())
    import root_cli
    root_cli.code_binding(code)
    kernel = code / 'nats-known-baseline/kernel'
    info = kernel.stat(follow_symlinks=False)
    if (not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or
            info.st_mode & 0o222):
        raise RuntimeError('captured_kernel_custody_invalid')
    return code


def inner(base):
    return _run('inner', base)


def inner_a3(base):
    return _run('inner_a3', base)


def inner_a3_existing(base):
    return _run('inner_a3_existing', base)


def _run(mode, base):
    subprocess.run([sys.executable, '-I', str(Path(__file__).resolve()), mode, str(base)], check=True)


def _execute(mode, base):
    _materialize_capture(Path(base))
    # Drivers are only visible after the production code has been bound.
    sys.path.insert(0, '/bundle')
    if mode == 'inner':
        from nats_contract_integration import inner as run_fixture
    elif mode == 'inner_a3':
        from nats_a3_roster_integration import inner as run_fixture
    elif mode == 'inner_a3_existing':
        from nats_a3_roster_integration import inner_existing as run_fixture
    else:
        raise RuntimeError('fixture_mode_invalid')
    return run_fixture(str(base))


if __name__ == '__main__':
    if len(sys.argv) != 3:
        raise SystemExit('fixture_arguments_invalid')
    _execute(sys.argv[1], sys.argv[2])
