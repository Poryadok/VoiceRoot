"""Fixed non-root executor for the authorized NATS contract migration.

The caller supplies only the already-authorized canonical plan as bounded data.
This module has no shell, Docker, Kubernetes, candidate import, or path input.
"""
from __future__ import annotations

import copy
import argparse
import base64
import binascii
import hashlib
import json
import os
from pathlib import Path
import re
import socket
import stat
import subprocess
import sys
import time
from urllib.request import urlopen

import nats_contract_plan
from nats_contract_actor import INFO, WRITE, persistent_config


SCHEMA = 'voice-nats-contract-execution-v1'
RECEIPT_SCHEMA = 'voice-nats-contract-receipt-v1'
INPUT_SCHEMA = 'voice-nats-contract-inputs-v1'
INPUT_RECEIPT_SCHEMA = 'voice-nats-contract-input-receipt-v1'
MAX_PLAN_BYTES = 1 << 20
MAX_INPUT_BYTES = 2 << 20
MAX_STATE_BYTES = 2 << 20
MAX_RESPONSE_BYTES = 64 << 10
MAX_CREDENTIAL_BYTES = 256 << 10
MAX_SERVER_CONFIG_BYTES = 256 << 10
MAX_EVENTS = 64
STATE_ROOT = Path('/state')
SERVER_CONFIG = Path('/broker-config/server.conf')
NATS = '/usr/local/bin/nats'
SUPERVISOR = ('127.0.0.1', 9444)
OP_RE = re.compile(r'[a-f0-9]{12}')
NONCE_RE = re.compile(r'[a-f0-9]{32}')
ATTEMPT_RE = re.compile(r'[a-f0-9]{8}')
UUID_RE = re.compile(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}')
STREAM_WRITES = frozenset(('chat_events', 'social_events', 'role_events'))


class ContractRunnerError(ValueError):
    pass


def _fail(code: str) -> None:
    raise ContractRunnerError('contract_' + code)


def _canonical(value: object) -> bytes:
    try:
        return json.dumps(value, sort_keys=True, separators=(',', ':'),
            ensure_ascii=False, allow_nan=False).encode('utf-8')
    except (TypeError, ValueError, UnicodeError):
        _fail('json_invalid')


def _response_bytes(value: object) -> bytes:
    """Serialize one bounded stdout receipt; never stream unbounded live INFO data."""
    raw = _canonical(value) + b'\n'
    if len(raw) > MAX_RESPONSE_BYTES:
        _fail('response_size')
    return raw


def validate_plan(plan: object) -> dict:
    """Validate the fixed additive plan language and its only allowed deltas."""
    if not isinstance(plan, dict) or set(plan) != {
        'schema', 'migration_id', 'sources', 'observed', 'consumer_targets', 'actions'}:
        _fail('plan_shape')
    try:
        declarations = nats_contract_plan.declarations_for(plan)
        nats_contract_plan.validated_targets(plan)
    except Exception:
        _fail('plan_version')
    if (not isinstance(plan['sources'], dict) or len(plan['sources']) not in (2,)
        or set(plan['sources']) != {'deploy/templates/nats-realtime-bootstrap.yaml',
                                    'deploy/templates/nats-analytics-chat-bootstrap.yaml'}
        or any(not isinstance(v, str) or re.fullmatch(r'[a-f0-9]{64}', v) is None
               for v in plan['sources'].values())):
        _fail('plan_sources')
    if not isinstance(plan['observed'], dict) or set(plan['observed']) != {
        'file_events', 'chat_events', 'social_events', 'role_events'}:
        _fail('plan_observed')
    for value in plan['observed'].values():
        if (not isinstance(value, dict) or set(value) != {'config_sha256'}
            or not isinstance(value['config_sha256'], str)
            or re.fullmatch(r'[a-f0-9]{64}', value['config_sha256']) is None):
            _fail('plan_observed')
    actions = plan['actions']
    if not isinstance(actions, list) or len(actions) > 7:
        _fail('plan_actions')
    allowed_updates = {
        'chat_events': frozenset(nats_contract_plan.CHAT_SUBJECTS_17),
        'social_events': frozenset(),
        'role_events': frozenset(nats_contract_plan.ROLE_SUBJECTS_10),
    }
    allowed_consumers = {(stream, config['durable_name']): config
        for stream, config in declarations}
    seen = set()
    for action in actions:
        if not isinstance(action, dict):
            _fail('plan_action_shape')
        api = action.get('api')
        obj = action.get('object')
        if api in {f'$JS.API.STREAM.UPDATE.{name}' for name in STREAM_WRITES}:
            name = api.rsplit('.', 1)[1]
            if (set(action) != {'api', 'object', 'before', 'after'} or obj != name
                or name in seen or not isinstance(action['before'], dict)
                or not isinstance(action['after'], dict)):
                _fail('plan_stream_action')
            before, after = action['before'], action['after']
            if before.get('name') != name or after.get('name') != name:
                _fail('plan_stream_name')
            expected = copy.deepcopy(before)
            if name == 'social_events':
                if before.get('duplicate_window') not in (120000000000, 86400000000000):
                    _fail('plan_social_window')
                expected['duplicate_window'] = 86400000000000
            else:
                old = before.get('subjects')
                if (not isinstance(old, list) or any(not isinstance(x, str) for x in old)
                    or len(old) != len(set(old))):
                    _fail('plan_stream_subjects')
                additions = allowed_updates[name]
                new_subjects = after.get('subjects')
                if (not isinstance(new_subjects, list)
                    or any(not isinstance(x, str) for x in new_subjects)
                    or len(new_subjects) != len(set(new_subjects))):
                    _fail('plan_stream_subjects')
                old_set, new_set = set(old), set(new_subjects)
                inserted = new_set - old_set
                if (not inserted.issubset(additions) or not old_set.issubset(new_set)
                    or [value for value in new_subjects if value in old_set] != old):
                    _fail('plan_stream_subjects')
                expected['subjects'] = new_subjects
            if after != expected:
                _fail('plan_stream_delta')
            seen.add(name)
            continue
        if isinstance(api, str) and api.startswith('$JS.API.CONSUMER.CREATE.'):
            if set(action) != {'api', 'object', 'before', 'after', 'normalized_config_sha256'}:
                _fail('plan_consumer_action_shape')
            pair = tuple(obj.split('/', 1)) if isinstance(obj, str) and obj.count('/') == 1 else ()
            config = allowed_consumers.get(pair)
            if (config is None or pair in seen or api != '$JS.API.CONSUMER.CREATE.' + pair[0] + '.' + pair[1]
                or action['before'] is not None
                or action['after'] != {'stream_name': pair[0], 'action': 'create', 'config': config}
                or action['normalized_config_sha256'] != next(
                    (row['config_sha256'] for row in plan['consumer_targets']
                     if row['stream'] == pair[0] and row['name'] == pair[1]), None)):
                _fail('plan_consumer_action')
            seen.add(pair)
            continue
        _fail('plan_api_forbidden')
    if len(seen) != len(actions):
        _fail('plan_duplicate_action')
    return copy.deepcopy(plan)


def validate_envelope(raw: bytes, *, expected: dict) -> dict:
    if not isinstance(raw, bytes) or not 1 <= len(raw) <= MAX_INPUT_BYTES:
        _fail('input_size')
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                _fail('input_duplicate_key')
            result[key] = value
        return result
    try:
        envelope = json.loads(raw.decode('utf-8'), object_pairs_hook=pairs,
            parse_constant=lambda _value: _fail('input_constant'))
    except (ValueError, UnicodeError, TypeError):
        _fail('input_json')
    required = {'schema', 'operation', 'nonce', 'contract_attempt',
        'source_pvc_uid', 'source_pv_uid', 'verification_pvc_uid', 'verification_pv_uid',
        'archive_sha256', 'archive_bytes', 'server_config_sha256', 'plan_sha256', 'plan', 'files'}
    if not isinstance(envelope, dict) or set(envelope) != required:
        _fail('input_shape')
    if envelope['schema'] != INPUT_SCHEMA:
        _fail('input_schema')
    for key, pattern in (('operation', OP_RE), ('nonce', NONCE_RE),
        ('contract_attempt', ATTEMPT_RE),
        ('source_pvc_uid', UUID_RE), ('source_pv_uid', UUID_RE),
        ('verification_pvc_uid', UUID_RE), ('verification_pv_uid', UUID_RE),
        ('server_config_sha256', re.compile(r'[a-f0-9]{64}')),
        ('archive_sha256', re.compile(r'[a-f0-9]{64}')),
        ('plan_sha256', re.compile(r'[a-f0-9]{64}'))):
        if not isinstance(envelope[key], str) or pattern.fullmatch(envelope[key]) is None:
            _fail('input_identity')
    if envelope['source_pvc_uid'] == envelope['verification_pvc_uid']:
        _fail('input_pvc_same')
    if type(envelope['archive_bytes']) is not int or not 0 < envelope['archive_bytes'] <= (48 << 30):
        _fail('archive_size')
    if not isinstance(envelope['files'], dict) or set(envelope['files']) != {'bootstrap.creds', 'server.conf'}:
        _fail('input_file_set')
    files = {}
    for name, digest_key, size_key, limit in (
        ('bootstrap.creds', 'bootstrap_creds_sha256', 'bootstrap_creds_bytes', MAX_CREDENTIAL_BYTES),
        ('server.conf', 'server_config_sha256', 'server_config_bytes', MAX_SERVER_CONFIG_BYTES)):
        encoded = envelope['files'][name]
        if not isinstance(encoded, str):
            _fail('input_file_encoding')
        try:
            content = base64.b64decode(encoded, validate=True)
        except (ValueError, binascii.Error):
            _fail('input_file_encoding')
        if base64.b64encode(content).decode('ascii') != encoded:
            _fail('input_file_encoding')
        if not 0 < len(content) <= limit:
            _fail('input_file_size')
        if (hashlib.sha256(content).hexdigest() != expected[digest_key]
            or len(content) != expected[size_key]):
            _fail('input_file_binding')
        files[name] = content
    context = {key: envelope[key] for key in (
        'operation', 'nonce', 'contract_attempt', 'source_pvc_uid', 'source_pv_uid',
        'verification_pvc_uid', 'verification_pv_uid', 'server_config_sha256',
        'archive_sha256', 'archive_bytes', 'plan_sha256')}
    if any(context[key] != expected[key] for key in context):
        _fail('input_binding')
    plan = validate_plan(envelope['plan'])
    if (envelope['plan_sha256'] != nats_contract_plan.digest(plan)
        or expected['plan_sha256'] != envelope['plan_sha256']
        or _canonical(envelope) != raw
        or hashlib.sha256(raw).hexdigest() != expected['input_bundle_sha256']
        or len(raw) != expected['input_bundle_bytes']
        or len(_canonical(plan)) != expected['plan_bytes']):
        _fail('input_digest_or_canonical')
    return {**envelope, '_files': files}


def _atomic_private(path: Path, value: dict) -> None:
    data = _canonical(value)
    temp = path.with_name(path.name + '.tmp')
    fd = os.open(temp, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        with os.fdopen(fd, 'wb') as stream:
            stream.write(data); stream.flush(); os.fsync(stream.fileno())
        os.link(temp, path, follow_symlinks=False)
        temp.unlink()
        directory = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try: os.fsync(directory)
        finally: os.close(directory)
    except Exception:
        try: temp.unlink()
        except FileNotFoundError: pass
        raise


def _atomic_credential(path: Path, raw: bytes) -> None:
    """Create the one private client credential without a pathname replay path."""
    if not isinstance(raw, bytes) or not 0 < len(raw) <= MAX_CREDENTIAL_BYTES:
        _fail('credential_size')
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o400)
    try:
        os.fchmod(fd, 0o400)
        with os.fdopen(fd, 'wb') as stream:
            stream.write(raw)
            stream.flush()
            os.fsync(stream.fileno())
        directory = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try: os.fsync(directory)
        finally: os.close(directory)
    except Exception:
        try: path.unlink()
        except FileNotFoundError: pass
        raise


def _atomic_projection(path: Path, raw: bytes) -> None:
    """Publish the captured public broker config into the fixed shared volume."""
    if not isinstance(raw, bytes) or not 0 < len(raw) <= MAX_SERVER_CONFIG_BYTES:
        _fail('server_config_size')
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o440)
    try:
        os.fchmod(fd, 0o440)
        with os.fdopen(fd, 'wb') as stream:
            stream.write(raw)
            stream.flush()
            os.fsync(stream.fileno())
        directory = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try: os.fsync(directory)
        finally: os.close(directory)
    except Exception:
        try: path.unlink()
        except FileNotFoundError: pass
        raise


def _private_read(path: Path) -> dict:
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        row = os.fstat(fd)
        if (not stat.S_ISREG(row.st_mode) or row.st_uid != os.geteuid()
            or row.st_gid != os.getegid() or row.st_nlink != 1
            or stat.S_IMODE(row.st_mode) != 0o600 or not 1 <= row.st_size <= MAX_STATE_BYTES):
            _fail('state_file_untrusted')
        with os.fdopen(fd, 'rb', closefd=False) as stream:
            value = json.loads(stream.read(MAX_STATE_BYTES + 1))
        if not isinstance(value, dict): _fail('state_file_invalid')
        return value
    finally:
        os.close(fd)


class NatsActor:
    """Fixed CLI adapter; NATS API subject is validated before subprocess use."""
    def __init__(self, *, isolation_check, expected_broker=None, creds_path: Path,
                 run=subprocess.run, sleep=time.sleep):
        self._isolation_check = isolation_check
        self._expected_broker = expected_broker
        self._creds_path = creds_path
        self._run = run
        self._sleep = sleep

    def assert_closed_isolation(self):
        self._isolation_check()
        status = _contract_supervisor('status')
        if status.get('running') is not True:
            _fail('broker_not_running')
        if self._expected_broker is not None:
            expected = self._expected_broker()
            if status.get('pid') != expected['pid'] or status.get('started_at_ns') != expected['started_at_ns']:
                _fail('broker_identity_changed')

    def _request(self, api: str, payload: dict | None, *, missing=False):
        if api not in INFO.values() and api not in WRITE:
            _fail('api_forbidden')
        self.assert_closed_isolation()
        command = [NATS, '--server', 'nats://127.0.0.1:4222', '--creds', str(self._creds_path),
            '--inbox-prefix', '_INBOX.voice.bootstrap.reply', '--timeout', '5s',
            'req', '--raw', api, _canonical(payload).decode('utf-8') if payload is not None else '']
        result = self._run(command, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL, timeout=10, check=False)
        if result.returncode != 0 or len(result.stdout) > (256 << 10):
            _fail('api_request_failed')
        try: row = json.loads(result.stdout)
        except (ValueError, TypeError): _fail('api_response_invalid')
        if not isinstance(row, dict): _fail('api_response_invalid')
        if row.get('error'):
            if (missing and api.startswith('$JS.API.CONSUMER.INFO.')
                and isinstance(row['error'], dict) and row['error'].get('err_code') == 10014):
                return None
            _fail('api_response_error')
        return row

    def info_config(self, obj: str):
        api = INFO.get(obj)
        if api is None: _fail('object_forbidden')
        row = self._request(api, None, missing='/' in obj)
        if row is None: return None
        config = row.get('config')
        if not isinstance(config, dict): _fail('api_config_invalid')
        if '/' in obj:
            stream, durable = obj.split('/', 1)
            if row.get('stream_name') != stream or row.get('name') != durable:
                _fail('api_object_mismatch')
        elif config.get('name') != obj:
            _fail('api_object_mismatch')
        return persistent_config(config)

    def mutate(self, api: str, payload: dict):
        if api not in WRITE: _fail('mutation_forbidden')
        self._request(api, payload)


def _supervisor(action: str) -> dict:
    if action not in ('start', 'stop', 'status'):
        _fail('supervisor_action')
    request = _canonical({'schema': 'voice-nats-supervisor-v1', 'action': action}) + b'\n'
    with socket.create_connection(SUPERVISOR, timeout=3) as conn:
        conn.sendall(request)
        response = bytearray()
        while len(response) <= 1024 and b'\n' not in response:
            chunk = conn.recv(1024 - len(response) + 1)
            if not chunk: break
            response.extend(chunk)
    line, sep, trailing = bytes(response).partition(b'\n')
    if not sep or trailing or len(line) > 1024:
        _fail('supervisor_response')
    try: value = json.loads(line)
    except (ValueError, TypeError): _fail('supervisor_response')
    if not isinstance(value, dict) or value.get('schema') != 'voice-nats-supervisor-v1' or value.get('ok') is not True:
        _fail('supervisor_action_failed')
    return value


def _contract_supervisor(action: str) -> dict:
    """Read the contract-mode supervisor receipt without discarding process identity."""
    value = _supervisor(action)
    if action == 'status':
        required = {'schema', 'ok', 'running', 'pid', 'started_at_ns'}
        if set(value) != required or type(value['running']) is not bool:
            _fail('supervisor_status_shape')
        if value['running'] and (type(value['pid']) is not int or type(value['started_at_ns']) is not int):
            _fail('supervisor_status_identity')
        if not value['running'] and (value['pid'] is not None or value['started_at_ns'] is not None):
            _fail('supervisor_status_identity')
    elif action == 'start':
        if (set(value) != {'schema', 'ok', 'running', 'pid', 'started_at_ns'}
            or value['running'] is not True or type(value['pid']) is not int
            or type(value['started_at_ns']) is not int):
            _fail('supervisor_start_shape')
    elif action == 'stop':
        if (set(value) != {'schema', 'ok', 'running', 'pid', 'started_at_ns', 'finished_at_ns'}
            or value['running'] is not False or type(value['finished_at_ns']) is not int):
            _fail('supervisor_stop_shape')
    return value


def _read_bound_file(path: Path, *, digest: str, mode: int, max_bytes: int,
                     expected_bytes: int | None = None) -> bytes:
    fd = -1
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
        before = os.fstat(fd)
        if (not stat.S_ISREG(before.st_mode) or before.st_uid != os.geteuid()
            or before.st_gid != os.getegid() or stat.S_IMODE(before.st_mode) != mode
            or before.st_nlink != 1 or not 1 <= before.st_size <= max_bytes
            or (expected_bytes is not None and before.st_size != expected_bytes)):
            _fail('input_file_profile')
        raw = bytearray()
        while len(raw) <= before.st_size:
            part = os.read(fd, min(64 << 10, before.st_size + 1 - len(raw)))
            if not part:
                break
            raw.extend(part)
        after = os.fstat(fd)
        if (len(raw) != before.st_size or (before.st_dev, before.st_ino, before.st_size,
            before.st_mtime_ns, before.st_ctime_ns) != (after.st_dev, after.st_ino,
            after.st_size, after.st_mtime_ns, after.st_ctime_ns)
            or hashlib.sha256(raw).hexdigest() != digest):
            _fail('input_file_changed')
        return bytes(raw)
    except OSError:
        _fail('input_file_missing')
    finally:
        if fd >= 0:
            os.close(fd)


def _credential_path(expected: dict, *, state_root: Path = STATE_ROOT) -> Path:
    return _state_dir(state_root, expected['operation'], expected['contract_attempt']) / 'bootstrap.creds'


def _ensure_input_profile(expected: dict, *, state_root: Path = STATE_ROOT) -> None:
    """Check the one private Actor credential and the exact broker config projection."""
    _read_bound_file(_credential_path(expected, state_root=state_root),
        digest=expected['bootstrap_creds_sha256'], mode=0o400, max_bytes=MAX_CREDENTIAL_BYTES,
        expected_bytes=expected['bootstrap_creds_bytes'])
    _ensure_server_config(expected)


def _ensure_server_config(expected: dict) -> None:
    """Validate the exact host-bound, public broker-config projection."""
    _read_bound_file(SERVER_CONFIG, digest=expected['server_config_sha256'],
        mode=0o440, max_bytes=MAX_SERVER_CONFIG_BYTES,
        expected_bytes=expected['server_config_bytes'])


def _verify_input_receipt(expected: dict, *, state_root: Path = STATE_ROOT) -> dict:
    """Bind the one-shot contract input and plan to this exact helper Pod."""
    path = _state_dir(state_root, expected['operation'], expected['contract_attempt']) / 'input.receipt.json'
    receipt = _private_read(path)
    required = {
        'schema': INPUT_RECEIPT_SCHEMA, 'phase': 'contract-inputs', 'purpose': 'nats-contract-migration',
        'operation': expected['operation'], 'nonce': expected['nonce'],
        'pod_uid': expected['pod_uid'], 'contract_attempt': expected['contract_attempt'],
        'source_pvc_uid': expected['source_pvc_uid'],
        'source_pv_uid': expected['source_pv_uid'],
        'verification_pvc_uid': expected['verification_pvc_uid'],
        'verification_pv_uid': expected['verification_pv_uid'],
        'archive_sha256': expected['archive_sha256'], 'archive_bytes': expected['archive_bytes'],
        'input_bundle_sha256': expected['input_bundle_sha256'],
        'input_bundle_bytes': expected['input_bundle_bytes'], 'status': 'COMPLETE',
        'plan_sha256': expected['plan_sha256'], 'plan_bytes': expected['plan_bytes'],
        'server_config_sha256': expected['server_config_sha256'],
        'server_config_bytes': expected['server_config_bytes'],
        'bootstrap_creds_sha256': expected['bootstrap_creds_sha256'],
        'bootstrap_creds_bytes': expected['bootstrap_creds_bytes'], 'file_count': 2,
    }
    if set(receipt) != set(required) or receipt != required:
        _fail('input_receipt_binding')
    _ensure_input_profile(expected, state_root=state_root)
    return receipt


def _broker_receipt(state_root: Path, expected: dict, action: str, fn) -> dict:
    directory = _state_dir(state_root, expected['operation'], expected['contract_attempt'])
    _load_bound_plan(directory, expected)
    started_path = directory / ('broker-' + action + '.started.json')
    receipt_path = directory / ('broker-' + action + '.receipt.json')
    if started_path.exists() or receipt_path.exists():
        _fail('broker_action_already_started')
    _atomic_private(started_path, {'schema': RECEIPT_SCHEMA, **expected,
        'status': 'STARTED', 'action': 'broker-' + action})
    value = fn()
    complete = {'schema': RECEIPT_SCHEMA, **expected, 'status': 'COMPLETE',
        'action': 'broker-' + action, 'broker': value}
    _atomic_private(receipt_path, complete)
    return complete


def read_broker_receipt(*, state_root: Path, expected: dict, action: str) -> dict:
    """Read one exact durable same-Pod supervisor receipt without contacting NATS."""
    if action not in ('start', 'stop'):
        _fail('broker_readback_action')
    directory = _state_dir(state_root, expected.get('operation'), expected.get('contract_attempt'))
    row = _private_read(directory / ('broker-' + action + '.receipt.json'))
    wanted = {'schema': RECEIPT_SCHEMA, **expected, 'status': 'COMPLETE',
        'action': 'broker-' + action}
    broker = row.get('broker') if isinstance(row, dict) else None
    if (not isinstance(row, dict) or set(row) != set(wanted) | {'broker'}
        or any(row.get(key) != value for key, value in wanted.items())
        or not isinstance(broker, dict)):
        _fail('broker_readback_binding')
    if action == 'start':
        if (set(broker) != {'schema', 'ok', 'running', 'pid', 'started_at_ns'}
            or broker.get('schema') != 'voice-nats-supervisor-v1' or broker.get('ok') is not True
            or broker.get('running') is not True or type(broker.get('pid')) is not int
            or broker['pid'] <= 0 or type(broker.get('started_at_ns')) is not int
            or broker['started_at_ns'] <= 0):
            _fail('broker_readback_process')
    else:
        if (set(broker) != {'schema', 'ok', 'running', 'pid', 'started_at_ns', 'finished_at_ns'}
            or broker.get('schema') != 'voice-nats-supervisor-v1' or broker.get('ok') is not True
            or broker.get('running') is not False or type(broker.get('pid')) is not int
            or broker['pid'] <= 0 or type(broker.get('started_at_ns')) is not int
            or broker['started_at_ns'] <= 0 or type(broker.get('finished_at_ns')) is not int
            or broker['finished_at_ns'] < broker['started_at_ns']):
            _fail('broker_readback_process')
    return row


def _broker_identity(state_root: Path, binding: dict) -> dict:
    expected = {key: binding[key] for key in ('operation', 'nonce', 'pod_uid',
        'contract_attempt', 'source_pvc_uid', 'source_pv_uid', 'verification_pvc_uid',
        'verification_pv_uid', 'input_bundle_sha256', 'input_bundle_bytes',
        'archive_sha256', 'archive_bytes', 'server_config_sha256', 'plan_sha256',
        'plan_bytes', 'bootstrap_creds_sha256', 'bootstrap_creds_bytes', 'server_config_bytes')}
    path = _state_dir(state_root, expected['operation'], expected['contract_attempt']) / 'broker-start.receipt.json'
    receipt = _private_read(path)
    broker = receipt.get('broker')
    if (receipt.get('status') != 'COMPLETE' or receipt.get('action') != 'broker-start'
        or any(receipt.get(key) != value for key, value in expected.items())
        or not isinstance(broker, dict) or type(broker.get('pid')) is not int
        or type(broker.get('started_at_ns')) is not int):
        _fail('broker_start_receipt')
    return {'pid': broker['pid'], 'started_at_ns': broker['started_at_ns']}


def _expected_from_args(args, *, pod_uid: str | None = None) -> dict:
    actual_pod_uid = os.environ.get('POD_UID', '')
    supplied_pod_uid = pod_uid or args.pod_uid
    if (not actual_pod_uid or (supplied_pod_uid is not None
        and supplied_pod_uid != actual_pod_uid)):
        _fail('pod_uid_mismatch')
    expected = {
        'operation': args.operation, 'nonce': args.nonce, 'pod_uid': actual_pod_uid,
        'contract_attempt': args.contract_attempt, 'source_pvc_uid': args.source_pvc_uid,
        'source_pv_uid': args.source_pv_uid, 'verification_pvc_uid': args.verification_pvc_uid,
        'verification_pv_uid': args.verification_pv_uid,
        'input_bundle_sha256': args.input_bundle_sha256, 'input_bundle_bytes': args.input_bundle_bytes,
        'bootstrap_creds_sha256': args.bootstrap_creds_sha256,
        'bootstrap_creds_bytes': args.bootstrap_creds_bytes,
        'server_config_bytes': args.server_config_bytes, 'plan_bytes': args.plan_bytes,
        'archive_sha256': args.archive_sha256,
        'archive_bytes': args.archive_bytes, 'server_config_sha256': args.server_config_sha256,
        'plan_sha256': args.plan_sha256,
    }
    # Validate the closed identity tuple before filesystem/process access.
    for key, pattern in (('operation', OP_RE), ('nonce', NONCE_RE), ('pod_uid', UUID_RE),
        ('contract_attempt', ATTEMPT_RE),
        ('source_pvc_uid', UUID_RE), ('source_pv_uid', UUID_RE),
        ('verification_pvc_uid', UUID_RE), ('verification_pv_uid', UUID_RE)):
        if not isinstance(expected[key], str) or not pattern.fullmatch(expected[key]):
            _fail('cli_identity')
    for key in ('input_bundle_sha256', 'bootstrap_creds_sha256', 'server_config_sha256',
        'archive_sha256', 'plan_sha256'):
        if not isinstance(expected[key], str) or re.fullmatch(r'[a-f0-9]{64}', expected[key]) is None:
            _fail('cli_digest')
    if type(expected['input_bundle_bytes']) is not int or not 0 < expected['input_bundle_bytes'] <= MAX_INPUT_BYTES:
        _fail('cli_size')
    if (type(expected['bootstrap_creds_bytes']) is not int
        or not 0 < expected['bootstrap_creds_bytes'] <= MAX_CREDENTIAL_BYTES
        or type(expected['server_config_bytes']) is not int
        or not 0 < expected['server_config_bytes'] <= MAX_SERVER_CONFIG_BYTES
        or type(expected['plan_bytes']) is not int or not 1 <= expected['plan_bytes'] <= MAX_PLAN_BYTES):
        _fail('cli_size')
    if type(expected['archive_bytes']) is not int or not 0 < expected['archive_bytes'] <= (48 << 30):
        _fail('cli_size')
    return expected


class _ClosedArgumentParser(argparse.ArgumentParser):
    def error(self, _message):
        _fail('cli_arguments')

    def exit(self, status=0, message=None):
        _fail('cli_exit')


def _cli_parser():
    parser = _ClosedArgumentParser(add_help=False, allow_abbrev=False)
    modes = parser.add_mutually_exclusive_group(required=True)
    for mode in ('serve', 'load-plan', 'read-plan', 'broker-start', 'broker-stop', 'broker-status',
        'read-broker-start', 'read-broker-stop',
        'inspect-action', 'execute-action', 'readback-action'):
        modes.add_argument('--' + mode, action='store_true')
    for name in ('operation', 'nonce', 'contract-attempt', 'source-pvc-uid', 'source-pv-uid',
        'verification-pvc-uid', 'verification-pv-uid', 'input-bundle-sha256',
        'bootstrap-creds-sha256', 'server-config-sha256', 'plan-sha256', 'archive-sha256'):
        parser.add_argument('--' + name, required=True)
    parser.add_argument('--input-bundle-bytes', required=True, type=int)
    parser.add_argument('--bootstrap-creds-bytes', required=True, type=int)
    parser.add_argument('--server-config-bytes', required=True, type=int)
    parser.add_argument('--plan-bytes', required=True, type=int)
    parser.add_argument('--archive-bytes', required=True, type=int)
    parser.add_argument('--pod-uid')
    parser.add_argument('--action-index', type=int)
    parser.add_argument('--intent-sha256')
    parser.add_argument('--issued-sha256')
    return parser


def _serve_loop() -> None:
    # The container command is a fixed resident control process. It performs no
    # broker start and no state mutation; the host must issue a typed action.
    if os.geteuid() != 65532 or os.getegid() != 65532:
        _fail('identity')
    while True:
        time.sleep(60)


def main(argv=None) -> int:
    """Closed executable interface used by the admitted contract-migration Job."""
    try:
        args = _cli_parser().parse_args(argv)
        mode = next(name for name in ('serve', 'load_plan', 'read_plan', 'broker_start', 'broker_stop',
            'broker_status', 'read_broker_start', 'read_broker_stop',
            'inspect_action', 'execute_action', 'readback_action')
            if getattr(args, name))
        # The Pod UID is taken from the downward API inside the container and
        # from the verified Pod object by the host exec caller.
        expected = _expected_from_args(args)
        if mode == 'serve':
            if os.geteuid() != 65532 or os.getegid() != 65532:
                _fail('identity')
            _serve_loop()
            return 0
        binding = {**expected}
        if mode == 'load_plan':
            if os.geteuid() != 65532 or os.getegid() != 65532:
                _fail('identity')
            raw = sys.stdin.buffer.read(MAX_INPUT_BYTES + 1)
            if len(raw) != args.input_bundle_bytes:
                _fail('input_size')
            result = load_plan(raw, expected=expected, state_root=STATE_ROOT)
        elif mode == 'read_plan':
            if os.geteuid() != 65532 or os.getegid() != 65532:
                _fail('identity')
            result = read_plan(state_root=STATE_ROOT, binding=binding)
        else:
            _verify_input_receipt(expected)
            plan = load_stored_plan(state_root=STATE_ROOT, binding=binding)
            if mode.startswith('read_broker_'):
                if os.geteuid() != 65532 or os.getegid() != 65532:
                    _fail('identity')
                result = read_broker_receipt(state_root=STATE_ROOT, expected=expected,
                    action=mode.removeprefix('read_broker_'))
            elif mode.startswith('broker_'):
                if os.geteuid() != 65532 or os.getegid() != 65532:
                    _fail('identity')
                _ensure_input_profile(expected)
                action = mode.removeprefix('broker_')
                if action == 'start':
                    def start_broker():
                        value = _contract_supervisor('start')
                        ready()
                        return value
                    result = _broker_receipt(STATE_ROOT, binding, action, start_broker)
                elif action == 'stop':
                    identity = _broker_identity(STATE_ROOT, binding)
                    live = _contract_supervisor('status')
                    if live.get('pid') != identity['pid'] or live.get('started_at_ns') != identity['started_at_ns']:
                        _fail('broker_identity_changed')
                    def stop_broker():
                        value = _contract_supervisor('stop')
                        if value.get('pid') != identity['pid'] or value.get('started_at_ns') != identity['started_at_ns']:
                            _fail('broker_stop_identity')
                        return value
                    result = _broker_receipt(STATE_ROOT, binding, action,
                        stop_broker)
                else:
                    result = {'schema': RECEIPT_SCHEMA, **binding, 'status': 'READ_ONLY',
                        'action': 'broker-status', 'broker': _contract_supervisor('status')}
            else:
                if type(args.action_index) is not int:
                    _fail('action_index')
                if mode == 'inspect_action':
                    result = inspect_action(plan, args.action_index,
                        NatsActor(isolation_check=lambda: _contract_supervisor('status'),
                            expected_broker=lambda: _broker_identity(STATE_ROOT, binding),
                            creds_path=_credential_path(expected)))
                elif mode == 'execute_action':
                    result = execute_action(plan, args.action_index, binding=binding,
                        before_event_sha256=args.intent_sha256, issued_event_sha256=args.issued_sha256,
                        state_root=STATE_ROOT,
                        actor=NatsActor(isolation_check=lambda: _contract_supervisor('status'),
                            expected_broker=lambda: _broker_identity(STATE_ROOT, binding),
                            creds_path=_credential_path(expected)))
                else:
                    result = action_readback(state_root=STATE_ROOT, binding=binding, index=args.action_index)
        sys.stdout.buffer.write(_response_bytes(result))
        return 0
    except (Exception, SystemExit):
        # Never print parser, filesystem, NATS, or credential-bearing errors.
        sys.stderr.write('contract_runner_failed\n')
        return 2


def ready(*, timeout=30, sleep=time.sleep, open_url=urlopen):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        try:
            with open_url('http://127.0.0.1:8222/healthz', timeout=1) as response:
                if response.status == 200:
                    return
        except Exception:
            sleep(0.25)
    _fail('broker_not_ready')


def _binding(envelope: dict, expected: dict) -> dict:
    return {key: expected[key] for key in ('operation', 'nonce', 'pod_uid',
        'contract_attempt', 'source_pvc_uid', 'source_pv_uid', 'verification_pvc_uid',
        'verification_pv_uid', 'input_bundle_sha256', 'input_bundle_bytes',
        'archive_sha256', 'archive_bytes', 'server_config_sha256', 'plan_sha256',
        'plan_bytes', 'bootstrap_creds_sha256', 'bootstrap_creds_bytes', 'server_config_bytes')}


def _state_dir(state_root: Path, operation: str, attempt: str) -> Path:
    if not OP_RE.fullmatch(operation or '') or not ATTEMPT_RE.fullmatch(attempt or ''):
        _fail('state_identity')
    # Kubernetes v1.34.3 emptyDir setup creates the fixed volume root at 0777.
    # Its authority comes from this admitted Pod's volume mount and verified
    # Pod UID, not from a root-owner check: only the trusted control container
    # mounts this volume (the NATS sidecar does not), and no fsGroup is applied.
    # This process creates one exclusive owner-only attempt directory below it.
    # Never create a missing mountpoint or follow an existing link here.
    root = Path(state_root)
    try:
        root_stat = root.lstat()
    except OSError:
        _fail('state_root_missing')
    if (not stat.S_ISDIR(root_stat.st_mode) or stat.S_ISLNK(root_stat.st_mode)
        or stat.S_IMODE(root_stat.st_mode) != 0o777):
        _fail('state_root_profile')
    return root / attempt


def load_plan(raw: bytes, *, expected: dict, state_root: Path) -> dict:
    """Persist one plan and its sole credential once; STARTED is never replayed."""
    envelope = validate_envelope(raw, expected=expected)
    binding = _binding(envelope, expected)
    directory = _state_dir(state_root, expected['operation'], expected['contract_attempt'])
    try:
        directory.mkdir(mode=0o700, parents=False, exist_ok=False)
    except FileExistsError:
        _fail('attempt_already_started')
    row = directory.lstat()
    if (not stat.S_ISDIR(row.st_mode) or stat.S_ISLNK(row.st_mode) or row.st_uid != os.geteuid()
        or row.st_gid != os.getegid() or stat.S_IMODE(row.st_mode) != 0o700):
        _fail('state_directory_untrusted')
    started = {'schema': INPUT_RECEIPT_SCHEMA, 'phase': 'contract-inputs',
        'purpose': 'nats-contract-migration', **binding, 'status': 'STARTED'}
    _atomic_private(directory / 'input.started.json', started)
    credential = envelope['_files']['bootstrap.creds']
    server_config = envelope['_files']['server.conf']
    _atomic_credential(directory / 'bootstrap.creds', credential)
    _atomic_projection(SERVER_CONFIG, server_config)
    _ensure_input_profile(expected, state_root=state_root)
    _atomic_private(directory / 'plan.json', {'schema': SCHEMA, **binding,
        'plan': envelope['plan']})
    complete = {'schema': INPUT_RECEIPT_SCHEMA, 'phase': 'contract-inputs',
        'purpose': 'nats-contract-migration', **binding, 'status': 'COMPLETE',
        'file_count': 2}
    _atomic_private(directory / 'input.receipt.json', complete)
    return complete


def _load_bound_plan(directory: Path, expected: dict) -> dict:
    receipt = _verify_input_receipt(expected, state_root=directory.parent)
    if receipt.get('status') != 'COMPLETE':
        _fail('plan_receipt_binding')
    stored = _private_read(directory / 'plan.json')
    if (stored.get('schema') != SCHEMA or stored.get('plan_sha256') != expected.get('plan_sha256')
        or any(stored.get(k) != expected.get(k) for k in expected)):
        _fail('plan_binding')
    plan = validate_plan(stored.get('plan'))
    if nats_contract_plan.digest(plan) != expected.get('plan_sha256'):
        _fail('plan_digest')
    return plan


def load_stored_plan(*, state_root: Path, binding: dict) -> dict:
    expected = {key: binding[key] for key in ('operation', 'nonce', 'pod_uid',
        'contract_attempt', 'source_pvc_uid', 'source_pv_uid', 'verification_pvc_uid',
        'verification_pv_uid', 'input_bundle_sha256', 'input_bundle_bytes',
        'archive_sha256', 'archive_bytes', 'server_config_sha256', 'plan_sha256',
        'plan_bytes', 'bootstrap_creds_sha256', 'bootstrap_creds_bytes', 'server_config_bytes')}
    return _load_bound_plan(_state_dir(state_root, expected['operation'],
        expected['contract_attempt']), expected)


def read_plan(*, state_root: Path, binding: dict) -> dict:
    """Read only a complete same-Pod plan/input receipt; no repair or replay."""
    expected = {key: binding[key] for key in ('operation', 'nonce', 'pod_uid',
        'contract_attempt', 'source_pvc_uid', 'source_pv_uid', 'verification_pvc_uid',
        'verification_pv_uid', 'input_bundle_sha256', 'input_bundle_bytes',
        'archive_sha256', 'archive_bytes', 'server_config_sha256', 'plan_sha256',
        'plan_bytes', 'bootstrap_creds_sha256', 'bootstrap_creds_bytes', 'server_config_bytes')}
    receipt = _verify_input_receipt(expected, state_root=state_root)
    _load_bound_plan(_state_dir(state_root, expected['operation'], expected['contract_attempt']), expected)
    return receipt


def _action(plan: dict, index: int) -> tuple[dict, str]:
    if type(index) is not int or not 0 <= index < len(plan['actions']):
        _fail('action_index')
    action = plan['actions'][index]
    target = action.get('normalized_config_sha256', nats_contract_plan.digest(action['after']))
    if not isinstance(target, str) or re.fullmatch(r'[a-f0-9]{64}', target) is None:
        _fail('action_target')
    return action, target


def inspect_action(plan: dict, index: int, actor: NatsActor) -> dict:
    """Read current state for exactly one plan index; no mutation or journaling."""
    plan = validate_plan(plan)
    action, target = _action(plan, index)
    actor.assert_closed_isolation()
    current = actor.info_config(action['object'])
    return {'schema': 'voice-nats-contract-inspection-v1', 'action_index': index,
        'object': action['object'], 'api': action['api'], 'before_sha256':
        nats_contract_plan.digest(current) if current is not None else None,
        'target_sha256': target, 'current': current,
        'state': ('TARGET' if current is not None and nats_contract_plan.digest(current) == target
            else 'BEFORE' if current == action['before'] else 'DRIFT')}


def execute_action(plan: dict, index: int, *, binding: dict, before_event_sha256: str,
                   issued_event_sha256: str, state_root: Path, actor: NatsActor) -> dict:
    """Apply one exact plan action once after host journal intent/issued commits."""
    plan = validate_plan(plan)
    action, target = _action(plan, index)
    for value in (before_event_sha256, issued_event_sha256):
        if not isinstance(value, str) or re.fullmatch(r'[a-f0-9]{64}', value) is None:
            _fail('host_journal_digest')
    expected_binding = {key: binding[key] for key in (
        'operation', 'nonce', 'pod_uid', 'contract_attempt', 'source_pvc_uid',
        'source_pv_uid', 'verification_pvc_uid', 'verification_pv_uid', 'input_bundle_sha256',
        'input_bundle_bytes', 'archive_sha256', 'archive_bytes', 'server_config_sha256',
        'plan_sha256', 'plan_bytes', 'bootstrap_creds_sha256', 'bootstrap_creds_bytes',
        'server_config_bytes')}
    directory = _state_dir(state_root, expected_binding['operation'], expected_binding['contract_attempt'])
    stored_plan = _load_bound_plan(directory, expected_binding)
    if stored_plan != plan:
        _fail('plan_changed')
    intent = {'kind': 'nats_contract_intent', 'object': action['object'], 'api': action['api'],
        'before_sha256': nats_contract_plan.digest(action['before']), 'target_sha256': target,
        'migration_attempt': expected_binding['contract_attempt']}
    issued = {'kind': 'nats_contract_mutation_issued', 'object': action['object'], 'api': action['api'],
        'target_sha256': target, 'migration_attempt': expected_binding['contract_attempt']}
    if (before_event_sha256 != hashlib.sha256(_canonical(intent)).hexdigest()
        or issued_event_sha256 != hashlib.sha256(_canonical(issued)).hexdigest()):
        _fail('host_journal_mismatch')
    started_path = directory / ('action-%02d.started.json' % index)
    complete_path = directory / ('action-%02d.receipt.json' % index)
    if started_path.exists() or complete_path.exists():
        _fail('action_already_started')
    actor.assert_closed_isolation()
    current = actor.info_config(action['object'])
    if current != action['before']:
        _fail('action_before_drift')
    started = {'schema': RECEIPT_SCHEMA, **expected_binding, 'status': 'STARTED',
        'action_index': index, 'object': action['object'], 'api': action['api'],
        'before_event_sha256': before_event_sha256,
        'issued_event_sha256': issued_event_sha256, 'target_sha256': target}
    _atomic_private(started_path, started)
    actor.assert_closed_isolation()
    actor.mutate(action['api'], copy.deepcopy(action['after']))
    actor.assert_closed_isolation()
    after = actor.info_config(action['object'])
    if after is None or nats_contract_plan.digest(after) != target:
        _fail('action_target_unverified')
    complete = {'schema': RECEIPT_SCHEMA, **expected_binding, 'status': 'COMPLETE',
        'action_index': index, 'object': action['object'], 'api': action['api'],
        'before_event_sha256': before_event_sha256,
        'issued_event_sha256': issued_event_sha256, 'target_sha256': target,
        'config_sha256': nats_contract_plan.digest(after), 'recovered_exact_target': False}
    _atomic_private(complete_path, complete)
    return complete


def action_readback(*, state_root: Path, binding: dict, index: int) -> dict:
    """Return only the exact durable action receipt; absence/incomplete is UNKNOWN."""
    directory = _state_dir(state_root, binding.get('operation'), binding.get('contract_attempt'))
    expected = {key: binding[key] for key in ('operation', 'nonce', 'pod_uid',
        'contract_attempt', 'source_pvc_uid', 'source_pv_uid', 'verification_pvc_uid',
        'verification_pv_uid', 'input_bundle_sha256', 'input_bundle_bytes', 'archive_sha256',
        'archive_bytes', 'server_config_sha256', 'plan_sha256', 'plan_bytes',
        'bootstrap_creds_sha256', 'bootstrap_creds_bytes', 'server_config_bytes')}
    started_path = directory / ('action-%02d.started.json' % index)
    complete_path = directory / ('action-%02d.receipt.json' % index)
    if not started_path.exists():
        _fail('action_unknown')
    started = _private_read(started_path)
    if (started.get('status') != 'STARTED' or started.get('action_index') != index
        or any(started.get(k) != v for k, v in expected.items())):
        _fail('action_readback_binding')
    if not complete_path.exists():
        _fail('action_unknown')
    complete = _private_read(complete_path)
    if (complete.get('status') != 'COMPLETE' or complete.get('action_index') != index
        or complete.get('target_sha256') != started.get('target_sha256')
        or any(complete.get(k) != v for k, v in expected.items())):
        _fail('action_receipt_invalid')
    return complete


if __name__ == '__main__':
    raise SystemExit(main())
