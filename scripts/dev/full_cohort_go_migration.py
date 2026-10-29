"""Generic Go migration objects. Rendering alone grants no execution authority."""
import re
import copy
import base64
import json
import os
import ipaddress
import selectors
import subprocess
import time
from urllib.parse import parse_qsl, unquote, urlsplit

import auth_live_migration_phase as auth
import auth_user_candidate_probe
import cohort_backup_phases
import frontend_runtime_executor
import full_cohort_backup_metadata as metadata
import full_cohort_go_migration_plan as source_plan
import full_cohort_schema_metadata as schema
import media_operation_guard
import media_operation_journal
import realtime_migration_job
import resource_fencing
import story_media_backup_cohort_restore as restore
import writer_cut


class MigrationError(RuntimeError):
    pass


def _postgres_password():
    """Read only the fixed admin key; stdout remains private memory."""
    try:
        template = ('{{.metadata.uid}}{{"\\n"}}{{.metadata.resourceVersion}}{{"\\n"}}'
                    '{{if eq (index .data "POSTGRES_PASSWORD") nil}}M{{else}}P{{index .data "POSTGRES_PASSWORD"}}{{end}}')
        raw = schema.linux_private_bytes(['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
               '--namespace', 'voice-dev', '--request-timeout=15s', 'get', 'secret', 'voice-app-secrets',
               '-o', 'go-template=' + template])
        uid, rv, payload = raw.split(b'\n', 2)
        if not uid or not rv or payload[:1] != b'P':
            raise ValueError()
        return {'uid': uid.decode('ascii'), 'resource_version': rv.decode('ascii'),
                'value': base64.b64decode(payload[1:], validate=True).decode('utf-8')}
    except BaseException:
        raise MigrationError('migration credential unavailable') from None


def _credential(database, guard):
    """Resolve two guarded reads of exactly the Job's source-owned key(s)."""
    try:
        if database not in source_plan.DATABASES:
            raise ValueError()
        reader = schema.kube_key_reader(schema.linux_private_bytes)
        service = database.removesuffix('_db')
        def read():
            schema._fence(guard)
            if service == 'user':
                username = reader('configmap', 'voice-app-config', 'POSTGRES_USER')
                password = _postgres_password()
                if (not username or not password or not isinstance(username['value'], str) or
                        not schema.SAFE.fullmatch(username['value']) or not isinstance(password['value'], str) or
                        not re.fullmatch(r'[A-Za-z0-9._~-]{1,1024}', password['value'])):
                    raise ValueError()
                result = {'role': username['value'], 'sslmode': 'disable', 'host': 'voice-postgres', 'password': password['value'],
                          'references': {'user': {k:username[k] for k in ('uid', 'resource_version')},
                                         'password': {k:password[k] for k in ('uid', 'resource_version')}}}
            else:
                key = reader('secret', 'voice-app-secrets', service.upper() + '_DATABASE_URL')
                if not key:
                    raise ValueError()
                role, sslmode, password, host = _parse_url(database, key['value'])
                result = {'role': role, 'sslmode': sslmode, 'host': host, 'password': password,
                          'references': {'url': {k:key[k] for k in ('uid', 'resource_version')}}}
            schema._fence(guard)
            return result
        first = read()
        if read() != first:
            raise ValueError()
        return first
    except schema.GuardLost:
        raise
    except BaseException:
        raise MigrationError('migration credential unavailable') from None


_LOGIN_SCRIPT = ("IFS= read -r PGPASSWORD; export PGPASSWORD; export PGAPPNAME=voice-cohort-observe; "
                 "export PGOPTIONS='-c default_transaction_read_only=on -c statement_timeout=5000 -c lock_timeout=3000'; "
                 'export PGSSLMODE="$3"; exec psql -h "$5" -p 5432 -X -qAt -v ON_ERROR_STOP=1 -U "$1" -d "$2" -c "$4"')
_LOGIN_SQL = ("SELECT json_build_object('database',current_database(),'database_oid',"
              "(SELECT oid::bigint FROM pg_catalog.pg_database WHERE datname=current_database()),"
              "'role',current_user,'server_start',pg_catalog.pg_postmaster_start_time(),"
              "'read_only',current_setting('transaction_read_only'))")


def _login(database, credential, source, guard):
    """Authenticate exact credentials via stdin, bound to the observed engine."""
    process = selector = None
    try:
        fence = lambda: schema._fence(guard)
        snapshot = schema.kube_engine_snapshot(schema.linux_private_bytes, fence)
        fence()
        if snapshot() != source['engine']:
            raise ValueError()
        args = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default', '--namespace', 'voice-dev',
                '--request-timeout=15s', 'exec', '-i', 'voice-postgres-0', '-c', 'postgres', '--', 'sh', '-ec',
                _LOGIN_SCRIPT, '--', credential['role'], database, credential['sslmode'], _LOGIN_SQL, credential['host']]
        process = subprocess.Popen(args, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
        process.stdin.write(credential['password'].encode('utf-8') + b'\n'); process.stdin.close()
        selector = selectors.DefaultSelector(); selector.register(process.stdout, selectors.EVENT_READ)
        deadline = time.monotonic() + 20; chunks = []; size = 0
        while selector.get_map():
            remaining = deadline - time.monotonic()
            if remaining <= 0 or not selector.select(remaining):
                raise ValueError()
            chunk = os.read(process.stdout.fileno(), min(8192, 8193 - size))
            if not chunk:
                selector.unregister(process.stdout); break
            size += len(chunk)
            if size > 8192:
                raise ValueError()
            chunks.append(chunk)
        if process.wait(timeout=max(.01, deadline - time.monotonic())):
            raise ValueError()
        observed = json.loads(b''.join(chunks))
        expected = {'database': database, 'database_oid': source['database_oid'], 'role': credential['role'],
                    'server_start': source['server_start'], 'read_only': 'on'}
        fence()
        if observed != expected or snapshot() != source['engine']:
            raise ValueError()
        return {k:v for k,v in credential.items() if k != 'password'}
    except schema.GuardLost:
        raise
    except BaseException:
        raise MigrationError('migration login unavailable') from None
    finally:
        if selector is not None: selector.close()
        if process is not None:
            if process.poll() is None: process.kill(); process.wait()
            if process.stdout is not None: process.stdout.close()


def _parse_url(database, value):
    """Resolve privately; callers must never emit the returned password."""
    try:
        if database not in source_plan.DATABASES or not isinstance(value, str) or len(value.encode()) > 4096:
            raise ValueError()
        host, actual, role = schema._url(value, 'voice-dev')
        parsed = urlsplit(value)
        if (actual != database or not schema.SAFE.fullmatch(role) or parsed.password is None or
                parsed.username != role or parsed.netloc.count('@') != 1):
            raise ValueError()
        password = unquote(parsed.password, errors='strict')
        if not password or len(password.encode()) > 1024 or any(c in password for c in '\r\n\0'):
            raise ValueError()
        options = dict(parse_qsl(parsed.query, keep_blank_values=True, strict_parsing=True))
        if set(options) != {'sslmode'}:
            raise ValueError()
        return role, options['sslmode'], password, host
    except BaseException:
        raise MigrationError('migration credential route unavailable') from None


def qualified_plan(metadata_phase, cohort_phase, cut, token, database):
    """Check actual backup/readback/isolated-restore authority for one target."""
    try:
        if (type(metadata_phase) is not metadata.BackupMetadataPhase or
                type(cohort_phase) is not cohort_backup_phases.CohortPhases or
                type(cut) is not writer_cut.WriterCut or database not in source_plan.DATABASES):
            raise ValueError()
        p = cohort_phase.phases
        if (metadata_phase.fence.context != p.fence.context or cut.fence.context != p.fence.context or
                metadata_phase.lock != p.lock or cut.lock != p.lock or
                metadata_phase.namespace_uid != p.namespace_uid or cut.namespace_uid != p.namespace_uid or
                metadata_phase.fence.ledger.read() != p.fence.ledger.read() or cut.fence.ledger.read() != p.fence.ledger.read()):
            raise ValueError()
        cohort_phase._guard(); cut.verify_drained()
        observed = metadata.verify_completed(metadata_phase, token)
        plans = observed['metadata']['migrations']
        auth_version = plans['auth_db']['from']
        if auth_version not in (26, 32) or database not in plans:
            raise ValueError()
        binding, provenance = auth.verified_sources(cohort_phase, observed['metadata'], auth_version)
        plan = plans[database]
        if (binding['restore_inputs'] != observed['restore_inputs'] or binding['databases'] != observed['databases'] or
                binding['sources'] != observed['sources'] or binding['full_catalog'] != observed['full_catalog'] or
                restore.classify_plan(database, plan, True) != 'migrate'):
            raise ValueError()
        receipt = p.journal.state['steps']['cohort-restore']['receipt']
        if receipt['databases'][database] != {'mode': 'migrate', 'from': plan['from'], 'to': plan['to']}:
            raise ValueError()
        restore.migration_inventory(metadata_phase.source_root, binding['databases'], plans, full_catalog=True)
        cut.verify_drained(); cohort_phase._guard()
        if (metadata.verify_completed(metadata_phase, token) != observed or
                auth.verified_sources(cohort_phase, observed['metadata'], auth_version) != (binding, provenance)):
            raise ValueError()
        return {'database': database, 'plan': copy.deepcopy(plan), 'source': copy.deepcopy(binding['sources'][database]),
                'credential': copy.deepcopy(observed['migration_roles'][database]),
                'metadata_receipt_sha256': p.fence.ledger.read()[0][metadata.LEDGER]['receipt_sha256'],
                'restore_provenance': copy.deepcopy(provenance)}
    except BaseException:
        raise MigrationError('source-qualified migrated restore unavailable') from None


def _secret_name(operation, database):
    if (not isinstance(operation, str) or not re.fullmatch(r'media-c[12]-[a-f0-9]{32}', operation) or
            database not in source_plan.DATABASES):
        raise MigrationError('migration secret identity invalid')
    return 'voice-go-input-' + database.removesuffix('_db') + '-' + operation[-32:]


def _secret(operation, database, credential, service_ip):
    """Construct private operation-only credentials; never log or journal body."""
    try:
        name = _secret_name(operation, database)
        if (credential['sslmode'] != 'disable' or not schema.SAFE.fullmatch(credential['role']) or
                str(ipaddress.IPv4Address(service_ip)) != service_ip or
                not isinstance(credential['password'], str) or not credential['password']):
            raise ValueError()
        return {'apiVersion': 'v1', 'kind': 'Secret', 'immutable': True, 'type': 'Opaque',
                'metadata': {'name': name, 'namespace': 'voice-dev'},
                'data': {'PGPASSWORD': base64.b64encode(credential['password'].encode('utf-8')).decode('ascii')}}
    except BaseException:
        raise MigrationError('migration credential snapshot unavailable') from None


def _check_secret(intended, actual, *, require_identity=True):
    try:
        meta = actual['metadata']
        if (not isinstance(meta, dict) or set(meta) - {'name', 'namespace', 'uid', 'resourceVersion',
                                                       'creationTimestamp', 'managedFields', 'generation'} or
                actual.get('apiVersion') != 'v1' or actual.get('kind') != 'Secret' or actual.get('immutable') is not True or
                actual.get('type') != 'Opaque' or actual.get('data') != intended['data'] or
                meta.get('name') != intended['metadata']['name'] or meta.get('namespace') != 'voice-dev' or
                require_identity and any(not isinstance(meta.get(k), str) or not meta[k]
                                         for k in ('uid', 'resourceVersion'))):
            raise ValueError()
        return {k: meta[k] for k in ('name', 'uid', 'resourceVersion')} if require_identity else None
    except BaseException:
        raise MigrationError('migration credential snapshot changed') from None


def _private_kube(args, body=None):
    """Fixed kubectl transport; Secret bodies and diagnostics stay in memory."""
    process = selector = None
    try:
        raw = b'' if body is None else json.dumps(body, separators=(',', ':')).encode()
        if len(raw) > 4096:
            raise ValueError()
        process = subprocess.Popen(frontend_runtime_executor.KubectlTransport.PREFIX + list(args),
                                   stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
        if raw: process.stdin.write(raw)
        process.stdin.close()
        selector = selectors.DefaultSelector(); selector.register(process.stdout, selectors.EVENT_READ)
        deadline = time.monotonic() + 25; chunks = []; size = 0
        while selector.get_map():
            left = deadline - time.monotonic()
            if left <= 0 or not selector.select(left): raise ValueError()
            chunk = os.read(process.stdout.fileno(), min(8192, 65537 - size))
            if not chunk:
                selector.unregister(process.stdout); break
            size += len(chunk)
            if size > 65536: raise ValueError()
            chunks.append(chunk)
        if process.wait(timeout=max(.01, deadline - time.monotonic())): raise ValueError()
        return json.loads(b''.join(chunks))
    except BaseException:
        raise MigrationError('private Kubernetes request unavailable') from None
    finally:
        if selector is not None: selector.close()
        if process is not None:
            if process.poll() is None: process.kill(); process.wait()
            if process.stdout is not None: process.stdout.close()


class SecretSnapshot:
    """One create-only Secret snapshot; not a migration execution admission."""
    def __init__(self, journal, fence, client, lock, namespace_uid, metadata_phase, cohort_phase, cut, token, database):
        self.journal, self.fence, self.client = journal, fence, client
        self.lock, self.namespace_uid = copy.deepcopy(lock), namespace_uid
        self.metadata_phase, self.cohort, self.cut, self.token, self.database = metadata_phase, cohort_phase, cut, token, database
        self.intent = copy.deepcopy(journal.intent); self.poisoned = False
        self._bound = None
        self._dependencies = (journal, fence, client, metadata_phase, cohort_phase, cut)
        self._fixed = (token, database, copy.deepcopy(lock), namespace_uid, copy.deepcopy(fence.context))
        if (type(journal) is not media_operation_journal.Journal or type(fence) is not resource_fencing.Fence or
                type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal or
                fence.ledger.client is not client or self.intent.get('resource_context') != fence.context or
                self.intent.get('namespace_uid') != namespace_uid or database not in source_plan.DATABASES or
                not isinstance(token, str) or not token):
            raise MigrationError('migration credential phase identity invalid')

    def _guard(self):
        if (self.poisoned or self.journal.poisoned or self.journal.intent != self.intent or
                self.journal.state.get('intent') != self.intent or
                (self.token, self.database, self.lock, self.namespace_uid, self.fence.context) != self._fixed or
                any(a is not b for a, b in zip((self.journal, self.fence, self.client, self.metadata_phase,
                                                self.cohort, self.cut), self._dependencies))):
            raise MigrationError('migration credential phase changed')
        schema._fence(lambda: media_operation_guard.verify(self.fence, self.client, self.lock, self.namespace_uid))

    def _ledger(self):
        row, revision = self.fence.ledger.read()
        if (not self.fence.owned(row) or row.get('status') != 'active' or not isinstance(revision, tuple) or
                len(revision) != 2 or any(not isinstance(v, str) or not v for v in revision)):
            raise MigrationError('migration credential ledger changed')
        return row, revision

    def create(self):
        """Create once. Uncertain responses retain ownership for reconciliation."""
        pending = False
        try:
            self._guard(); proof = qualified_plan(self.metadata_phase, self.cohort, self.cut, self.token, self.database)
            source = proof['source']; secret = _credential(self.database, self._guard)
            safe = _login(self.database, secret, source, self._guard)
            if safe != proof['credential'] or _credential(self.database, self._guard) != secret:
                raise ValueError()
            intended = _secret(self.fence.context['operation_id'], self.database, secret, source['engine']['service_ip'])
            self._guard()
            dry = _private_kube(['create', '--dry-run=server', '-f', '-', '-o', 'json'], intended)
            _check_secret(intended, dry, require_identity=False)
            self.cut.verify_drained(); self._guard(); row, revision = self._ledger()
            local_key = 'go-migration-secret-' + self.database
            if local_key in self.journal.state['steps'] or self.database in row.get('go_migration_secrets', {}):
                raise ValueError()
            marker = {'status': 'pending', 'nonce': os.urandom(16).hex(), 'operation_id': self.fence.context['operation_id'],
                      'source_sha': self.fence.context['commit'], 'database': self.database,
                      'name': intended['metadata']['name'], 'metadata_receipt_sha256': proof['metadata_receipt_sha256'],
                      'credential': copy.deepcopy(safe)}
            pending = True; self.journal.state['steps'][local_key] = copy.deepcopy(marker); self.journal.save()
            self._guard(); current, rev = self._ledger()
            if current != row or rev != revision or self.journal.state['steps'].get(local_key) != marker:
                raise ValueError()
            current.setdefault('go_migration_secrets', {})[self.database] = copy.deepcopy(marker)
            current['step'] = local_key + '-pending'; self.fence.ledger.write(rev, current)
            self._guard()
            if self._ledger()[0].get('go_migration_secrets', {}).get(self.database) != marker or \
                    _credential(self.database, self._guard) != secret or \
                    qualified_plan(self.metadata_phase, self.cohort, self.cut, self.token, self.database) != proof:
                raise ValueError()
            self.cut.verify_drained()
            created = _private_kube(['create', '-f', '-', '-o', 'json'], intended)
            receipt = _check_secret(intended, created)
            self._guard()
            actual = _private_kube(['get', 'secret', intended['metadata']['name'], '-o', 'json'])
            if _check_secret(intended, actual) != receipt or _credential(self.database, self._guard) != secret:
                raise ValueError()
            if _login(self.database, secret, source, self._guard) != safe:
                raise ValueError()
            self.cut.verify_drained(); self._guard(); row, revision = self._ledger()
            if row.get('go_migration_secrets', {}).get(self.database) != marker or self.journal.state['steps'].get(local_key) != marker:
                raise ValueError()
            done = marker | {'status': 'done', 'receipt': receipt}
            self.journal.state['steps'][local_key] = copy.deepcopy(done); self.journal.save()
            row['go_migration_secrets'][self.database] = copy.deepcopy(done)
            row['step'] = local_key + '-done'; self.fence.ledger.write(revision, row)
            self._guard()
            if self._ledger()[0].get('go_migration_secrets', {}).get(self.database) != done or self.journal.state['steps'].get(local_key) != done:
                raise ValueError()
            self._bound = (copy.deepcopy(done), copy.deepcopy(secret), copy.deepcopy(source))
            return copy.deepcopy(receipt)
        except schema.GuardLost:
            if pending:
                self.poisoned = True
                try: self.fence.uncertain('Go migration secret outcome uncertain')
                except BaseException: pass
            raise
        except BaseException:
            if pending:
                self.poisoned = True
                try: self.fence.uncertain('Go migration secret outcome uncertain')
                except BaseException: pass
            raise MigrationError('migration credential snapshot uncertain' if pending else 'migration credential snapshot not admitted') from None

    def verify(self):
        """Recheck the same owned Secret and source refs; never adopt by name."""
        try:
            self._guard()
            if self._bound is None: raise ValueError()
            done, credential, source = self._bound
            local_key = 'go-migration-secret-' + self.database
            if (self.journal.state['steps'].get(local_key) != done or
                    self._ledger()[0].get('go_migration_secrets', {}).get(self.database) != done or
                    _credential(self.database, self._guard) != credential):
                raise ValueError()
            snapshot = schema.kube_engine_snapshot(schema.linux_private_bytes, self._guard)
            if snapshot() != source['engine']: raise ValueError()
            intended = _secret(self.fence.context['operation_id'], self.database, credential, source['engine']['service_ip'])
            actual = _private_kube(['get', 'secret', intended['metadata']['name'], '-o', 'json'])
            if _check_secret(intended, actual) != done['receipt'] or snapshot() != source['engine']:
                raise ValueError()
            self._guard()
            if (self.journal.state['steps'].get(local_key) != done or
                    self._ledger()[0].get('go_migration_secrets', {}).get(self.database) != done or
                    _credential(self.database, self._guard) != credential):
                raise ValueError()
            return copy.deepcopy(done['receipt'])
        except schema.GuardLost:
            raise
        except BaseException:
            raise MigrationError('migration credential snapshot unavailable') from None


def verify_writer_job(authored, actual):
    """Apply the existing exact Kubernetes Job admission checks."""
    try:
        return realtime_migration_job.verify_writer_job(authored, actual)
    except BaseException:
        raise MigrationError('migration Job identity unavailable') from None


def verify_writer_pod(authored, job, pod, execution):
    """Bind the Job-owned Pod, platform image and IP to the CUT observer."""
    try:
        accepted = verify_writer_job(authored, job)
        fields = {'database', 'database_oid', 'role', 'application_name', 'container_name',
                  'node_name', 'node_uid', 'platform_digest'}
        if (not isinstance(execution, dict) or set(execution) != fields or
                execution['database'] not in source_plan.DATABASES or
                type(execution['database_oid']) is not int or execution['database_oid'] <= 0 or
                not isinstance(execution['role'], str) or not schema.SAFE.fullmatch(execution['role']) or
                execution['application_name'] != authored['metadata']['name'] or execution['container_name'] != 'migrate' or
                not isinstance(execution['node_uid'], str) or not execution['node_uid'] or
                execution['platform_digest'] != realtime_migration_job.PLATFORMS['amd64']):
            raise ValueError()
        meta, spec, status = pod['metadata'], copy.deepcopy(pod['spec']), pod['status']
        owners = [o for o in meta.get('ownerReferences', []) if o.get('controller') is True]
        if (pod.get('apiVersion') != 'v1' or pod.get('kind') != 'Pod' or meta.get('namespace') != 'voice-dev' or
                any(not isinstance(meta.get(k), str) or not meta[k] for k in ('uid', 'resourceVersion')) or
                meta.get('deletionTimestamp') or len(owners) != 1 or owners[0].get('kind') != 'Job' or
                owners[0].get('uid') != job['metadata']['uid'] or spec.pop('nodeName', None) != execution['node_name'] or
                spec.get('hostNetwork', False) is not False):
            raise ValueError()
        name, uid = authored['metadata']['name'], job['metadata']['uid']
        labels = copy.deepcopy(meta.get('labels', {}))
        if labels != accepted['template']['metadata']['labels']: raise ValueError()
        generated = {'batch.kubernetes.io/controller-uid': uid, 'controller-uid': uid,
                     'batch.kubernetes.io/job-name': name, 'job-name': name}
        for key, value in generated.items():
            if key in labels and labels.pop(key) != value: raise ValueError()
        template = authored['spec']['template']
        if labels != template['metadata']['labels']: raise ValueError()
        intended = {'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'name': 'template', 'namespace': 'voice-dev',
                    'labels': template['metadata']['labels']}, 'spec': template['spec']}
        observed = copy.deepcopy(intended); observed['spec'] = spec
        auth_user_candidate_probe.admitted_pod(intended, observed)
        ip, containers = status.get('podIP'), status.get('containerStatuses', [])
        image = containers[0].get('imageID', '') if len(containers) == 1 else ''
        image_match = re.fullmatch(r'(?:docker-pullable://[^@]+@|containerd://|docker://)?(sha256:[0-9a-f]{64})', image)
        if (status.get('phase') not in ('Running', 'Succeeded') or not isinstance(ip, str) or
                str(ipaddress.ip_address(ip)) != ip or len(containers) != 1 or containers[0].get('name') != 'migrate' or
                not containers[0].get('containerID') or type(containers[0].get('restartCount')) is not int or
                containers[0]['restartCount'] != 0 or not image_match or image_match.group(1) != execution['platform_digest']):
            raise ValueError()
        return {'uid': meta['uid'], 'ip': ip}
    except BaseException:
        raise MigrationError('migration Pod identity unavailable') from None


def render(operation, source_sha, database, files, node, route):
    """Render closed fixed objects from inputs authenticated by the phase."""
    try:
        if (not isinstance(operation, str) or not re.fullmatch(r'media-c[12]-[a-f0-9]{32}', operation) or
                not isinstance(source_sha, str) or not re.fullmatch(r'[a-f0-9]{40}', source_sha) or
                database not in source_plan.DATABASES or not isinstance(node, dict) or
                set(node) != {'name', 'uid', 'hostname', 'architecture'} or node['architecture'] != 'amd64' or
                any(not isinstance(node[k], str) or not node[k] for k in ('name', 'uid', 'hostname')) or
                not isinstance(files, dict) or not files):
            raise ValueError()
        service = database.removesuffix('_db')
        if (not isinstance(route, dict) or set(route) != {'host', 'service_ip', 'role', 'sslmode'} or
                route['host'] not in ('voice-postgres', 'voice-postgres.voice-dev', 'voice-postgres.voice-dev.svc',
                                      'voice-postgres.voice-dev.svc.cluster.local') or
                not isinstance(route['service_ip'], str) or
                str(ipaddress.IPv4Address(route['service_ip'])) != route['service_ip'] or
                not isinstance(route['role'], str) or not schema.SAFE.fullmatch(route['role']) or
                route['sslmode'] != 'disable' or service == 'user' and route['host'] != 'voice-postgres'):
            raise ValueError()
        data = {}
        for name, raw in sorted(files.items()):
            if (not isinstance(name, str) or not re.fullmatch(r'[0-9]{6}_[a-z0-9_]+\.(up|down)\.sql', name) or
                    not isinstance(raw, bytes) or not 0 < len(raw) <= 4 * 1024 * 1024):
                raise ValueError()
            data[name] = raw.decode('utf-8')
        if sum(len(raw) for raw in files.values()) > 900000:
            raise ValueError()
        suffix = operation[-32:]
        name = 'voice-go-migrate-' + service + '-' + suffix
        config_name = 'voice-go-migrations-' + service + '-' + suffix
        annotation = {'voice.io/source-sha': source_sha}
        config = {'apiVersion': 'v1', 'kind': 'ConfigMap', 'immutable': True,
                  'metadata': {'name': config_name, 'namespace': 'voice-dev', 'annotations': annotation}, 'data': data}
        url = 'postgres://' + route['role'] + '@' + route['service_ip'] + ':5432/' + database + '?sslmode=disable'
        env = [{'name': 'PGPASSWORD', 'valueFrom': {'secretKeyRef': {'name': _secret_name(operation, database), 'key': 'PGPASSWORD'}}},
               {'name': 'DATABASE_URL', 'value': url},
               {'name': 'PGAPPNAME', 'value': name}, {'name': 'VOICE_DB_SERVICE_IP', 'value': route['service_ip']},
               {'name': 'VOICE_DB_ROLE', 'value': route['role']}, {'name': 'VOICE_DATABASE', 'value': database}]
        command = ('[ "$DATABASE_URL" = "postgres://$VOICE_DB_ROLE@$VOICE_DB_SERVICE_IP:5432/$VOICE_DATABASE?sslmode=disable" ] || exit 1; '
                   'exec migrate -path=/migrations -database "$DATABASE_URL" up')
        pod = {'restartPolicy': 'Never', 'automountServiceAccountToken': False,
               'nodeSelector': {'kubernetes.io/hostname': node['hostname']},
               'containers': [{'name': 'migrate', 'image': realtime_migration_job.MIGRATE_IMAGE,
                               'imagePullPolicy': 'IfNotPresent', 'command': ['/bin/sh', '-ec', command], 'env': env,
                               'volumeMounts': [{'name': 'migrations', 'mountPath': '/migrations', 'readOnly': True}],
                               'resources': {'requests': {'cpu': '20m', 'memory': '64Mi'},
                                             'limits': {'cpu': '500m', 'memory': '512Mi'}}}],
               'volumes': [{'name': 'migrations', 'configMap': {'name': config_name, 'optional': False, 'defaultMode': 420}}]}
        job = {'apiVersion': 'batch/v1', 'kind': 'Job',
               'metadata': {'name': name, 'namespace': 'voice-dev', 'annotations': annotation},
               'spec': {'backoffLimit': 0, 'activeDeadlineSeconds': 600,
                        'template': {'metadata': {'labels': {'app': name}}, 'spec': pod}}}
        return config, job
    except (UnicodeError, TypeError, KeyError, ValueError):
        raise MigrationError('generic migration render refused') from None
