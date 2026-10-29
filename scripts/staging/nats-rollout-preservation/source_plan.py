"""Compile canonical passive source; never execute a source script or mutate Kubernetes.

The decoder consumes one YAML document as bytes. kube.run returns parsed JSON
and must keep all private inputs/results out of logs. Server dry-run is the only
apply command here. Root repeats verify_db_init immediately before its fence.
"""
import base64
import copy
import hashlib
from pathlib import Path
import re
from urllib.parse import quote
import nonnats_plan as guard
import release_policy_wiring

PlanError = guard.PlanError
NS = 'voice-staging'
HOSTS = {'gateway_host': 'VOICE_GATEWAY_INGRESS_HOST', 'livekit_host': 'VOICE_LIVEKIT_INGRESS_HOST',
         'web_host': 'VOICE_WEB_INGRESS_HOST', 'admin_host': 'VOICE_ADMIN_INGRESS_HOST',
         'developer_portal_host': 'VOICE_DEVELOPER_PORTAL_INGRESS_HOST'}
FRONTENDS = ('developer-portal', 'web', 'admin')
PRINCIPAL_PUBLISHER = 'voice-p3-principal-jwks'
PRINCIPAL_PUBLIC = 'voice-p3-principal-public'
PRINCIPAL_CONFIG = 'voice-p3-principal-publisher-config'
# Exact reviewed HTTPS GET-only /realtime.json configuration, not a general
# nginx workload that could expose its mounted TLS private key.
PRINCIPAL_CONFIG_SHA256 = '6afc7058fb4fbef8ca8458bd642db1f83c4e79ef9bd9de63ff0d57408468fd08'


def principal_publisher_shape(row):
    """One existing public-document publisher, never an arbitrary workload."""
    try:
        spec = row['spec']['template']['spec']
        if (row['kind'] != 'Deployment' or row['metadata']['name'] != PRINCIPAL_PUBLISHER
                or row['metadata']['namespace'] != NS or spec.get('initContainers')
                or len(spec['containers']) != 1):
            raise ValueError()
        container = spec['containers'][0]
        if (container['name'] != 'jwks' or container.get('env') or container.get('envFrom')
                or container.get('command') or container.get('args')
                or container['volumeMounts'] != [
                    {'name': 'public', 'mountPath': '/srv/jwks', 'readOnly': True},
                    {'name': 'tls', 'mountPath': '/etc/voice/tls', 'readOnly': True},
                    {'name': 'config', 'mountPath': '/etc/nginx/conf.d', 'readOnly': True}]
                or row['spec']['selector']['matchLabels'] != {'app': PRINCIPAL_PUBLISHER}
                or row['spec']['template']['metadata']['labels'] != {'app': PRINCIPAL_PUBLISHER}):
            raise ValueError()
        volumes = spec['volumes']
        if (len(volumes) != 3 or volumes[0] != {'name': 'public', 'configMap': {
                'name': PRINCIPAL_PUBLIC, 'items': [{'key': 'realtime.json', 'path': 'realtime.json'}]}}
                or volumes[1] != {'name': 'tls', 'secret': {'secretName': 'voice-p3-principal-jwks-tls',
                    'defaultMode': 288, 'items': [{'key': 'tls.crt', 'path': 'tls.crt'}, {'key': 'tls.key', 'path': 'tls.key'}]}}
                or volumes[2] != {'name': 'config', 'configMap': {'name': PRINCIPAL_CONFIG}}):
            raise ValueError()
    except (KeyError, TypeError, ValueError):
        _fail('canonical_principal_publisher_shape')


def principal_publisher_source(source, decoder):
    """Read only the exact source helper triple; historical sources may omit it."""
    path = Path(source) / 'deploy/staging/services.yaml'
    if not path.exists():
        return False
    if path.is_symlink() or not path.is_file() or path.stat().st_size > 2 << 20:
        _fail('canonical_principal_publisher_source')
    selected = []
    for piece in re.split(r'^---\s*$', path.read_text(encoding='utf-8'), flags=re.M):
        if re.search(r'^  name: (?:voice-p3-principal-jwks|voice-p3-principal-publisher-config)\s*$', piece, re.M):
            selected.append(decoder(piece.encode()))
    return principal_publisher_documents(selected)


def principal_publisher_documents(rows):
    selected = [row for row in rows if row.get('metadata', {}).get('name') in (PRINCIPAL_PUBLISHER, PRINCIPAL_CONFIG)]
    if not selected:
        return False
    expected = {('Deployment', PRINCIPAL_PUBLISHER), ('Service', PRINCIPAL_PUBLISHER), ('ConfigMap', PRINCIPAL_CONFIG)}
    if len(selected) != 3 or {(r.get('kind'), r.get('metadata', {}).get('name')) for r in selected} != expected:
        _fail('canonical_principal_publisher_inventory')
    for row in selected:
        if row['metadata'].get('namespace') != NS:
            _fail('canonical_principal_publisher_namespace')
        if row['kind'] == 'Deployment':
            principal_publisher_shape(row)
        if row['kind'] == 'Service' and row['spec'] != {
                'selector': {'app': PRINCIPAL_PUBLISHER}, 'ports': [{'name': 'https', 'port': 8443}]}:
            _fail('canonical_principal_publisher_service')
        if row['kind'] == 'ConfigMap':
            data = row.get('data', {})
            if (set(data) != {'default.conf'} or row.get('binaryData')
                    or not isinstance(data['default.conf'], str)
                    or hashlib.sha256(data['default.conf'].encode()).hexdigest() != PRINCIPAL_CONFIG_SHA256):
                _fail('canonical_principal_publisher_config')
    return True


def principal_publisher_image(image):
    if not isinstance(image, str) or not re.fullmatch(r'(?:docker\.io/library/)?nginx@sha256:[a-f0-9]{64}', image):
        _fail('canonical_principal_publisher_image')
    return image

APP_SOURCES = ('deploy/staging/services.yaml', 'deploy/staging/gateway-deployment.yaml',
               'deploy/templates/network-policy-social-privacy-principal.yaml',
               'deploy/templates/network-policy-file-user-principal.yaml',
               'deploy/templates/network-policy-search-user-projection.yaml')
DATABASES = tuple(service + '_db' for service in ('auth', 'user', 'social', 'chat', 'messaging', 'file', 'space',
    'role', 'notification', 'matchmaking', 'gateway', 'search', 'subscription', 'moderation', 'bot', 'story', 'voice', 'realtime'))
PG_SQL = "SELECT json_build_object('databases',(SELECT json_agg(datname ORDER BY datname) FROM pg_database WHERE datname LIKE '%\\_db'), 'authenticated',true)"
GATEWAY_SQL = "SELECT json_build_object('columns',(SELECT json_agg(json_build_array(column_name,data_type,is_nullable) ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema='public' AND table_name='client_versions'),'primary_key',(SELECT json_agg(a.attname) FROM pg_index i JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=ANY(i.indkey) WHERE i.indrelid='public.client_versions'::regclass AND i.indisprimary),'windows_seed',EXISTS(SELECT 1 FROM client_versions WHERE platform='windows'))"
CH_SQL = "SELECT name, engine, create_table_query FROM system.tables WHERE database='voice' ORDER BY name FORMAT JSON"


def _fail(label):
    raise PlanError(label)


def _value(secret, key):
    try:
        value = base64.b64decode(secret['data'][key], validate=True).decode('utf-8')
        if not value.strip():
            _fail('canonical_required_authority_missing')
        return value
    except (KeyError, ValueError, TypeError, UnicodeError):
        _fail('canonical_required_authority_missing')


class _Source:
    def __init__(self, root, parameters, decoder, kube):
        self.root, self.p, self.decoder, self.kube = Path(root).resolve(), dict(parameters), decoder, kube
        self.rows = {row: {'id': row, 'disposition': 'preserve', 'objects': [], 'sources': []} for row in guard.ROW_IDS}
        self.actions, self.action_keys = [], {}
        self.raw, self.live = {}, {}

    def read(self, path, row):
        target = self.root / path
        if not target.resolve().is_relative_to(self.root) or not target.is_file():
            _fail('canonical_source_missing_' + row)
        raw = target.read_bytes()
        entry = {'path': path, 'sha256': hashlib.sha256(raw).hexdigest()}
        if entry not in self.rows[row]['sources']:
            self.rows[row]['sources'].append(entry)
        self.raw[path] = raw
        return raw.decode('utf-8')

    def get(self, kind, name):
        key = kind, name
        if key not in self.live:
            try:
                self.live[key] = self.kube.get(kind, name)
            except Exception:
                _fail('canonical_live_resource_missing_' + kind.lower() + '_' + name)
        return copy.deepcopy(self.live[key])

    def secret(self, row, name, keys, predicates=None, secret_type=None):
        desired = {'required_keys': sorted(keys)}
        if name == 'voice-media-jvm-trust' and 'truststore.p12' in keys:
            desired['binary_keys'] = ['truststore.p12']
        if predicates:
            desired['predicates'] = predicates
        if secret_type:
            desired['type'] = secret_type
        live = self.get('Secret', name)
        guard._secret(live, desired)
        for existing in self.rows[row]['objects']:
            if existing['kind'] == 'Secret' and existing['name'] == name:
                existing['desired']['required_keys'] = sorted(set(existing['desired']['required_keys']) | set(keys))
                if 'binary_keys' in desired:
                    existing['desired']['binary_keys'] = desired['binary_keys']
                existing['desired'].setdefault('predicates', {}).update(predicates or {})
                return live
        self.rows[row]['objects'].append({'kind': 'Secret', 'name': name, 'namespace': NS, 'desired': desired})
        return live

    def documents(self, path, row, extra=None, skip_nats=False):
        text = self.read(path, row)
        release = None
        if path == 'deploy/staging/services.yaml':
            release = release_policy_wiring.source_input(self.root, self.p)
            if release is not None:
                text = release_policy_wiring.render(text, release, self.p['tag'], NS)
        replacements = dict(self.replacements)
        replacements.update(extra or {})
        result = []
        for piece in re.split(r'^---\s*$', text, flags=re.M):
            if not piece.strip() or not re.search(r'^kind:', piece, re.M):
                continue
            if skip_nats and re.search(r'^  name: voice-nats(?:[-\s]|$)', piece, re.M):
                continue
            for token, value in replacements.items():
                piece = piece.replace(token, str(value))
            if re.search(r'__[A-Z0-9_]+__|IMAGE_PLACEHOLDER', '\n'.join(line for line in piece.splitlines() if not line.lstrip().startswith('#'))):
                _fail('canonical_render_unresolved_' + row)
            obj = self.decoder(piece.encode())
            if not isinstance(obj, dict) or not isinstance(obj.get('metadata'), dict):
                _fail('canonical_document_invalid_' + row)
            obj['metadata'].setdefault('namespace', NS)
            result.append(obj)
        if release is not None:
            release_policy_wiring.check_documents(result, release, self.p['tag'], NS)
        return result

    def add(self, row, obj, action=False, private=False):
        kind, name = obj['kind'], obj['metadata']['name']
        if obj['metadata'].get('namespace') != NS:
            _fail('canonical_namespace_invalid')
        if (row == 'frontends' and kind == 'ConfigMap' and name in guard.FRONTEND_BOOTSTRAP
                and action and not private and self.p['mode'] in ('full','app-only')):
            live = guard.frontend_live(self.kube, name)
            if live is None:
                try: normalized = self.kube.run(['create', '--dry-run=server', '-f', '/dev/stdin', '-o', 'json'], body=obj)
                except Exception: _fail('canonical_normalization_failed_' + row)
                if normalized.get('kind') != kind or normalized.get('metadata', {}).get('name') != name or normalized.get('metadata', {}).get('namespace') != NS:
                    _fail('canonical_normalized_identity_invalid')
                self.rows[row]['objects'].append({'kind':kind,'name':name,'namespace':NS,
                    'desired':guard.semantic(normalized),'disposition':'create'})
                return normalized
        else:
            live = self.get(kind, name)
        try:
            normalized = obj if private else self.kube.run(['apply', '--dry-run=server', '-f', '/dev/stdin', '-o', 'json'], body=obj)
        except Exception:
            _fail('canonical_normalization_failed_' + row)
        if normalized.get('kind') != kind or normalized.get('metadata', {}).get('name') != name:
            _fail('canonical_normalized_identity_invalid')
        desired = guard.semantic(normalized)
        actual = guard.semantic(live)
        descriptor = {'kind': kind, 'name': name, 'namespace': NS, 'desired': desired}
        if private:
            descriptor['desired'] = {'private_semantic_sha256': guard.digest(desired)}
        if desired != actual:
            if not action or private or kind not in guard.ACTION_KINDS or kind == 'StatefulSet' or self.p['mode'] == 'images-only':
                _fail('unsupported_canonical_change_' + row + '_' + name)
            key = kind, name
            action_id = self.action_keys.get(key)
            if action_id is None:
                action_id = 'a' + guard.digest(key)[:20]
                self.action_keys[key] = action_id
                self.actions.append({'id': action_id, 'row': row, 'manifest': normalized})
            else:
                existing = next(a for a in self.actions if a['id'] == action_id)
                if guard.semantic(existing['manifest']) != desired:
                    _fail('conflicting_canonical_targets')
            descriptor.update(disposition='action', action_id=action_id)
        self.rows[row]['objects'].append(descriptor)
        return normalized

    def skip(self, row, reason='not-requested'):
        self.rows[row].update(disposition='skip', reason=reason, enabled=False, objects=[])


def _configuration(builder):
    p = builder.p
    text = builder.read('deploy/staging/domains.defaults', 'domains')
    defaults = {}
    for line in text.splitlines():
        if not line.strip() or line.lstrip().startswith('#'):
            continue
        match = re.fullmatch(r'(VOICE_[A-Z_]+)=([a-zA-Z0-9.-]+)', line.strip())
        if not match:
            _fail('canonical_domain_defaults_not_passive')
        defaults[match[1]] = match[2]
    for parameter, variable in HOSTS.items():
        p.setdefault(parameter, defaults.get(variable, ''))
    p.setdefault('storage_host', '')
    p.setdefault('gateway_tls_secret', 'voice-gateway-tls')
    p.setdefault('storage_tls_secret', 'voice-storage-tls')
    p.setdefault('web_origin', 'https://' + p['web_host'])
    if p['web_origin'] != 'https://' + p['web_host']:
        _fail('canonical_web_origin_mismatch')
    for key in (*HOSTS, 'storage_host'):
        if not isinstance(p[key], str) or p[key] and not re.fullmatch(r'[a-zA-Z0-9.-]+', p[key]):
            _fail('canonical_domain_invalid')
    text = builder.read('scripts/staging/apply-infra.sh', 'infra')
    for key, variable in (('minio_image', 'VOICE_MINIO_IMAGE'), ('minio_mc_image', 'VOICE_MINIO_MC_IMAGE'),
                          ('minio_storage_class', 'VOICE_MINIO_STORAGE_CLASS'), ('minio_storage_size', 'VOICE_MINIO_STORAGE_SIZE')):
        match = re.search(r'\$\{' + variable + r':-([^}]+)\}', text)
        if not match:
            _fail('canonical_minio_default_missing')
        p.setdefault(key, match.group(1))
    generation = p.get('generation', 'legacy')
    if not re.fullmatch(r'legacy|[gr][0-9]{8}[a-z0-9]{0,8}', generation):
        _fail('canonical_generation_invalid')
    r = {'__IMAGE_REGISTRY__': p['registry'], '__IMAGE_TAG__': p['tag'],
         'IMAGE_PLACEHOLDER': p['registry'] + '/gateway:' + p['tag'], '__K_NAMESPACE__': NS,
         '__NAMESPACE__': NS, '__S3_SIGNING_ENDPOINT__': p['s3_signing_endpoint'],
         '__GATEWAY_INGRESS_HOST__': p['gateway_host'], '__DEVELOPER_PORTAL_INGRESS_HOST__': p['developer_portal_host'],
         '__WEB_INGRESS_HOST__': p['web_host'], '__ADMIN_INGRESS_HOST__': p['admin_host'], '__LIVEKIT_INGRESS_HOST__': p['livekit_host'],
         '__VOICE_MINIO_IMAGE__': p['minio_image'], '__VOICE_MINIO_MC_IMAGE__': p['minio_mc_image'],
         '__VOICE_MINIO_STORAGE_CLASS__': p['minio_storage_class'], '__VOICE_MINIO_STORAGE_SIZE__': p['minio_storage_size'],
         '__WEB_ORIGIN__': p['web_origin']}
    if generation != 'legacy':
        for name in ('operator', 'hub-tls', 'bootstrap-credentials', 'service-credentials'):
            r['voice-nats-' + name] = 'voice-nats-' + name + '-' + generation
    builder.replacements = r


def _ingress(builder, app_secret, row='ingress'):
    p = builder.p
    if not p['gateway_host']:
        if row != 'signing-endpoint':
            builder.skip(row)
        return
    buckets = {token: _value(app_secret, key) for token, key in (
        ('__FILE_BUCKET__', 'FILE_R2_BUCKET'), ('__AVATAR_BUCKET__', 'USER_R2_BUCKET'))}
    if any(not re.fullmatch(r'[a-z0-9][a-z0-9.-]*[a-z0-9]', value) for value in buckets.values()):
        _fail('canonical_bucket_invalid')
    replacements = {**buckets, '__INGRESS_HOST__': p['gateway_host'], '__TLS_SECRET_NAME__': p['gateway_tls_secret']}
    for obj in builder.documents('deploy/gateway/ingress.yaml', row, replacements):
        builder.add(row, obj, action=True)
    if p['storage_host']:
        tls_row = 'principal-secrets' if row != 'signing-endpoint' else row
        builder.secret(tls_row, p['storage_tls_secret'], ('tls.crt', 'tls.key'), secret_type='kubernetes.io/tls')
        for obj in builder.documents('deploy/storage/ingress.yaml', row,
            {**buckets, '__STORAGE_INGRESS_HOST__': p['storage_host'], '__STORAGE_TLS_SECRET__': p['storage_tls_secret']}):
            builder.add(row, obj, action=True)


def frontend_objects(builder, *, deployments_only=False):
    """Plan frontend objects using the same source and live comparison."""
    for frontend, path, host in (('developer-portal', 'deploy/staging/developer-portal.yaml', 'developer_portal_host'),
        ('web', 'deploy/staging/flutter-web.yaml', 'web_host'), ('admin', 'deploy/staging/admin.yaml', 'admin_host')):
        if not (builder.root / path).exists(): continue
        for obj in builder.documents(path, 'frontends'):
            if deployments_only and obj['kind'] != 'Deployment': continue
            if obj['kind'] == 'Deployment':
                _pin_images(obj, builder.p)
                if builder.p.get('image_pull_secret'):
                    obj['spec']['template']['spec']['imagePullSecrets'] = [{'name': builder.p['image_pull_secret']}]
            builder.add('frontends', obj, action=True)


def _gateway_delegated_config(obj):
    """Validate the canonical staging delegation prerequisites before planning writes."""
    if obj['metadata']['name'] != 'voice-gateway':
        return
    spec = obj['spec']['template']['spec']
    containers = [c for c in spec['containers'] if c['name'] == 'gateway']
    if len(containers) != 1:
        _fail('canonical_gateway_delegated_config_invalid')
    container = containers[0]
    entries = container.get('env', [])
    env = {e['name']: e for e in entries}
    required = {
        'GATEWAY_SESSION_EPOCH_STRICT': {'value': 'true'},
        'GATEWAY_PRINCIPAL_SIGNING_KEYS_DIR': {'value': '/etc/voice/media/keys'},
        'GATEWAY_PRINCIPAL_ACTIVE_KID': {'valueFrom': {'secretKeyRef':
            {'name': 'voice-gateway-principal-signing', 'key': 'active-kid'}}},
        'GATEWAY_REDIS_ADDR': {'valueFrom': {'configMapKeyRef':
            {'name': 'voice-app-config', 'key': 'REDIS_ADDR'}}},
    }
    mounts = [m for m in container.get('volumeMounts', []) if m.get('mountPath') == '/etc/voice/media/keys']
    volumes = [v for v in spec.get('volumes', []) if len(mounts) == 1 and v.get('name') == mounts[0].get('name')]
    if (len(env) != len(entries) or any(env.get(k) != dict(name=k, **v) for k, v in required.items())
            or len(mounts) != 1 or mounts[0].get('readOnly') is not True or len(volumes) != 1):
        _fail('canonical_gateway_delegated_config_invalid')
    secret = volumes[0].get('secret', {})
    if (secret.get('secretName') != 'voice-gateway-principal-signing' or secret.get('optional', False)
            or secret.get('items') != [{'key': 'current.pem', 'path': 'current.pem'}, {'key': 'next.pem', 'path': 'next.pem'}]):
        _fail('canonical_gateway_delegated_config_invalid')


def _leaf_monitor_config(obj):
    # These staging roles use the shared leaf config; monitor only inside the Pod.
    roles = {'analytics', 'auth', 'bot', 'chat', 'file', 'matchmaking', 'messaging',
             'moderation', 'notification', 'realtime', 'role', 'search', 'social',
             'space', 'story', 'subscription', 'user', 'voice'}
    if obj['metadata']['name'].removeprefix('voice-') not in roles:
        return
    spec = obj['spec']['template']['spec']
    leaves = [c for c in spec.get('initContainers', []) + spec.get('containers', []) if c['name'] == 'nats-leaf']
    if len(leaves) != 1:
        _fail('canonical_leaf_monitor_invalid')
    leaf = leaves[0]
    mounts = [m for m in leaf.get('volumeMounts', []) if m.get('mountPath') == '/etc/nats/leaf.conf']
    volumes = [v for v in spec.get('volumes', []) if len(mounts) == 1 and v.get('name') == mounts[0].get('name')]
    if (leaf.get('command') or leaf.get('args') != ['-c', '/etc/nats/leaf.conf', '-a', '127.0.0.1', '-m', '8222']
            or len(mounts) != 1 or mounts[0].get('subPath') != 'leaf.conf' or mounts[0].get('readOnly') is not True
            or len(volumes) != 1 or volumes[0].get('configMap') != {'name': 'voice-nats-leaf-config'}):
        _fail('canonical_leaf_monitor_invalid')


def _application(builder, *, images_only=False):
    p = builder.p
    apps = []
    gis_objects = []
    for path in APP_SOURCES:
        documents = builder.documents(path, 'app-restart')
        principal_publisher_documents(documents)
        for obj in documents:
            if p.get('gis_first_install') is True and (obj.get('kind'), obj.get('metadata', {}).get('name')) in {
                ('Service', 'voice-gameintegration'), ('Deployment', 'voice-gameintegration')
            }:
                # The finite first-install owns these two absent identities via
                # create-only calls after the protected database phase. Never
                # let the ordinary source planner turn absence into apply.
                if obj.get('kind') == 'Deployment':
                    _gateway_delegated_config(obj)
                    _leaf_monitor_config(obj)
                    _pin_images(obj, p)
                gis_objects.append(obj)
                continue
            if obj['kind'] == 'Deployment':
                if obj['metadata']['name'] == PRINCIPAL_PUBLISHER:
                    principal_publisher_shape(obj)
                    public = builder.get('ConfigMap', PRINCIPAL_PUBLIC)
                    if (public.get('immutable') is not True or set(public.get('data', {})) != {'realtime.json'}
                            or public.get('binaryData') or not isinstance(public['data']['realtime.json'], str)
                            or not 1 <= len(public['data']['realtime.json'].encode()) <= 65536):
                        _fail('canonical_principal_public_immutable')
                    builder.add('principal-secrets', public, private=True)
                _gateway_delegated_config(obj)
                _leaf_monitor_config(obj)
                spec = obj['spec']['template']['spec']
                pull = p.get('image_pull_secret', '')
                if pull:
                    spec['imagePullSecrets'] = [{'name': pull}]
                _pin_images(obj, p)
                apps.append(obj)
            builder.add('app-restart', obj, action=True)
    if p.get('gis_first_install') is True:
        if p.get('mode') not in ('full', 'app-only'):
            _fail('gis_first_install_mode_invalid')
        identities = {(obj.get('kind'), obj.get('metadata', {}).get('name')) for obj in gis_objects}
        expected = {('Service', 'voice-gameintegration'), ('Deployment', 'voice-gameintegration')}
        if identities != expected or len(gis_objects) != 2:
            _fail('gis_first_install_source_pair_invalid')
        # A bounded namespace list gives authoritative presence/absence without
        # parsing kubectl's human error text for a 404 signal.
        require_gis_absent(builder.kube)
        builder.gis_first_install_objects = gis_objects
    for obj in apps:
        service = obj['metadata']['name'].removeprefix('voice-')
        if service in ('file', 'user'):
            container = next(c for c in obj['spec']['template']['spec']['containers'] if c['name'] == service)
            expected = service.upper() + '_R2_SIGNING_ENDPOINT'
            if not any(e.get('name') == expected and e.get('value') == p['s3_signing_endpoint'] for e in container.get('env', [])):
                _fail('canonical_signing_source_missing')
            builder.add('signing-endpoint', obj, action=True)
    # The pre-bootstrap inventory needs the same rendered and normalized
    # Deployment actions, before the missing database credential can exist.
    # The ordinary source plan still validates every Secret reference below.
    if images_only:
        return
    # Derive every non-NATS Secret ref/key from canonical rendered Pod specs.
    refs = {}
    def visit(value):
        if isinstance(value, dict):
            if 'secretKeyRef' in value:
                ref = value['secretKeyRef']
                if not ref.get('optional') and not ref['name'].startswith('voice-nats-'):
                    refs.setdefault(ref['name'], set()).add(ref['key'])
            if 'secret' in value and isinstance(value['secret'], dict) and 'secretName' in value['secret']:
                ref = value['secret']
                if not ref['secretName'].startswith('voice-nats-'):
                    refs.setdefault(ref['secretName'], set()).update(item['key'] for item in ref.get('items', []))
            for v in value.values(): visit(v)
        elif isinstance(value, list):
            for v in value: visit(v)
    for obj in apps: visit(obj)
    for name, keys in refs.items():
        if name == 'voice-app-secrets':
            allow_empty = {'USER_R2_ENDPOINT', 'FILE_R2_ENDPOINT', 'USER_R2_PUBLIC_BASE_URL'}
            for key in keys & allow_empty:
                if key not in builder.get('Secret', name).get('data', {}):
                    _fail('canonical_app_secret_key_missing')
            keys -= allow_empty
        if keys: builder.secret('principal-secrets', name, keys)


def preservation_projection(source, parameters, decoder, kube):
    """Read-only, source-rendered store comparison before the writer cut.

    The returned record is private evidence, not authority by itself. Its caller
    must bind it to the protected source, session and current storage identities.
    """
    try:
        p=dict(parameters)
        if p.get('mode') not in ('full','app-only'):_fail('preservation_projection_mode_invalid')
        b=_Source(source,p,decoder,kube);_configuration(b)
        app=b.get('Secret','voice-app-secrets')
        credentials=b.secret('minio-credentials','voice-minio-credentials',('MINIO_ROOT_USER','MINIO_ROOT_PASSWORD'))
        selected_keys=('USER_R2_ENDPOINT','USER_R2_REGION','USER_R2_BUCKET','USER_R2_PUBLIC_BASE_URL',
                       'FILE_R2_ENDPOINT','FILE_R2_REGION','FILE_R2_BUCKET','CLICKHOUSE_DSN','CLICKHOUSE_PASSWORD')
        if any(key not in app.get('data',{}) for key in selected_keys):_fail('preservation_store_secret_missing')
        b.read('scripts/staging/ensure-minio-credentials.sh','minio-credentials')
        b.read('scripts/storage/apply-minio-cors.sh','minio-cors')
        minio=b.documents('deploy/staging/minio.yaml','minio-cors')
        minio_ids=[(obj['kind'],obj['metadata']['name']) for obj in minio]
        expected_minio={('StatefulSet','voice-minio'),('Service','voice-minio'),('Job','voice-minio-create-avatars-bucket'),('Job','voice-minio-create-files-bucket')}
        if set(minio_ids)!=expected_minio or len(minio_ids)!=len(expected_minio):_fail('preservation_minio_source_invalid')
        states=[obj for obj in minio if obj['kind']=='StatefulSet' and obj['metadata']['name']=='voice-minio']
        if len(states)!=1:_fail('preservation_minio_source_invalid')
        b.add('minio-cors',states[0],action=True);_preserve_claims(b,states[0])
        if p['mode']=='full':
            private={'__LIVEKIT_API_KEY__':_value(app,'LIVEKIT_API_KEY'),'__LIVEKIT_API_SECRET__':_value(app,'LIVEKIT_API_SECRET')}
            infra=b.documents('deploy/staging/infra.yaml','infra',private,skip_nats=True)
            wanted={('Service','voice-redis'),('Deployment','voice-redis'),('Service','voice-clickhouse'),('StatefulSet','voice-clickhouse')}
            selected=[obj for obj in infra if (obj['kind'],obj['metadata']['name']) in wanted]
            if {(obj['kind'],obj['metadata']['name']) for obj in selected}!=wanted or len(selected)!=len(wanted):_fail('preservation_infra_source_invalid')
            for obj in selected:
                b.add('infra',obj)
                if obj['kind']=='StatefulSet':_preserve_claims(b,obj)
            b.read('scripts/staging/apply-clickhouse-init.sh','db-init')
            ddl=b.read('docker/clickhouse/init/001_events.sql','db-init')
            cm={'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':'voice-clickhouse-init','namespace':NS},'data':{'001_events.sql':ddl}}
            b.add('db-init',cm)
            b.get('Pod','voice-clickhouse-0')
            _verify_clickhouse(kube,app,_clickhouse_statements(ddl))
            for obj in minio:
                if obj['kind']=='Job':
                    observed=b.get('Job',obj['metadata']['name'])
                    if observed.get('status',{}).get('succeeded')!=1 or observed.get('status',{}).get('active',0):_fail('canonical_minio_bucket_job_incomplete')
                    desired=copy.deepcopy(obj['spec']['template']);actual=copy.deepcopy(observed['spec']['template'])
                    actual.setdefault('metadata',{}).setdefault('labels',{})
                    for key in ('controller-uid','job-name','batch.kubernetes.io/controller-uid','batch.kubernetes.io/job-name'):actual['metadata']['labels'].pop(key,None)
                    if not actual['metadata']['labels']:actual['metadata'].pop('labels',None)
                    if not actual['metadata']:actual.pop('metadata',None)
                    if actual!=desired:_fail('unsupported_minio_bucket_job_change')
                    desired_spec=obj['spec'];actual_spec=observed['spec']
                    defaults={'parallelism':1,'completions':1,'backoffLimit':6,'completionMode':'NonIndexed',
                              'suspend':False,'manualSelector':False}
                    allowed=set(desired_spec)|set(defaults)|{'selector'}
                    if not set(actual_spec)<=allowed:_fail('unsupported_minio_bucket_job_change')
                    for key in set(desired_spec)-{'template'}:
                        if actual_spec.get(key)!=desired_spec[key]:_fail('unsupported_minio_bucket_job_change')
                    for key,value in defaults.items():
                        if key not in desired_spec and key in actual_spec and actual_spec[key]!=value:_fail('unsupported_minio_bucket_job_change')
                    if 'selector' in actual_spec and 'selector' not in desired_spec:
                        selector=actual_spec['selector'];labels=selector.get('matchLabels',{}) if type(selector) is dict else {}
                        uid=observed['metadata'].get('uid')
                        if (set(selector)!={'matchLabels'} or len(labels)!=1 or not uid
                            or next(iter(labels)) not in ('controller-uid','batch.kubernetes.io/controller-uid')
                            or next(iter(labels.values()))!=uid):_fail('unsupported_minio_bucket_job_change')
                elif obj['kind']=='Service':b.add('infra',obj)
                elif obj['kind']!='StatefulSet':_fail('preservation_minio_source_invalid')
        services=b.documents('deploy/staging/services.yaml','migrations');r2_envs={}
        for name in ('user','file'):
            docs=[obj for obj in services if obj.get('kind')=='Deployment' and obj['metadata']['name']=='voice-'+name]
            if len(docs)!=1:_fail('preservation_r2_source_invalid')
            def env(obj):
                containers=[c for c in obj['spec']['template']['spec']['containers'] if c['name']==name]
                if len(containers)!=1:_fail('preservation_r2_container_invalid')
                values=[entry for entry in containers[0].get('env',[]) if entry.get('name','').startswith(name.upper()+'_R2_')]
                if len({entry['name'] for entry in values})!=len(values):_fail('preservation_r2_environment_invalid')
                return sorted(values,key=lambda item:item['name'])
            expected={name.upper()+'_R2_'+suffix for suffix in ('ENDPOINT','SIGNING_ENDPOINT','REGION','ACCESS_KEY_ID','SECRET_ACCESS_KEY','BUCKET')}
            if name=='user':expected.add('USER_R2_PUBLIC_BASE_URL')
            desired_env=env(docs[0])
            if {entry['name'] for entry in desired_env}!=expected:_fail('preservation_r2_source_invalid')
            if desired_env!=env(b.get('Deployment','voice-'+name)):_fail('unsupported_r2_configuration_change_'+name)
            r2_envs['voice-'+name]=desired_env
        config=b.documents('deploy/staging/configmap-app.yaml','domains')[0]
        current=b.get('ConfigMap',config['metadata']['name'])
        redis_addr=config.get('data',{}).get('REDIS_ADDR')
        if not isinstance(redis_addr,str) or not redis_addr or redis_addr!=current.get('data',{}).get('REDIS_ADDR'):_fail('unsupported_redis_configuration_change')
        identities=[]
        for (kind,name),obj in sorted(b.live.items()):
            meta=obj.get('metadata',{})
            if not meta.get('uid') or not meta.get('resourceVersion'):_fail('preservation_store_identity_invalid')
            semantic=guard.semantic(obj)
            if kind=='Secret':
                keys=selected_keys if name=='voice-app-secrets' else ('MINIO_ROOT_USER','MINIO_ROOT_PASSWORD')
                semantic={'data':{key:obj['data'][key] for key in keys}}
            elif kind=='Deployment' and name in r2_envs:semantic={'r2_env':r2_envs[name]}
            elif kind=='ConfigMap' and name==config['metadata']['name']:semantic={'REDIS_ADDR':redis_addr}
            identities.append({'kind':kind,'name':name,'uid':meta['uid'],'resourceVersion':meta['resourceVersion'],'semantic_sha256':guard.digest(semantic)})
        sources=sorted({(entry['path'],entry['sha256']) for row in b.rows.values() for entry in row['sources']})
        return {'schema':'voice-staging-store-projection-v1','mode':p['mode'],'sources':[{'path':path,'sha256':sha} for path,sha in sources],
                'identities':identities}
    except PlanError:raise
    except Exception:_fail('preservation_store_projection_invalid')


def compile_plan(source, parameters, decoder, kube):
    """Return a canonical plan, with specific unsupported-change vetoes."""
    try:
        p = dict(parameters)
        if p.get('mode') not in ('full', 'app-only', 'images-only'):
            _fail('canonical_mode_invalid')
        if p.get('secret_updates') or any(p.get(k) for k in ('STAGING_APP_SECRETS_YAML_B64', 'STAGING_APP_SECRETS_YAML', 'principal_secret_updates')):
            _fail('unsupported_authority_update')
        if (Path(source) / 'deploy/staging/secret.yaml').exists():
            _fail('unsupported_source_authority_update')
        # Validate before the first _configuration or live resource observation.
        release_policy_wiring.source_input(source, p)
        b = _Source(source, p, decoder, kube)
        _configuration(b)
        b.read('scripts/staging/preflight-resend-key.sh', 'auth-mail')
        app = b.secret('auth-mail', 'voice-app-secrets', ('AUTH_RESEND_API_KEY', 'AUTH_RESEND_FROM'), {'auth_mail': True})
        if any(p.get(key) for key in ('STAGING_STAFF_TOKEN', 'staff_token', 'VOICE_STAGING_CLICKHOUSE_PASSWORD')):
            _fail('unsupported_explicit_authority_update')
        if p['mode'] == 'images-only':
            b.read('scripts/storage/apply-signing-env.sh', 'signing-endpoint')
            for service in ('file', 'user'):
                obj = b.get('Deployment', 'voice-' + service)
                containers = obj['spec']['template']['spec']['containers']
                actual = next(c for c in containers if c['name'] == service)
                key = service.upper() + '_R2_SIGNING_ENDPOINT'
                if not any(e.get('name') == key and e.get('value') == p['s3_signing_endpoint'] for e in actual.get('env', [])):
                    _fail('unsupported_images_signing_endpoint_change')
                b.add('signing-endpoint', obj)
            _ingress(b, app, 'signing-endpoint')
            if b.p['livekit_host']:
                for obj in b.documents('deploy/livekit/ingress.yaml', 'signing-endpoint', {'__INGRESS_HOST__': b.p['livekit_host']}):
                    b.add('signing-endpoint', obj)
            minio = b.get('StatefulSet', 'voice-minio')
            env = next(c for c in minio['spec']['template']['spec']['containers'] if c['name'] == 'minio')['env']
            if not any(e.get('name') == 'MINIO_API_CORS_ALLOW_ORIGIN' and e.get('value') == b.p['web_origin'] for e in env):
                _fail('unsupported_images_minio_cors_change')
            b.read('scripts/storage/apply-minio-cors.sh', 'signing-endpoint')
            b.add('signing-endpoint', minio)
            selected = ('auth-mail', 'signing-endpoint')
        else:
            for row, contract in guard.SECRET_CONTRACTS.items():
                for name, keys in contract.items(): b.secret(row, name, keys)
            b.read('scripts/staging/check-social-principal-secrets.sh', 'principal-secrets')
            b.read('scripts/staging/ensure-user-search-projection-secret.sh', 'user-search-cursor')
            b.read('scripts/staging/ensure-minio-credentials.sh', 'minio-credentials')
            _application(b)
            _ingress(b, app)
            if b.p['livekit_host']:
                for obj in b.documents('deploy/livekit/ingress.yaml', 'livekit-ingress', {'__INGRESS_HOST__': b.p['livekit_host']}):
                    b.add('livekit-ingress', obj, action=True)
            else: b.skip('livekit-ingress')
            frontend_objects(b)
            if not b.rows['frontends']['objects']: b.skip('frontends')
            config = b.documents('deploy/staging/configmap-app.yaml', 'domains')[0]
            b.add('domains', config, action=p['mode'] == 'full')
            minio_docs = b.documents('deploy/staging/minio.yaml', 'minio-cors')
            minio = next(o for o in minio_docs if o['kind'] == 'StatefulSet')
            b.add('minio-cors', minio, action=True)
            b.read('scripts/storage/apply-minio-cors.sh', 'minio-cors')
            # Migration runner owns actual Jobs. Here freeze its complete required
            # canonical source/DSN prerequisites; never execute or recreate Jobs.
            b.read('scripts/staging/apply-migrate-jobs.sh', 'migrations')
            keys = {service.upper() + '_DATABASE_URL' for service in ('bot', 'story', 'moderation', 'subscription', 'user', 'search',
                'social', 'chat', 'messaging', 'file', 'space', 'role', 'notification', 'matchmaking', 'voice', 'realtime')}
            keys.update(('AUTH_JWT_PRIVATE_KEY', 'AUTH_TOTP_ENCRYPTION_KEY', 'POSTGRES_PASSWORD'))
            b.secret('migrations', 'voice-app-secrets', keys)
            for database in DATABASES:
                path = b.root / 'src/backend/migrations' / database
                if not path.exists():
                    if database in ('auth_db',): continue
                    _fail('canonical_migration_source_missing_' + database)
                for sql in sorted(path.glob('*.sql')):
                    b.read(sql.relative_to(b.root).as_posix(), 'migrations')
            auth_migrations = b.root / 'src/backend/auth/src/main/resources/db/migration'
            if not auth_migrations.is_dir(): _fail('canonical_auth_migration_source_missing')
            for sql in sorted(auth_migrations.glob('*.sql')):
                b.read(sql.relative_to(b.root).as_posix(), 'migrations')
            if p['mode'] == 'full':
                helper = b.read('scripts/staging/check-resend-key.py', 'app-config')
                match = re.search(r'REQUIRED_KEYS\s*=\s*frozenset\(\s*"""(.*?)"""\.split\(\)', helper, re.S)
                if not match: _fail('canonical_app_secret_catalog_invalid')
                required = set(match[1].split())
                empty = {'USER_R2_ENDPOINT', 'FILE_R2_ENDPOINT', 'USER_R2_PUBLIC_BASE_URL'}
                if not required <= set(app['data']): _fail('canonical_app_secret_key_missing')
                b.secret('app-config', 'voice-app-secrets', required - empty)
                b.add('app-config', config, action=True)
                patch = b.read('scripts/staging/patch-app-secrets-database-urls.sh', 'app-config')
                pairs = re.findall(r'^\s+([A-Z_]+_DATABASE_URL):([a-z_]+_db)\s*$', patch, re.M)
                if not pairs: _fail('canonical_database_url_catalog_invalid')
                password = quote(_value(app, 'POSTGRES_PASSWORD'), safe='')
                for key, database in pairs:
                    expected = 'postgres://voice:' + password + '@voice-postgres:5432/' + database + '?sslmode=disable'
                    if _value(app, key) != expected: _fail('unsupported_database_url_authority_sync')
                if _value(app, 'CLICKHOUSE_DSN') != 'clickhouse://default:' + _value(app, 'CLICKHOUSE_PASSWORD') + '@voice-clickhouse:9000/voice':
                    _fail('unsupported_clickhouse_dsn_authority_sync')
                b.read('scripts/staging/patch-gateway-staff-token.sh', 'app-config')
                private = {'__LIVEKIT_API_KEY__': _value(app, 'LIVEKIT_API_KEY'), '__LIVEKIT_API_SECRET__': _value(app, 'LIVEKIT_API_SECRET')}
                for obj in b.documents('deploy/staging/infra.yaml', 'infra', private, skip_nats=True):
                    b.add('infra', obj, private=obj['metadata']['name'] == 'voice-livekit-config')
                    if obj['kind'] == 'StatefulSet': _preserve_claims(b, obj)
                for obj in minio_docs:
                    if obj['kind'] == 'Job':
                        observed = b.get('Job', obj['metadata']['name'])
                        if observed.get('status', {}).get('succeeded') != 1 or observed.get('status', {}).get('active', 0):
                            _fail('canonical_minio_bucket_job_incomplete')
                        # API controller adds selector/labels; remove only its UID
                        # labels from comparison of the canonical Pod template.
                        desired_template = copy.deepcopy(obj['spec']['template'])
                        actual_template = copy.deepcopy(observed['spec']['template'])
                        actual_template.setdefault('metadata', {}).setdefault('labels', {})
                        for key in ('controller-uid', 'job-name', 'batch.kubernetes.io/controller-uid', 'batch.kubernetes.io/job-name'):
                            actual_template['metadata']['labels'].pop(key, None)
                        if not actual_template['metadata']['labels']: actual_template['metadata'].pop('labels', None)
                        if not actual_template['metadata']: actual_template.pop('metadata', None)
                        if actual_template != desired_template:
                            _fail('unsupported_minio_bucket_job_change')
                        b.rows['infra']['objects'].append({'kind': 'Job', 'name': obj['metadata']['name'], 'namespace': NS,
                            'desired': guard.semantic(observed)})
                    else:
                        b.add('infra', obj)
                        if obj['kind'] == 'StatefulSet': _preserve_claims(b, obj)
                _db_evidence(b)
            else:
                for row in guard.FULL_ONLY: b.skip(row, 'mode-inapplicable')
            selected = guard.ROW_IDS
            if b.p.get('apply_observability'):
                _fail('unsupported_enabled_observability_reconciliation')
            b.skip('observability')
        plan = {'schema': guard.SCHEMA, 'mode': p['mode'], 'checks': [b.rows[row] for row in selected], 'actions': b.actions}
        guard.preflight(kube, plan, p['mode'])
        if p.get('gis_first_install') is True:
            plan['gis_first_install'] = {'objects': copy.deepcopy(getattr(b, 'gis_first_install_objects', []))}
        return plan
    except PlanError:
        raise
    except Exception:
        _fail('canonical_plan_invalid')


def require_gis_absent(kube):
    """Require bounded, authoritative absence for the only supported pair."""
    expected = {('Service', 'voice-gameintegration'), ('Deployment', 'voice-gameintegration')}
    try:
        response = kube.run(['get', 'services,deployments', '-o', 'json'])
        items = response.get('items') if isinstance(response, dict) and set(response) == {'apiVersion', 'items', 'kind', 'metadata'} else None
        if not isinstance(items, list) or len(items) > 2048:
            _fail('gis_first_install_live_inventory_invalid')
        found = {(row.get('kind'), row.get('metadata', {}).get('name')) for row in items
                 if isinstance(row, dict) and isinstance(row.get('metadata'), dict)}
    except PlanError:
        raise
    except Exception:
        _fail('gis_first_install_live_read_failed')
    if found & expected:
        _fail('gis_first_install_identity_present')
    return True


def _pin_images(obj, parameters):
    spec = obj['spec']['template']['spec']
    for container in spec.get('containers', []) + spec.get('initContainers', []):
        key = obj['metadata']['name'] + '/' + container['name']
        if key in parameters.get('images', {}):
            image = parameters['images'][key]
            if not isinstance(image, str) or not re.fullmatch(r'[^\s@]+@sha256:[a-f0-9]{64}', image):
                _fail('canonical_image_not_immutable')
            container['image'] = image


def _preserve_claims(builder, statefulset):
    for template in statefulset['spec'].get('volumeClaimTemplates', []):
        name = template['metadata']['name'] + '-' + statefulset['metadata']['name'] + '-0'
        pvc = builder.get('PersistentVolumeClaim', name)
        actual = pvc['spec']
        desired = template['spec']
        if (pvc.get('status', {}).get('phase') != 'Bound' or not actual.get('volumeName')
                or any(actual.get(key) != value for key, value in desired.items())):
            _fail('unsupported_existing_storage_change_' + name)
        builder.rows['infra']['objects'].append({'kind': 'PersistentVolumeClaim', 'name': name,
            'namespace': NS, 'desired': guard.semantic(pvc)})


def _db_evidence(builder):
    for path in ('scripts/staging/init-postgres-databases.sh', 'scripts/staging/sync-postgres-password.sh',
                 'scripts/staging/ensure-gateway-schema.sh', 'scripts/staging/apply-clickhouse-init.sh'):
        builder.read(path, 'db-init')
    gateway = builder.read('src/backend/migrations/gateway_db/000001_client_versions.up.sql', 'db-init')
    ddl = builder.read('docker/clickhouse/init/001_events.sql', 'db-init')
    builder.rows['db-init']['evidence'] = {'gateway_sql_sha256': hashlib.sha256(gateway.encode()).hexdigest(),
        'clickhouse_sql_sha256': hashlib.sha256(ddl.encode()).hexdigest(),
        'clickhouse_ddl': _clickhouse_statements(ddl)}
    for name in ('voice-postgres', 'voice-clickhouse'):
        obj = builder.get('StatefulSet', name)
        builder.add('db-init', obj)
        # Bind exact executing Pod too; replacement invalidates the authority observation.
        pod = builder.get('Pod', name + '-0')
        builder.rows['db-init']['objects'].append({'kind': 'Pod', 'name': name + '-0', 'namespace': NS, 'desired': guard.semantic(pod)})
    cm = {'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {'name': 'voice-clickhouse-init', 'namespace': NS}, 'data': {'001_events.sql': ddl}}
    builder.add('db-init', cm)
    verify_db_init(builder.kube, {'mode': 'full', 'checks': list(builder.rows.values())})


def _clickhouse_statements(ddl):
    text = re.sub(r'^--.*$', '', ddl, flags=re.M)
    statements = {}
    for statement in text.split(';'):
        if 'CREATE ' not in statement or 'CREATE DATABASE' in statement:
            continue
        name = re.search(r'voice\.(\w+)', statement)
        if not name or name[1] in statements:
            _fail('canonical_clickhouse_source_invalid')
        statements[name[1]] = _normalized_ddl(statement)
    return statements


def _normalized_ddl(statement):
    # Only textual differences introduced by SHOW CREATE normalization are
    # removed. Columns, defaults, partition/order keys, TTL and view SQL remain.
    statement = statement.lower().replace('`', '')
    statement = re.sub(r'\bif\s+not\s+exists\s+', '', statement)
    statement = re.sub(r'\b(mergetree|aggregatingmergetree|summingmergetree)\(\)', r'\1', statement)
    statement = re.sub(r'\s+settings\s+index_granularity\s*=\s*8192\s*$', '', statement)
    statement = statement.replace('tointervalday(90)', 'interval90day')
    return re.sub(r'\s+', '', statement).rstrip(';')


CLICKHOUSE_READ_ARGS=('exec','voice-clickhouse-0','-c','clickhouse','--','sh','-ceu',
    'printf "{\\"password_sha256\\":\\"%s\\",\\"catalog\\":" "$(printf %s \"$CLICKHOUSE_PASSWORD\" | sha256sum | cut -d\" \" -f1)"; clickhouse-client --host voice-clickhouse --user default --password "$CLICKHOUSE_PASSWORD" --query "$1"; printf "}"',
    '--',CH_SQL)
POSTGRES_READ_SCRIPT='printf "{\\"password_sha256\\":\\"%s\\",\\"catalog\\":" "$(printf %s \"$POSTGRES_PASSWORD\" | sha256sum | cut -d\" \" -f1)"; PGPASSWORD="$POSTGRES_PASSWORD" PGOPTIONS="-c default_transaction_read_only=on -c search_path=pg_catalog,public,pg_temp" psql -h 127.0.0.1 -U "$POSTGRES_USER" -d "$1" -Atqc "$2"; printf "}"'


def postgres_read_args(database):
    if database not in ('postgres','gateway_db'):_fail('canonical_postgres_database_invalid')
    sql=PG_SQL if database=='postgres' else GATEWAY_SQL
    return ['exec','voice-postgres-0','-c','postgres','--','sh','-ceu',POSTGRES_READ_SCRIPT,'--',database,sql]


def _verify_clickhouse(kube, secret, ddl):
    result = kube.run(list(CLICKHOUSE_READ_ARGS))
    if result['password_sha256'] != hashlib.sha256(_value(secret, 'CLICKHOUSE_PASSWORD').encode()).hexdigest():
        _fail('unsupported_clickhouse_authority_sync')
    tables = {row['name']: row for row in result['catalog']['data']}
    expected = {'events': 'MergeTree', 'events_logical': 'View', 'dau_mv': 'AggregatingMergeTree',
        'dau_mv_mv': 'MaterializedView', 'events_by_type_mv': 'SummingMergeTree', 'events_by_type_mv_mv': 'MaterializedView'}
    if any(name not in tables or tables[name]['engine'] != engine for name, engine in expected.items()):
        _fail('canonical_clickhouse_schema_init_missing')
    if set(ddl) != set(expected) or any(_normalized_ddl(tables[name]['create_table_query']) != ddl[name] for name in expected):
        _fail('canonical_clickhouse_ddl_incompatible')


def verify_db_init(kube, plan):
    """Repeat fixed read-only catalog/auth checks, with no source execution."""
    if plan['mode'] != 'full':
        return {'skipped': True}
    try:
        secret = kube.get('Secret', 'voice-app-secrets')
        for pod, password, user, database, sql in (
            ('voice-postgres-0', 'POSTGRES_PASSWORD', 'POSTGRES_USER', 'postgres', PG_SQL),
            ('voice-postgres-0', 'POSTGRES_PASSWORD', 'POSTGRES_USER', 'gateway_db', GATEWAY_SQL)):
            import commands,resource_claim
            session=getattr(commands.authority_guard,'__self__',None)
            guarded=isinstance(session,resource_claim.Session) and session.check().get('cohort_preservation_sha256') is not None
            if guarded:
                import staging_cohort_preservation
                before=staging_cohort_preservation.guard_postgres(session)
            result = kube.run(postgres_read_args(database))
            if guarded and staging_cohort_preservation.guard_postgres(session)!=before:_fail('cohort_postgres_changed')
            if result['password_sha256'] != hashlib.sha256(_value(secret, password).encode()).hexdigest():
                _fail('unsupported_postgres_authority_sync')
            catalog = result['catalog']
            if database == 'postgres':
                if not set(DATABASES) <= set(catalog['databases']) or catalog.get('authenticated') is not True:
                    _fail('canonical_postgres_database_init_missing')
            else:
                expected = [['platform', 'character varying', 'NO'], ['min_supported', 'character varying', 'NO'],
                    ['latest_version', 'character varying', 'NO'], ['update_url', 'text', 'NO'], ['release_notes', 'text', 'YES'],
                    ['shorebird_patch', 'integer', 'YES'], ['updated_at', 'timestamp with time zone', 'YES']]
                if catalog['columns'] != expected or catalog['primary_key'] != ['platform'] or catalog['windows_seed'] is not True:
                    _fail('canonical_gateway_schema_init_missing')
        row = next(row for row in plan['checks'] if row['id'] == 'db-init')
        _verify_clickhouse(kube,secret,row['evidence']['clickhouse_ddl'])
        return {'verified': True}
    except PlanError:
        raise
    except Exception:
        _fail('canonical_db_init_readonly_proof_failed')
