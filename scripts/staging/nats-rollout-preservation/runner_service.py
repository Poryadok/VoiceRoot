"""Systemd-activated, fixed-package nonroot runner process."""
from __future__ import annotations

import json
import os
from pathlib import Path
import re
import socket
import stat
import sys
import sysconfig


CONFIG_SCHEMA = 'voice-nats-trusted-runner-config-v2'
CONFIG_FILE = Path('/etc/voice-nats-preservation/trusted-runner.json')
CA_FILE = '/etc/voice-nats-preservation/k3s-ca.crt'
TOKEN_FILE = '/var/lib/voice-nats-preservation/trusted-runner/token'
STATE_DIR = '/var/lib/voice-nats-preservation/trusted-runner/state'
OPERATIONS_DIR = Path(STATE_DIR) / 'operations'
SOCKET_PATH = '/run/voice-nats-preservation/runner.sock'
SUBMITTER_GROUP = 'pmd'
REQUIRED_CONFIG = frozenset(('schema', 'runner_source_sha', 'runner_closure_sha256',
    'api_server', 'ca_file', 'token_file', 'state_dir', 'runner_image', 'phase_image', 'storage_binding',
    'verification_pvc_size_bytes', 'verification_pvc_max_bytes', 'verification_storage_class',
    'replacement_pvc_size_bytes', 'replacement_pvc_max_bytes', 'replacement_storage_class'))


class ServiceError(ValueError):
    pass


def _fail(code):
    raise ServiceError('runner_service_' + code)


def _pairs(items):
    row = {}
    for key, value in items:
        if key in row: _fail('config_duplicate_field')
        row[key] = value
    return row


def load_config(path=CONFIG_FILE, package_root=None, *, owner_uid=0):
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    except OSError:
        _fail('config_unavailable')
    try:
        before = os.fstat(fd)
        if (not stat.S_ISREG(before.st_mode) or before.st_uid != owner_uid
            or stat.S_IMODE(before.st_mode) != 0o444 or before.st_nlink != 1
            or not 1 <= before.st_size <= 1 << 20):
            _fail('config_custody_invalid')
        raw = os.read(fd, before.st_size + 1)
        after = os.fstat(fd)
        if len(raw) != before.st_size or (before.st_dev, before.st_ino, before.st_size,
            before.st_mtime_ns, before.st_ctime_ns) != (after.st_dev, after.st_ino,
            after.st_size, after.st_mtime_ns, after.st_ctime_ns):
            _fail('config_changed')
    finally:
        os.close(fd)
    try:
        row = json.loads(raw, object_pairs_hook=_pairs)
    except (ValueError, TypeError, UnicodeError):
        _fail('config_invalid')
    if not isinstance(row, dict) or set(row) != REQUIRED_CONFIG or row.get('schema') != CONFIG_SCHEMA:
        _fail('config_shape_invalid')
    root = Path(package_root or Path(__file__).resolve().parent)
    if (not re.fullmatch('[a-f0-9]{40}', root.name)
        or row.get('runner_source_sha') != root.name
        or not isinstance(row.get('runner_closure_sha256'), str)
        or not re.fullmatch('[a-f0-9]{64}', row['runner_closure_sha256'])):
        _fail('package_pin_invalid')
    if (row.get('ca_file') != CA_FILE or row.get('token_file') != TOKEN_FILE
        or row.get('state_dir') != STATE_DIR):
        _fail('trusted_path_invalid')
    if not isinstance(row.get('api_server'), str) or not re.fullmatch(r'https://[^/@?#]+(?::(?:443|6443))?', row['api_server']):
        _fail('server_invalid')
    if not isinstance(row.get('runner_image'), str) or not re.fullmatch(
        r'[a-z0-9./_-]+@sha256:[a-f0-9]{64}', row['runner_image']):
        _fail('image_invalid')
    if not isinstance(row.get('phase_image'), str) or not re.fullmatch(
        r'[a-z0-9./_-]+@sha256:[a-f0-9]{64}', row['phase_image']):
        _fail('phase_image_invalid')
    size = row.get('verification_pvc_size_bytes')
    ceiling = row.get('verification_pvc_max_bytes')
    if (type(size) is not int or not 0 < size <= (1 << 63) - 1
        or type(ceiling) is not int or not 0 < ceiling <= (1 << 63) - 1 or size > ceiling):
        _fail('verification_pvc_ceiling_invalid')
    storage_class = row.get('verification_storage_class')
    if (not isinstance(storage_class, str) or len(storage_class) > 253
        or any(not label or len(label) > 63
            or not re.fullmatch(r'[a-z0-9](?:[-a-z0-9]*[a-z0-9])?', label)
            for label in storage_class.split('.'))):
        _fail('verification_storage_class_invalid')
    replacement_size = row.get('replacement_pvc_size_bytes')
    replacement_ceiling = row.get('replacement_pvc_max_bytes')
    if (type(replacement_size) is not int or not 0 < replacement_size <= (1 << 63) - 1
        or type(replacement_ceiling) is not int or not 0 < replacement_ceiling <= (1 << 63) - 1
        or replacement_size > replacement_ceiling):
        _fail('replacement_pvc_ceiling_invalid')
    replacement_storage_class = row.get('replacement_storage_class')
    if (not isinstance(replacement_storage_class, str) or len(replacement_storage_class) > 253
        or any(not label or len(label) > 63
            or not re.fullmatch(r'[a-z0-9](?:[-a-z0-9]*[a-z0-9])?', label)
            for label in replacement_storage_class.split('.'))):
        _fail('replacement_storage_class_invalid')
    try:
        from tokenless_job import validate_binding
        binding = validate_binding(row['storage_binding'])
    except (KeyError, ValueError):
        _fail('binding_invalid')
    return row | {'storage_binding': binding}


def binding_for(config, operation):
    # The bootstrap pins stable storage identity. Per-release operation IDs and
    # nonces are admitted dynamically and journaled by the runner; requiring a
    # root config rewrite for every release would turn app data into code/config.
    if not isinstance(operation, str) or not re.fullmatch('[a-f0-9]{12}', operation):
        _fail('operation_invalid')
    return config['storage_binding']


def _trusted_import_path(package_root):
    standard = sysconfig.get_paths()['stdlib']
    dynamic = str(Path(standard) / 'lib-dynload')
    sys.path[:] = [str(package_root), standard, dynamic]


def _systemd_listener():
    if os.environ.get('LISTEN_PID') != str(os.getpid()) or os.environ.get('LISTEN_FDS') != '1':
        _fail('socket_activation_required')
    try:
        listener = socket.socket(fileno=3)
        info = os.lstat(SOCKET_PATH)
        import grp
        expected_gid = grp.getgrnam(SUBMITTER_GROUP).gr_gid
        if (listener.family != socket.AF_UNIX or listener.getsockopt(socket.SOL_SOCKET, socket.SO_TYPE) != socket.SOCK_STREAM
            or listener.getsockname() != SOCKET_PATH or not stat.S_ISSOCK(info.st_mode)
            or info.st_uid != 0 or info.st_gid != expected_gid or stat.S_IMODE(info.st_mode) != 0o660):
            _fail('socket_custody_invalid')
        return listener
    except ServiceError:
        raise
    except (OSError, ImportError):
        _fail('socket_unavailable')


def serve(listener, runner):
    while True:
        connection, _ = listener.accept()
        try:
            from runner_rpc import serve_connection
            serve_connection(runner, connection)
        except Exception:
            try: connection.close()
            except OSError: pass


def main():
    if sys.platform != 'linux' or os.geteuid() == 0 or not sys.flags.isolated:
        _fail('runtime_boundary_invalid')
    package_root = Path(__file__).resolve().parent
    _trusted_import_path(package_root)
    import runner_manifest
    installed = runner_manifest.load(package_root, owner_uid=0)
    config = load_config(package_root=package_root, owner_uid=0)
    if (config['runner_closure_sha256'] != installed['closure_sha256']
        or config['runner_image'] != installed['images']['nats_phase']
        or config['phase_image'] != installed['images']['nats_phase']):
        _fail('installed_pin_mismatch')
    from bridge_state import BridgeState
    from runner_actions import Actions
    from runner_rpc import Runner
    state = BridgeState(OPERATIONS_DIR, owner_uid=os.geteuid())
    runner = Runner(Actions(state))
    serve(_systemd_listener(), runner)


if __name__ == '__main__':
    if len(sys.argv) != 1:
        _fail('arguments_invalid')
    main()
