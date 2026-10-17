"""Fixed HTTPS Kubernetes JSON API client for the unprivileged rollout runner.

Origin, CA, namespace, token file, service account, audience, and lifetime are
installed trusted configuration. Callers can provide resource identities and
fixed protocol objects only; this module never invokes kubectl or a shell.
"""
from __future__ import annotations

import base64
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import socket
import ssl
import stat
import tempfile
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
MAX_VERIFICATION_INPUT_BYTES = 4 << 20
VERIFICATION_INPUT_FILES = frozenset(('bootstrap.creds','social.creds','realtime.creds',
    'fixture.json','server.conf'))
MAX_EXEC_STDIN_CHUNK = 48 << 10
CENSUS_INPUT_MAGIC = b'VOICE-NATS-CENSUS-INPUT-v1\0'


class KubeAPIError(ValueError):
    """Safe operation labels only; API bodies, URLs, and credentials stay private."""


class KubeNotFound(KubeAPIError):
    pass


_UID = re.compile(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}')
_SOURCE_CLAIM = re.compile(r'voice-nats-jsdata-d[0-9]{8}[a-z0-9]{1,8}')
_CAPTURE_SECRET = re.compile(r'voice-nats-(?:operator|bootstrap-credentials|service-credentials)-r[0-9]{8}[a-z0-9]{0,8}|voice-nats-bootstrap-contract-[a-f0-9]{24}|voice-nats-hub-tls-r[0-9]{8}[a-z0-9]{0,8}')
_CAPTURE_CONFIGMAP = re.compile(r'voice-nats-(?:notification|analytics-chat|search|realtime)-bootstrap')


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

    def _request(self, method, path, body=None, *, log=False, token_refresh=True,
                 content_type='application/json'):
        if not isinstance(path, str) or not path.startswith('/') or path.startswith('//'):
            _fail('path_invalid')
        data = None if body is None else json.dumps(body, sort_keys=True, separators=(',', ':')).encode()
        token = self._operation_token() if token_refresh else self._refresh_token_credential()
        headers = {'Accept': 'application/json', 'Authorization': 'Bearer ' + token}
        if data is not None:
            headers['Content-Type'] = content_type
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
        if (namespace != NAMESPACE or not isinstance(name, str)
            or (name not in ('voice-nats-generation', 'voice-app-config')
                and not _CAPTURE_CONFIGMAP.fullmatch(name))):
            _fail('configmap_invalid')
        return self._request('GET', '/api/v1/namespaces/' + self._segment(namespace)
            + '/configmaps/' + self._segment(name))

    def get_secret(self, namespace, name):
        if namespace != NAMESPACE or not isinstance(name, str) or not _CAPTURE_SECRET.fullmatch(name):
            _fail('secret_invalid')
        row = self._request('GET', '/api/v1/namespaces/' + self._segment(namespace)
            + '/secrets/' + self._segment(name))
        metadata = row.get('metadata')
        if (not isinstance(metadata, dict) or metadata.get('name') != name
            or metadata.get('namespace') != namespace
            or not isinstance(metadata.get('uid'), str) or not _UID.fullmatch(metadata['uid'])
            or not isinstance(metadata.get('resourceVersion'), str) or not metadata['resourceVersion']):
            _fail('secret_identity_invalid')
        return row

    def get_secret_metadata(self, namespace, name):
        """Read only the fixed TLS Secret identity needed for capture provenance."""
        if (namespace != NAMESPACE or not isinstance(name, str)
            or not re.fullmatch(r'voice-nats-hub-tls-r[0-9]{8}[a-z0-9]{0,8}', name)):
            _fail('secret_metadata_invalid')
        row = self.get_secret(namespace, name)
        metadata = row['metadata']
        return {key: metadata[key] for key in ('name', 'namespace', 'uid', 'resourceVersion')}

    def get_deployment(self, namespace, name):
        if (namespace != NAMESPACE or not isinstance(name, str)
            or not re.fullmatch(r'[a-z0-9](?:[-a-z0-9.]*[a-z0-9])?', name)):
            _fail('deployment_invalid')
        return self._request('GET', '/apis/apps/v1/namespaces/' + self._segment(namespace)
            + '/deployments/' + self._segment(name))

    def get_service(self, namespace, name):
        if namespace != NAMESPACE or name != 'voice-nats':
            _fail('service_invalid')
        return self._request('GET', '/api/v1/namespaces/' + self._segment(namespace)
            + '/services/' + self._segment(name))

    def get_pod(self, namespace, name):
        if (namespace != NAMESPACE or not isinstance(name, str)
            or not re.fullmatch(r'[a-z0-9](?:[-a-z0-9.]*[a-z0-9])?', name)):
            _fail('pod_invalid')
        return self._request('GET', '/api/v1/namespaces/' + self._segment(namespace)
            + '/pods/' + self._segment(name))

    def get_replicaset(self, namespace, name):
        if (namespace != NAMESPACE or not isinstance(name, str)
            or not re.fullmatch(r'[a-z0-9](?:[-a-z0-9.]*[a-z0-9])?', name)):
            _fail('replicaset_invalid')
        return self._request('GET', '/apis/apps/v1/namespaces/' + self._segment(namespace)
            + '/replicasets/' + self._segment(name))

    def _list_namespaced(self, group, version, resource, *, limit=2048):
        if resource not in ('pods', 'replicasets', 'deployments', 'horizontalpodautoscalers', 'jobs'):
            _fail('resource_invalid')
        if type(limit) is not int or not 1 <= limit <= 2048:
            _fail('list_limit_invalid')
        prefix = '/api/' if group is None else '/apis/' + group + '/'
        version_path = version if group is None else version
        result = self._request('GET', prefix + version_path + '/namespaces/'
            + self._segment(NAMESPACE) + '/' + resource + '?' + urlencode({'limit': str(limit)}))
        items = result.get('items')
        if not isinstance(items, list) or len(items) > limit or not all(isinstance(item, dict) for item in items):
            _fail('resource_list_invalid')
        if result.get('metadata', {}).get('continue'):
            _fail('resource_list_truncated')
        return items

    def list_fence_objects(self, namespace):
        if namespace != NAMESPACE:
            _fail('namespace_invalid')
        return {'pods': self._list_namespaced(None, 'v1', 'pods'),
            'replicasets': self._list_namespaced('apps', 'v1', 'replicasets')}

    def list_control_objects(self, namespace):
        if namespace != NAMESPACE:
            _fail('namespace_invalid')
        return {'deployments': self._list_namespaced('apps', 'v1', 'deployments'),
            'horizontalpodautoscalers': self._list_namespaced('autoscaling', 'v2', 'horizontalpodautoscalers'),
            'jobs': self._list_namespaced('batch', 'v1', 'jobs')}

    def add_owned_pod_finalizer(self, namespace, name, pod_uid, expected_rv,
                                expected_finalizers, finalizer):
        return self._patch_pod_finalizers(namespace, name, pod_uid, expected_rv,
            expected_finalizers, finalizer, add=True)

    def remove_owned_pod_finalizer(self, namespace, name, pod_uid, expected_rv,
                                   expected_finalizers, finalizer):
        return self._patch_pod_finalizers(namespace, name, pod_uid, expected_rv,
            expected_finalizers, finalizer, add=False)

    def _patch_pod_finalizers(self, namespace, name, pod_uid, expected_rv,
                              expected_finalizers, finalizer, *, add):
        if (namespace != NAMESPACE or not isinstance(name, str)
            or not re.fullmatch(r'[a-z0-9](?:[-a-z0-9.]*[a-z0-9])?', name)
            or not isinstance(pod_uid, str) or not _UID.fullmatch(pod_uid)
            or not isinstance(expected_rv, str) or not expected_rv.isdigit()
            or not isinstance(expected_finalizers, list) or len(expected_finalizers) > 32
            or any(not isinstance(item, str) or not re.fullmatch(r'[a-z0-9][a-z0-9./-]{0,252}', item)
                for item in expected_finalizers)
            or len(set(expected_finalizers)) != len(expected_finalizers)
            or not isinstance(finalizer, str) or not re.fullmatch(r'[a-z0-9][a-z0-9./-]{0,252}', finalizer)
            or type(add) is not bool):
            _fail('pod_finalizer_request_invalid')
        current = self.get_pod(namespace, name)
        metadata = current.get('metadata', {})
        actual_finalizers = metadata.get('finalizers', [])
        if (metadata.get('uid') != pod_uid or metadata.get('resourceVersion') != expected_rv
            or actual_finalizers != expected_finalizers):
            _fail('pod_finalizer_precondition_changed')
        if add:
            if finalizer in expected_finalizers:
                _fail('pod_finalizer_already_present')
            next_finalizers = expected_finalizers + [finalizer]
        else:
            if finalizer not in expected_finalizers:
                _fail('pod_finalizer_missing')
            next_finalizers = [item for item in expected_finalizers if item != finalizer]
        patch = [
            {'op': 'test', 'path': '/metadata/uid', 'value': pod_uid},
            {'op': 'test', 'path': '/metadata/resourceVersion', 'value': expected_rv},
        ]
        if 'finalizers' in metadata:
            patch.append({'op': 'test', 'path': '/metadata/finalizers', 'value': expected_finalizers})
            patch.append({'op': 'replace', 'path': '/metadata/finalizers', 'value': next_finalizers})
        else:
            patch.append({'op': 'add', 'path': '/metadata/finalizers', 'value': next_finalizers})
        return self._request('PATCH', '/api/v1/namespaces/' + self._segment(namespace)
            + '/pods/' + self._segment(name), patch, content_type='application/json-patch+json')

    def delete_owned_pod(self, namespace, name, pod_uid, expected_rv):
        if (namespace != NAMESPACE or not isinstance(name, str)
            or not re.fullmatch(r'[a-z0-9](?:[-a-z0-9.]*[a-z0-9])?', name)
            or not isinstance(pod_uid, str) or not _UID.fullmatch(pod_uid)
            or not isinstance(expected_rv, str) or not expected_rv.isdigit()):
            _fail('pod_delete_request_invalid')
        current = self.get_pod(namespace, name)
        metadata = current.get('metadata', {})
        if metadata.get('uid') != pod_uid or metadata.get('resourceVersion') != expected_rv:
            _fail('pod_delete_precondition_changed')
        body = {'apiVersion': 'v1', 'kind': 'DeleteOptions',
            'preconditions': {'uid': pod_uid, 'resourceVersion': expected_rv}}
        self._request('DELETE', '/api/v1/namespaces/' + self._segment(namespace)
            + '/pods/' + self._segment(name), body)
        try:
            observed = self.get_pod(namespace, name)
        except KubeNotFound:
            return True
        if observed.get('metadata', {}).get('uid') != pod_uid:
            return True
        _fail('pod_delete_unconfirmed')

    def cas_rollout_deployment(self, namespace, name, expected_uid, expected_rv,
                               expected_replicas, expected_template_sha256, *,
                               new_replicas=None, new_template=None,
                               expected_jsdata_claim=None, new_jsdata_claim=None):
        if (namespace != NAMESPACE or not isinstance(name, str)
            or not re.fullmatch(r'[a-z0-9](?:[-a-z0-9.]*[a-z0-9])?', name)
            or not isinstance(expected_uid, str) or not _UID.fullmatch(expected_uid)
            or not isinstance(expected_rv, str) or not expected_rv.isdigit()
            or type(expected_replicas) is not int or expected_replicas < 0
            or not isinstance(expected_template_sha256, str)
            or not re.fullmatch(r'[a-f0-9]{64}', expected_template_sha256)):
            _fail('deployment_cas_request_invalid')
        if (new_replicas is None and new_template is None
            or new_replicas is not None and (type(new_replicas) is not int or new_replicas < 0)
            or new_template is not None and not isinstance(new_template, dict)
            or (new_jsdata_claim is not None and new_template is None)):
            _fail('deployment_cas_mutation_invalid')
        current = self.get_deployment(namespace, name)
        metadata, spec = current.get('metadata', {}), current.get('spec', {})
        template = spec.get('template')
        if (metadata.get('uid') != expected_uid or metadata.get('resourceVersion') != expected_rv
            or spec.get('replicas') != expected_replicas or not isinstance(template, dict)
            or hashlib.sha256(json.dumps(template, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
                != expected_template_sha256):
            _fail('deployment_cas_precondition_changed')
        if expected_jsdata_claim is not None:
            volumes = template.get('spec', {}).get('volumes', [])
            jsdata = [row for row in volumes if isinstance(row, dict) and row.get('name') == 'jsdata']
            if (len(jsdata) != 1 or jsdata[0].get('persistentVolumeClaim', {}).get('claimName')
                != expected_jsdata_claim):
                _fail('deployment_jsdata_precondition_changed')
        patch = [
            {'op': 'test', 'path': '/metadata/uid', 'value': expected_uid},
            {'op': 'test', 'path': '/metadata/resourceVersion', 'value': expected_rv},
            {'op': 'test', 'path': '/spec/replicas', 'value': expected_replicas},
        ]
        if new_replicas is not None:
            patch.append({'op': 'replace', 'path': '/spec/replicas', 'value': new_replicas})
        if new_template is not None:
            if expected_jsdata_claim is not None and new_jsdata_claim is not None:
                volumes = new_template.get('spec', {}).get('volumes', [])
                jsdata = [row for row in volumes if isinstance(row, dict) and row.get('name') == 'jsdata']
                if len(jsdata) != 1 or jsdata[0].get('persistentVolumeClaim', {}).get('claimName') != new_jsdata_claim:
                    _fail('deployment_jsdata_target_invalid')
            patch.append({'op': 'replace', 'path': '/spec/template', 'value': copy.deepcopy(new_template)})
        return self._request('PATCH', '/apis/apps/v1/namespaces/' + self._segment(namespace)
            + '/deployments/' + self._segment(name), patch, content_type='application/json-patch+json')

    def cas_rollout_marker(self, name, operation, expected_uid, expected_rv,
                           expected_phase, new_phase, *, expected_claim,
                           release=False):
        phases = ('active', 'rollout-capturing', 'rollout-prepared',
            'rollout-applying', 'rollout-verified')
        transitions = {
            'active': 'rollout-capturing',
            'rollout-capturing': 'rollout-prepared',
            'rollout-prepared': 'rollout-applying',
            'rollout-applying': 'rollout-verified',
            'rollout-verified': 'active',
        }
        if (name != 'voice-nats-generation' or not isinstance(operation, str)
            or not re.fullmatch(r'[a-f0-9]{12}', operation)
            or not isinstance(expected_uid, str) or not _UID.fullmatch(expected_uid)
            or not isinstance(expected_rv, str) or not expected_rv.isdigit()
            or expected_phase not in phases or transitions.get(expected_phase) != new_phase
            or not isinstance(expected_claim, str) or not _SOURCE_CLAIM.fullmatch(expected_claim)
            or type(release) is not bool
            or release != (expected_phase == 'rollout-verified' and new_phase == 'active')):
            _fail('marker_cas_request_invalid')
        current = self.get_configmap(NAMESPACE, name)
        metadata, data = current.get('metadata', {}), current.get('data', {})
        if (metadata.get('uid') != expected_uid or metadata.get('resourceVersion') != expected_rv
            or data.get('phase') != expected_phase or data.get('dataPVC') != expected_claim):
            _fail('marker_cas_precondition_changed')
        if expected_phase == 'active':
            if 'knownRolloutOperation' in data:
                _fail('marker_rollout_already_owned')
        elif data.get('knownRolloutOperation') != operation:
            _fail('marker_rollout_ownership_changed')
        patch = [
            {'op': 'test', 'path': '/metadata/uid', 'value': expected_uid},
            {'op': 'test', 'path': '/metadata/resourceVersion', 'value': expected_rv},
            {'op': 'test', 'path': '/data/phase', 'value': expected_phase},
            {'op': 'test', 'path': '/data/dataPVC', 'value': expected_claim},
        ]
        if expected_phase == 'active':
            patch.extend((
                {'op': 'replace', 'path': '/data/phase', 'value': new_phase},
                {'op': 'add', 'path': '/data/knownRolloutOperation', 'value': operation},
            ))
        else:
            patch.extend((
                {'op': 'test', 'path': '/data/knownRolloutOperation', 'value': operation},
                {'op': 'replace', 'path': '/data/phase', 'value': new_phase},
            ))
        if release:
            patch.append({'op': 'remove', 'path': '/data/knownRolloutOperation'})
        return self._request('PATCH', '/api/v1/namespaces/' + self._segment(NAMESPACE)
            + '/configmaps/' + self._segment(name), patch, content_type='application/json-patch+json')

    def cas_user_bootstrap_template(self, namespace, name, expected_uid, expected_rv,
                                    expected_replicas, expected_template_sha256, *,
                                    operation, original_template=None, action):
        if (namespace != NAMESPACE or name != 'voice-user'
            or not isinstance(expected_uid, str) or not _UID.fullmatch(expected_uid)
            or not isinstance(expected_rv, str) or not expected_rv.isdigit()
            or type(expected_replicas) is not int or expected_replicas < 0
            or not isinstance(expected_template_sha256, str)
                or not re.fullmatch(r'[a-f0-9]{64}', expected_template_sha256)
            or not isinstance(operation, str) or not re.fullmatch(r'[a-f0-9]{12}', operation)
            or action not in ('install', 'restore')
            or action == 'install' and original_template is not None
            or action == 'restore' and not isinstance(original_template, dict)):
            _fail('user_bootstrap_cas_request_invalid')
        current = self.get_deployment(namespace, name)
        metadata, spec = current.get('metadata', {}), current.get('spec', {})
        template = spec.get('template')
        if (metadata.get('uid') != expected_uid or metadata.get('resourceVersion') != expected_rv
            or spec.get('replicas') != expected_replicas or not isinstance(template, dict)
            or hashlib.sha256(json.dumps(template, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
                != expected_template_sha256):
            _fail('user_bootstrap_cas_precondition_changed')
        next_template = copy.deepcopy(template)
        if action == 'install':
            template_metadata = next_template.get('metadata', {})
            annotations = template_metadata.get('annotations', {})
            containers = next_template.get('spec', {}).get('containers', [])
            users = [row for row in containers if isinstance(row, dict) and row.get('name') == 'user']
            if (len(users) != 1 or not isinstance(annotations, dict)
                or 'voice.io/nats-user-space-bootstrap' in annotations):
                _fail('user_bootstrap_shape_invalid')
            env = users[0].get('env', [])
            if not isinstance(env, list) or any(not isinstance(row, dict) or row.get('name') == 'SPACE_GRPC_ADDR'
                for row in env):
                _fail('user_bootstrap_env_invalid')
            users[0]['env'] = env + [{'name': 'SPACE_GRPC_ADDR', 'value': ''}]
            template_metadata.setdefault('annotations', {})['voice.io/nats-user-space-bootstrap'] = operation
            next_template['metadata'] = template_metadata
        else:
            override = copy.deepcopy(original_template)
            template_metadata = override.get('metadata', {})
            annotations = template_metadata.get('annotations', {})
            containers = override.get('spec', {}).get('containers', [])
            users = [row for row in containers if isinstance(row, dict) and row.get('name') == 'user']
            if (len(users) != 1 or not isinstance(annotations, dict)
                or 'voice.io/nats-user-space-bootstrap' in annotations):
                _fail('user_bootstrap_original_invalid')
            env = users[0].get('env', [])
            if not isinstance(env, list) or any(not isinstance(row, dict) or row.get('name') == 'SPACE_GRPC_ADDR'
                for row in env):
                _fail('user_bootstrap_original_invalid')
            users[0]['env'] = env + [{'name': 'SPACE_GRPC_ADDR', 'value': ''}]
            template_metadata.setdefault('annotations', {})['voice.io/nats-user-space-bootstrap'] = operation
            override['metadata'] = template_metadata
            if template != override:
                _fail('user_bootstrap_override_changed')
            next_template = copy.deepcopy(original_template)
        patch = [
            {'op': 'test', 'path': '/metadata/uid', 'value': expected_uid},
            {'op': 'test', 'path': '/metadata/resourceVersion', 'value': expected_rv},
            {'op': 'test', 'path': '/spec/replicas', 'value': expected_replicas},
            {'op': 'test', 'path': '/spec/template', 'value': template},
            {'op': 'replace', 'path': '/spec/template', 'value': next_template},
        ]
        return self._request('PATCH', '/apis/apps/v1/namespaces/' + self._segment(namespace)
            + '/deployments/' + self._segment(name), patch, content_type='application/json-patch+json')

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

    def list_hub_pods(self, namespace, labels):
        if namespace != NAMESPACE or labels != {'app': 'voice-nats-pvc-candidate'}:
            _fail('pod_selector_invalid')
        result = self._request('GET', '/api/v1/namespaces/' + self._segment(namespace)
            + '/pods?' + urlencode({'labelSelector': 'app=voice-nats-pvc-candidate', 'limit': '2048'}))
        items = result.get('items')
        if not isinstance(items, list) or len(items) > 2048 or not all(isinstance(item, dict) for item in items):
            _fail('pod_list_invalid')
        if result.get('metadata', {}).get('continue'):
            _fail('pod_list_truncated')
        return items

    def get_node(self, namespace, name=None):
        if name is None:
            name, namespace = namespace, NAMESPACE
        if namespace != NAMESPACE:
            _fail('namespace_invalid')
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

    def exec_pod_verification(self, namespace, pod_name, pod_uid, request, binding, action, *, archive=None,
                              session_inputs=None):
        """Run only a producer-declared readback vector in the bound Pod."""
        if namespace != NAMESPACE:
            _fail('namespace_invalid')
        try:
            import verification_job
            vectors = verification_job.exec_vectors(request, pod_uid, binding, archive=archive,
                session_inputs=session_inputs)
        except (ImportError, TypeError, ValueError):
            _fail('verification_exec_request_invalid')
        if action not in vectors or action in ('census_load_inputs', 'census_receive_archive', 'restore_archive_once'):
            # Input loading needs a separate bounded stdin protocol; it is not
            # silently routed through the output-only WebSocket operation.
            _fail('verification_exec_action_invalid')
        argv = vectors[action]
        archive_actions = ('archive_create_and_stream', 'archive_same_pod_readback')
        if action in archive_actions:
            max_bytes = MAX_ARCHIVE_STREAM_BYTES
        elif action == 'census_same_pod_readback':
            import census_stream
            max_bytes = census_stream.MAX_TRANSPORT_BYTES
        elif action in ('census_read_input_receipt', 'restore_same_pod_readback'):
            max_bytes = MAX_LOG_RESPONSE
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

    def exec_hub_jsz(self, namespace, pod_name, pod_uid, container, *, timeout_seconds=30):
        """Read the captured Hub's fixed loopback JSZ endpoint over pods/exec."""
        if (namespace != NAMESPACE or container != 'nats' or type(timeout_seconds) is not int
            or not 1 <= timeout_seconds <= 60 or not isinstance(pod_uid, str)
            or not _UID.fullmatch(pod_uid)):
            _fail('hub_jsz_identity_invalid')
        pod = self.get_pod(namespace, pod_name)
        metadata = pod.get('metadata', {})
        if metadata.get('uid') != pod_uid or metadata.get('namespace') != namespace:
            _fail('hub_jsz_pod_changed')
        command = ['/bin/busybox', 'wget', '-q', '-O', '-',
            'http://127.0.0.1:8222/jsz?accounts=true&streams=true&consumers=true&config=true&limit=2048']
        query = [('container', container), ('stdin', 'false'), ('stdout', 'true'),
            ('stderr', 'false'), ('tty', 'false')]
        query.extend(('command', item) for item in command)
        path = ('/api/v1/namespaces/' + self._segment(namespace) + '/pods/'
            + self._segment(pod_name) + '/exec?' + urlencode(query))
        token = self._operation_token()
        deadline = time.monotonic() + timeout_seconds
        chunks = []
        size = 0
        for chunk in _websocket_exec(self, path, token, max_bytes=64 << 20):
            if time.monotonic() > deadline:
                _fail('hub_jsz_timeout')
            size += len(chunk)
            if size > 64 << 20:
                _fail('hub_jsz_too_large')
            chunks.append(chunk)
        return b''.join(chunks)

    def exec_pod_verification_input(self, namespace, pod_name, pod_uid, request, binding,
                              archive, session_inputs, bundle_bytes):
        """Send one bounded canonical input bundle to the fixed census loader.

        The only accepted action is the producer's first, create-once loader
        vector. The payload is raw canonical JSON on stdin; no shell, path, or
        candidate-selected command is accepted. A dropped connection is an
        unknown outcome and callers may only use same-Pod receipt readback.
        """
        if namespace != NAMESPACE:
            _fail('namespace_invalid')
        if (not isinstance(bundle_bytes, bytes) or not 0 < len(bundle_bytes) <= MAX_VERIFICATION_INPUT_BYTES):
            _fail('verification_input_bundle_invalid')
        try:
            row=json.loads(bundle_bytes.decode('utf-8'),object_pairs_hook=_unique_json_pairs,
                parse_constant=lambda value: (_ for _ in ()).throw(ValueError(value)))
            canonical=json.dumps(row,sort_keys=True,separators=(',',':'),ensure_ascii=False,
                allow_nan=False).encode('utf-8')
        except (UnicodeDecodeError,ValueError,TypeError,RecursionError):
            _fail('verification_input_bundle_invalid')
        if (bundle_bytes!=canonical or not isinstance(request,dict) or not isinstance(row,dict)
            or set(row)!={'schema','operation','nonce','pod_uid','files'}
            or row.get('schema')!='voice-nats-verification-inputs-v1'
            or row.get('operation')!=request.get('operation') or row.get('nonce')!=request.get('nonce')
            or row.get('pod_uid')!=pod_uid or not isinstance(row.get('files'),dict)
            or set(row['files'])!=VERIFICATION_INPUT_FILES):
            _fail('verification_input_bundle_identity_invalid')
        for value in row['files'].values():
            if not isinstance(value,str):_fail('verification_input_bundle_file_invalid')
            try:decoded=base64.b64decode(value,validate=True)
            except (ValueError,base64.binascii.Error):_fail('verification_input_bundle_file_invalid')
            if not 0<len(decoded)<=256<<10:_fail('verification_input_bundle_file_invalid')
        try:
            import verification_job
            expected = hashlib.sha256(bundle_bytes).hexdigest()
            if (not isinstance(session_inputs, dict) or session_inputs.get('sha256') != expected
                or session_inputs.get('bytes') != len(bundle_bytes)):
                _fail('verification_input_bundle_binding_invalid')
            vectors = verification_job.exec_vectors(request, pod_uid, binding,
                archive=archive, session_inputs=session_inputs)
        except (ImportError, TypeError, ValueError):
            _fail('verification_exec_request_invalid')
        argv = vectors.get('census_load_inputs')
        if not isinstance(argv, list) or not argv:
            _fail('verification_exec_action_invalid')
        pod = self._request('GET', '/api/v1/namespaces/' + self._segment(namespace)
            + '/pods/' + self._segment(pod_name))
        metadata = pod.get('metadata', {})
        if metadata.get('uid') != pod_uid or metadata.get('namespace') != namespace:
            _fail('verification_exec_pod_changed')
        # tempfile.TemporaryFile is anonymous on the supported Linux runner.
        # The descriptor remains private to this process and never gains a
        # pathname that could enter a journal or candidate-controlled tree.
        with tempfile.TemporaryFile() as source:
            os.fchmod(source.fileno(), 0o600)
            source.write(bundle_bytes)
            source.flush(); os.fsync(source.fileno()); source.seek(0)
            info = os.fstat(source.fileno())
            if (not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid()
                or info.st_gid != os.getegid() or stat.S_IMODE(info.st_mode) != 0o600
                or info.st_nlink != 0 or info.st_size != len(bundle_bytes)):
                _fail('verification_input_descriptor_custody_invalid')
            query = [('container', 'nats-phase'), ('stdin', 'true'), ('stdout', 'true'),
                ('stderr', 'false'), ('tty', 'false')]
            query.extend(('command', item) for item in argv)
            path = ('/api/v1/namespaces/' + self._segment(namespace) + '/pods/'
                + self._segment(pod_name) + '/exec?' + urlencode(query))
            token = self._operation_token()
            _websocket_exec_input(self, path, token, source.fileno(), b'',
                archive_bytes=len(bundle_bytes), archive_sha256=expected, raw_stdin=True)

    def exec_pod_census_input(self, namespace, pod_name, pod_uid, request, binding,
                              archive_path, manifest_bytes, archive_sha256, archive_bytes, session_inputs):
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
                archive={'sha256': archive_sha256, 'bytes': archive_bytes},
                session_inputs=session_inputs)
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

    def read_verification_census(self, namespace, pod_name, pod_uid, request, binding, archive, session_inputs):
        """Return a validated, identity-bound census from its durable same-Pod receipt."""
        try:
            import census_stream
            import verification_job
            row = verification_job.validate_request(request)
            storage = verification_job.validate_binding(binding)
            if row['phase'] != 'census':
                _fail('verification_census_phase_invalid')
            if (not isinstance(archive, dict) or set(archive) != {'sha256', 'bytes'}
                or not isinstance(archive['sha256'], str) or not re.fullmatch('[a-f0-9]{64}', archive['sha256'])
                or type(archive['bytes']) is not int or not 0 < archive['bytes'] <= MAX_ARCHIVE_STREAM_BYTES):
                _fail('verification_census_archive_invalid')
            chunks = self.exec_pod_verification(namespace, pod_name, pod_uid,
                row, storage, 'census_same_pod_readback', archive=archive, session_inputs=session_inputs)
            return census_stream.consume(chunks, operation=row['operation'], nonce=row['nonce'],
                pod_uid=pod_uid, source_pvc_uid=storage['source_uid'],
                source_pv_uid=storage['source_pv_uid'],
                verification_pvc_uid=storage['verification_uid'],
                verification_pv_uid=storage['verification_pv_uid'],
                archive_sha256=archive['sha256'], archive_bytes=archive['bytes'])
        except (ImportError, TypeError, ValueError):
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


def _websocket_exec_input(api, path, token, archive_fd, manifest_bytes, *, archive_bytes, archive_sha256,
                          raw_stdin=False):
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
        # Census archive transfer has a fixed binary envelope; bounded JSON
        # input upload is raw stdin using this same v5 transport and closure.
        if raw_stdin:
            if manifest_bytes != b'':
                _fail('exec_raw_input_envelope_invalid')
        else:
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
        hub = api.get_deployment(NAMESPACE, 'voice-nats-pvc-candidate')
        ns_meta, marker_meta, hub_meta = (namespace['metadata'], marker['metadata'], hub['metadata'])
        data = marker['data']
        if (ns_meta['name'] != NAMESPACE or not _UID.fullmatch(ns_meta['uid'])
            or marker_meta['name'] != 'voice-nats-generation' or marker_meta['namespace'] != NAMESPACE
            or not _UID.fullmatch(marker_meta['uid']) or hub_meta['name'] != 'voice-nats-pvc-candidate'
            or hub_meta['namespace'] != NAMESPACE or not _UID.fullmatch(hub_meta['uid'])
            or not isinstance(data.get('generation'), str) or not data['generation']):
            _fail('source_marker_invalid')
        if data.get('phase') == 'active':
            if any(key in data for key in ('knownBaselineOperation', 'knownRolloutOperation')):
                _fail('source_marker_invalid')
            fenced = False
        elif ((data.get('phase') == 'known-baseline-maintenance'
                and data.get('knownBaselineOperation') == operation)
            or (data.get('phase') in ('known-rollout-maintenance', 'rollout-capturing',
                    'rollout-prepared', 'rollout-applying', 'rollout-verified')
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
        if selector != {'app': 'voice-nats-pvc-candidate'}:
            _fail('source_hub_selector_invalid')
        pods = api.list_hub_pods(NAMESPACE, {'app': 'voice-nats-pvc-candidate'})
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
