"""Guarded Space v25 feature probe. No runtime values are returned or logged.

The caller must first authenticate the P1 source snapshot and candidate image,
own the short prepare-only guard, and verify its workload and DB binding. This
probe never proves Space's source catalog, startup, or migration eligibility.
"""
import json
from urllib.parse import urlsplit
import full_cohort_schema_metadata as common

Hold = common.Hold
SOURCE_FILES = {
    'src/backend/space/main.go': '0305026e763ee771c7893ade9c24aa0455490dd96e07a42915a42883a6057ce8',
    'src/backend/space/lifecycle_runtime_config.go': '2f9ca8eaf2d8e8a2744ba99ff3b4ac2b2f79eb508275797053532d57aca4445c',
    'src/backend/space/internal/chatcreation/runtime.go': '489415b9fd4c137e0730acb1134cf253a3eca96041a8fb7a7499c588c45d7fad',
    'src/backend/space/internal/lifecycleprincipal/runtime.go': '58593d8821d53daa8113e0d05b7f380bf0b9480a84a416a07c55d8dd46eef0c8',
    'src/backend/space/internal/auditprincipal/config.go': 'c43699af224e6025cd9e1b52854fe24be5385fda8f69c85fc7697b6ef1359516',
    'src/backend/space/internal/botmembershipprincipal/config.go': '262fd5f72b208a9741afd3cea173642c7e58f4ca0fd4281c8e4e84782fd320a4',
    'src/backend/pkg/authoritysource/runtime_config.go': '19035d9eb915f29de1ed221148e72cd6723fe4c2023c7f76a8941dc5cce29b67',
    'src/backend/pkg/socialprincipal/config.go': '19c89b0e9ece048b0c1658b511621490431a134bf3143d1df312e4721832180c',
    'src/backend/space/communication_runtime.go': '197107b33ca88d21a978ce1656c54c91a43da687df44810d7178d519bf617ab6',
}
COMPOSITE = tuple('SPACE_CHAT_COMPOSITE_' + s for s in (
    'GRPC_LISTEN', 'TLS_CERT_FILE', 'TLS_KEY_FILE', 'CLIENT_CA_FILE', 'PRINCIPAL_JWKS_URL',
    'PRINCIPAL_JWKS_CA_FILE', 'PRINCIPAL_REDIS_ADDR', 'PRINCIPAL_REDIS_PASSWORD',
    'PRINCIPAL_JWKS_CLIENT_CERT_FILE', 'PRINCIPAL_JWKS_CLIENT_KEY_FILE'))
GATEWAY = tuple('SPACE_GATEWAY_' + s for s in (
    'LIFECYCLE_GRPC_LISTEN', 'LIFECYCLE_TLS_CERT_FILE', 'LIFECYCLE_TLS_KEY_FILE',
    'LIFECYCLE_CLIENT_CA_FILE', 'PRINCIPAL_JWKS_URL', 'PRINCIPAL_JWKS_CA_FILE',
    'PRINCIPAL_REDIS_ADDR', 'PRINCIPAL_REDIS_PASSWORD'))
def names(prefix, suffixes):
    return tuple(prefix + s for s in suffixes)
AUDIT = names('SPACE_AUDIT_PRINCIPAL_', ('GRPC_LISTEN', 'TLS_CERT_FILE', 'TLS_KEY_FILE',
                                        'CLIENT_CA_FILE', 'REPLAY_REDIS_ADDR', 'REPLAY_REDIS_PASSWORD'))
BOT = names('SPACE_BOT_MEMBERSHIP_', ('GRPC_LISTEN', 'TLS_CERT_FILE', 'TLS_KEY_FILE',
                                     'CLIENT_CA_FILE', 'REPLAY_REDIS_ADDR', 'REPLAY_REDIS_PASSWORD'))
SOURCE = names('SPACE_AUTHORITY_SOURCE_', ('ENABLED', 'JWKS_CA_FILE', 'TLS_CERT_FILE', 'TLS_KEY_FILE',
                                           'CLIENT_CA_FILE', 'REPLAY_REDIS_ADDR', 'REPLAY_REDIS_PASSWORD', 'GRPC_LISTEN'))
COMM_SUFFIX = ('REPLAY_REDIS_ADDR', 'REPLAY_REDIS_PASSWORD', 'TLS_CERT_FILE', 'TLS_KEY_FILE',
               'GRPC_LISTEN', 'JWKS_CA_FILE', 'CLIENT_CA_FILE', 'JWKS_CLIENT_CERT_FILE', 'JWKS_CLIENT_KEY_FILE')
VOICE_COMM = names('SPACE_VOICE_COMMUNICATION_', COMM_SUFFIX)
MESSAGING_COMM = names('SPACE_MESSAGING_COMMUNICATION_', COMM_SUFFIX)
KEYS = frozenset((*COMPOSITE, *GATEWAY, *AUDIT, *BOT, *SOURCE, *VOICE_COMM, *MESSAGING_COMM,
                  'SPACE_LIFECYCLE_ENABLED'))
ENV_SCRIPT = ("[ \"$(readlink /proc/1/exe)\" = /service ] || exit 1; "
              "printf 'x\\000' | (IFS= read -r -d '' p && [ \"$p\" = x ]) || exit 1; "
              "while IFS= read -r -d '' line; do case \"${line%%=*}\" in " +
              '|'.join(sorted(KEYS)) + ") printf '%s\\000' \"$line\";; esac; done </proc/1/environ")
SPACE_SQL = common.MIGRATE_SQL.replace("'ledger_rows',count(*)", "'ledger_rows',count(*),"
    "'native_guard_present',to_regnamespace('native_membership_guard') IS NOT NULL,"
    "'ordinary_heads_present',to_regclass('public.space_ordinary_member_heads') IS NOT NULL,"
    "'composite_fences_present',to_regclass('space_chat_creation_fences') IS NOT NULL,"
    "'composite_evidence_present',to_regclass('space_chat_creation_evidence') IS NOT NULL,"
    "'purge_fences_present',to_regclass('space_permanent_purge_fences') IS NOT NULL,"
    "'audit_lock_present',to_regprocedure('lock_space_audit_id_v1(uuid)') IS NOT NULL")


def decode_private(raw):
    try:
        if not isinstance(raw, bytes) or len(raw) > 65536 or (raw and not raw.endswith(b'\0')):
            raise Hold('space_private_input_invalid')
        values = {}
        for entry in raw.split(b'\0'):
            if not entry:
                continue
            name, value = entry.split(b'=', 1)
            key = name.decode('ascii')
            if key not in KEYS or key in values:
                raise Hold('space_private_input_invalid')
            values[key] = value.decode('utf-8')
        return values
    except Exception:
        raise Hold('space_private_input_invalid') from None


def _shape(values, keys, optional_password, url_key):
    if not any(values.get(k, '') != '' for k in keys):
        return {'activation': 'disabled', 'shape': 'not_applicable'}
    required = [k for k in keys if k != optional_password]
    if any(not values.get(k) or values[k] != values[k].strip() for k in required):
        return {'activation': 'configured', 'shape': 'incomplete'}
    try:
        u = urlsplit(values[url_key])
        valid = u.scheme == 'https' and bool(u.netloc) and u.username is None and not u.query and not u.fragment
    except ValueError:
        valid = False
    return {'activation': 'configured', 'shape': 'basic_shape_only' if valid else 'invalid_url'}


def classify(values):
    if not isinstance(values, dict) or not set(values) <= KEYS:
        raise Hold('space_private_input_invalid')
    result = {'composite': _shape(values, COMPOSITE, 'SPACE_CHAT_COMPOSITE_PRINCIPAL_REDIS_PASSWORD',
                                  'SPACE_CHAT_COMPOSITE_PRINCIPAL_JWKS_URL'),
              'gateway_lifecycle': _shape(values, GATEWAY, 'SPACE_GATEWAY_PRINCIPAL_REDIS_PASSWORD',
                                          'SPACE_GATEWAY_PRINCIPAL_JWKS_URL')}
    for label, keys in [('audit', AUDIT), ('bot', BOT), ('voice_communication', VOICE_COMM),
                        ('messaging_communication', MESSAGING_COMM)]:
        result[label] = {'activation': 'configured' if any(k in values for k in keys) else 'disabled',
                         'shape': 'not_evaluated'}
    flag = values.get('SPACE_AUTHORITY_SOURCE_ENABLED')
    source_other = any(k in values for k in SOURCE[1:])
    result['authority_source'] = {'activation': ('disabled' if flag == 'false' or flag is None and not source_other else
                                                  'configured' if flag == 'true' else 'invalid'),
                                  'shape': 'not_evaluated'}
    raw = values.get('SPACE_LIFECYCLE_ENABLED', '').strip()
    result['lifecycle'] = {'activation': ('disabled' if raw in ('', '0', 'f', 'F', 'false', 'FALSE', 'False') else
                                          'configured' if raw in ('1', 't', 'T', 'true', 'TRUE', 'True') else 'invalid'),
                           'shape': 'not_evaluated'}
    return result


def probe(source_snapshot, pod_name, expected_engine_role, guard,
          run_bytes=common.linux_private_bytes, run_psql=common.linux_psql):
    """Return enum-only configuration evidence, surrounded by Pod identity checks.

    The operator must separately run the real schema-locked source verifier.
    """
    try:
        if any(source_snapshot['files'].get(p) != h for p, h in SOURCE_FILES.items()):
            raise Hold('space_source_policy_unknown')
        snapshot = common.kube_workload_snapshot('space', pod_name, run_bytes, guard)
        key_reader = common.kube_key_reader(run_bytes)
        runtime = common.kube_runtime_reader('space', pod_name, run_bytes)
        dns = common.kube_dns_reader('space', pod_name, run_bytes)
        catalog = common.catalog(source_snapshot)
        binding = common.resolve('space', snapshot, key_reader, runtime, dns, guard)
        engine = common.kube_engine_snapshot(run_bytes, guard)
        baseline = common.observe(catalog, binding, engine, expected_engine_role, run_psql, guard)
        common._fence(guard)
        before = snapshot()
        args = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
                '--namespace', 'voice-dev', '--request-timeout=15s', 'exec', pod_name,
                '-c', 'space', '--', 'sh', '-ec', ENV_SCRIPT]
        common._fence(guard)
        raw = common._external(run_bytes, args)
        common._fence(guard)
        result = classify(decode_private(raw))
        common._fence(guard)
        args = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
                '--namespace', 'voice-dev', '--request-timeout=15s', 'exec', 'voice-postgres-0',
                '-c', 'postgres', '--', 'sh', '-ec', common.PSQL_SCRIPT, '--', 'space_db', SPACE_SQL]
        observed = json.loads(common._external(run_psql, args))
        common._fence(guard)
        engine_after = engine()
        if (engine_after != baseline['engine'] or observed.get('database') != 'space_db' or
                observed.get('database_oid') != baseline['session']['database_oid'] or
                observed.get('server_start') != baseline['session']['server_start'] or
                observed.get('role') != expected_engine_role or observed.get('read_only') != 'on' or
                observed.get('data_directory') != '/var/lib/postgresql/data' or
                observed.get('version') != baseline['ledger']['version'] or
                observed.get('dirty') is not False or observed.get('ledger_rows') != 1):
            raise Hold('space_metadata_changed')
        fields = ('native_guard_present', 'ordinary_heads_present', 'composite_fences_present',
                  'composite_evidence_present', 'purge_fences_present', 'audit_lock_present')
        if any(type(observed.get(field)) is not bool for field in fields):
            raise Hold('space_metadata_invalid')
        after = snapshot()
        if any(before[k] != after[k] for k in common.IDENTITY):
            raise Hold('pod_changed')
        if common.resolve('space', snapshot, key_reader, runtime, dns, guard) != binding:
            raise Hold('workload_binding_changed')
        common._fence(guard)
        return {'source_sha': source_snapshot['source_sha'],
                'binding': {k: before[k] for k in common.IDENTITY}, 'features': result,
                'ledger': baseline['ledger'], 'objects': {field: observed[field] for field in fields},
                'source_catalog': 'not_evaluated', 'status': 'HOLD',
                'limits': ['shape_is_not_full_validation', 'source_catalog_and_startup_not_qualified',
                           'objects_use_observer_search_path', 'no_existing_application_connection_peer_proof']}
    except common.GuardLost:
        raise
    except Exception:
        raise Hold('space_probe_failed') from None
