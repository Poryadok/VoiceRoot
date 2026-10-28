"""Read-only cohort database binding; caller owns source and operation guard.

Callbacks run in the operator process: snapshot returns an authenticated,
owner-chain-verified Pod projection; read_key returns only the requested key
and its Kubernetes object UID/resourceVersion. Their private values are never
included in receipts or error text. The guard must fence each observation.
"""
import re
import json
import subprocess
import base64
import os
import selectors
import time
import ipaddress
import shlex
from urllib.parse import unquote, urlsplit, parse_qsl

OWNERS = dict((name, name + '_db') for name in (
    'user social role space chat file messaging matchmaking search notification '
    'realtime bot subscription moderation story'.split()))
OWNERS.update(auth='auth_db', voice='voice_db', gameintegration='game_integration_db')
OPTIONAL_DATABASES = {'gateway': 'gateway_db'}
CONTAINERS = {name: name for name in OWNERS}
CONTAINERS.update(analytics='analytics', gateway='gateway', web='web', admin='admin',
                  **{'developer-portal': 'portal'})
SECONDARY = {'chat': {'SPACE_DATABASE_URL': 'space_db'},
             'messaging': {'CHAT_DATABASE_URL': 'chat_db', 'SPACE_DATABASE_URL': 'space_db'}}
AUTH_OVERRIDE_VARS = ('VOICE_AUTH_JDBC_URL', 'voice.auth.jdbc.url', 'SPRING_APPLICATION_JSON',
                      'SPRING_CONFIG_LOCATION', 'SPRING_CONFIG_ADDITIONAL_LOCATION', 'SPRING_CONFIG_NAME',
                      'SPRING_CONFIG_IMPORT', 'SPRING_PROFILES_ACTIVE', 'SPRING_FLYWAY_URL',
                      'SPRING_FLYWAY_ENABLED', 'JDK_JAVA_OPTIONS', '_JAVA_OPTIONS')
VOICE_LIFECYCLE_SOURCE = {
    'src/backend/voice/main.go': 'e12135163d53d2abf3636b54a16a9b3167621e75fe6c0efdb6b01745924f04de',
    'src/backend/voice/database.go': '25bdcd5d85f93e27c269fcd60e2e433ab9203d1d68d6139cd1b3f22763714580',
    'src/backend/voice/internal/roomlifecycle/postgres_store.go': 'a8fa8db25582ea44c7f00bbd3fad647c89e753423bc2ccb750476ec559b85b03',
}
IDENTITY = ('namespace', 'namespace_uid', 'deployment', 'deployment_uid', 'deployment_rv', 'pod_name', 'pod_uid',
            'pod_rv', 'container_id', 'restart_count', 'container_name')
SAFE = re.compile(r'[a-zA-Z_][a-zA-Z0-9_]{0,62}\Z')
ENGINE_IDENTITY = ('namespace', 'namespace_uid', 'pod_name', 'pod_uid', 'pod_rv',
                   'container_name', 'container_id', 'restart_count', 'pvc_uid', 'pvc_rv',
                   'service_uid', 'service_rv', 'service_ip', 'pod_ip', 'endpoints_uid', 'endpoints_rv', 'slices_identity')
PSQL_SCRIPT = ('export PGPASSWORD="$POSTGRES_PASSWORD"; export PGAPPNAME=voice-cohort-observe; '
               'export PGOPTIONS="-c default_transaction_read_only=on -c statement_timeout=5000 -c lock_timeout=3000"; '
               'exec psql -h 127.0.0.1 -X -qAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$1" -c "$2"')
ENV_SCRIPT = ("printf 'x\\000' | (IFS= read -r -d '' probe && [ \"$probe\" = x ]) || exit 1; "
              "n=0; result=; while IFS= read -r -d '' line; do "
              "case \"$line\" in \"$1\"=*) n=$((n+1)); result=${line#*=};; esac; "
              "done </proc/1/environ; [ \"$n\" -le 1 ] || exit 1; "
              "if [ \"$n\" = 0 ]; then printf M; else printf 'P%s' \"$result\"; fi")
AUTH_ENV_POLICY = ("while IFS= read -r -d '' line; do key=${line%%=*}; case \"$key\" in "
                   "SPRING_DATASOURCE_URL|SPRING_DATASOURCE_USERNAME|SPRING_DATASOURCE_PASSWORD) continue;; esac; "
                   "key=${key,,}; key=${key//./_}; key=${key//-/_}; case \"$key\" in "
                   "spring_config*|spring_application_json|spring_profiles*|spring_datasource*|spring_flyway*|"
                   "voice_auth_jdbc*) exit 1;; esac; done </proc/1/environ; printf ok")
COMMON_SQL = ("'database',current_database(),'database_oid',(SELECT oid::bigint FROM pg_database WHERE datname=current_database()),"
              "'role',current_user,'pid',pg_backend_pid(),'server_start',pg_postmaster_start_time(),"
              "'read_only',current_setting('transaction_read_only'),'data_directory',current_setting('data_directory')")
MIGRATE_SQL = ("SELECT json_build_object(" + COMMON_SQL +
               ",'version',max(version),'dirty',bool_or(dirty),'ledger_rows',count(*)) FROM public.schema_migrations")
VOICE_SQL = MIGRATE_SQL.replace("'ledger_rows',count(*)", "'ledger_rows',count(*),"
    "'voice_lifecycle_tables_present',(SELECT bool_and(to_regclass(table_name) IS NOT NULL) FROM unnest(ARRAY["
    "'voice_room_instances','voice_room_memberships','voice_lifecycle_operations','voice_lifecycle_effects',"
    "'voice_media_epoch_denials','voice_event_outbox']) AS table_name)")
FLYWAY_SQL = ("SELECT json_build_object(" + COMMON_SQL +
              ",'version',max(CASE WHEN version ~ '^[0-9]+$' THEN version::bigint END),"
              "'dirty',bool_or(NOT success),'ledger_rows',count(*),"
              "'bad_versions',count(*) FILTER (WHERE version IS NOT NULL AND version !~ '^[0-9]+$'),"
              "'other_ledger',to_regclass('public.schema_migrations') IS NOT NULL) FROM public.flyway_schema_history")
_AUTH_FLYWAY_INNER = ("SELECT row_to_json(t) AS payload FROM (SELECT 'flyway' AS ledger_kind, "
                      "max(CASE WHEN version ~ '^[0-9]+$' THEN version::bigint END) AS version, "
                      "bool_or(NOT success) AS dirty, count(*) AS ledger_rows, "
                      "count(*) FILTER (WHERE version IS NOT NULL AND version !~ '^[0-9]+$') AS bad_versions "
                      "FROM public.flyway_schema_history) t")
_AUTH_CANONICAL_INNER = ("SELECT row_to_json(t) AS payload FROM (SELECT 'canonical' AS ledger_kind, "
                         "max(version) AS version, bool_or(dirty) AS dirty, count(*) AS ledger_rows, "
                         "0 AS bad_versions FROM public.schema_migrations) t")
def _xml_json_query(inner):
    escaped = inner.replace("'", "''")
    return "(xpath('string(/row/payload)',query_to_xml('" + escaped + "',false,true,'')))[1]::text::jsonb"


AUTH_SQL = ("SELECT json_build_object(" + COMMON_SQL + ")::jsonb || CASE "
            "WHEN to_regclass('public.flyway_schema_history') IS NOT NULL AND to_regclass('public.schema_migrations') IS NULL THEN " +
            _xml_json_query(_AUTH_FLYWAY_INNER) +
            " WHEN to_regclass('public.schema_migrations') IS NOT NULL AND to_regclass('public.flyway_schema_history') IS NULL THEN " +
            _xml_json_query(_AUTH_CANONICAL_INNER) +
            " ELSE jsonb_build_object('ledger_kind','ambiguous') END")


class Hold(ValueError):
    pass


class GuardLost(Hold):
    pass


def _fence(callback):
    try:
        callback()
    except Exception:
        raise GuardLost('operation_guard_failed') from None


def linux_psql(args):
    """Run on the operator-selected Linux host; diagnostics remain private."""
    try:
        return _bounded_process(args, 8192).decode('utf-8')
    except Exception:
        raise Hold('metadata_transport_failed') from None


def kube_key_reader(run_bytes, namespace='voice-dev'):
    """Return a private, single-key kubectl reader; no values enter receipts.

    run_bytes must capture stdout/stderr privately. Invoke only under guard.
    None denotes an absent object; value=None denotes an absent key.
    """
    if namespace not in ('voice-dev', 'voice-staging'):
        raise Hold('namespace_invalid')
    def read(kind, name, key):
        try:
            if (kind not in ('secret', 'configmap') or not re.fullmatch(r'[a-z0-9][a-z0-9.-]*', name) or
                    not re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*', key)):
                raise Hold('reference_invalid')
            dbkeys = {name.upper() + '_DATABASE_URL' for name in OWNERS}
            dbkeys.update(('GAME_INTEGRATION_DATABASE_URL', 'DATABASE_URL', 'GATEWAY_DATABASE_URL',
                           'VOICE_DATABASE_URL', 'SPRING_DATASOURCE_URL', 'SPRING_DATASOURCE_USERNAME'))
            if kind == 'secret':
                allowed = ((name == 'voice-app-secrets' or
                            re.fullmatch(r'voice-auth-user-[a-z0-9][a-z0-9-]{0,30}', name)) and
                           key in dbkeys | {'AUTH_DB_USERNAME'})
            else:
                allowed = name == 'voice-app-config' and key in dbkeys | {'POSTGRES_USER'}
            if not allowed:
                raise Hold('reference_not_allowed')
            template = ('{{.metadata.uid}}{{"\\n"}}{{.metadata.resourceVersion}}{{"\\n"}}'
                        '{{if eq (index .data "' + key + '") nil}}M{{else}}P{{index .data "' + key + '"}}{{end}}')
            args = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
                    '--namespace', namespace, '--request-timeout=15s', 'get', kind, name,
                    '-o', 'go-template=' + template]
            raw = _external(run_bytes, args)
            if not isinstance(raw, bytes) or len(raw) > 65536:
                raise Hold('reference_response_invalid')
            uid, rv, data = raw.split(b'\n', 2)
            if not uid or not rv or not data or data[:1] not in (b'M', b'P'):
                raise Hold('reference_response_invalid')
            if data[:1] == b'M':
                if data != b'M':
                    raise Hold('reference_response_invalid')
                value = None
            else:
                value = (base64.b64decode(data[1:], validate=True) if kind == 'secret' else data[1:]).decode('utf-8')
            return {'uid': uid.decode('ascii'), 'resource_version': rv.decode('ascii'), 'value': value}
        except Exception:
            raise Hold('reference_read_failed') from None
    read.namespace = namespace
    return read


def kube_runtime_reader(workload, pod_name, run_bytes, namespace='voice-dev'):
    """Read one allowlisted PID 1 environment variable privately.

    Caller must verify PID 1 is the application, its Pod owner chain, and its
    unchanged container identity around each use. Failure is always HOLD.
    """
    if (workload not in CONTAINERS or namespace not in ('voice-dev', 'voice-staging') or
            not isinstance(pod_name, str) or
            not re.fullmatch(r'voice-' + re.escape(workload) + r'-[a-z0-9-]+', pod_name)):
        raise Hold('runtime_target_invalid')
    def read(variable):
        if variable not in ('DATABASE_URL', 'VOICE_DATABASE_URL', 'GATEWAY_DATABASE_URL',
                            'CHAT_DATABASE_URL', 'SPACE_DATABASE_URL',
                            'SPRING_DATASOURCE_URL', 'SPRING_DATASOURCE_USERNAME',
                            'AUTH_FLYWAY_ENABLED', 'JAVA_TOOL_OPTIONS', *AUTH_OVERRIDE_VARS):
            raise Hold('runtime_variable_not_allowed')
        executable = '/gateway' if workload == 'gateway' else '/service'
        if workload == 'auth':
            script = ("[ \"$(readlink /proc/1/exe)\" = /opt/java/openjdk/bin/java ] || exit 1; "
                      "set -- \"$1\"; i=0; while IFS= read -r -d '' arg; do "
                      "i=$((i+1)); case $i:$arg in 1:java|2:-jar|3:/auth.jar) ;; *) exit 1;; esac; "
                      "done </proc/1/cmdline; [ \"$i\" = 3 ] || exit 1; " + ENV_SCRIPT)
        else:
            script = '[ "$(readlink /proc/1/exe)" = "' + executable + '" ] || exit 1; ' + ENV_SCRIPT
        args = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
                '--namespace', namespace, '--request-timeout=15s', 'exec', pod_name,
                '-c', CONTAINERS[workload], '--', 'bash' if workload == 'auth' else 'sh', '-ec', script, '--', variable]
        try:
            value = _external(run_bytes, args)
            if not isinstance(value, bytes) or len(value) > 65536:
                raise Hold('runtime_response_invalid')
            if value == b'M':
                return None
            if not value.startswith(b'P'):
                raise Hold('runtime_response_invalid')
            return value[1:].decode('utf-8')
        except Exception:
            raise Hold('runtime_read_failed') from None
    def auth_policy():
        if workload != 'auth':
            raise Hold('runtime_target_invalid')
        args = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
                '--namespace', namespace, '--request-timeout=15s', 'exec', pod_name,
                '-c', 'auth', '--', 'bash', '-ec', AUTH_ENV_POLICY]
        if _external(run_bytes, args) != b'ok':
            raise Hold('auth_external_config_unknown')
        return True
    read.namespace = namespace
    read.workload = workload
    read.pod_name = pod_name
    read.auth_policy = auth_policy
    return read


def kube_dns_reader(workload, pod_name, run_bytes, namespace='voice-dev'):
    """Private NSS resolution in the same app container, bound to its identity."""
    target = kube_runtime_reader(workload, pod_name, run_bytes, namespace)
    def lookup(host):
        try:
            allowed = ('voice-postgres', 'voice-postgres.' + namespace,
                       'voice-postgres.' + namespace + '.svc', 'voice-postgres.' + namespace + '.svc.cluster.local')
            if host not in allowed:
                raise Hold('dns_name_invalid')
            args = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
                    '--namespace', namespace, '--request-timeout=15s', 'exec', pod_name,
                    '-c', CONTAINERS[workload], '--', 'getent', 'hosts', host]
            raw = _external(run_bytes, args)
            if not isinstance(raw, bytes) or len(raw) > 8192:
                raise Hold('dns_response_invalid')
            addresses = sorted({str(ipaddress.ip_address(line.split()[0])) for line in raw.decode('ascii').splitlines() if line.split()})
            if not addresses:
                raise Hold('dns_response_invalid')
            return addresses
        except Exception:
            raise Hold('dns_read_failed') from None
    lookup.namespace, lookup.workload, lookup.pod_name = target.namespace, target.workload, target.pod_name
    return lookup


def kube_workload_snapshot(workload, pod_name, run_bytes, guard, namespace='voice-dev'):
    """Return a guarded private Pod projection after checking controller ancestry."""
    if (workload not in CONTAINERS or namespace not in ('voice-dev', 'voice-staging') or
            not isinstance(pod_name, str) or
            not re.fullmatch(r'voice-' + re.escape(workload) + r'-[a-z0-9-]+', pod_name)):
        raise Hold('workload_target_invalid')
    def get(kind, name):
        _fence(guard)
        args = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
                '--namespace', namespace, '--request-timeout=15s', 'get', kind, name, '-o', 'json']
        raw = _external(run_bytes, args)
        _fence(guard)
        if not isinstance(raw, bytes) or len(raw) > 65536:
            raise Hold('workload_response_invalid')
        return json.loads(raw)
    def snap():
        try:
            deployment = get('deployment', 'voice-' + workload)
            pod = get('pod', pod_name)
            ns = get('namespace', namespace)['metadata']
            pm, dm = pod['metadata'], deployment['metadata']
            if (ns['name'] != namespace or pm['name'] != pod_name or pm['namespace'] != namespace or
                    dm['name'] != 'voice-' + workload or dm['namespace'] != namespace):
                raise Hold('workload_identity_invalid')
            owners = [r for r in pm.get('ownerReferences', []) if r.get('controller') is True]
            if len(owners) != 1 or owners[0].get('kind') != 'ReplicaSet':
                raise Hold('workload_owner_invalid')
            owner = owners[0]
            if not re.fullmatch(r'voice-' + re.escape(workload) + r'-[a-z0-9-]+', owner['name']):
                raise Hold('workload_owner_invalid')
            rs = get('replicaset', owner['name'])['metadata']
            parents = [r for r in rs.get('ownerReferences', []) if r.get('controller') is True]
            if (rs['uid'] != owner['uid'] or rs['namespace'] != namespace or rs['name'] != owner['name'] or
                    len(parents) != 1 or parents[0].get('kind') != 'Deployment' or parents[0].get('uid') != dm['uid']):
                raise Hold('workload_owner_invalid')
            name = CONTAINERS[workload]
            containers = [c for c in pod['spec']['containers'] if c.get('name') == name]
            statuses = [c for c in pod['status']['containerStatuses'] if c.get('name') == name]
            if (len(containers) != 1 or len(statuses) != 1 or 'running' not in statuses[0].get('state', {}) or
                    containers[0].get('command') or containers[0].get('args')):
                raise Hold('workload_container_invalid')
            c, status = containers[0], statuses[0]
            unsafe_mounts = ('/', '/config', '/application.yml', '/application.yaml', '/application.properties')
            safe_spring = all(not any(m.get('mountPath', '') == p or m.get('mountPath', '').startswith(p + '/')
                                      for p in unsafe_mounts) for m in c.get('volumeMounts', []))
            return {'namespace': namespace, 'namespace_uid': ns['uid'], 'deployment': dm['name'], 'deployment_uid': dm['uid'],
                    'deployment_rv': dm['resourceVersion'], 'pod_name': pm['name'], 'pod_uid': pm['uid'],
                    'pod_rv': pm['resourceVersion'], 'container_name': name, 'container_id': status['containerID'],
                    'restart_count': status['restartCount'], 'env': c.get('env', []), 'envFrom': c.get('envFrom', []),
                    'spring_config_safe': safe_spring}
        except Hold:
            raise
        except Exception:
            raise Hold('workload_snapshot_failed') from None
    return snap


def kube_engine_snapshot(run_bytes, guard):
    """Fence logical K8s PostgreSQL Pod/PVC and Service endpoint identity."""
    namespace = 'voice-dev'
    def get(kind, name):
        _fence(guard)
        args = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
                '--namespace', namespace, '--request-timeout=15s', 'get', kind, name, '-o', 'json']
        raw = _external(run_bytes, args)
        _fence(guard)
        if not isinstance(raw, bytes) or len(raw) > 65536:
            raise Hold('engine_response_invalid')
        return json.loads(raw)
    def snap():
        try:
            ns = get('namespace', namespace)['metadata']
            stateful = get('statefulset', 'voice-postgres')['metadata']
            pod = get('pod', 'voice-postgres-0')
            pm = pod['metadata']
            owner = [o for o in pm.get('ownerReferences', []) if o.get('controller') is True]
            if (ns['name'] != namespace or stateful['name'] != 'voice-postgres' or
                    stateful['namespace'] != namespace or pm['name'] != 'voice-postgres-0' or
                    pm['namespace'] != namespace or len(owner) != 1 or
                    owner[0].get('kind') != 'StatefulSet' or owner[0].get('uid') != stateful['uid']):
                raise Hold('engine_owner_invalid')
            containers = [c for c in pod['spec']['containers'] if c.get('name') == 'postgres']
            statuses = [c for c in pod['status']['containerStatuses'] if c.get('name') == 'postgres']
            if len(containers) != 1 or len(statuses) != 1 or 'running' not in statuses[0].get('state', {}):
                raise Hold('engine_container_invalid')
            mounts = [m for m in containers[0].get('volumeMounts', []) if m.get('mountPath') == '/var/lib/postgresql/data']
            if len(mounts) != 1:
                raise Hold('engine_pvc_invalid')
            volumes = [v for v in pod['spec']['volumes'] if v.get('name') == mounts[0]['name']]
            if len(volumes) != 1 or volumes[0].get('persistentVolumeClaim', {}).get('claimName') != 'pgdata-voice-postgres-0':
                raise Hold('engine_pvc_invalid')
            pvc = get('persistentvolumeclaim', 'pgdata-voice-postgres-0')['metadata']
            service_object = get('service', 'voice-postgres')
            service, service_spec = service_object['metadata'], service_object['spec']
            service_ports = [p for p in service_spec.get('ports', [])
                             if p.get('port') == 5432 and p.get('protocol', 'TCP') == 'TCP']
            selector = service_spec.get('selector')
            labels = pm.get('labels', {})
            if (service_spec.get('type', 'ClusterIP') != 'ClusterIP' or not service_spec.get('clusterIP') or
                    not isinstance(selector, dict) or not selector or
                    any(labels.get(k) != v for k, v in selector.items()) or len(service_ports) != 1):
                raise Hold('engine_service_routing_invalid')
            target = service_ports[0].get('targetPort', 5432)
            if target != 5432 and not (isinstance(target, str) and any(
                    p.get('name') == target and p.get('containerPort') == 5432 and p.get('protocol', 'TCP') == 'TCP'
                    for p in containers[0].get('ports', []))):
                raise Hold('engine_service_routing_invalid')
            port_name = service_ports[0].get('name') or ''
            endpoints = get('endpoints', 'voice-postgres')
            subsets = endpoints.get('subsets', [])
            if (len(subsets) != 1 or len(subsets[0].get('addresses', [])) != 1 or
                    subsets[0].get('notReadyAddresses') or
                    not any(p.get('port') == 5432 and p.get('protocol', 'TCP') == 'TCP' and (p.get('name') or '') == port_name
                            for p in subsets[0].get('ports', []))):
                raise Hold('engine_service_routing_invalid')
            address = subsets[0]['addresses'][0]
            if (address.get('ip') != pod['status'].get('podIP') or address.get('targetRef', {}).get('kind') != 'Pod' or
                    address.get('targetRef', {}).get('uid') != pm['uid']):
                raise Hold('engine_service_routing_invalid')
            _fence(guard)
            args = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
                    '--namespace', namespace, '--request-timeout=15s', 'get', 'endpointslices',
                    '-l', 'kubernetes.io/service-name=voice-postgres', '-o', 'json']
            raw = _external(run_bytes, args)
            _fence(guard)
            if not isinstance(raw, bytes) or len(raw) > 65536:
                raise Hold('engine_service_routing_invalid')
            slices = json.loads(raw).get('items', [])
            if len(slices) != 1:
                raise Hold('engine_service_routing_invalid')
            sl = slices[0]
            if (not any(p.get('port') == 5432 and p.get('protocol', 'TCP') == 'TCP' and (p.get('name') or '') == port_name
                        for p in sl.get('ports', [])) or len(sl.get('endpoints', [])) != 1):
                raise Hold('engine_service_routing_invalid')
            target_endpoint = sl['endpoints'][0]
            if (target_endpoint.get('addresses') != [pod['status']['podIP']] or
                    target_endpoint.get('conditions', {}).get('ready') is not True or
                    target_endpoint.get('targetRef', {}).get('kind') != 'Pod' or
                    target_endpoint.get('targetRef', {}).get('uid') != pm['uid']):
                raise Hold('engine_service_routing_invalid')
            status, ep = statuses[0], endpoints['metadata']
            return {'namespace': namespace, 'namespace_uid': ns['uid'], 'pod_name': pm['name'],
                    'pod_uid': pm['uid'], 'pod_rv': pm['resourceVersion'], 'container_name': 'postgres',
                    'container_id': status['containerID'], 'restart_count': status['restartCount'],
                    'pvc_uid': pvc['uid'], 'pvc_rv': pvc['resourceVersion'],
                    'service_uid': service['uid'], 'service_rv': service['resourceVersion'],
                    'service_ip': service_spec['clusterIP'], 'pod_ip': pod['status']['podIP'],
                    'endpoints_uid': ep['uid'], 'endpoints_rv': ep['resourceVersion'],
                    'slices_identity': sl['metadata']['uid'] + ':' + sl['metadata']['resourceVersion']}
        except Hold:
            raise
        except Exception:
            raise Hold('engine_snapshot_failed') from None
    return snap


def linux_private_bytes(args):
    try:
        return _bounded_process(args, 65536)
    except Exception:
        raise Hold('private_transport_failed') from None


def _bounded_process(args, limit):
    # Linux only. Drain stdout with a hard cap; never retain stderr or emit it.
    process = subprocess.Popen(args, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    selector = selectors.DefaultSelector()
    deadline = time.monotonic() + 20
    chunks, total = [], 0
    try:
        selector.register(process.stdout, selectors.EVENT_READ)
        while selector.get_map():
            left = deadline - time.monotonic()
            if left <= 0 or not selector.select(left):
                raise Hold('private_transport_timeout')
            chunk = os.read(process.stdout.fileno(), min(8192, limit + 1 - total))
            if not chunk:
                selector.unregister(process.stdout)
                break
            total += len(chunk)
            if total > limit:
                raise Hold('private_response_size')
            chunks.append(chunk)
        if process.wait(timeout=max(0.01, deadline - time.monotonic())) != 0:
            raise Hold('private_transport_failed')
        return b''.join(chunks)
    finally:
        selector.close()
        if process.poll() is None:
            process.kill()
            process.wait()
        process.stdout.close()


def observe(source_catalog, binding, engine_snapshot, expected_engine_role, run, guard):
    """Observe one ledger, fenced by authenticated before/after engine snapshots.

    Caller must authenticate source_catalog, verify Pod/PVC and service routing,
    and own the operation guard. This does not evaluate application readiness.
    """
    try:
        database = binding['database']
        entry = source_catalog['databases'][database]
        if (binding['state'] != 'resolved' or database not in set(OWNERS.values()) | set(OPTIONAL_DATABASES.values()) or
                binding['binding']['namespace'] != 'voice-dev'):
            raise Hold('database_binding_invalid')
        _fence(guard)
        before = _external(engine_snapshot)
        if (not isinstance(before, dict) or any(k not in before for k in ENGINE_IDENTITY) or
                any(not isinstance(before[k], str) or not before[k] for k in ENGINE_IDENTITY if k != 'restart_count') or
                type(before['restart_count']) is not int or before['restart_count'] < 0 or
                before['namespace'] != 'voice-dev' or before['pod_name'] != 'voice-postgres-0' or
                before['container_name'] != 'postgres' or not isinstance(expected_engine_role, str) or
                not SAFE.fullmatch(expected_engine_role) or
                before['namespace_uid'] != binding['binding']['namespace_uid']):
            raise Hold('engine_identity_invalid')
        voice_check = database == 'voice_db' and entry.get('voice_lifecycle_policy_recognized') is True
        query = AUTH_SQL if entry['kind'] == 'flyway' else VOICE_SQL if voice_check else MIGRATE_SQL
        args = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
                '--namespace', 'voice-dev', '--request-timeout=15s', 'exec', 'voice-postgres-0',
                '-c', 'postgres', '--', 'sh', '-ec', PSQL_SCRIPT, '--', database, query]
        response = json.loads(_external(run, args))
        _fence(guard)
        after = _external(engine_snapshot)
        if any(before[k] != after[k] for k in ENGINE_IDENTITY):
            raise Hold('engine_changed')
        _fence(guard)
        if (response.get('database') != database or response.get('read_only') != 'on' or
                response.get('data_directory') != '/var/lib/postgresql/data' or
                type(response.get('database_oid')) is not int or response['database_oid'] <= 0 or
                type(response.get('pid')) is not int or response['pid'] <= 0 or
                response.get('role') != expected_engine_role or
                not isinstance(response.get('server_start'), str) or not response['server_start'] or
                type(response.get('version')) is not int or type(response.get('dirty')) is not bool or
                type(response.get('ledger_rows')) is not int or response['ledger_rows'] < 1 or
                ((entry['kind'] != 'flyway' or response.get('ledger_kind') == 'canonical') and response['ledger_rows'] != 1)):
            raise Hold('metadata_response_invalid')
        target_ip = before['pod_ip'] if before['service_ip'] == 'None' else before['service_ip']
        if binding.get('addresses') != [target_ip]:
            raise Hold('database_dns_binding_mismatch')
        if response['dirty']:
            raise Hold('dirty_ledger')
        if entry['kind'] == 'flyway' and (response.get('ledger_kind') not in ('flyway', 'canonical') or
                                          response.get('bad_versions') != 0):
            raise Hold('auth_mixed_or_unknown_ledger')
        versions = (entry['canonical_versions'] if entry['kind'] == 'flyway' and response['ledger_kind'] == 'canonical'
                    else entry['versions'])
        if response['version'] not in versions:
            raise Hold('unknown_ledger_version')
        if voice_check and response.get('voice_lifecycle_tables_present') is not True:
            raise Hold('voice_lifecycle_tables_missing')
        return {'source_sha': source_catalog['source_sha'], 'database': database,
                'engine': {k: before[k] for k in ENGINE_IDENTITY},
                'session': {k: response[k] for k in ('database_oid', 'role', 'pid', 'server_start')},
                'ledger': {k: response[k] for k in ('version', 'dirty', 'ledger_rows')},
                'ledger_kind': response.get('ledger_kind', entry['kind']),
                'source_checks': {'voice_lifecycle_tables': 'present' if voice_check else 'not_evaluated'},
                'startup_qualified': False,
                'limits': ['source_authority_and_kubernetes_routing_supplied_by_caller',
                           'ledger_does_not_prove_migration_content_or_startup_compatibility',
                           'dns_observation_does_not_prove_existing_application_connection_peer',
                           'getent_may_not_return_all_address_families',
                           'voice_table_lookup_uses_observer_search_path_and_does_not_prove_other_readiness_checks']}
    except Hold:
        raise
    except Exception:
        raise Hold('metadata_observation_failed') from None


def catalog(snapshot):
    """Project a caller-authorized source_snapshot, without asserting authority.

    The consumer must obtain it via the canonical source authority and
    publish_candidate.source_snapshot on the selected exact clean checkout.
    No ledger or startup compatibility is implied by the source inventory.
    """
    try:
        if (not re.fullmatch(r'[0-9a-f]{40}', snapshot['source_sha']) or
                not re.fullmatch(r'[0-9a-f]{64}', snapshot['git_tree_sha256']) or
                not isinstance(snapshot['files'], dict)):
            raise Hold('source_snapshot_invalid')
        databases = {}
        for database in sorted(set(OWNERS.values()) | set(OPTIONAL_DATABASES.values())):
            auth = database == 'auth_db'
            prefix = ('src/backend/auth/src/main/resources/db/migration/' if auth else
                      'src/backend/migrations/' + database + '/')
            files = {p[len(prefix):]: digest for p, digest in snapshot['files'].items()
                     if p.startswith(prefix) and p.endswith('.sql')}
            if not files and database in OPTIONAL_DATABASES.values():
                continue
            if not files or any(not re.fullmatch(r'[0-9a-f]{64}', h) for h in files.values()):
                raise Hold('migration_inventory_incomplete_' + database)
            versions = {}
            for name in sorted(files):
                match = re.fullmatch(r'V([0-9]+)__[^/]+\.sql' if auth else
                                     r'([0-9]+)_[^/]+\.(up|down)\.sql', name)
                if not match:
                    raise Hold('migration_filename_unknown')
                version = int(match.group(1))
                direction = 'flyway' if auth else match.group(2)
                row = versions.setdefault(version, {})
                if direction in row:
                    raise Hold('migration_duplicate')
                row[direction] = {'name': name, 'sha256': files[name]}
            # Some canonical migrations are deliberately forward-only.
            if not auth and any('up' not in v for v in versions.values()):
                raise Hold('migration_inventory_incomplete_' + database)
            databases[database] = {'kind': 'flyway' if auth else
                                   'space-native' if database == 'space_db' else 'golang-migrate',
                                   'versions': versions}
            if database == 'voice_db':
                databases[database]['voice_lifecycle_policy_recognized'] = all(
                    snapshot['files'].get(path) == digest for path, digest in VOICE_LIFECYCLE_SOURCE.items())
            if auth:
                canonical_prefix = 'src/backend/migrations/auth_db/'
                canonical = {p[len(canonical_prefix):]: h for p, h in snapshot['files'].items()
                             if p.startswith(canonical_prefix) and p.endswith('.sql')}
                if not canonical or any(not re.fullmatch(r'[0-9]+_[^/]+\.(up|down)\.sql', n) or
                                        not re.fullmatch(r'[0-9a-f]{64}', h) for n, h in canonical.items()):
                    raise Hold('auth_canonical_inventory_incomplete')
                databases[database]['canonical_files'] = dict(sorted(canonical.items()))
                databases[database]['canonical_versions'] = sorted({int(n.split('_', 1)[0]) for n in canonical if n.endswith('.up.sql')})
        return {'source_sha': snapshot['source_sha'], 'tree_sha256': snapshot['git_tree_sha256'],
                'databases': databases, 'live_schema_qualified': False}
    except Hold:
        raise
    except Exception:
        raise Hold('source_snapshot_invalid') from None


def catalog_from_source(root, expected_sha, authority):
    """Check exact clean source through the existing source-snapshot helper.

    The consumer's canonical authority must independently authorize expected_sha;
    the callback must raise on a missing, revoked or mismatched authority.
    """
    try:
        from pathlib import Path
        from publish_candidate import source_snapshot
        if not isinstance(expected_sha, str) or not re.fullmatch(r'[0-9a-f]{40}', expected_sha):
            raise Hold('source_identity_invalid')
        _external(authority, expected_sha)
        result = catalog(source_snapshot(Path(root), expected_sha))
        _external(authority, expected_sha)
        return result
    except Hold:
        raise
    except Exception:
        raise Hold('source_verification_failed') from None


def inspect(source_catalog, workload, pod_name, expected_engine_role, guard,
            run_bytes=linux_private_bytes, run_psql=linux_psql):
    """Inspect one Pod against the caller-authorized immutable source catalog.

    Caller must own the live prepare-only guard and verify the selected workload
    image is the candidate. Non-PostgreSQL applications remain unqualified.
    """
    snap = kube_workload_snapshot(workload, pod_name, run_bytes, guard)
    key = kube_key_reader(run_bytes)
    runtime = kube_runtime_reader(workload, pod_name, run_bytes)
    dns = kube_dns_reader(workload, pod_name, run_bytes)
    binding = resolve(workload, snap, key, runtime, dns, guard)
    if binding['state'] != 'resolved':
        return {'binding': binding, 'startup_qualified': False}
    observation = observe(source_catalog, binding, kube_engine_snapshot(run_bytes, guard),
                          expected_engine_role, run_psql, guard)
    final_binding = resolve(workload, snap, key, runtime, dns, guard)
    if final_binding != binding:
        raise Hold('workload_binding_changed')
    secondary, secondary_bindings = [], []
    for variable in SECONDARY.get(workload, {}):
        other = resolve(workload, snap, key, runtime, dns, guard, variable=variable)
        other_observation = observe(source_catalog, other, kube_engine_snapshot(run_bytes, guard),
                                    expected_engine_role, run_psql, guard)
        if (other_observation['engine'] != observation['engine'] or
                other_observation['session']['server_start'] != observation['session']['server_start']):
            raise Hold('engine_changed')
        if resolve(workload, snap, key, runtime, dns, guard, variable=variable) != other:
            raise Hold('workload_binding_changed')
        secondary.append({'binding': other, 'metadata': other_observation, 'owner_consistency': 'unknown'})
        secondary_bindings.append((variable, other))
    if resolve(workload, snap, key, runtime, dns, guard) != binding:
        raise Hold('workload_binding_changed')
    for variable, prior in secondary_bindings:
        if resolve(workload, snap, key, runtime, dns, guard, variable=variable) != prior:
            raise Hold('workload_binding_changed')
    _fence(guard)
    return {'binding': binding, 'metadata': observation, 'secondary': secondary, 'startup_qualified': False}


def inspect_all(source_catalog, pods, expected_engine_role, guard,
                run_bytes=linux_private_bytes, run_psql=linux_psql):
    """Inspect a complete caller-provided Pod inventory; never infer missing apps."""
    if (not isinstance(pods, dict) or set(pods) != set(CONTAINERS) or
            any(not isinstance(names, list) or not names or any(not isinstance(n, str) or not n for n in names)
                for names in pods.values())):
        raise Hold('workload_inventory_incomplete')
    flattened = [name for names in pods.values() for name in names]
    if len(flattened) != len(set(flattened)):
        raise Hold('workload_inventory_duplicate')
    result = {}
    for workload, names in sorted(pods.items()):
        result[workload] = []
        for name in names:
            _fence(guard)
            try:
                value = inspect(source_catalog, workload, name, expected_engine_role, guard, run_bytes, run_psql)
                result[workload].append({'status': 'observed', 'result': value})
            except GuardLost:
                raise
            except Hold as error:
                # Do not continue under a lost claim or expose arbitrary errors.
                _fence(guard)
                code = str(error)
                result[workload].append({'status': 'hold', 'reason': code if re.fullmatch(r'[a-z0-9_]+', code) else 'observation_failed'})
    owners = {database: workload for workload, database in OWNERS.items()}
    for entries in result.values():
        for entry in entries:
            if entry['status'] != 'observed':
                continue
            for secondary in entry['result'].get('secondary', []):
                target = secondary['metadata']
                owner_entries = result[owners[target['database']]]
                def equivalent(owner):
                    if owner.get('status') != 'observed':
                        return False
                    primary = owner['result'].get('metadata')
                    return (primary is not None and primary['database'] == target['database'] and
                            primary['engine'] == target['engine'] and
                            primary['session']['server_start'] == target['session']['server_start'] and
                            primary['session']['database_oid'] == target['session']['database_oid'])
                secondary['owner_consistency'] = ('matched' if owner_entries and all(map(equivalent, owner_entries)) else 'hold')
    _fence(guard)
    return {'source_sha': source_catalog['source_sha'], 'workloads': result, 'status': 'HOLD', 'startup_qualified': False,
            'limits': ['pod_inventory_completeness_supplied_by_caller',
                       'workload_image_source_and_startup_compatibility_not_verified']}


def resolve(workload, snapshot, read_key, runtime_env, runtime_dns, guard, *, variable=None):
    """Resolve primary DB from the effective Pod environment under a caller guard.

    The authenticated snapshot callback must verify the Deployment -> ReplicaSet
    -> Pod ownership chain and running container before returning this projection.
    This function alone is not a cluster admission or PostgreSQL identity proof.
    """
    raw_snapshot, raw_read, raw_runtime, raw_dns, raw_guard = snapshot, read_key, runtime_env, runtime_dns, guard
    guard = lambda: _fence(raw_guard)
    def guarded(callback, *args):
        guard()
        result = _external(callback, *args)
        guard()
        return result
    snapshot = lambda: guarded(raw_snapshot)
    read_key = lambda *args: guarded(raw_read, *args)
    runtime_env = lambda *args: guarded(raw_runtime, *args)
    runtime_dns = lambda *args: guarded(raw_dns, *args)
    try:
        if workload not in CONTAINERS:
            raise Hold('unknown_workload')
        guard()
        before = snapshot()
        _identity(workload, before)
        if getattr(raw_read, 'namespace', None) != before['namespace']:
            raise Hold('reference_namespace_mismatch')
        if (getattr(raw_runtime, 'namespace', None) != before['namespace'] or
                getattr(raw_runtime, 'workload', None) != workload or
                getattr(raw_runtime, 'pod_name', None) != before['pod_name']):
            raise Hold('runtime_target_mismatch')
        if (getattr(raw_dns, 'namespace', None) != before['namespace'] or
                getattr(raw_dns, 'workload', None) != workload or
                getattr(raw_dns, 'pod_name', None) != before['pod_name']):
            raise Hold('dns_target_mismatch')
        if workload not in OWNERS and workload != 'gateway':
            result = {'workload': workload, 'database': None, 'state': 'not_observed'}
            refs = []
        else:
            if variable is not None and variable not in SECONDARY.get(workload, {}):
                raise Hold('database_variable_invalid')
            variable = variable or ('SPRING_DATASOURCE_URL' if workload == 'auth' else
                                    'VOICE_DATABASE_URL' if workload == 'voice' else
                                    'GATEWAY_DATABASE_URL' if workload == 'gateway' else 'DATABASE_URL')
            value, refs = _env(workload, before, variable, read_key)
            if value is None:
                raise Hold('database_configuration_missing')
            if runtime_env(variable) != value:
                raise Hold('runtime_environment_mismatch')
            actual_value = value
            auth_runtime = {}
            if workload == 'auth':
                if before.get('spring_config_safe') is not True:
                    raise Hold('auth_external_config_unknown')
                policy = getattr(raw_runtime, 'auth_policy', None)
                if not callable(policy) or guarded(policy) is not True:
                    raise Hold('auth_external_config_unknown')
                for key in (*AUTH_OVERRIDE_VARS, 'AUTH_FLYWAY_ENABLED', 'JAVA_TOOL_OPTIONS'):
                    auth_runtime[key] = runtime_env(key)
                if any(auth_runtime[key] not in (None, '') for key in AUTH_OVERRIDE_VARS):
                    raise Hold('auth_external_config_unknown')
                if (auth_runtime['AUTH_FLYWAY_ENABLED'] or '').lower() != 'false':
                    raise Hold('auth_flyway_runtime_not_disabled')
                options = shlex.split(auth_runtime['JAVA_TOOL_OPTIONS'] or '')
                if any(not (option.startswith('-XX:') or option.startswith('-Djavax.net.ssl.trustStore=') or
                            option.startswith('-Djavax.net.ssl.trustStorePassword=') or
                            option.startswith('-Djavax.net.ssl.trustStoreType=')) for option in options):
                    raise Hold('auth_jvm_options_unknown')
                if not value.startswith('jdbc:'):
                    raise Hold('database_url_invalid')
                value = value[5:]
                role, more = _env(workload, before, 'SPRING_DATASOURCE_USERNAME', read_key)
                refs += more
                if role is None:
                    role = 'voice'  # application.yml default
                if runtime_env('SPRING_DATASOURCE_USERNAME') not in (role, None if role == 'voice' else role):
                    raise Hold('runtime_environment_mismatch')
            host, database, parsed_role = _url(value, before['namespace'])
            if workload != 'auth':
                role = parsed_role
            if not isinstance(role, str) or not SAFE.fullmatch(role):
                raise Hold('database_role_invalid')
            expected_db = SECONDARY.get(workload, {}).get(variable, OWNERS.get(workload, OPTIONAL_DATABASES.get(workload)))
            if expected_db is not None and database != expected_db:
                raise Hold('database_owner_mismatch')
            result = {'workload': workload, 'database': database, 'host': host,
                      'port': 5432, 'role': role, 'state': 'resolved', 'addresses': runtime_dns(host)}
        guard()
        after = snapshot()
        _identity(workload, after)
        if any(before[k] != after[k] for k in IDENTITY) or before.get('env') != after.get('env') or before.get('envFrom') != after.get('envFrom'):
            raise Hold('pod_changed')
        if workload in OWNERS or workload == 'gateway':
            if runtime_env(variable) != actual_value:
                raise Hold('runtime_environment_changed')
            if workload == 'auth':
                if (after.get('spring_config_safe') is not True or
                        guarded(policy) is not True or
                        any(runtime_env(key) != expected for key, expected in auth_runtime.items()) or
                        runtime_env('SPRING_DATASOURCE_USERNAME') not in (role, None if role == 'voice' else role)):
                    raise Hold('runtime_environment_changed')
            if runtime_dns(host) != result['addresses']:
                raise Hold('database_dns_changed')
            final = snapshot()
            _identity(workload, final)
            if any(before[k] != final[k] for k in IDENTITY) or before.get('env') != final.get('env') or before.get('envFrom') != final.get('envFrom'):
                raise Hold('pod_changed')
            if workload == 'auth' and final.get('spring_config_safe') is not True:
                raise Hold('pod_changed')
        for kind, name, key, old in refs:
            current = read_key(kind, name, key)
            if current != old:
                raise Hold('config_changed')
        guard()
        result['binding'] = {k: before[k] for k in IDENTITY}
        result['references'] = [{'kind': k, 'uid': v['uid'],
                                 'resource_version': v['resource_version']}
                                for k, n, key, v in refs if v is not None]
        return result
    except Hold:
        raise
    except Exception:
        raise Hold('observation_failed') from None


def _external(callback, *args):
    try:
        return callback(*args)
    except GuardLost:
        raise
    except Exception:
        raise Hold('observation_failed') from None


def _identity(workload, pod):
    if not isinstance(pod, dict) or any(k not in pod for k in IDENTITY):
        raise Hold('identity_missing')
    if (pod['namespace'] not in ('voice-dev', 'voice-staging') or
            pod['deployment'] != 'voice-' + workload or pod['container_name'] != CONTAINERS[workload] or
            any(not isinstance(pod[k], str) or not pod[k] for k in IDENTITY if k != 'restart_count') or
            type(pod['restart_count']) is not int or pod['restart_count'] < 0):
        raise Hold('identity_invalid')


def _env(workload, pod, variable, read_key):
    env = pod.get('env', [])
    if not isinstance(env, list) or sum(e.get('name') == variable for e in env) > 1:
        raise Hold('environment_ambiguous')
    selected = next((e for e in env if e.get('name') == variable), None)
    if selected is not None:
        if 'value' in selected and 'valueFrom' not in selected:
            return selected['value'], []
        ref = selected.get('valueFrom', {})
        if len(ref) != 1:
            raise Hold('environment_source_unknown')
        return _ref(workload, variable, ref, read_key)
    found, refs = None, []
    for source in pod.get('envFrom', []):
        prefix = source.get('prefix', '')
        if not isinstance(prefix, str):
            raise Hold('environment_source_unknown')
        if not variable.startswith(prefix):
            continue
        ref = {k: v for k, v in source.items() if k != 'prefix'}
        value, observed = _ref(workload, variable, ref, read_key, variable[len(prefix):])
        refs += observed
        if value is not None:
            found = value
    return found, refs


def _ref(workload, variable, ref, read_key, key=None):
    if len(ref) != 1:
        raise Hold('environment_source_unknown')
    ref_type, spec = next(iter(ref.items()))
    kinds = {'secretKeyRef': 'secret', 'configMapKeyRef': 'configmap',
             'secretRef': 'secret', 'configMapRef': 'configmap'}
    if ref_type not in kinds or not isinstance(spec, dict):
        raise Hold('environment_source_unknown')
    name, key = spec.get('name'), key if key is not None else spec.get('key')
    if not isinstance(name, str) or not name or not isinstance(key, str) or not key:
        raise Hold('environment_reference_invalid')
    if kinds[ref_type] == 'configmap':
        allowed = name == 'voice-app-config' and key in (variable, 'POSTGRES_USER')
    else:
        expected = (variable if variable in SECONDARY.get(workload, {}) else
                    'GAME_INTEGRATION_DATABASE_URL' if workload == 'gameintegration' else
                    workload.upper().replace('-', '_') + '_DATABASE_URL')
        allowed_name = name == 'voice-app-secrets' or (workload in ('auth', 'user') and
            re.fullmatch(r'voice-auth-user-[a-z0-9][a-z0-9-]{0,30}', name))
        allowed = bool(allowed_name and ((ref_type == 'secretKeyRef' and
                       key == ('AUTH_DB_USERNAME' if variable == 'SPRING_DATASOURCE_USERNAME' else expected)) or
                       (ref_type == 'secretRef' and name == 'voice-app-secrets' and key == variable)))
    if not allowed:
        raise Hold('environment_reference_not_allowed')
    value = read_key(kinds[ref_type], name, key)
    if value is None:
        if not spec.get('optional', False):
            raise Hold('environment_key_missing')
    elif (not isinstance(value, dict) or set(value) != {'uid', 'resource_version', 'value'} or
          any(not isinstance(value[k], str) or not value[k] for k in ('uid', 'resource_version')) or
          not (isinstance(value['value'], str) or (key is not None and value['value'] is None))):
        raise Hold('environment_reference_invalid')
    if value is not None and value['value'] is None and ref_type in ('secretKeyRef', 'configMapKeyRef') and not spec.get('optional', False):
        raise Hold('environment_key_missing')
    return (None if value is None else value['value']), [(kinds[ref_type], name, key, value)]


def _url(value, namespace):
    if not isinstance(value, str) or '$(' in value or any(c in value for c in '\r\n\x00'):
        raise Hold('database_url_invalid')
    parsed = urlsplit(value)
    base = 'voice-postgres.' + namespace
    if (parsed.scheme not in ('postgres', 'postgresql') or parsed.hostname not in
            ('voice-postgres', base, base + '.svc', base + '.svc.cluster.local') or
            parsed.port not in (None, 5432) or parsed.fragment):
        raise Hold('database_endpoint_invalid')
    params = parse_qsl(parsed.query, keep_blank_values=True, strict_parsing=True)
    if any(k != 'sslmode' or v not in ('disable', 'require', 'verify-ca', 'verify-full') for k, v in params) or len(params) > 1:
        raise Hold('database_url_options_unknown')
    db = unquote(parsed.path.removeprefix('/'))
    role = unquote(parsed.username or '')
    if not SAFE.fullmatch(db):
        raise Hold('database_name_invalid')
    return parsed.hostname, db, role
