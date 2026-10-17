"""Fixed HTTPS Kubernetes JSON API client for the unprivileged rollout runner.

Origin, CA, namespace, token file, service account, audience, and lifetime are
installed trusted configuration. Callers can provide resource identities and
fixed protocol objects only; this module never invokes kubectl or a shell.
"""
from __future__ import annotations

import base64
import hashlib
import json
import os
from pathlib import Path
import re
import socket
import ssl
import stat
import time
from urllib.error import HTTPError, URLError
from urllib.parse import quote, urlencode, urlsplit
from urllib.request import HTTPSHandler, ProxyHandler, Request, build_opener


NAMESPACE = 'voice-staging'
SA_NAME = 'voice-nats-staging-runner'
TOKEN_AUDIENCE = 'voice-staging-runner'
TOKEN_TTL_SECONDS = 3600
TOKEN_RENEW_BEFORE_SECONDS = 1800
MIN_OPERATION_TOKEN_SECONDS = 300
MAX_JSON_RESPONSE = 4 << 20
MAX_LOG_RESPONSE = 65536
MAX_ARCHIVE_STREAM_BYTES = 66 << 30
MAX_CENSUS_MANIFEST_BYTES = 512 << 20
MAX_EXEC_STDIN_CHUNK = 48 << 10
CENSUS_INPUT_MAGIC = b'VOICE-NATS-CENSUS-INPUT-v1\0'


class KubeAPIError(ValueError):
    """Safe operation labels only; API bodies, URLs, and credentials stay private."""


class KubeNotFound(KubeAPIError):
    pass


_UID = re.compile(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}')
_SOURCE_CLAIM = re.compile(r'voice-nats-jsdata-d[0-9]{8}[a-z0-9]{1,8}')


def _fail(code):
    raise KubeAPIError('kube_api_' + code)


def _b64url(raw: str) -> bytes:
    return base64.urlsafe_b64decode(raw + '=' * ((4 - len(raw) % 4) % 4))


def _jwt_claims(token: str) -> dict:
    parts = token.split('.')
    if len(parts) != 3 or any(not re.fullmatch('[A-Za-z0-9_-]+', part) for part in parts):
        _fail('token_format_invalid')
    try:
        claims = json.loads(_b64url(parts[1]))
    except (ValueError, TypeError):
        _fail('token_claims_invalid')
    if not isinstance(claims, dict):
        _fail('token_claims_invalid')
    return claims


class TokenFile:
    """Read an atomic, private, short-lived TokenRequest JWT owned by the runner."""
    def __init__(self, path, *, audience=TOKEN_AUDIENCE, owner_uid=None, clock=time.time):
        self.path = Path(path)
        self.audience = audience
        self.owner_uid = os.geteuid() if owner_uid is None else owner_uid
        self.clock = clock

    def read(self, *, minimum_seconds=TOKEN_RENEW_BEFORE_SECONDS) -> str:
        try:
            fd = os.open(self.path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
        except OSError:
            _fail('token_file_unavailable')
        try:
            before = os.fstat(fd)
            if (not stat.S_ISREG(before.st_mode) or before.st_uid != self.owner_uid
                or stat.S_IMODE(before.st_mode) != 0o600 or before.st_nlink != 1
                or not 1 <= before.st_size <= 16384):
                _fail('token_file_custody_invalid')
            raw = os.read(fd, before.st_size + 1)
            after = os.fstat(fd)
            if len(raw) != before.st_size or (before.st_dev, before.st_ino, before.st_size,
                before.st_mtime_ns, before.st_ctime_ns) != (after.st_dev, after.st_ino,
                after.st_size, after.st_mtime_ns, after.st_ctime_ns):
                _fail('token_file_changed')
        finally:
            os.close(fd)
        try:
            token = raw.decode('ascii').strip()
        except UnicodeError:
            _fail('token_format_invalid')
        claims = _jwt_claims(token)
        audience = claims.get('aud')
        audiences = [audience] if isinstance(audience, str) else audience
        expires = claims.get('exp')
        if (not isinstance(audiences, list) or self.audience not in audiences
            or claims.get('sub') != 'system:serviceaccount:voice-staging:voice-nats-staging-runner'
            or type(expires) is not int or expires - int(self.clock()) < minimum_seconds):
            _fail('token_budget_invalid')
        return token


class _KubernetesJSONAPIBase:
    """Small fixed-resource adapter over TLS; every request reloads the token file."""
    def __init__(self, api_server, ca_file, token_file, *, audience=TOKEN_AUDIENCE,
                 owner_uid=None, trusted_config_uid=0, opener=None, timeout=15, clock=time.time):
        try:
            parsed = urlsplit(api_server)
            if (parsed.scheme != 'https' or not parsed.hostname or parsed.username or parsed.password
                or parsed.query or parsed.fragment or parsed.path not in ('', '/')
                or parsed.port not in (443, 6443)):
                _fail('server_invalid')
        except (TypeError, ValueError):
            _fail('server_invalid')
        self.origin = 'https://' + parsed.netloc
        self.ca_file = Path(ca_file)
        self.timeout = timeout
        try:
            ca_info = self.ca_file.lstat()
            if (not stat.S_ISREG(ca_info.st_mode) or ca_info.st_uid != trusted_config_uid
                or stat.S_IMODE(ca_info.st_mode) & 0o222
                or ca_info.st_size == 0 or ca_info.st_size > 1 << 20):
                _fail('ca_custody_invalid')
            context = ssl.create_default_context(cafile=str(self.ca_file))
        except (OSError, ssl.SSLError):
            _fail('ca_unavailable')
        self.context = context
        self.tokens = TokenFile(token_file, audience=audience, owner_uid=owner_uid, clock=clock)
        if opener is None:
            # Explicitly disable environment-configured HTTP(S) proxies.
            self.opener = build_opener(ProxyHandler({}), HTTPSHandler(context=context)).open
        else:
            self.opener = opener

    @staticmethod
    def _segment(value):
        if not isinstance(value, str) or not value or len(value) > 253 or '\x00' in value:
            _fail('resource_name_invalid')
        return quote(value, safe='')

    def _request(self, method, path, body=None, *, log=False, token_refresh=True):
        if not isinstance(path, str) or not path.startswith('/') or path.startswith('//'):
            _fail('path_invalid')
        data = None if body is None else json.dumps(body, sort_keys=True, separators=(',', ':')).encode()
        token = self._operation_token() if token_refresh else self._refresh_token_credential()
        headers = {'Accept': 'application/json', 'Authorization': 'Bearer ' + token}
        if data is not None:
            headers['Content-Type'] = 'application/json'
        request = Request(self.origin + path, data=data, headers=headers, method=method)
        try:
            with self.opener(request, context=self.context, timeout=self.timeout) as response:
                limit = MAX_LOG_RESPONSE if log else MAX_JSON_RESPONSE
                raw = response.read(limit + 1)
                if len(raw) > limit:
                    _fail('response_too_large')
                if log:
                    return raw.decode('utf-8')
                try:
                    result = json.loads(raw)
                except (ValueError, UnicodeError):
                    _fail('json_response_invalid')
                if not isinstance(result, dict):
                    _fail('json_response_shape_invalid')
                return result
        except KubeAPIError:
            raise
        except HTTPError as error:
            if error.code == 404:
                raise KubeNotFound('kube_api_not_found') from None
            _fail('transport_failed')
        except (URLError, TimeoutError, OSError, UnicodeError):
            _fail('transport_failed')

    def _refresh_token_credential(self):
        try:
            return self.tokens.read(minimum_seconds=30)
        except KubeAPIError:
            _fail('token_unavailable')

    def _operation_token(self):
        try:
            return self.tokens.read(minimum_seconds=TOKEN_RENEW_BEFORE_SECONDS)
        except KubeAPIError:
            # Renewal is fixed to the trusted SA subresource; any failure blocks
            # work before its next API request.
            self._refresh_token_credential()
            refreshed = self.request_token_refresh()
            self.save_refreshed_token(refreshed, self.tokens.path)
            try:
                return self.tokens.read(minimum_seconds=MIN_OPERATION_TOKEN_SECONDS)
            except KubeAPIError:
                _fail('token_budget_invalid')

    def get_pvc(self, namespace, name):
        if namespace != NAMESPACE: _fail('namespace_invalid')
        return self._request('GET', '/api/v1/namespaces/' + self._segment(namespace) + '/persistentvolumeclaims/' + self._segment(name))

    def get_pv(self, name):
        return self._request('GET', '/api/v1/persistentvolumes/' + self._segment(name))

    def get_namespace(self, name):
        if name != NAMESPACE: _fail('namespace_invalid')
        return self._request('GET', '/api/v1/namespaces/' + self._segment(name))

    def get_configmap(self, namespace, name):
        if namespace != NAMESPACE or name != 'voice-nats-generation':
            _fail('configmap_invalid')
        return self._request('GET', '/api/v1/namespaces/' + self._segment(namespace)
            + '/configmaps/' + self._segment(name))

    def get_deployment(self, namespace, name):
        if namespace != NAMESPACE or name != 'voice-nats-hub':
            _fail('deployment_invalid')
        return self._request('GET', '/apis/apps/v1/namespaces/' + self._segment(namespace)
            + '/deployments/' + self._segment(name))

    def list_nodes_by_hostname(self, hostname):
        if (not isinstance(hostname, str) or len(hostname) > 253
            or not re.fullmatch(r'[a-z0-9](?:[-a-z0-9.]*[a-z0-9])?', hostname)):
            _fail('node_hostname_invalid')
        selector = urlencode({'labelSelector': 'kubernetes.io/hostname=' + hostname})
        result = self._request('GET', '/api/v1/nodes?' + selector)
        items = result.get('items')
        if not isinstance(items, list) or len(items) > 2048 or not all(isinstance(item, dict) for item in items):
            _fail('node_list_invalid')
        return items

    def list_hub_pods(self, labels):
        if (not isinstance(labels, dict) or not labels or len(labels) > 32
            or any(not isinstance(key, str) or not isinstance(value, str) or not key or not value
                for key, value in labels.items())):
            _fail('pod_selector_invalid')
        selector = ','.join(quote(key, safe='') + '=' + quote(value, safe='')
            for key, value in sorted(labels.items()))
        result = self._request('GET', '/api/v1/namespaces/' + self._segment(NAMESPACE)
            + '/pods?' + urlencode({'labelSelector': selector}))
        items = result.get('items')
        if not isinstance(items, list) or len(items) > 2048 or not all(isinstance(item, dict) for item in items):
            _fail('pod_list_invalid')
        return items

    def get_node(self, name):
        return self._request('GET', '/api/v1/nodes/' + self._segment(name))

    def create_job(self, namespace, document):
        if namespace != NAMESPACE: _fail('namespace_invalid')
        return self._request('POST', '/apis/batch/v1/namespaces/' + self._segment(namespace) + '/jobs', document)

    def create_verification_pvc(self, operation, nonce, size_bytes, storage_class):
        """Create the one fixed operation target claim from trusted installed config."""
        if (not isinstance(operation, str) or not re.fullmatch('[a-f0-9]{12}', operation)
            or not isinstance(nonce, str) or not re.fullmatch('[a-f0-9]{32}', nonce)
            or type(size_bytes) is not int or not 0 < size_bytes <= (1 << 63) - 1
            or not isinstance(storage_class, str) or len(storage_class) > 253
            or any(not label or len(label) > 63
                or not re.fullmatch(r'[a-z0-9](?:[-a-z0-9]*[a-z0-9])?', label)
                for label in storage_class.split('.'))):
            _fail('verification_pvc_request_invalid')
        name = 'voice-nats-verify-' + operation
        document = {
            'apiVersion': 'v1', 'kind': 'PersistentVolumeClaim',
            'metadata': {'name': name, 'namespace': NAMESPACE,
                'labels': {'voice.io/nats-operation': operation,
                    'voice.io/nats-job-nonce': nonce,
                    'voice.io/nats-pvc-role': 'verification'}},
            'spec': {'accessModes': ['ReadWriteOnce'], 'volumeMode': 'Filesystem',
                'storageClassName': storage_class,
                'resources': {'requests': {'storage': str(size_bytes)}}},
        }
        return self._request('POST', '/api/v1/namespaces/' + self._segment(NAMESPACE)
            + '/persistentvolumeclaims', document)

    def list_operation_jobs(self, namespace, operation):
        if namespace != NAMESPACE: _fail('namespace_invalid')
        if not isinstance(operation, str) or not re.fullmatch('[a-f0-9]{12}', operation):
            _fail('operation_invalid')
        selector = urlencode({'labelSelector': 'voice.io/nats-operation=' + operation})
        result = self._request('GET', '/apis/batch/v1/namespaces/' + self._segment(namespace)
            + '/jobs?' + selector)
        items = result.get('items')
        if not isinstance(items, list) or not all(isinstance(item, dict) for item in items):
            _fail('job_list_invalid')
        return items

    def get_job(self, namespace, name):
        if namespace != NAMESPACE: _fail('namespace_invalid')
        return self._request('GET', '/apis/batch/v1/namespaces/' + self._segment(namespace) + '/jobs/' + self._segment(name))

    def wait_job(self, namespace, name, uid, timeout):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            row = self.get_job(namespace, name)
            status = row.get('status', {})
            if row.get('metadata', {}).get('uid') != uid:
                _fail('job_identity_changed')
            if status.get('succeeded') == 1 or status.get('failed', 0) > 0:
                return row
            time.sleep(0.5)
        _fail('job_wait_timeout')

    def get_job_pod(self, namespace, job_name):
        if namespace != NAMESPACE: _fail('namespace_invalid')
        labels = urlencode({'labelSelector': 'batch.kubernetes.io/job-name=' + job_name + ',job-name=' + job_name})
        listing = self._request('GET', '/api/v1/namespaces/' + self._segment(namespace) + '/pods?' + labels)
        items = listing.get('items')
        if not isinstance(items, list) or not all(isinstance(item, dict) for item in items):
            _fail('job_pod_cardinality_invalid')
        owned = [item for item in items if any(owner.get('kind') == 'Job' and owner.get('name') == job_name
            and owner.get('controller') is True for owner in item.get('metadata', {}).get('ownerReferences', []))]
        if len(owned) != 1:
            _fail('job_pod_cardinality_invalid')
        return owned[0]

    def wait_job_pod(self, namespace, job_name, job_uid, timeout):
        """Wait for the exact operation Job's one live Pod before pods/exec."""
        if namespace != NAMESPACE or not isinstance(job_uid, str) or not re.fullmatch('[a-f0-9-]{36}', job_uid):
            _fail('job_identity_invalid')
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            job = self.get_job(namespace, job_name)
            if job.get('metadata', {}).get('uid') != job_uid:
                _fail('job_identity_changed')
            if job.get('status', {}).get('failed', 0) > 0:
                _fail('job_failed')
            pod = self.get_job_pod(namespace, job_name)
            if pod.get('status', {}).get('phase') == 'Running':
                statuses = pod.get('status', {}).get('containerStatuses', [])
                if (len(statuses) != 1 or statuses[0].get('name') != 'nats-phase'
                    or not isinstance(statuses[0].get('containerID'), str)
                    or not statuses[0]['containerID'].startswith('containerd://')
                    or statuses[0].get('restartCount') != 0 or statuses[0].get('ready') is not True):
                    _fail('job_pod_container_invalid')
                return pod
            if pod.get('status', {}).get('phase') in ('Failed', 'Succeeded'):
                _fail('archive_pod_not_live')
            time.sleep(0.5)
        _fail('job_pod_wait_timeout')

    def pod_logs(self, namespace, pod_name):
        if namespace != NAMESPACE: _fail('namespace_invalid')
        query = urlencode({'limitBytes': str(MAX_LOG_RESPONSE), 'timestamps': 'false'})
        return self._request('GET', '/api/v1/namespaces/' + self._segment(namespace) + '/pods/' + self._segment(pod_name) + '/log?' + query, log=True)

    def exec_pod_archive(self, namespace, pod_name, pod_uid, operation, nonce, *, existing=False,
                         max_bytes=MAX_ARCHIVE_STREAM_BYTES):
        """Run one fixed archive helper command over Kubernetes pods/exec.

        The trusted runner chooses only create-and-stream or stream-existing.
        Both command vectors and all other URL parameters are constructed here;
        callers cannot select a binary, shell, container, path, or flags.
        """
        if namespace != NAMESPACE: _fail('namespace_invalid')
        if (not isinstance(pod_uid, str) or not re.fullmatch('[a-f0-9-]{36}', pod_uid)
            or not isinstance(operation, str) or not re.fullmatch('[a-f0-9]{12}', operation)
            or not isinstance(nonce, str) or not re.fullmatch('[a-f0-9]{32}', nonce)
            or type(existing) is not bool or type(max_bytes) is not int
            or not 0 < max_bytes <= MAX_ARCHIVE_STREAM_BYTES):
            _fail('archive_stream_identity_invalid')
        pod = self._request('GET', '/api/v1/namespaces/' + self._segment(namespace)
            + '/pods/' + self._segment(pod_name))
        meta = pod.get('metadata', {})
        if meta.get('uid') != pod_uid or meta.get('namespace') != namespace:
            _fail('archive_stream_pod_changed')
        mode = '--stream-existing' if existing else '--create-and-stream'
        command = ['/voice-nats-runner/phase', mode, '--operation', operation,
                   '--nonce', nonce, '--pod-uid', pod_uid]
        query = [('container', 'nats-phase'), ('stdin', 'false'), ('stdout', 'true'),
                 ('stderr', 'false'), ('tty', 'false')]
        query.extend(('command', item) for item in command)
        path = '/api/v1/namespaces/' + self._segment(namespace) + '/pods/' + self._segment(pod_name) + '/exec?' + urlencode(query)
        token = self._operation_token()
        yield from _websocket_exec(self, path, token, max_bytes=max_bytes)

    def exec_pod_verification(self, namespace, pod_name, pod_uid, request, binding, action):
        """Run only a producer-declared archive or census vector in the bound Pod."""
        if namespace != NAMESPACE:
            _fail('namespace_invalid')
        try:
            import verification_job
            vectors = verification_job.exec_vectors(request, pod_uid, binding)
        except (ImportError, ValueError):
            _fail('verification_exec_request_invalid')
        if action not in vectors or action == 'load_inputs':
            # Input loading needs a separate bounded stdin protocol; it is not
            # silently routed through the output-only WebSocket operation.
            _fail('verification_exec_action_invalid')
        argv = vectors[action]
        archive_actions = ('archive_create_and_stream', 'archive_same_pod_readback')
        if action in archive_actions:
            max_bytes = MAX_ARCHIVE_STREAM_BYTES
        elif action == 'census_receipt_readback':
            import census_stream
            max_bytes = census_stream.MAX_TRANSPORT_BYTES
        else:
            max_bytes = MAX_LOG_RESPONSE
        pod = self._request('GET', '/api/v1/namespaces/' + self._segment(namespace)
            + '/pods/' + self._segment(pod_name))
        metadata = pod.get('metadata', {})
        if metadata.get('uid') != pod_uid or metadata.get('namespace') != namespace:
            _fail('verification_exec_pod_changed')
        query = [('container', 'nats-phase'), ('stdin', 'false'), ('stdout', 'true'),
                 ('stderr', 'false'), ('tty', 'false')]
        query.extend(('command', item) for item in argv)
        path = ('/api/v1/namespaces/' + self._segment(namespace) + '/pods/'
            + self._segment(pod_name) + '/exec?' + urlencode(query))
        token = self._operation_token()
        yield from _websocket_exec(self, path, token, max_bytes=max_bytes)

    def exec_pod_census_input(self, namespace, pod_name, pod_uid, request, binding,
                              archive_path, manifest_bytes, archive_sha256, archive_bytes):
        """Stream one verified archive into the fixed, operation-scoped census helper.

        The request and binding are converted to the producer's single closed
        receive-archive vector. The archive remains file-backed throughout; an
        interrupted stream is an unknown helper outcome and must be reconciled
        through its same-Pod receipt, never replayed here.
        """
        if namespace != NAMESPACE:
            _fail('namespace_invalid')
        if (not isinstance(manifest_bytes, bytes) or not 0 < len(manifest_bytes) <= MAX_CENSUS_MANIFEST_BYTES
            or type(archive_bytes) is not int or not 0 < archive_bytes <= MAX_ARCHIVE_STREAM_BYTES
            or not isinstance(archive_sha256, str) or not re.fullmatch('[a-f0-9]{64}', archive_sha256)):
            _fail('census_input_invalid')
        _validate_canonical_manifest(manifest_bytes)
        try:
            import verification_job
            vectors = verification_job.exec_vectors(request, pod_uid, binding,
                archive={'sha256': archive_sha256, 'bytes': archive_bytes})
        except (ImportError, TypeError, ValueError):
            _fail('verification_exec_request_invalid')
        argv = vectors.get('census_receive_archive')
        if not isinstance(argv, list) or not argv:
            _fail('verification_exec_action_invalid')
        pod = self._request('GET', '/api/v1/namespaces/' + self._segment(namespace)
            + '/pods/' + self._segment(pod_name))
        metadata = pod.get('metadata', {})
        if metadata.get('uid') != pod_uid or metadata.get('namespace') != namespace:
            _fail('verification_exec_pod_changed')
        # Check file custody and digest before opening a one-shot mutating stream.
        source = _open_verified_archive(archive_path, archive_bytes, archive_sha256)
        try:
            query = [('container', 'nats-phase'), ('stdin', 'true'), ('stdout', 'true'),
                     ('stderr', 'false'), ('tty', 'false')]
            query.extend(('command', item) for item in argv)
            path = ('/api/v1/namespaces/' + self._segment(namespace) + '/pods/'
                + self._segment(pod_name) + '/exec?' + urlencode(query))
            token = self._operation_token()
            _websocket_exec_input(self, path, token, source, manifest_bytes,
                archive_bytes=archive_bytes, archive_sha256=archive_sha256)
        finally:
            os.close(source)

    def read_verification_census(self, namespace, pod_name, pod_uid, request, binding):
        """Return a validated, identity-bound census from its durable same-Pod receipt."""
        try:
            import census_stream
            import verification_job
            row = verification_job.validate_request(request)
            storage = verification_job.validate_binding(binding)
            if row['phase'] != 'census':
                _fail('verification_census_phase_invalid')
            chunks = self.exec_pod_verification(namespace, pod_name, pod_uid,
                row, storage, 'census_receipt_readback')
            return census_stream.consume(chunks, operation=row['operation'], nonce=row['nonce'],
                pod_uid=pod_uid, source_pvc_uid=storage['source_uid'],
                verification_pvc_uid=storage['verification_uid'])
        except (ImportError, ValueError):
            _fail('verification_census_invalid')


def _websocket_exec(api, path, token, *, max_bytes):
    """Consume Kubernetes channel.k8s.io v5 stdout frames with a strict byte cap."""
    parsed = urlsplit(api.origin)
    host, port = parsed.hostname, parsed.port or 443
    key = base64.b64encode(os.urandom(16)).decode('ascii')
    sock = None
    total = 0
    try:
        raw = socket.create_connection((host, port), timeout=max(api.timeout, 120))
        sock = api.context.wrap_socket(raw, server_hostname=host)
        request = (f'GET {path} HTTP/1.1\r\nHost: {parsed.netloc}\r\n'
            f'Authorization: Bearer {token}\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n'
            f'Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: {key}\r\n'
            'Sec-WebSocket-Protocol: v5.channel.k8s.io\r\n\r\n').encode('ascii')
        sock.sendall(request)
        response = sock.makefile('rb')
        status = response.readline(4096)
        headers = {}
        while True:
            line = response.readline(8192)
            if not line or line in (b'\r\n', b'\n'):
                break
            if b':' not in line:
                _fail('exec_handshake_invalid')
            name, value = line.split(b':', 1)
            headers[name.strip().lower()] = value.strip()
        if (not status.startswith(b'HTTP/1.1 101 ') or headers.get(b'upgrade', b'').lower() != b'websocket'
            or b'upgrade' not in headers.get(b'connection', b'').lower().split(b', ')
            or headers.get(b'sec-websocket-protocol', b'').lower() != b'v5.channel.k8s.io'
            or headers.get(b'sec-websocket-accept') != base64.b64encode(hashlib.sha1(
                (key + '258EAFA5-E914-47DA-95CA-C5AB0DC85B11').encode('ascii')).digest())):
            _fail('exec_handshake_rejected')
        while True:
            first = response.read(2)
            if not first:
                _fail('exec_stream_truncated')
            if len(first) != 2:
                _fail('exec_frame_truncated')
            fin, opcode = bool(first[0] & 0x80), first[0] & 0x0f
            if first[0] & 0x70 or first[1] & 0x80:
                _fail('exec_frame_invalid')
            size = first[1] & 0x7f
            if size == 126:
                size = int.from_bytes(_read_exact(response, 2), 'big')
            elif size == 127:
                size = int.from_bytes(_read_exact(response, 8), 'big')
            if size > (96 << 10) + 1:
                _fail('exec_frame_too_large')
            payload = _read_exact(response, size)
            if opcode == 8:
                break
            if opcode == 9:
                _send_ws_control(sock, 10, payload)
                continue
            if opcode in (10,):
                continue
            if opcode != 2 or not fin or not payload:
                _fail('exec_frame_invalid')
            channel, data = payload[0], payload[1:]
            if channel == 1:
                total += len(data)
                if total > max_bytes:
                    _fail('archive_stream_too_large')
                if data:
                    yield data
            elif channel == 3:
                try:
                    result = json.loads(data)
                except (ValueError, UnicodeError):
                    _fail('exec_status_invalid')
                if not isinstance(result, dict) or result.get('status') != 'Success':
                    _fail('exec_command_failed')
            elif channel in (0, 2):
                if data:
                    _fail('exec_unexpected_channel_data')
            else:
                _fail('exec_channel_invalid')
    except KubeAPIError:
        raise
    except (OSError, ssl.SSLError, TimeoutError, ValueError):
        _fail('exec_transport_failed')
    finally:
        if sock is not None:
            try: sock.close()
            except OSError: pass


def _open_verified_archive(path, expected_bytes, expected_sha256):
    """Open a private immutable archive artifact and verify exact bytes/hash."""
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
        before = os.fstat(fd)
        if (not stat.S_ISREG(before.st_mode) or before.st_uid != os.geteuid()
            or stat.S_IMODE(before.st_mode) != 0o600 or before.st_nlink != 1
            or before.st_size != expected_bytes):
            _fail('census_archive_custody_invalid')
        digest = hashlib.sha256()
        total = 0
        while True:
            chunk = os.read(fd, MAX_EXEC_STDIN_CHUNK)
            if not chunk:
                break
            total += len(chunk)
            if total > expected_bytes:
                _fail('census_archive_size_changed')
            digest.update(chunk)
        after = os.fstat(fd)
        if ((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns, before.st_ctime_ns)
            != (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_ctime_ns)
            or total != expected_bytes or digest.hexdigest() != expected_sha256):
            _fail('census_archive_digest_mismatch')
        os.lseek(fd, 0, os.SEEK_SET)
        return fd
    except KubeAPIError:
        try: os.close(fd)
        except (OSError, UnboundLocalError): pass
        raise
    except OSError:
        try: os.close(fd)
        except (OSError, UnboundLocalError): pass
        _fail('census_archive_unavailable')


def _validate_canonical_manifest(raw):
    try:
        value = json.loads(raw, object_pairs_hook=_unique_json_pairs,
            parse_constant=lambda _value: (_ for _ in ()).throw(ValueError('non-finite JSON')))
        canonical = json.dumps(value, sort_keys=True, separators=(',', ':'),
            ensure_ascii=False, allow_nan=False).encode('utf-8')
    except (ValueError, TypeError, UnicodeError, RecursionError):
        _fail('census_manifest_invalid')
    if canonical != raw:
        _fail('census_manifest_noncanonical')


def _unique_json_pairs(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            raise ValueError('duplicate JSON field')
        value[key] = item
    return value


def _ws_binary(sock, payload):
    """Send one masked client binary WebSocket frame."""
    mask = os.urandom(4)
    size = len(payload)
    if size > 65535:
        _fail('exec_input_frame_too_large')
    header = bytes((0x82, 0x80 | (size if size < 126 else 126)))
    if size >= 126:
        header += size.to_bytes(2, 'big')
    masked = bytes(value ^ mask[i % 4] for i, value in enumerate(payload))
    sock.sendall(header + mask + masked)


def _websocket_exec_input(api, path, token, archive_fd, manifest_bytes, *, archive_bytes, archive_sha256):
    """Duplex v5 exec for the one fixed census receive action.

    The helper contract emits no stdout while consuming input. We therefore
    stream with TCP backpressure, send the v5 stdin half-close, then require an
    empty stdout and one successful Kubernetes status before returning.
    """
    parsed = urlsplit(api.origin)
    host, port = parsed.hostname, parsed.port or 443
    key = base64.b64encode(os.urandom(16)).decode('ascii')
    sock = None
    response = None
    try:
        raw = socket.create_connection((host, port), timeout=max(api.timeout, 120))
        sock = api.context.wrap_socket(raw, server_hostname=host)
        request = (f'GET {path} HTTP/1.1\r\nHost: {parsed.netloc}\r\n'
            f'Authorization: Bearer {token}\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n'
            f'Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: {key}\r\n'
            'Sec-WebSocket-Protocol: v5.channel.k8s.io\r\n\r\n').encode('ascii')
        sock.sendall(request)
        response = sock.makefile('rb')
        status = response.readline(4096)
        headers = {}
        while True:
            line = response.readline(8192)
            if not line or line in (b'\r\n', b'\n'):
                break
            if b':' not in line:
                _fail('exec_handshake_invalid')
            name, value = line.split(b':', 1)
            headers[name.strip().lower()] = value.strip()
        if (not status.startswith(b'HTTP/1.1 101 ') or headers.get(b'upgrade', b'').lower() != b'websocket'
            or b'upgrade' not in headers.get(b'connection', b'').lower().split(b', ')
            or headers.get(b'sec-websocket-protocol', b'').lower() != b'v5.channel.k8s.io'
            or headers.get(b'sec-websocket-accept') != base64.b64encode(hashlib.sha1(
                (key + '258EAFA5-E914-47DA-95CA-C5AB0DC85B11').encode('ascii')).digest())):
            _fail('exec_handshake_rejected')
        # Prefix carries only the bounded canonical manifest; tar bytes remain
        # read directly from the custody-checked descriptor in fixed chunks.
        for prefix in (CENSUS_INPUT_MAGIC, len(manifest_bytes).to_bytes(4, 'big'),
                       manifest_bytes, archive_bytes.to_bytes(8, 'big')):
            for offset in range(0, len(prefix), MAX_EXEC_STDIN_CHUNK):
                _ws_binary(sock, b'\x00' + prefix[offset:offset + MAX_EXEC_STDIN_CHUNK])
        digest = hashlib.sha256()
        total = 0
        while True:
            chunk = os.read(archive_fd, MAX_EXEC_STDIN_CHUNK)
            if not chunk:
                break
            total += len(chunk)
            if total > archive_bytes:
                _fail('census_archive_size_changed')
            digest.update(chunk)
            _ws_binary(sock, b'\x00' + chunk)
        if total != archive_bytes or digest.hexdigest() != archive_sha256:
            _fail('census_archive_changed_during_exec')
        # Kubernetes client-go v5 StreamClose payload: channel id 0 (stdin).
        _ws_binary(sock, b'\xff\x00')
        status_seen = False
        while True:
            first = response.read(2)
            if not first:
                _fail('exec_stream_truncated')
            if len(first) != 2:
                _fail('exec_frame_truncated')
            fin, opcode = bool(first[0] & 0x80), first[0] & 0x0f
            if first[0] & 0x70 or first[1] & 0x80:
                _fail('exec_frame_invalid')
            size = first[1] & 0x7f
            if size == 126:
                size = int.from_bytes(_read_exact(response, 2), 'big')
            elif size == 127:
                size = int.from_bytes(_read_exact(response, 8), 'big')
            if size > (96 << 10) + 1:
                _fail('exec_frame_too_large')
            payload = _read_exact(response, size)
            if opcode == 8:
                break
            if opcode == 9:
                _send_ws_control(sock, 10, payload)
                continue
            if opcode == 10:
                continue
            if opcode != 2 or not fin or not payload:
                _fail('exec_frame_invalid')
            channel, data = payload[0], payload[1:]
            if channel == 1:
                if data:
                    _fail('census_input_unexpected_stdout')
            elif channel == 3:
                if status_seen:
                    _fail('exec_status_duplicate')
                try:
                    result = json.loads(data)
                except (ValueError, UnicodeError):
                    _fail('exec_status_invalid')
                if not isinstance(result, dict) or result.get('status') != 'Success':
                    _fail('exec_command_failed')
                status_seen = True
            elif channel in (0, 2):
                if data:
                    _fail('exec_unexpected_channel_data')
            else:
                _fail('exec_channel_invalid')
        if not status_seen:
            _fail('exec_status_missing')
        after = os.fstat(archive_fd)
        if not stat.S_ISREG(after.st_mode) or after.st_size != archive_bytes:
            _fail('census_archive_changed_during_exec')
    except KubeAPIError:
        raise
    except (OSError, ssl.SSLError, TimeoutError, ValueError):
        _fail('exec_input_transport_failed')
    finally:
        if response is not None:
            try: response.close()
            except OSError: pass
        if sock is not None:
            try: sock.close()
            except OSError: pass


def _read_exact(stream, size):
    parts = bytearray()
    while len(parts) < size:
        chunk = stream.read(size - len(parts))
        if not chunk:
            _fail('exec_frame_truncated')
        parts.extend(chunk)
    return bytes(parts)


def _send_ws_control(sock, opcode, payload):
    mask = os.urandom(4)
    size = len(payload)
    if size >= 126: _fail('exec_control_too_large')
    masked = bytes(value ^ mask[i % 4] for i, value in enumerate(payload))
    sock.sendall(bytes((0x80 | opcode, 0x80 | size)) + mask + masked)


class KubernetesJSONAPI(_KubernetesJSONAPIBase):
    def delete_job(self, namespace, name, uid):
        if namespace != NAMESPACE: _fail('namespace_invalid')
        if not isinstance(uid, str) or not re.fullmatch('[a-f0-9-]{36}', uid):
            _fail('job_uid_invalid')
        options = {'apiVersion': 'v1', 'kind': 'DeleteOptions', 'preconditions': {'uid': uid}}
        self._request('DELETE', '/apis/batch/v1/namespaces/' + self._segment(namespace) + '/jobs/' + self._segment(name), options)
        deadline = time.monotonic() + 15
        while time.monotonic() < deadline:
            try:
                current = self.get_job(namespace, name)
            except KubeNotFound:
                return True
            if current.get('metadata', {}).get('uid') != uid:
                _fail('job_cleanup_identity_changed')
            time.sleep(0.25)
        _fail('job_cleanup_timeout')

    def request_token_refresh(self, *, service_account=SA_NAME, audience=TOKEN_AUDIENCE, ttl=TOKEN_TTL_SECONDS):
        if service_account != SA_NAME or audience != TOKEN_AUDIENCE or ttl != TOKEN_TTL_SECONDS:
            _fail('token_refresh_scope_invalid')
        body = {'apiVersion': 'authentication.k8s.io/v1', 'kind': 'TokenRequest',
            'spec': {'audiences': [TOKEN_AUDIENCE], 'expirationSeconds': TOKEN_TTL_SECONDS}}
        path = '/api/v1/namespaces/' + self._segment(NAMESPACE) + '/serviceaccounts/' + self._segment(SA_NAME) + '/token'
        result = self._request('POST', path, body, token_refresh=False)
        status = result.get('status', {})
        token, expiry = status.get('token'), status.get('expirationTimestamp')
        if not isinstance(token, str) or not isinstance(expiry, str):
            _fail('token_refresh_response_invalid')
        claims = _jwt_claims(token)
        aud = claims.get('aud'); auds = [aud] if isinstance(aud, str) else aud
        if (not isinstance(auds, list) or TOKEN_AUDIENCE not in auds
            or claims.get('sub') != 'system:serviceaccount:voice-staging:voice-nats-staging-runner'
            or type(claims.get('exp')) is not int
            or claims.get('exp') - int(self.tokens.clock()) < MIN_OPERATION_TOKEN_SECONDS):
            _fail('token_refresh_claims_invalid')
        return token

    def save_refreshed_token(self, token, path):
        # Called under the runner's durable operation lock. The target directory
        # is runner-owned; replacement is atomic and never exposes token text.
        claims = _jwt_claims(token)
        aud = claims.get('aud'); auds = [aud] if isinstance(aud, str) else aud
        if (not isinstance(auds, list) or TOKEN_AUDIENCE not in auds
            or claims.get('sub') != 'system:serviceaccount:voice-staging:voice-nats-staging-runner'
            or type(claims.get('exp')) is not int
            or claims.get('exp') - int(self.tokens.clock()) < MIN_OPERATION_TOKEN_SECONDS):
            _fail('token_refresh_claims_invalid')
        target = Path(path)
        parent_path = target.parent
        parent = parent_path.lstat()
        if (not stat.S_ISDIR(parent.st_mode) or parent.st_uid != os.geteuid()
            or stat.S_IMODE(parent.st_mode) != 0o700):
            _fail('token_directory_custody_invalid')
        scratch = parent_path / ('.' + target.name + '.pending')
        try:
            fd = os.open(scratch, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
            with os.fdopen(fd, 'w', encoding='ascii') as output:
                output.write(token)
                output.flush()
                os.fsync(output.fileno())
            os.replace(scratch, target)
            directory = os.open(parent_path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            try: os.fsync(directory)
            finally: os.close(directory)
        except OSError:
            scratch.unlink(missing_ok=True)
            _fail('token_refresh_persist_failed')


def resolve_source_binding(api, *, operation, expected=None):
    """Resolve live selected source and its scheduler node from fresh API reads.

    The returned binding is suitable for a single archive request. Call again
    before every create, exec, receipt adoption, or recovery and pass the prior
    value as ``expected``; the prior journal is an expectation, never authority.
    """
    if not isinstance(operation, str) or not re.fullmatch('[a-f0-9]{12}', operation):
        _fail('source_operation_invalid')
    try:
        namespace = api.get_namespace(NAMESPACE)
        marker = api.get_configmap(NAMESPACE, 'voice-nats-generation')
        hub = api.get_deployment(NAMESPACE, 'voice-nats-hub')
        ns_meta, marker_meta, hub_meta = (namespace['metadata'], marker['metadata'], hub['metadata'])
        data = marker['data']
        if (ns_meta['name'] != NAMESPACE or not _UID.fullmatch(ns_meta['uid'])
            or marker_meta['name'] != 'voice-nats-generation' or marker_meta['namespace'] != NAMESPACE
            or not _UID.fullmatch(marker_meta['uid']) or hub_meta['name'] != 'voice-nats-hub'
            or hub_meta['namespace'] != NAMESPACE or not _UID.fullmatch(hub_meta['uid'])
            or not isinstance(data.get('generation'), str) or not data['generation']):
            _fail('source_marker_invalid')
        if data.get('phase') == 'active':
            if any(key in data for key in ('knownBaselineOperation', 'knownRolloutOperation')):
                _fail('source_marker_invalid')
            fenced = False
        elif ((data.get('phase') == 'known-baseline-maintenance'
                and data.get('knownBaselineOperation') == operation)
            or (data.get('phase') == 'known-rollout-maintenance'
                and data.get('knownRolloutOperation') == operation)):
            fenced = True
        else:
            _fail('source_marker_invalid')
        template_spec = hub['spec']['template']['spec']
        hub_volumes = template_spec['volumes']
        jsdata = [row for row in hub_volumes if row.get('name') == 'jsdata']
        if (len(jsdata) != 1 or set(jsdata[0]) != {'name', 'persistentVolumeClaim'}
            or not isinstance(jsdata[0]['persistentVolumeClaim'], dict)
            or set(jsdata[0]['persistentVolumeClaim']) != {'claimName'}):
            _fail('source_hub_volume_invalid')
        claim_name = jsdata[0]['persistentVolumeClaim']['claimName']
        if not isinstance(claim_name, str) or not _SOURCE_CLAIM.fullmatch(claim_name) or data.get('dataPVC') != claim_name:
            _fail('source_claim_selection_invalid')
        claim = api.get_pvc(NAMESPACE, claim_name)
        claim_meta, claim_spec = claim['metadata'], claim['spec']
        claim_uid = claim_meta['uid']
        pv_name = claim_spec['volumeName']
        if (claim_meta['name'] != claim_name or claim_meta['namespace'] != NAMESPACE
            or not _UID.fullmatch(claim_uid) or claim.get('status', {}).get('phase') != 'Bound'
            or claim_spec.get('accessModes') != ['ReadWriteOnce'] or claim_spec.get('volumeMode', 'Filesystem') != 'Filesystem'
            or not isinstance(pv_name, str) or not re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,252}', pv_name)):
            _fail('source_claim_identity_invalid')
        pv = api.get_pv(pv_name)
        pv_meta, pv_spec = pv['metadata'], pv['spec']
        pv_uid = pv_meta['uid']
        claim_ref = pv_spec['claimRef']
        if (pv_meta['name'] != pv_name or not _UID.fullmatch(pv_uid)
            or pv.get('status', {}).get('phase') != 'Bound'
            or claim_ref.get('namespace') != NAMESPACE or claim_ref.get('name') != claim_name
            or claim_ref.get('uid') != claim_uid or pv_spec.get('volumeMode', 'Filesystem') != 'Filesystem'):
            _fail('source_pv_identity_invalid')
        affinity = pv_spec['nodeAffinity']['required']['nodeSelectorTerms']
        if (not isinstance(affinity, list) or len(affinity) != 1
            or set(affinity[0]) != {'matchExpressions'} or len(affinity[0]['matchExpressions']) != 1):
            _fail('source_node_affinity_invalid')
        expression = affinity[0]['matchExpressions'][0]
        if (set(expression) != {'key', 'operator', 'values'} or expression.get('key') != 'kubernetes.io/hostname'
            or expression.get('operator') != 'In' or not isinstance(expression.get('values'), list)
            or len(expression['values']) != 1):
            _fail('source_node_affinity_invalid')
        hostname = expression['values'][0]
        nodes = api.list_nodes_by_hostname(hostname)
        matches = [node for node in nodes if node.get('metadata', {}).get('labels', {}).get('kubernetes.io/hostname') == hostname]
        if len(matches) != 1:
            _fail('source_node_ambiguous')
        node = matches[0]
        node_meta = node['metadata']
        node_name = node_meta['name']
        node_uid = node_meta['uid']
        conditions = node.get('status', {}).get('conditions', [])
        if (not isinstance(node_name, str) or not re.fullmatch(r'[a-z0-9](?:[-a-z0-9.]*[a-z0-9])?', node_name)
            or not _UID.fullmatch(node_uid)
            or node.get('spec', {}).get('unschedulable') is True
            or sum(1 for row in conditions if row.get('type') == 'Ready' and row.get('status') == 'True') != 1):
            _fail('source_node_not_ready')
        selector = hub['spec']['selector']['matchLabels']
        pods = api.list_hub_pods(selector)
        active_pods = [pod for pod in pods if not pod.get('metadata', {}).get('deletionTimestamp')]
        if ((fenced and active_pods) or (not fenced and not active_pods)
            or any(pod.get('spec', {}).get('nodeName') not in ('', node_name)
                for pod in active_pods if pod.get('spec', {}).get('nodeName') is not None)):
            _fail('source_hub_pod_node_changed')
        for pod in active_pods:
            if not _UID.fullmatch(pod.get('metadata', {}).get('uid', '')):
                _fail('source_hub_pod_identity_invalid')
            pod_claims = [volume.get('persistentVolumeClaim', {}).get('claimName')
                for volume in pod.get('spec', {}).get('volumes', []) if 'persistentVolumeClaim' in volume]
            if pod_claims != [claim_name]:
                _fail('source_hub_pod_claim_changed')
        binding = {'source_claim': claim_name, 'source_uid': claim_uid,
            'source_pv_name': pv_name, 'source_pv_uid': pv_uid, 'source_node_name': node_name}
        proof = {'namespace_uid': ns_meta['uid'], 'marker_uid': marker_meta['uid'],
            'generation': data['generation'], 'deployment_uid': hub_meta['uid'], 'node_uid': node_uid,
            'node_hostname': hostname}
        if expected is not None and (expected.get('binding') != binding or expected.get('proof') != proof):
            _fail('source_binding_changed')
        return {'binding': binding, 'proof': proof}
    except KubeAPIError:
        raise
    except (KeyError, TypeError, AttributeError, ValueError):
        _fail('source_binding_invalid')
