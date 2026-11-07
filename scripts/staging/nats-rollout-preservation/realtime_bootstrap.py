"""Create-only staging Realtime bootstrap, under the root prepare transaction."""
import base64
import copy
import hashlib
import json
import os
import re
import secrets
import stat
from pathlib import Path
import commands
from root_main import save
from controller import Blocked


def validate_plan(plan, session):
    import resource_claim
    fields = {'schema', 'operation', 'context', 'helper_sha256', 'source_sha',
              'source_files_sha256', 'source_authority_sha256', 'execution_sha256', 'stage_sha256',
              'credential_sha256', 'actual_images', 'namespace_uid', 'postgres',
              'secret', 'active_marker_sha256'}
    parented=isinstance(plan,dict) and plan.get('schema') in ('voice-staging-realtime-bootstrap-v2','voice-staging-realtime-bootstrap-v4')
    if parented:fields.add('prerequisites_sha256')
    def nonempty(value):
        return isinstance(value, str) and bool(value)
    def meta(row, fields):
        return type(row) is dict and set(row) == fields and all(nonempty(v) for v in row.values())
    try:
        resource_claim.record_fields(plan,'realtime_plan',fields,installation=getattr(session,'installation',None))
        if (type(plan) is not dict
            or parented and (not isinstance(plan.get('prerequisites_sha256'),str)
                or not re.fullmatch(r'[a-f0-9]{64}',plan['prerequisites_sha256']))
            or plan['operation'] != session.operation or plan['context'] != session.context
            or plan['helper_sha256'] != session.helper
            or not re.fullmatch(r'[a-f0-9]{12}', plan['operation'])
            or not re.fullmatch(r'[a-f0-9]{40}', plan['source_sha'])
            or plan['source_sha'] != session.context['commit']
            or any(not isinstance(plan[k], str) or not re.fullmatch(r'[a-f0-9]{64}', plan[k])
                   for k in ('helper_sha256', 'source_files_sha256', 'source_authority_sha256',
                             'execution_sha256', 'stage_sha256', 'credential_sha256', 'active_marker_sha256'))
            or type(plan['actual_images']) is not dict or not plan['actual_images']
            or not all(isinstance(k, str) and re.fullmatch(r'voice-[a-z0-9-]+/[a-z0-9-]+', k)
                       and isinstance(v, str) and re.fullmatch(r'[^\s@]+@sha256:[a-f0-9]{64}', v)
                       for k, v in plan['actual_images'].items())
            or not nonempty(plan['namespace_uid'])):
            raise ValueError()
        pg = plan['postgres']; secret = plan['secret']
        if (type(pg) is not dict or set(pg) != {'statefulset', 'pod', 'pvc', 'pv', 'service', 'endpoints', 'slice', 'system_identifier'}
            or not meta(pg['statefulset'], {'uid', 'resourceVersion'})
            or type(pg['pod']) is not dict or set(pg['pod']) != {'uid', 'resourceVersion', 'container_id', 'restart_count'}
            or not all(nonempty(pg['pod'][k]) for k in ('uid', 'resourceVersion', 'container_id'))
            or type(pg['pod']['restart_count']) is not int or pg['pod']['restart_count'] < 0
            or not meta(pg['pvc'], {'name', 'uid', 'resourceVersion'})
            or not meta(pg['pv'], {'name', 'uid', 'resourceVersion'})
            or not meta(pg['service'], {'uid', 'resourceVersion', 'cluster_ip'})
            or not meta(pg['endpoints'], {'uid', 'resourceVersion', 'pod_ip'})
            or not meta(pg['slice'], {'uid', 'resourceVersion'})
            or not isinstance(pg['system_identifier'], str) or not re.fullmatch(r'[0-9]{1,20}', pg['system_identifier'])
            or not meta(secret, {'uid', 'resourceVersion', 'data_sha256'})
            or not re.fullmatch(r'[a-f0-9]{64}', secret['data_sha256'])):
            raise ValueError()
    except (KeyError, TypeError, ValueError, AttributeError, RuntimeError):
        raise Blocked('realtime_bootstrap_plan_invalid') from None


def require_absent(catalog):
    if catalog != {'role': None, 'database': None}:
        raise Blocked('realtime_bootstrap_existing_role_or_database')


def valid_role(role):
    fields = {'oid', 'login', 'superuser', 'createdb', 'createrole', 'inherit',
              'replication', 'bypassrls', 'memberships', 'connection_limit',
              'valid_until', 'config'}
    return (isinstance(role, dict) and set(role) == fields
            and type(role['oid']) is int and role['oid'] > 0 and role['login'] is True
            and all(role[k] is False for k in ('superuser', 'createdb', 'createrole',
                                                'inherit', 'replication', 'bypassrls'))
            and type(role['memberships']) is int and role['memberships'] == 0
            and type(role['connection_limit']) is int and role['connection_limit'] == -1
            and role['valid_until'] is None and role['config'] is None)


def secret_patch(secret, password):
    try:
        metadata = secret['metadata']; data = secret['data']
        if (metadata['name'] != 'voice-app-secrets' or metadata['namespace'] != 'voice-staging' or metadata.get('deletionTimestamp')
            or not metadata['uid'] or not metadata['resourceVersion'] or secret.get('type') != 'Opaque'
            or secret.get('immutable') is True or type(data) is not dict
            or 'REALTIME_DATABASE_URL' in data or not isinstance(password, bytes)
            or not re.fullmatch(rb'[A-Za-z0-9_-]{32,128}', password)):
            raise ValueError()
        dsn = b'postgres://realtime:' + password + b'@voice-postgres:5432/realtime_db?sslmode=disable'
        return [{'op': 'test', 'path': '/metadata/uid', 'value': metadata['uid']},
                {'op': 'test', 'path': '/metadata/resourceVersion', 'value': metadata['resourceVersion']},
                {'op': 'test', 'path': '/data', 'value': copy.deepcopy(data)},
                {'op': 'add', 'path': '/data/REALTIME_DATABASE_URL',
                 'value': base64.b64encode(dsn).decode('ascii')}]
    except (KeyError, TypeError, ValueError):
        raise Blocked('realtime_bootstrap_secret_invalid') from None


def engine_snapshot(kube):
    try:
        namespace = kube.get('namespace', 'voice-staging')['metadata']
        sts = kube.get('statefulset', 'voice-postgres')['metadata']
        pod = kube.get('pod', 'voice-postgres-0'); pm = pod['metadata']
        owners = [o for o in pm.get('ownerReferences', []) if o.get('controller') is True]
        containers = [c for c in pod['spec']['containers'] if c.get('name') == 'postgres']
        statuses = [c for c in pod['status']['containerStatuses'] if c.get('name') == 'postgres']
        if (namespace['name'] != 'voice-staging' or namespace.get('deletionTimestamp') or sts.get('deletionTimestamp')
            or sts['name'] != 'voice-postgres'
            or sts['namespace'] != 'voice-staging' or pm['name'] != 'voice-postgres-0'
            or pm['namespace'] != 'voice-staging' or pm.get('deletionTimestamp')
            or len(owners) != 1 or owners[0].get('kind') != 'StatefulSet' or owners[0].get('uid') != sts['uid']
            or len(containers) != 1 or len(statuses) != 1 or 'running' not in statuses[0].get('state', {})):
            raise ValueError()
        mounts = [m for m in containers[0].get('volumeMounts', []) if m.get('mountPath') == '/var/lib/postgresql/data']
        if len(mounts) != 1: raise ValueError()
        volumes = [v for v in pod['spec']['volumes'] if v.get('name') == mounts[0]['name']]
        if len(volumes) != 1 or volumes[0].get('persistentVolumeClaim', {}).get('claimName') != 'pgdata-voice-postgres-0':
            raise ValueError()
        pvc = kube.get('persistentvolumeclaim', 'pgdata-voice-postgres-0')
        volume_name = pvc['spec']['volumeName']; pv = kube.get('persistentvolume', volume_name)
        claim = pv['spec']['claimRef']; cm = pvc['metadata']; vm = pv['metadata']
        if (cm.get('deletionTimestamp') or vm.get('deletionTimestamp')
            or cm['name'] != 'pgdata-voice-postgres-0' or cm['namespace'] != 'voice-staging'
            or pvc.get('status', {}).get('phase') != 'Bound' or pv.get('status', {}).get('phase') != 'Bound'
            or vm['name'] != volume_name or claim.get('name') != cm['name']
            or claim.get('namespace') != 'voice-staging' or claim.get('uid') != cm['uid']):
            raise ValueError()
        service = kube.get('service', 'voice-postgres'); service_spec = service['spec']
        selector = service_spec['selector']; labels = pm.get('labels', {})
        ports = [p for p in service_spec.get('ports', []) if p.get('port') == 5432 and p.get('protocol', 'TCP') == 'TCP']
        if (service['metadata'].get('deletionTimestamp') or service_spec.get('type', 'ClusterIP') != 'ClusterIP' or not service_spec.get('clusterIP')
            or type(selector) is not dict or not selector or any(labels.get(k) != v for k, v in selector.items())
            or len(ports) != 1 or ports[0].get('targetPort', 5432) != 5432):
            raise ValueError()
        port_name = ports[0].get('name') or ''; ip = pod['status']['podIP']
        endpoints = kube.get('endpoints', 'voice-postgres'); subsets = endpoints.get('subsets', [])
        if (endpoints['metadata'].get('deletionTimestamp') or len(subsets) != 1 or len(subsets[0].get('addresses', [])) != 1 or subsets[0].get('notReadyAddresses')
            or not any(p.get('port') == 5432 and p.get('protocol', 'TCP') == 'TCP' and (p.get('name') or '') == port_name
                       for p in subsets[0].get('ports', []))): raise ValueError()
        address = subsets[0]['addresses'][0]
        if (address.get('ip') != ip or address.get('targetRef', {}).get('kind') != 'Pod'
            or address.get('targetRef', {}).get('uid') != pm['uid']): raise ValueError()
        slices = kube.run(['get', 'endpointslices', '-l', 'kubernetes.io/service-name=voice-postgres', '-o', 'json'])['items']
        if len(slices) != 1: raise ValueError()
        sl = slices[0]
        if (sl['metadata'].get('deletionTimestamp') or len(sl.get('endpoints', [])) != 1 or not any(p.get('port') == 5432 and p.get('protocol', 'TCP') == 'TCP'
              and (p.get('name') or '') == port_name for p in sl.get('ports', []))): raise ValueError()
        endpoint = sl['endpoints'][0]
        if (endpoint.get('addresses') != [ip] or endpoint.get('conditions', {}).get('ready') is not True
            or endpoint.get('targetRef', {}).get('kind') != 'Pod' or endpoint.get('targetRef', {}).get('uid') != pm['uid']):
            raise ValueError()
        result = {'statefulset': {k: sts[k] for k in ('uid', 'resourceVersion')},
                  'pod': {'uid': pm['uid'], 'resourceVersion': pm['resourceVersion'],
                          'container_id': statuses[0]['containerID'], 'restart_count': statuses[0]['restartCount']},
                  'pvc': {k: cm[k] for k in ('name', 'uid', 'resourceVersion')},
                  'pv': {k: vm[k] for k in ('name', 'uid', 'resourceVersion')},
                  'service': {**{k: service['metadata'][k] for k in ('uid', 'resourceVersion')}, 'cluster_ip': service_spec['clusterIP']},
                  'endpoints': {**{k: endpoints['metadata'][k] for k in ('uid', 'resourceVersion')}, 'pod_ip': ip},
                  'slice': {k: sl['metadata'][k] for k in ('uid', 'resourceVersion')}}
        return namespace['uid'], result
    except (KeyError, TypeError, ValueError):
        raise Blocked('realtime_bootstrap_engine_invalid') from None


def require_no_writer(deployment, pods):
    """Reject publication if it could make a currently pending writer start."""
    try:
        meta = deployment['metadata']; spec = deployment['spec']; template = spec['template']['spec']
        if (meta['name'] != 'voice-realtime' or meta['namespace'] != 'voice-staging'
            or not isinstance(pods, list)):
            raise ValueError()
        def exposed(pod_spec):
            found = False
            for container in pod_spec['containers'] + pod_spec.get('initContainers', []):
                for row in container.get('env', []):
                    ref = row.get('valueFrom', {}).get('secretKeyRef', {})
                    found |= ref.get('name') == 'voice-app-secrets' and ref.get('key') == 'REALTIME_DATABASE_URL'
                for row in container.get('envFrom', []):
                    found |= row.get('secretRef', {}).get('name') == 'voice-app-secrets'
            for volume in pod_spec.get('volumes', []):
                found |= volume.get('secret', {}).get('secretName') == 'voice-app-secrets'
                for source in volume.get('projected', {}).get('sources', []):
                    found |= source.get('secret', {}).get('name') == 'voice-app-secrets'
            return found
        if ((exposed(template) and (spec.get('replicas', 1) != 0 or pods))
            or any(exposed(pod['spec']) for pod in pods)):
            raise ValueError()
    except (KeyError, TypeError, ValueError):
        raise Blocked('realtime_bootstrap_existing_writer_possible') from None


_PSQL = ('export PGPASSWORD="$POSTGRES_PASSWORD"; export PGAPPNAME=voice-realtime-bootstrap; '
         'export PGOPTIONS="-c statement_timeout=15000 -c lock_timeout=5000"; '
         'exec psql -h 127.0.0.1 -X -qAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d postgres -f -')
_COMMAND = ['/usr/local/bin/k3s', 'kubectl', '--namespace', 'voice-staging', 'exec', '-i',
            'voice-postgres-0', '-c', 'postgres', '--', 'sh', '-ceu', _PSQL]
_ENV = {'PATH': '/usr/local/bin:/usr/bin:/bin', 'HOME': '/root'}
_ADMIN_IDENTITY = ('printf "%s\\n" "$POSTGRES_USER"; '
                   'printf %s "$POSTGRES_PASSWORD" | sha256sum | cut -d" " -f1')
_CATALOG = b"""SELECT json_build_object(
 'role', (SELECT json_build_object('oid', r.oid::bigint, 'login', r.rolcanlogin,
   'superuser', r.rolsuper, 'createdb', r.rolcreatedb, 'createrole', r.rolcreaterole,
   'inherit', r.rolinherit, 'replication', r.rolreplication, 'bypassrls', r.rolbypassrls,
   'connection_limit', r.rolconnlimit, 'valid_until', r.rolvaliduntil, 'config', r.rolconfig,
   'memberships', (SELECT count(*) FROM pg_auth_members m WHERE m.member=r.oid OR m.roleid=r.oid))
   FROM pg_roles r WHERE r.rolname='realtime'),
 'database', (SELECT json_build_object('oid', d.oid::bigint, 'owner_oid', d.datdba::bigint)
   FROM pg_database d WHERE d.datname='realtime_db'));
"""


def _capture_readonly(kind, credential=None):
    """Issue only the fixed read-only SQL/identity observations."""
    if kind=='catalog' and credential is None:
        command=_COMMAND;body=_CATALOG;limit=65536
    elif kind=='system_identifier' and credential is None:
        command=_COMMAND;body=b'SELECT system_identifier FROM pg_control_system();\n';limit=256
    elif kind=='admin_identity' and credential is None:
        command=['/usr/local/bin/k3s','kubectl','--namespace','voice-staging','exec',
            'voice-postgres-0','-c','postgres','--','sh','-ceu',_ADMIN_IDENTITY];body=b'';limit=256
    elif kind=='runtime_auth' and isinstance(credential,bytes) and re.fullmatch(rb'[A-Za-z0-9_-]{32,128}',credential):
        script=('IFS= read -r PGPASSWORD; export PGPASSWORD; '
            'export PGOPTIONS="-c statement_timeout=15000 -c lock_timeout=5000"; '
            'exec psql -h voice-postgres -X -qAt -v ON_ERROR_STOP=1 -U realtime -d realtime_db -f -')
        command=['/usr/local/bin/k3s','kubectl','--namespace','voice-staging','exec','-i',
            'voice-postgres-0','-c','postgres','--','sh','-ceu',script]
        body=credential+b'\nSELECT current_user,current_database();\n';limit=256
    else:raise Blocked('realtime_bootstrap_readonly_kind_invalid')
    import resource_claim
    session=getattr(commands.authority_guard,'__self__',None)
    if isinstance(session,resource_claim.Session) and session.check().get('phase')=='prerequisites-captured':
        session=resource_claim.require_session(session,installation=session.installation)
        import staging_prerequisites
        if session._prerequisite_readonly_busy or session._prerequisite_readonly_permit is not None:
            raise Blocked('realtime_bootstrap_readonly_permit_busy')
        session._prerequisite_readonly_busy=True
        try:
            captured=staging_prerequisites.load_capture(session)
            if session.check().get('target_sha256')!=resource_claim.digest(captured):raise Blocked('realtime_bootstrap_readonly_capture_changed')
            session._prerequisite_readonly_capture=copy.deepcopy(captured)
            session._prerequisite_readonly_permit=commands.launch_snapshot(command,body,25,limit,_ENV,())
            result=commands.capture(command,body=body,timeout=25,limit=limit,env=_ENV)
            if session._prerequisite_readonly_permit is not None:raise Blocked('realtime_bootstrap_readonly_permit_unconsumed')
            session.readonly_current()
            return result
        finally:
            session._prerequisite_readonly_permit=None
            session._prerequisite_readonly_capture=None
            session._prerequisite_readonly_busy=False
    return commands.capture(command,body=body,timeout=25,limit=limit,env=_ENV)


def _catalog():
    try:
        return json.loads(_capture_readonly('catalog'))
    except (ValueError, TypeError):
        raise Blocked('realtime_bootstrap_catalog_invalid') from None


def _verify_admin(secret):
    try:
        password = base64.b64decode(secret['data']['POSTGRES_PASSWORD'], validate=True)
        result = _capture_readonly('admin_identity').splitlines()
        if result != [b'voice', hashlib.sha256(password).hexdigest().encode()]: raise ValueError()
    except (KeyError, ValueError, TypeError):
        raise Blocked('realtime_bootstrap_admin_identity_invalid') from None


def _system_identifier():
    try:
        value = _capture_readonly('system_identifier').decode('ascii').strip()
        if not re.fullmatch(r'[0-9]{1,20}', value): raise ValueError()
        return value
    except (UnicodeError, ValueError):
        raise Blocked('realtime_bootstrap_pg_identity_invalid') from None


def _sql_identity(system_identifier):
    if not isinstance(system_identifier, str) or not re.fullmatch(r'[0-9]{1,20}', system_identifier):
        raise Blocked('realtime_bootstrap_pg_identity_invalid')
    return system_identifier.encode('ascii')


def _create_role(password, system_identifier):
    if not isinstance(password, bytes) or not re.fullmatch(rb'[A-Za-z0-9_-]{32,128}', password):
        raise Blocked('realtime_bootstrap_credential_invalid')
    ident = _sql_identity(system_identifier)
    sql = (b"SET log_statement = 'none';\nSET log_min_error_statement = 'panic';\n"
           b"SET password_encryption = 'scram-sha-256';\n"
           b"SELECT pg_advisory_lock(hashtextextended('voice-staging-realtime-bootstrap',0));\n"
           b"DO $$ BEGIN IF (SELECT system_identifier::text FROM pg_control_system()) <> '" + ident +
           b"' OR EXISTS (SELECT 1 FROM pg_roles WHERE rolname='realtime') "
           b"OR EXISTS (SELECT 1 FROM pg_database WHERE datname='realtime_db') "
           b"THEN RAISE EXCEPTION 'bootstrap predecessor mismatch'; END IF; END $$;\n"
           b"CREATE ROLE realtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT "
           b"NOREPLICATION NOBYPASSRLS PASSWORD '" + password + b"';\n")
    commands.capture(_COMMAND, body=sql, timeout=25, limit=65536, env=_ENV)


def _create_database(system_identifier, role_oid):
    ident = _sql_identity(system_identifier)
    if type(role_oid) is not int or role_oid <= 0: raise Blocked('realtime_bootstrap_role_invalid')
    oid = str(role_oid).encode('ascii')
    sql = (b"SELECT pg_advisory_lock(hashtextextended('voice-staging-realtime-bootstrap',0));\n"
           b"DO $$ BEGIN IF (SELECT system_identifier::text FROM pg_control_system()) <> '" + ident +
           b"' OR EXISTS (SELECT 1 FROM pg_database WHERE datname='realtime_db') OR NOT EXISTS "
           b"(SELECT 1 FROM pg_roles r WHERE r.rolname='realtime' AND r.oid=" + oid +
           b" AND r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolcreatedb AND NOT r.rolcreaterole "
           b"AND NOT r.rolinherit AND NOT r.rolreplication AND NOT r.rolbypassrls "
           b"AND r.rolconnlimit=-1 AND r.rolvaliduntil IS NULL AND r.rolconfig IS NULL "
           b"AND NOT EXISTS (SELECT 1 FROM pg_auth_members m WHERE m.member=r.oid OR m.roleid=r.oid)) "
           b"THEN RAISE EXCEPTION 'bootstrap predecessor mismatch'; END IF; END $$;\n"
           b'CREATE DATABASE realtime_db OWNER realtime;\n')
    commands.capture(_COMMAND, body=sql,
                     timeout=25, limit=65536, env=_ENV)


def _verify_runtime_auth(password):
    if not isinstance(password, bytes) or not re.fullmatch(rb'[A-Za-z0-9_-]{32,128}', password):
        raise Blocked('realtime_bootstrap_credential_invalid')
    result = _capture_readonly('runtime_auth',password)
    if result.strip() != b'realtime|realtime_db':
        raise Blocked('realtime_bootstrap_runtime_auth_invalid')


def _create_custody(installed, operation, password, before_secret):
    if (os.geteuid() != 0 or not re.fullmatch(r'[a-f0-9]{12}', operation)
        or not isinstance(password, bytes) or not re.fullmatch(rb'[A-Za-z0-9_-]{32,128}', password)):
        raise Blocked('realtime_bootstrap_custody_invalid')
    installed = Path(installed)
    info = installed.lstat()
    if not stat.S_ISDIR(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o022:
        raise Blocked('realtime_bootstrap_custody_invalid')
    parent = installed / 'realtime-bootstrap'
    try:
        parent.mkdir(mode=0o700)
        fd = os.open(installed, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try: os.fsync(fd)
        finally: os.close(fd)
    except FileExistsError:
        info = parent.lstat()
        if not stat.S_ISDIR(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o077:
            raise Blocked('realtime_bootstrap_custody_invalid') from None
    base = parent / operation
    base.mkdir(mode=0o700)
    fd = os.open(parent, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try: os.fsync(fd)
    finally: os.close(fd)
    fd = os.open(base / 'credential', os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        with os.fdopen(fd, 'wb') as output:
            output.write(password); output.flush(); os.fsync(output.fileno())
    except BaseException:
        raise Blocked('realtime_bootstrap_custody_incomplete') from None
    save(base / 'before-secret.json', before_secret)
    return base


def _read_private(path, limit=2 << 20):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        info = os.fstat(fd)
        if (not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o077
            or info.st_nlink != 1 or not 1 <= info.st_size <= limit):
            raise Blocked('realtime_bootstrap_custody_invalid')
        value = os.read(fd, info.st_size + 1)
        if len(value) != info.st_size: raise Blocked('realtime_bootstrap_custody_changed')
        return value
    finally:
        os.close(fd)


def _secret_identity(secret):
    m = secret['metadata']
    if (m.get('name') != 'voice-app-secrets' or m.get('namespace') != 'voice-staging' or m.get('deletionTimestamp')
        or not m.get('uid') or not m.get('resourceVersion') or secret.get('type') != 'Opaque'
        or type(secret.get('data')) is not dict):
        raise Blocked('realtime_bootstrap_secret_invalid')
    return {'uid': m['uid'], 'resourceVersion': m['resourceVersion'], 'data_sha256': digest(secret['data'])}


def _stage_view(stage):
    rows = {}
    for name, row in stage.snapshots.items():
        meta = row['metadata']
        if meta.get('deletionTimestamp') or type(meta.get('generation')) is not int or meta['generation'] < 1:
            raise Blocked('realtime_bootstrap_workload_changed')
        rows[name] = {'uid': meta['uid'], 'generation': meta['generation'], 'spec': copy.deepcopy(row['spec'])}
    if stage.marker.get('metadata', {}).get('deletionTimestamp'):
        raise Blocked('realtime_bootstrap_marker_changed')
    return {'snapshots': rows, 'marker': copy.deepcopy(stage.marker)}


def _check_stage(kube, view):
    if kube.get('configmap', 'voice-nats-generation') != view['marker']:
        raise Blocked('realtime_bootstrap_marker_changed')
    for name, expected in view['snapshots'].items():
        current = kube.get('deployment', name)
        if (current['metadata']['uid'] != expected['uid'] or current['metadata'].get('deletionTimestamp')
            or current['metadata'].get('generation') != expected['generation'] or current['spec'] != expected['spec']):
            raise Blocked('realtime_bootstrap_workload_changed')


def _source_contract(source_root, approved):
    import compiler
    from root_main import public_read
    names = ('deploy/staging/services.yaml', 'deploy/templates/migrate-realtime-db-job.yaml',
             'src/backend/migrations/realtime_db/000001_admission.up.sql')
    contents = {}
    for name in names:
        raw = public_read(Path(source_root) / name, limit=4 << 20)
        if hashlib.sha256(raw).hexdigest() != approved['source_files'].get(name):
            raise Blocked('realtime_bootstrap_source_changed')
        contents[name] = raw
    text = contents[names[0]].decode('utf-8')
    matches = [piece for piece in re.split(r'^---\s*$', text, flags=re.M)
               if re.search(r'^kind: Deployment\s*$', piece, flags=re.M)
               and re.search(r'^  name: voice-realtime\s*$', piece, flags=re.M)]
    if len(matches) != 1: raise Blocked('realtime_bootstrap_source_invalid')
    deployment = compiler.decode_yaml(matches[0].encode())
    job = compiler.decode_yaml(contents[names[1]])
    def refs(obj):
        containers = obj['spec']['template']['spec']['containers']
        return [e for c in containers for e in c.get('env', []) if e.get('name') == 'DATABASE_URL']
    expected = {'name': 'DATABASE_URL', 'valueFrom': {'secretKeyRef': {'name': 'voice-app-secrets', 'key': 'REALTIME_DATABASE_URL'}}}
    if (deployment.get('kind') != 'Deployment' or deployment.get('metadata', {}).get('name') != 'voice-realtime'
        or job.get('kind') != 'Job' or job.get('metadata', {}).get('name') != 'voice-migrate-realtime-db'
        or refs(deployment) != [expected] or refs(job) != [expected]):
        raise Blocked('realtime_bootstrap_source_invalid')


def _check_live(session, kube, plan, stage_view):
    import guard
    import resource_claim
    session.check()
    path = guard.ROOT / 'installed' / 'realtime-bootstrap' / session.operation / 'authority.json'
    resource_claim.trusted_json(path,limit=2<<20)
    authority = json.loads(_read_private(path))
    if (type(authority) is not dict or set(authority) != {'approved', 'execution'}
        or digest(authority['approved']) != plan['source_authority_sha256']
        or digest(authority['approved']['source_files']) != plan['source_files_sha256']
        or digest(authority['execution']) != plan['execution_sha256']):
        raise Blocked('realtime_bootstrap_source_authority_changed')
    _verify_source_capture(session, authority['approved'], authority['execution'])
    _source_contract(guard.ROOT / 'installed' / 'sources' / session.operation, authority['approved'])
    namespace, pg = engine_snapshot(kube)
    pg['system_identifier'] = _system_identifier()
    if namespace != plan['namespace_uid'] or pg != plan['postgres']:
        raise Blocked('realtime_bootstrap_engine_changed')
    _check_stage(kube, stage_view)
    session.check()


def _verify_source_capture(session, approved, execution):
    import guard
    import resource_claim
    session.check()
    path = guard.ROOT / 'installed' / 'source-authority' / (session.operation + '.json')
    captured = resource_claim.trusted_json(path)
    expected = {'schema': 'voice-staging-source-capture-v2', 'installation_sha256': session.installation.sha256,
                'operation': session.operation,
                'context': session.context, 'helper_sha256': session.helper,
                'approved_sha256': digest(approved), 'execution_sha256': digest(execution)}
    try:resource_claim.validate_source_capture(captured,installation=session.installation)
    except (ValueError,RuntimeError,TypeError,AttributeError):
        raise Blocked('realtime_bootstrap_source_authority_changed') from None
    if captured != expected:
        raise Blocked('realtime_bootstrap_source_authority_changed')
    session.check()


def _verify_secret_after(before, after, password):
    if before['metadata'].get('resourceVersion') == after['metadata'].get('resourceVersion'):
        raise Blocked('realtime_bootstrap_secret_readback_invalid')
    expected = copy.deepcopy(before)
    patch = secret_patch(before, password)
    expected['data']['REALTIME_DATABASE_URL'] = patch[-1]['value']
    for key in ('resourceVersion', 'managedFields'):
        expected['metadata'].pop(key, None)
    candidate = copy.deepcopy(after)
    for key in ('resourceVersion', 'managedFields'):
        candidate['metadata'].pop(key, None)
    if candidate != expected:
        raise Blocked('realtime_bootstrap_secret_readback_invalid')
    return _secret_identity(after)


def _writer_check(kube, stage_view):
    deployment = kube.get('deployment', 'voice-realtime')
    captured = stage_view['snapshots']['voice-realtime']
    if deployment['metadata']['uid'] != captured['uid'] or deployment['spec'] != captured['spec']:
        raise Blocked('realtime_bootstrap_workload_changed')
    selector = deployment['spec']['selector']
    if selector.get('matchExpressions') or not selector.get('matchLabels'):
        raise Blocked('realtime_bootstrap_writer_selector_invalid')
    labels = selector['matchLabels']
    items = kube.run(['get', 'pods', '-o', 'json'])['items']
    pods = [pod for pod in items if all(pod.get('metadata', {}).get('labels', {}).get(k) == v for k, v in labels.items())]
    require_no_writer(deployment, pods)


def _patch_secret(before, password):
    body = json.dumps(secret_patch(before, password), separators=(',', ':')).encode()
    command = ['/usr/local/bin/k3s', 'kubectl', '--namespace', 'voice-staging', 'patch',
               'secret', 'voice-app-secrets', '--type=json', '--patch-file=/dev/stdin', '-o', 'name']
    output = commands.capture(command, body=body, timeout=25, limit=256, env=_ENV)
    if output.strip() != b'secret/voice-app-secrets':
        raise Blocked('realtime_bootstrap_secret_response_invalid')


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def _effect(session, path, state, name, before, request_sha256, submit, observe, valid):
    """Persist one fixed caller-selected effect; a pending entry is never replayed."""
    preceding = {'role': (), 'database': ('role',), 'secret': ('role', 'database')}
    if (name not in ('role', 'database', 'secret') or state.get('status') != 'RUNNING'
        or set(state['steps']) != set(preceding[name])
        or any(state['steps'][key].get('status') != 'DONE' for key in preceding[name])
        or not re.fullmatch(r'[a-f0-9]{64}', request_sha256)):
        raise Blocked('realtime_bootstrap_reconciliation_required')
    session.check()
    if observe() != before: raise Blocked('realtime_bootstrap_before_changed')
    state['steps'][name] = {'status': 'PENDING', 'request_sha256': request_sha256,
                            'before_sha256': digest(before)}
    save(path, state)
    session.check()
    if observe() != before: raise Blocked('realtime_bootstrap_before_changed')
    if state['plan'].get('schema') in ('voice-staging-realtime-bootstrap-v2','voice-staging-realtime-bootstrap-v4'):
        import staging_prerequisites
        parent=staging_prerequisites.load_capture(session,actual=state['plan']['actual_images'])
        if digest(parent)!=state['plan']['prerequisites_sha256']:raise Blocked('realtime_bootstrap_parent_changed')
        session.check()
    submit()
    session.check()
    after = observe()
    if not valid(after): raise Blocked('realtime_bootstrap_readback_invalid')
    state['steps'][name] = {**state['steps'][name], 'status': 'DONE', 'readback': after}
    save(path, state)
    session.check()
    return after


def run_prepare(actions, request, approved, execution, source_root, stage):
    """Run the one fixed cold bootstrap within the admitted root prepare."""
    import guard
    import resource_claim
    from stage_runtime import Kube
    try:
        session = actions.tt_session
        if (not isinstance(session, resource_claim.Session) or not session.admitted
            or getattr(commands.authority_guard, '__self__', None) is not session
            or request.get('action') != 'prepare' or request.get('mode') not in ('full', 'app-only')
            or request.get('provisional_staging') is True
            or request.get('nonce', '')[:12] != session.operation
            or approved.get('schema') != 'voice-source-authority-v1' or approved.get('verified') is not True
            or approved.get('source_sha') != session.context['commit']
            or execution.get('head_sha') != approved['source_sha']):
            raise Blocked('realtime_bootstrap_authority_invalid')
        resource_claim.require_session(session,installation=actions.installation)
        session.check()
        parent=None
        if session.check().get('phase')=='prerequisites-captured':
            import staging_prerequisites
            parent=staging_prerequisites.load_capture(session,actual=stage.old_images)
        _verify_source_capture(session, approved, execution)
        helper_key = 'nats-rollout-preservation/realtime_bootstrap.py'
        helper_sha = hashlib.sha256(Path(__file__).read_bytes()).hexdigest()
        if (digest(actions.binding) != session.helper or actions.binding.get(helper_key) != helper_sha
            or approved['source_files'].get('scripts/staging/' + helper_key) != helper_sha
            or Path(source_root) != guard.ROOT / 'installed' / 'sources' / session.operation):
            raise Blocked('realtime_bootstrap_source_changed')
        kube = Kube(); namespace, pg = engine_snapshot(kube)
        pg['system_identifier'] = _system_identifier()
        secret = kube.get('secret', 'voice-app-secrets')
        _verify_admin(secret)
        initial = _catalog()
        if initial != {'role': None, 'database': None} or 'REALTIME_DATABASE_URL' in secret.get('data', {}):
            import realtime_bootstrap_existing
            return realtime_bootstrap_existing.run_existing(actions, request, approved, execution, source_root, stage)
        require_absent(initial)
        view = _stage_view(stage); _check_stage(kube, view)
        if namespace != stage.expected['namespace_uid']:
            raise Blocked('realtime_bootstrap_namespace_changed')
        password = secrets.token_urlsafe(32).encode('ascii')
        secret_patch(secret, password)
        if stage.expected.get('generation')!=session.installation.generation:
            raise Blocked('realtime_bootstrap_generation_changed')
        plan = {'schema': 'voice-staging-realtime-bootstrap-v3','installation_sha256':session.installation.sha256, 'operation': session.operation,
                'context': copy.deepcopy(session.context), 'helper_sha256': session.helper,
                'source_sha': approved['source_sha'], 'source_files_sha256': digest(approved['source_files']),
                'source_authority_sha256': digest(approved), 'execution_sha256': digest(execution),
                'stage_sha256': digest(view), 'credential_sha256': hashlib.sha256(password).hexdigest(),
                'actual_images': copy.deepcopy(stage.old_images), 'namespace_uid': namespace,
                'postgres': pg, 'secret': _secret_identity(secret), 'active_marker_sha256': digest(view['marker'])}
        if parent is not None:
            plan['schema']='voice-staging-realtime-bootstrap-v4'
            plan['prerequisites_sha256']=digest(parent)
        validate_plan(plan, session)
        base = _create_custody(guard.ROOT / 'installed', session.operation, password, secret)
        save(base / 'authority.json', {'approved': approved, 'execution': execution})
        save(base / 'stage.json', view)
        state = {'schema': 'voice-staging-realtime-bootstrap-checkpoint-v2','installation_sha256':session.installation.sha256, 'plan': plan,
                 'resource_fence': None, 'status': 'PLANNED', 'steps': {}, 'receipt': None}
        path = base / 'checkpoint.json'; save(path, state)
        state['resource_fence'] = session.bind_realtime_bootstrap(plan, stage.old_images)
        state['status'] = 'RUNNING'; save(path, state)
        _source_contract(source_root, approved)
        def catalog():
            _check_live(session, kube, plan, view)
            current = kube.get('secret', 'voice-app-secrets')
            if _secret_identity(current) != plan['secret']: raise Blocked('realtime_bootstrap_secret_changed')
            _verify_admin(current)
            value = _catalog()
            _check_live(session, kube, plan, view)
            return value
        role = _effect(session, path, state, 'role', initial, digest({'kind': 'role', 'credential_sha256': plan['credential_sha256']}),
                       lambda: _create_role(password, plan['postgres']['system_identifier']), catalog,
                       lambda row: valid_role(row.get('role')) and row.get('database') is None)
        final = _effect(session, path, state, 'database', role, digest({'kind': 'database', 'owner_oid': role['role']['oid']}),
                        lambda: _create_database(plan['postgres']['system_identifier'], role['role']['oid']), catalog,
                        lambda row: row.get('role') == role['role'] and type(row.get('database')) is dict
                        and set(row['database']) == {'oid', 'owner_oid'}
                        and type(row['database']['oid']) is int and row['database']['oid'] > 0
                        and row['database']['owner_oid'] == role['role']['oid'])
        _check_live(session, kube, plan, view); _writer_check(kube, view); _verify_runtime_auth(password)
        def observe_secret():
            _check_live(session, kube, plan, view)
            if _catalog() != final: raise Blocked('realtime_bootstrap_catalog_changed')
            return _secret_identity(kube.get('secret', 'voice-app-secrets'))
        def submit_secret():
            _check_live(session, kube, plan, view); _writer_check(kube, view)
            _patch_secret(secret, password)
        def valid_secret(observed):
            current = kube.get('secret', 'voice-app-secrets')
            return _verify_secret_after(secret, current, password) == observed
        changed = _effect(session, path, state, 'secret', plan['secret'],
                          digest({'kind': 'secret', 'plan_sha256': digest(plan)}),
                          submit_secret, observe_secret, valid_secret)
        _check_live(session, kube, plan, view); _writer_check(kube, view); _verify_runtime_auth(password)
        receipt = {'schema': 'voice-staging-realtime-bootstrap-receipt-v2','installation_sha256':session.installation.sha256, 'plan_sha256': digest(plan),
                   'steps_sha256': digest(state['steps']), 'role_oid': role['role']['oid'],
                   'database_oid': final['database']['oid'], 'owner_oid': role['role']['oid'],
                   'secret': changed, 'migration_pass': False}
        state['receipt'] = receipt; state['status'] = 'COMPLETE'; save(path, state)
        return load_completed(session, actual=stage.old_images)
    except Blocked:
        raise
    except Exception:
        raise Blocked('realtime_bootstrap_failed_reconcile') from None


def _records(state, session):
    import resource_claim
    installation=resource_claim.require_installation(getattr(session,'installation',None))
    try:
        resource_claim.record_fields(state,'realtime_checkpoint',{'schema','plan','resource_fence','status','steps','receipt'},installation=installation)
    except (RuntimeError,ValueError,TypeError):raise Blocked('realtime_bootstrap_completion_invalid') from None
    scoped=state.get('schema')=='voice-staging-realtime-bootstrap-checkpoint-v2'
    if state['status'] != 'COMPLETE':
        raise Blocked('realtime_bootstrap_completion_invalid')
    plan = state['plan']; receipt = state['receipt']; steps = state['steps']
    validate_plan(plan, session)
    if scoped!=(plan.get('schema') in ('voice-staging-realtime-bootstrap-v3','voice-staging-realtime-bootstrap-v4')):
        raise Blocked('realtime_bootstrap_completion_invalid')
    initial = {'schema': ('voice-staging-resource-fence-v2' if scoped else 'voice-staging-resource-fence-v1'), 'context': session.context,
               'operation': session.operation, 'helper_sha256': session.helper, 'target_sha256': digest(plan)}
    if scoped:initial['installation_sha256']=installation.sha256
    if state['resource_fence'] != initial: raise Blocked('realtime_bootstrap_fence_changed')
    fields = {'status', 'request_sha256', 'before_sha256', 'readback'}
    if (type(steps) is not dict or set(steps) != {'role', 'database', 'secret'}
        or any(type(steps[k]) is not dict or set(steps[k]) != fields or steps[k]['status'] != 'DONE' for k in steps)):
        raise Blocked('realtime_bootstrap_completion_invalid')
    role = steps['role']['readback']; final = steps['database']['readback']
    secret = steps['secret']['readback']
    if (not valid_role(role.get('role')) or role.get('database') is not None
        or final.get('role') != role['role'] or type(final.get('database')) is not dict
        or set(final['database']) != {'oid', 'owner_oid'} or type(final['database']['oid']) is not int
        or final['database']['oid'] <= 0 or final['database']['owner_oid'] != role['role']['oid']
        or steps['role']['before_sha256'] != digest({'role': None, 'database': None})
        or steps['database']['before_sha256'] != digest(role) or steps['secret']['before_sha256'] != digest(plan['secret'])
        or steps['role']['request_sha256'] != digest({'kind': 'role', 'credential_sha256': plan['credential_sha256']})
        or steps['database']['request_sha256'] != digest({'kind': 'database', 'owner_oid': role['role']['oid']})
        or steps['secret']['request_sha256'] != digest({'kind': 'secret', 'plan_sha256': digest(plan)})
        or type(secret) is not dict or set(secret) != {'uid', 'resourceVersion', 'data_sha256'}
        or secret.get('uid') != plan['secret']['uid'] or not isinstance(secret.get('resourceVersion'), str)
        or not secret['resourceVersion'] or secret['resourceVersion'] == plan['secret']['resourceVersion']
        or not isinstance(secret.get('data_sha256'), str) or not re.fullmatch(r'[a-f0-9]{64}', secret['data_sha256'])):
        raise Blocked('realtime_bootstrap_completion_invalid')
    expected = {'schema': ('voice-staging-realtime-bootstrap-receipt-v2' if scoped else 'voice-staging-realtime-bootstrap-receipt-v1'), 'plan_sha256': digest(plan),
                'steps_sha256': digest(steps), 'role_oid': role['role']['oid'], 'database_oid': final['database']['oid'],
                'owner_oid': role['role']['oid'], 'secret': steps['secret']['readback'], 'migration_pass': False}
    if scoped:expected['installation_sha256']=installation.sha256
    if receipt != expected: raise Blocked('realtime_bootstrap_completion_invalid')
    return plan, receipt, final, initial


def validate_historical_checkpoint(checkpoint, rollout_pass_state,*,installation=None):
    """Validate a captured original receipt, without granting current TT authority."""
    import resource_claim
    from types import SimpleNamespace
    try:
        installation=resource_claim.require_installation(installation)
        if rollout_pass_state.get('status') != 'PASS': raise ValueError()
        binding = resource_claim.historical_binding(rollout_pass_state,installation=installation)
        original = SimpleNamespace(context=binding['context'], operation=binding['operation'], helper=binding['helper_sha256'],installation=installation)
        plan, receipt, _, _ = _records(checkpoint, original)
        if (binding['schema']=='voice-staging-resource-fence-v2') != (checkpoint.get('schema')=='voice-staging-realtime-bootstrap-checkpoint-v2'): raise ValueError()
        if rollout_pass_state['target'].get('realtime_bootstrap_sha256') != digest(receipt): raise ValueError()
        if plan['schema'] in ('voice-staging-realtime-bootstrap-v2','voice-staging-realtime-bootstrap-v4'):
            import staging_prerequisites
            historical=staging_prerequisites.validate_historical_capture(original.operation,installation=installation)
            if digest(historical)!=plan['prerequisites_sha256']:raise ValueError()
        return copy.deepcopy(plan), copy.deepcopy(receipt)
    except Exception:
        raise Blocked('realtime_bootstrap_historical_invalid') from None


def _load_completed(session, actual, rollout_target):
    import guard
    import resource_claim
    from stage_runtime import Kube
    if (not isinstance(session, resource_claim.Session) or not session.admitted
        or getattr(commands.authority_guard, '__self__', None) is not session):
        raise Blocked('realtime_bootstrap_authority_invalid')
    session.check()
    base = guard.ROOT / 'installed' / 'realtime-bootstrap' / session.operation
    for path in (base / 'checkpoint.json', base / 'before-secret.json', base / 'authority.json', base / 'stage.json'):
        resource_claim.trusted_json(path,limit=2<<20)
    state = json.loads(_read_private(base / 'checkpoint.json'))
    before = json.loads(_read_private(base / 'before-secret.json'))
    authority = json.loads(_read_private(base / 'authority.json'))
    view = json.loads(_read_private(base / 'stage.json'))
    password = _read_private(base / 'credential', limit=128)
    plan, receipt, final, initial_binding = _records(state, session)
    if plan['schema'] in ('voice-staging-realtime-bootstrap-v2','voice-staging-realtime-bootstrap-v4'):
        import staging_prerequisites
        parent=staging_prerequisites.load_capture(session,actual=actual,rollout_target=rollout_target)
        if digest(parent)!=plan['prerequisites_sha256']:raise Blocked('realtime_bootstrap_parent_changed')
    current = session.binding(); row = session.check()
    if row.get('phase') == 'realtime-bootstrap':
        if rollout_target is not None or current != initial_binding: raise Blocked('realtime_bootstrap_fence_changed')
    elif row.get('phase') == 'prepare':
        if (type(rollout_target) is not dict or current.get('target_sha256') != digest(rollout_target)
            or rollout_target.get('realtime_bootstrap_sha256') != digest(receipt)):
            raise Blocked('realtime_bootstrap_fence_changed')
    else:
        raise Blocked('realtime_bootstrap_fence_changed')
    if actual is not None and actual != plan['actual_images']: raise Blocked('realtime_bootstrap_actual_changed')
    if (type(authority) is not dict or set(authority) != {'approved', 'execution'}
        or digest(authority['approved']) != plan['source_authority_sha256']
        or digest(authority['approved']['source_files']) != plan['source_files_sha256']
        or digest(authority['execution']) != plan['execution_sha256'] or digest(view) != plan['stage_sha256']
        or digest(view['marker']) != plan['active_marker_sha256']
        or hashlib.sha256(password).hexdigest() != plan['credential_sha256']
        or _secret_identity(before) != plan['secret']):
        raise Blocked('realtime_bootstrap_custody_changed')
    _verify_source_capture(session, authority['approved'], authority['execution'])
    kube = Kube(); _check_live(session, kube, plan, view)
    secret = kube.get('secret', 'voice-app-secrets')
    if _verify_secret_after(before, secret, password) != receipt['secret'] or _catalog() != final:
        raise Blocked('realtime_bootstrap_live_readback_changed')
    _verify_admin(secret); _verify_runtime_auth(password)
    _source_contract(guard.ROOT / 'installed' / 'sources' / session.operation, authority['approved'])
    _check_live(session, kube, plan, view)
    return copy.deepcopy(plan), copy.deepcopy(receipt)


def load_completed(session, *, actual=None, rollout_target=None):
    try:
        return _load_completed(session, actual, rollout_target)[1]
    except Blocked:
        raise
    except Exception:
        raise Blocked('realtime_bootstrap_completion_invalid') from None


def load_plan(session, *, rollout_target=None):
    try:
        return _load_completed(session, None, rollout_target)[0]
    except Blocked:
        raise
    except Exception:
        raise Blocked('realtime_bootstrap_completion_invalid') from None


def load_existing(session, *, actual=None, rollout_target=None):
    import realtime_bootstrap_existing
    return realtime_bootstrap_existing.load_existing(session, actual=actual, rollout_target=rollout_target)
