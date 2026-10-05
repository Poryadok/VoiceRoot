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

PlanError = guard.PlanError
NS = 'voice-staging'
HOSTS = {'gateway_host': 'VOICE_GATEWAY_INGRESS_HOST', 'livekit_host': 'VOICE_LIVEKIT_INGRESS_HOST',
         'web_host': 'VOICE_WEB_INGRESS_HOST', 'admin_host': 'VOICE_ADMIN_INGRESS_HOST',
         'developer_portal_host': 'VOICE_DEVELOPER_PORTAL_INGRESS_HOST'}
FRONTENDS = ('developer-portal', 'web', 'admin')
APP_SOURCES = ('deploy/staging/services.yaml', 'deploy/staging/gateway-deployment.yaml',
               'deploy/templates/network-policy-social-privacy-principal.yaml',
               'deploy/templates/network-policy-file-user-principal.yaml',
               'deploy/templates/network-policy-search-user-projection.yaml')
DATABASES = tuple(service + '_db' for service in ('auth', 'user', 'social', 'chat', 'messaging', 'file', 'space',
    'role', 'notification', 'matchmaking', 'gateway', 'search', 'subscription', 'moderation', 'bot', 'story', 'voice'))
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
        if predicates:
            desired['predicates'] = predicates
        if secret_type:
            desired['type'] = secret_type
        live = self.get('Secret', name)
        guard._secret(live, desired)
        for existing in self.rows[row]['objects']:
            if existing['kind'] == 'Secret' and existing['name'] == name:
                existing['desired']['required_keys'] = sorted(set(existing['desired']['required_keys']) | set(keys))
                existing['desired'].setdefault('predicates', {}).update(predicates or {})
                return live
        self.rows[row]['objects'].append({'kind': 'Secret', 'name': name, 'namespace': NS, 'desired': desired})
        return live

    def documents(self, path, row, extra=None, skip_nats=False):
        text = self.read(path, row)
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
        return result

    def add(self, row, obj, action=False, private=False):
        kind, name = obj['kind'], obj['metadata']['name']
        live = self.get(kind, name)
        if obj['metadata'].get('namespace') != NS:
            _fail('canonical_namespace_invalid')
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
    if not re.fullmatch(r'legacy|g[0-9]{8}[a-z0-9]{0,8}', generation):
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


def _application(builder):
    p = builder.p
    apps = []
    for path in APP_SOURCES:
        for obj in builder.documents(path, 'app-restart'):
            if obj['kind'] == 'Deployment':
                spec = obj['spec']['template']['spec']
                pull = p.get('image_pull_secret', '')
                if pull:
                    spec['imagePullSecrets'] = [{'name': pull}]
                _pin_images(obj, p)
                apps.append(obj)
            builder.add('app-restart', obj, action=True)
    for obj in apps:
        service = obj['metadata']['name'].removeprefix('voice-')
        if service in ('file', 'user'):
            container = next(c for c in obj['spec']['template']['spec']['containers'] if c['name'] == service)
            expected = service.upper() + '_R2_SIGNING_ENDPOINT'
            if not any(e.get('name') == expected and e.get('value') == p['s3_signing_endpoint'] for e in container.get('env', [])):
                _fail('canonical_signing_source_missing')
            builder.add('signing-endpoint', obj, action=True)
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
            for frontend, path, host in (('developer-portal', 'deploy/staging/developer-portal.yaml', 'developer_portal_host'),
                ('web', 'deploy/staging/flutter-web.yaml', 'web_host'), ('admin', 'deploy/staging/admin.yaml', 'admin_host')):
                if not (b.root / path).exists(): continue
                for obj in b.documents(path, 'frontends'):
                    if obj['kind'] == 'Deployment':
                        _pin_images(obj, b.p)
                        if b.p.get('image_pull_secret'):
                            obj['spec']['template']['spec']['imagePullSecrets'] = [{'name': b.p['image_pull_secret']}]
                    b.add('frontends', obj, action=True)
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
                'social', 'chat', 'messaging', 'file', 'space', 'role', 'notification', 'matchmaking', 'voice')}
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
        return plan
    except PlanError:
        raise
    except Exception:
        _fail('canonical_plan_invalid')


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


def verify_db_init(kube, plan):
    """Repeat fixed read-only catalog/auth checks, with no source execution."""
    if plan['mode'] != 'full':
        return {'skipped': True}
    try:
        secret = kube.get('Secret', 'voice-app-secrets')
        for pod, password, user, database, sql in (
            ('voice-postgres-0', 'POSTGRES_PASSWORD', 'POSTGRES_USER', 'postgres', PG_SQL),
            ('voice-postgres-0', 'POSTGRES_PASSWORD', 'POSTGRES_USER', 'gateway_db', GATEWAY_SQL)):
            script = 'printf "{\\"password_sha256\\":\\"%s\\",\\"catalog\\":" "$(printf %s \"$POSTGRES_PASSWORD\" | sha256sum | cut -d\" \" -f1)"; PGPASSWORD="$POSTGRES_PASSWORD" psql -h voice-postgres -U "$POSTGRES_USER" -d "$1" -Atqc "$2"; printf "}"'
            result = kube.run(['exec', pod, '--', 'sh', '-ceu', script, '--', database, sql])
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
        script = 'printf "{\\"password_sha256\\":\\"%s\\",\\"catalog\\":" "$(printf %s \"$CLICKHOUSE_PASSWORD\" | sha256sum | cut -d\" \" -f1)"; clickhouse-client --host voice-clickhouse --user default --password "$CLICKHOUSE_PASSWORD" --query "$1"; printf "}"'
        result = kube.run(['exec', 'voice-clickhouse-0', '--', 'sh', '-ceu', script, '--', CH_SQL])
        if result['password_sha256'] != hashlib.sha256(_value(secret, 'CLICKHOUSE_PASSWORD').encode()).hexdigest():
            _fail('unsupported_clickhouse_authority_sync')
        tables = {row['name']: row for row in result['catalog']['data']}
        expected = {'events': 'MergeTree', 'events_logical': 'View', 'dau_mv': 'AggregatingMergeTree',
            'dau_mv_mv': 'MaterializedView', 'events_by_type_mv': 'SummingMergeTree', 'events_by_type_mv_mv': 'MaterializedView'}
        if any(name not in tables or tables[name]['engine'] != engine for name, engine in expected.items()):
            _fail('canonical_clickhouse_schema_init_missing')
        row = next(row for row in plan['checks'] if row['id'] == 'db-init')
        ddl = row['evidence']['clickhouse_ddl']
        if set(ddl) != set(expected) or any(_normalized_ddl(tables[name]['create_table_query']) != ddl[name] for name in expected):
            _fail('canonical_clickhouse_ddl_incompatible')
        return {'verified': True}
    except PlanError:
        raise
    except Exception:
        _fail('canonical_db_init_readonly_proof_failed')
