"""Closed local RPC between the untrusted workflow submitter and fixed runner."""
from __future__ import annotations

import json
import fcntl
import hashlib
import os
import re
import socket
import stat
import struct
from pathlib import Path

from bridge_state import StateError


SCHEMA = 'voice-staging-bridge-runner-request-v1'
MAX_FRAME = 16384
SUBMITTER_UID = 1000
SOCKET_OWNER_UID = 0
FIELDS = frozenset(('schema', 'protocol', 'action', 'operation', 'nonce'))


class RpcError(ValueError):
    pass


def validate(row: object) -> dict:
    if (not isinstance(row, dict) or set(row) != FIELDS or row.get('schema') != SCHEMA
        or type(row.get('protocol')) is not int or row['protocol'] != 1
        or row.get('action') != 'status'
        or not isinstance(row.get('operation'), str) or not re.fullmatch('[a-f0-9]{12}', row['operation'])
        or not isinstance(row.get('nonce'), str) or not re.fullmatch('[a-f0-9]{64}', row['nonce'])):
        raise RpcError('runner_request_invalid')
    return {'action': row['action'], 'operation': row['operation'], 'nonce': row['nonce']}


def _read_frame(connection: socket.socket) -> bytes:
    chunks = bytearray()
    while len(chunks) <= MAX_FRAME:
        part = connection.recv(min(4096, MAX_FRAME + 1 - len(chunks)))
        if not part:
            break
        chunks.extend(part)
        if b'\n' in part:
            break
    if len(chunks) > MAX_FRAME or not chunks.endswith(b'\n') or chunks.count(b'\n') != 1:
        raise RpcError('runner_frame_invalid')
    return bytes(chunks[:-1])


def _write_frame(connection: socket.socket, row: dict) -> None:
    raw = (json.dumps(row, sort_keys=True, separators=(',', ':')) + '\n').encode()
    if len(raw) > MAX_FRAME:
        raise RpcError('runner_response_too_large')
    connection.sendall(raw)


def send_request(connection: socket.socket, row: dict) -> dict:
    raw = (json.dumps(row, sort_keys=True, separators=(',', ':')) + '\n').encode()
    if len(raw) > MAX_FRAME:
        raise RpcError('runner_request_too_large')
    connection.sendall(raw)
    try:
        response = json.loads(_read_frame(connection), object_pairs_hook=_unique_pairs)
    except (ValueError, TypeError, UnicodeError):
        raise RpcError('runner_response_invalid') from None
    if not isinstance(response, dict) or response.get('schema') != SCHEMA or response.get('status') not in ('COMPLETE', 'BLOCKED'):
        raise RpcError('runner_response_invalid')
    return response


class Runner:
    """Public RPC delegates only to the fixed installed high-level action table."""
    def __init__(self, actions):
        if os.geteuid() == 0:
            raise RpcError('runner_must_be_nonroot')
        self.actions = actions

    def handle(self, value: object) -> dict:
        request = validate(value)
        try:
            result = self.actions.execute(request)
            return {'schema': SCHEMA, 'status': 'COMPLETE', 'operation': request['operation'],
                    'nonce': request['nonce'], 'result': result}
        except (StateError, RpcError):
            raise
        except Exception as error:
            raise RpcError('runner_operation_failed') from error


class DurableState:
    """Small runner-owned journal; an incomplete create intent is never replayed."""
    def __init__(self, directory: str | Path, binding_resolver):
        self.directory = Path(directory)
        self.binding_resolver = binding_resolver
        row = self.directory.lstat()
        if (not stat.S_ISDIR(row.st_mode) or row.st_uid != os.geteuid()
            or stat.S_IMODE(row.st_mode) != 0o700):
            raise RpcError('runner_state_directory_invalid')
        self.lock_path = self.directory / 'operation.lock'

    def _path(self, request):
        return self.directory / (request['operation'] + '-' + request['nonce'] + '.json')

    def _read(self, path):
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
        try:
            before = os.fstat(fd)
            if (not stat.S_ISREG(before.st_mode) or before.st_uid != os.geteuid()
                or stat.S_IMODE(before.st_mode) != 0o600 or before.st_nlink != 1
                or not 1 <= before.st_size <= MAX_FRAME):
                raise RpcError('runner_journal_custody_invalid')
            raw = os.read(fd, before.st_size + 1)
            after = os.fstat(fd)
            if len(raw) != before.st_size or (before.st_dev, before.st_ino, before.st_size,
                before.st_mtime_ns, before.st_ctime_ns) != (after.st_dev, after.st_ino,
                after.st_size, after.st_mtime_ns, after.st_ctime_ns):
                raise RpcError('runner_journal_changed')
            return json.loads(raw, object_pairs_hook=_unique_pairs)
        finally:
            os.close(fd)

    def _write(self, path, row):
        raw = (json.dumps(row, sort_keys=True, separators=(',', ':')) + '\n').encode()
        if len(raw) > MAX_FRAME:
            raise RpcError('runner_journal_too_large')
        scratch = self.directory / ('.' + path.name + '.pending')
        fd = os.open(scratch, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        try:
            with os.fdopen(fd, 'wb') as output:
                output.write(raw)
                output.flush()
                os.fsync(output.fileno())
            os.replace(scratch, path)
            folder = os.open(self.directory, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            try:
                os.fsync(folder)
            finally:
                os.close(folder)
        finally:
            scratch.unlink(missing_ok=True)

    class _Operation:
        def __init__(self, state, request):
            self.state = state
            self.request = request
            self.fd = None
            self.row = None

        def __enter__(self):
            try:
                return self._open()
            except BaseException:
                self.__exit__(None, None, None)
                raise

        def _open(self):
            state, request = self.state, self.request
            self.fd = os.open(state.lock_path, os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
            lock = os.fstat(self.fd)
            if not stat.S_ISREG(lock.st_mode) or lock.st_uid != os.geteuid() or stat.S_IMODE(lock.st_mode) != 0o600 or lock.st_nlink != 1:
                raise RpcError('runner_lock_custody_invalid')
            try:
                fcntl.flock(self.fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError as error:
                os.close(self.fd)
                self.fd = None
                raise RpcError('runner_operation_busy') from error
            path = state._path(request)
            fingerprint = hashlib.sha256(json.dumps(request, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
            try:
                self.row = state._read(path)
            except FileNotFoundError:
                self.row = {'schema': 'voice-nats-runner-journal-v1', 'request': request,
                    'request_sha256': fingerprint, 'state': 'NEW', 'phase_state': None, 'response': None}
            if self.row.get('request_sha256') != fingerprint or self.row.get('request') != request:
                raise RpcError('runner_nonce_reused')
            if self.row.get('state') == 'COMPLETE':
                return None, None, None, self.row.get('response')
            if self.row.get('state') not in ('NEW', 'JOB_CREATE_INTENT', 'JOB_RESULT_READBACK'):
                raise RpcError('runner_journal_state_invalid')
            if self.row.get('state') != 'NEW':
                raise RpcError('runner_interrupted_requires_reconciliation')
            from tokenless_job import validate_binding
            binding = validate_binding(state.binding_resolver(request['operation']))
            def persist_intent(value):
                self.row['state'] = 'JOB_CREATE_INTENT'
                self.row['phase_state'] = value
                state._write(path, self.row)
            def persist_result(value):
                self.row['state'] = 'JOB_RESULT_READBACK'
                self.row['phase_state'] = value
                state._write(path, self.row)
            return binding, persist_intent, persist_result, None

        def __exit__(self, _type, _value, _traceback):
            if self.fd is not None:
                fcntl.flock(self.fd, fcntl.LOCK_UN)
                os.close(self.fd)

    def operation(self, request):
        return self._Operation(self, request)

    def complete(self, request, response):
        path = self._path(request)
        row = self._read(path)
        if row.get('request') != request or row.get('state') != 'JOB_RESULT_READBACK':
            raise RpcError('runner_completion_state_invalid')
        row['state'] = 'COMPLETE'
        row['response'] = response
        self._write(path, row)


def serve_connection(runner: Runner, connection: socket.socket) -> None:
    try:
        try:
            peer = connection.getsockopt(socket.SOL_SOCKET, socket.SO_PEERCRED, struct.calcsize('3i'))
            _pid, uid, _gid = struct.unpack('3i', peer)
            if uid != SUBMITTER_UID:
                raise RpcError('runner_peer_invalid')
        except (AttributeError, OSError, struct.error):
            # Platforms without SO_PEERCRED are unsupported; never infer trust.
            raise RpcError('runner_peer_credentials_unavailable')
        try:
            request = json.loads(_read_frame(connection), object_pairs_hook=_unique_pairs)
            response = runner.handle(request)
        except (RpcError, StateError) as error:
            response = {'schema': SCHEMA, 'status': 'BLOCKED', 'error': str(error)}
        _write_frame(connection, response)
    finally:
        connection.close()


def _unique_pairs(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise RpcError('runner_duplicate_field')
        result[key] = value
    return result


def request(path: str, row: dict, timeout: float = 30) -> dict:
    info = os.lstat(path)
    if not stat.S_ISSOCK(info.st_mode) or info.st_uid != SOCKET_OWNER_UID or info.st_mode & 0o002:
        raise RpcError('runner_socket_invalid')
    connection = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    connection.settimeout(timeout)
    try:
        try:
            connection.connect(path)
            return send_request(connection, row)
        except OSError:
            raise RpcError('runner_socket_unavailable') from None
    finally:
        connection.close()
