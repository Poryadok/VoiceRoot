"""Fixed HTTPS Kubernetes JSON API client for the unprivileged rollout runner.

Origin, CA, namespace, token file, service account, audience, and lifetime are
installed trusted configuration. Callers can provide resource identities and
fixed protocol objects only; this module never invokes kubectl or a shell.
"""
from __future__ import annotations

import base64
import json
import os
from pathlib import Path
import re
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


class KubeAPIError(ValueError):
    """Safe operation labels only; API bodies, URLs, and credentials stay private."""


class KubeNotFound(KubeAPIError):
    pass


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


class KubernetesJSONAPI:
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

    def get_node(self, name):
        return self._request('GET', '/api/v1/nodes/' + self._segment(name))

    def create_job(self, namespace, document):
        if namespace != NAMESPACE: _fail('namespace_invalid')
        return self._request('POST', '/apis/batch/v1/namespaces/' + self._segment(namespace) + '/jobs', document)

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

    def pod_logs(self, namespace, pod_name):
        if namespace != NAMESPACE: _fail('namespace_invalid')
        query = urlencode({'limitBytes': str(MAX_LOG_RESPONSE), 'timestamps': 'false'})
        return self._request('GET', '/api/v1/namespaces/' + self._segment(namespace) + '/pods/' + self._segment(pod_name) + '/log?' + query, log=True)

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
