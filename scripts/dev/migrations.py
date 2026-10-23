"""Dev-only migration jobs with same-pod network readiness and SecretRefs."""
import hashlib
import json
import os
import re
from pathlib import Path

SERVICES = ('bot', 'story', 'moderation', 'subscription', 'user', 'search',
            'social', 'chat', 'realtime', 'messaging', 'file', 'space', 'role',
            'notification', 'matchmaking', 'voice', 'auth', 'gameintegration')

SPACE_EVIDENCE_CM = 'voice-space-native-installer-evidence'


def space_source_digest(root):
    """Same relative-path byte manifest as both trusted Docker build stages."""
    base = Path(root) / 'src/backend'
    entries = []
    for path in sorted(base.rglob('*.go'), key=lambda p: p.relative_to(base).as_posix().encode()):
        if path.name.endswith('_test.go'):
            continue
        entries.append(hashlib.sha256(path.read_bytes()).hexdigest() + '  ./' + path.relative_to(base).as_posix() + '\n')
    if not entries:
        raise ValueError('Space installer build source is missing')
    return hashlib.sha256(''.join(entries).encode()).hexdigest()


def space_image():
    image = os.environ.get('SPACE_NATIVE_INSTALLER_IMAGE', '')
    if not re.fullmatch(r'[A-Za-z0-9._/:\-]+@sha256:[a-f0-9]{64}', image):
        raise ValueError('SPACE_NATIVE_INSTALLER_IMAGE must be an actual release source-bound image digest')
    return image


def space_input_digest(root, image):
    sql = Path(root) / 'src/backend/migrations/space_db'
    files = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(sql.glob('*.sql'))}
    return hashlib.sha256(json.dumps({'image': image, 'source': space_source_digest(root), 'sql': files}, sort_keys=True).encode()).hexdigest()


def space_evidence(root, namespace, image, previous_job, pods, log):
    """Retain observations from the authenticated controller's actual Job API.

    This artifact introduces no key, bearer permission or DB marker authority.
    The real installer still authenticates the full current source and target.
    """
    expected = space_input_digest(root, image)
    meta = previous_job.get('metadata', {})
    podspec = previous_job.get('spec', {}).get('template', {}).get('spec', {})
    containers = podspec.get('containers', [])
    if (not meta.get('uid') or previous_job.get('status', {}).get('succeeded') != 1
            or meta.get('annotations', {}).get('voice.io/native-input-hash') != expected
            or len(containers) != 1 or containers[0].get('image') != image):
        raise ValueError('HOLD: no completed source/image-bound native installer Job')
    owned = [p for p in pods if any(r.get('uid') == meta['uid'] for r in p.get('metadata', {}).get('ownerReferences', []))]
    if len(owned) != 1 or not any(s.get('imageID') for s in owned[0].get('status', {}).get('containerStatuses', [])):
        raise ValueError('HOLD: original installer Pod/image identity is unavailable')
    actual = owned[0]
    requested = actual.get('spec', {}).get('containers', [])
    statuses = [s for s in actual.get('status', {}).get('containerStatuses', []) if s.get('name') == containers[0].get('name')]
    expected_image_digest = image.rsplit('sha256:', 1)[1]
    if (len(requested) != 1 or requested[0].get('image') != image or len(statuses) != 1
            or statuses[0].get('imageID', '').rsplit('sha256:', 1)[-1] != expected_image_digest):
        raise ValueError('HOLD: original installer runtime image differs from selected platform digest')
    if isinstance(log, bytes):
        log = log.decode('utf8')
    if len(log.encode()) > 128 * 1024:
        raise ValueError('HOLD: oversized installer evidence')
    completed = [row.get('receipt') for row in map(json.loads, log.splitlines()) if row.get('event') == 'complete']
    if (len(completed) != 1 or not completed[0].get('created')
            or completed[0].get('source_sha256') != space_source_digest(root)
            or not all(completed[0].get(k) for k in ('creation', 'trusted44', 'transition45', 'transition50', 'final'))):
        raise ValueError('HOLD: actual complete linked installer output missing')
    return {'apiVersion': 'v1', 'kind': 'ConfigMap', 'immutable': True,
            'metadata': {'name': SPACE_EVIDENCE_CM, 'namespace': namespace,
                         'annotations': {'voice.io/native-input-hash': expected}},
            'data': {'prior.jsonl': log, 'origin-job.json': json.dumps(previous_job, sort_keys=True),
                     'origin-pod.json': json.dumps(owned[0], sort_keys=True)}}


def apply_space(root, postgres_image, apply, run, get):
    image = space_image()
    expected = space_input_digest(root, image)
    job_name, config_name = names('space')
    existing = next((x for x in get('jobs') if x['metadata']['name'] == job_name), None)
    evidence = next((x for x in get('configmaps') if x['metadata']['name'] == SPACE_EVIDENCE_CM), None)
    if evidence and evidence.get('metadata', {}).get('annotations', {}).get('voice.io/native-input-hash') != expected:
        raise ValueError('HOLD: retained native installer build/source/image changed')
    if existing:
        if existing.get('status', {}).get('active', 0):
            raise ValueError('native installer Job is already active')
        if not evidence:
            output = run('kubectl', '-n', 'voice-dev', 'logs', 'job/' + job_name).stdout
            evidence = space_evidence(root, 'voice-dev', image, existing, get('pods'), output)
            apply([evidence])
        run('kubectl', '-n', 'voice-dev', 'delete', 'job', job_name, '--wait=true')
    sql = Path(root) / 'src/backend/migrations/space_db'
    config = {'apiVersion': 'v1', 'kind': 'ConfigMap',
              'metadata': {'name': config_name, 'namespace': 'voice-dev'},
              'data': {p.name: p.read_bytes().decode('utf8') for p in sorted(sql.glob('*.sql'))}}
    rendered = job('space', postgres_image)
    rendered['metadata']['annotations'] = {'voice.io/native-input-hash': expected}
    container = rendered['spec']['template']['spec']['containers'][0]
    container['env'].append({'name': 'SPACE_NATIVE_REQUIRED_SOURCE_SHA256', 'value': space_source_digest(root)})
    if evidence:
        container['env'].append({'name': 'SPACE_NATIVE_INSTALLER_MODE', 'value': 'verify'})
    apply([config, rendered])
    run('kubectl', '-n', 'voice-dev', 'wait', '--for=condition=complete', 'job/' + job_name, '--timeout=600s', timeout=660)
    if not evidence:
        current = next(x for x in get('jobs') if x['metadata']['name'] == job_name)
        output = run('kubectl', '-n', 'voice-dev', 'logs', 'job/' + job_name).stdout
        apply([space_evidence(root, 'voice-dev', image, current, get('pods'), output)])
    print('DEV_NATIVE_SPACE_VERIFIED' if evidence else 'DEV_NATIVE_SPACE_PROVISIONED', flush=True)

MIGRATION_DATABASES = {'gameintegration': 'game_integration_db'}
GAMEINTEGRATION_IMAGES = {
    'postgres': 'docker.io/library/postgres@sha256:e013e867e712fec275706a6c51c966f0bb0c93cfa8f51000f85a15f9865a28cb',
    'migrate': 'migrate/migrate@sha256:f21c436af23c282f4516b00ba3e93bccf5c5fe5cd52530fd5c319a936998f539',
}
GAMEINTEGRATION_JOBS = {
    'provision': 'voice-provision-gameintegration-runtime-role',
    'migrate': 'voice-migrate-gameintegration-db',
    'verify': 'voice-verify-gameintegration-db',
}
GAMEINTEGRATION_SECRET_REFS = {
    'provision_admin': ('voice-app-secrets', 'POSTGRES_PASSWORD'),
    'runtime_password': ('voice-app-secrets', 'GAME_INTEGRATION_DB_PASSWORD'),
    'migration_admin': ('voice-app-secrets', 'POSTGRES_PASSWORD'),
    'verification_admin': ('voice-app-secrets', 'POSTGRES_PASSWORD'),
    'runtime_database_url': ('voice-app-secrets', 'GAME_INTEGRATION_DATABASE_URL'),
}


def gameintegration_prerequisite_inputs(root):
    """Return source-bound, non-secret inputs for the protected GIS DB jobs."""
    root = Path(root)
    migration_dir = root / 'src/backend/migrations/game_integration_db'
    migrations = []
    for path in sorted(migration_dir.glob('*.sql')):
        raw = path.read_bytes()
        migrations.append({'name': path.name, 'sha256': hashlib.sha256(raw).hexdigest()})
    if (not migrations or any(not item['name'].endswith(('.up.sql', '.down.sql')) for item in migrations)
            or len(migrations) % 2):
        raise ValueError('GIS migration inputs must contain ordered up/down SQL files')

    def digest(path):
        return hashlib.sha256((root / path).read_bytes()).hexdigest()

    return {
        'schema': 'voice-gis-prerequisite-v1',
        'database': 'game_integration_db',
        'configmaps': {
            'provision': 'voice-gameintegration-runtime-role-sql',
            'migrations': 'voice-gameintegration-db-migrations',
            'verify': 'voice-gameintegration-db-verifier-sql',
        },
        'migrations': migrations,
        'migration_inputs_sha256': hashlib.sha256(
            json.dumps(migrations, sort_keys=True, separators=(',', ':')).encode()).hexdigest(),
        'provision_sql_sha256': digest('docker/postgres/provision-gameintegration-runtime-role.sql'),
        'verifier_sql_sha256': digest('docker/postgres/verify-gameintegration-runtime-role.sql'),
        'templates': {
            role: digest(path) for role, path in {
                'provision': 'deploy/templates/provision-gameintegration-runtime-role-job.yaml',
                'migrate': 'deploy/templates/migrate-game-integration-db-job.yaml',
                'verify': 'deploy/templates/verify-game-integration-db-job.yaml',
            }.items()
        },
        'images': dict(GAMEINTEGRATION_IMAGES),
        'job_images': {
            'provision': {'wait-postgres': 'postgres', 'provision': 'postgres'},
            'migrate': {'wait-postgres': 'postgres', 'migrate': 'migrate'},
            'verify': {'wait-postgres': 'postgres', 'verify': 'postgres'},
        },
        'jobs': dict(GAMEINTEGRATION_JOBS),
        'secret_refs': {name: {'name': ref[0], 'key': ref[1]}
                        for name, ref in GAMEINTEGRATION_SECRET_REFS.items()},
    }


def gameintegration_verifier_configmap(root, namespace):
    """Project the fixed verifier SQL as an immutable, source-derived input."""
    if namespace not in ('voice-dev', 'voice-staging'):
        raise ValueError('GIS prerequisite jobs are not enabled for this namespace')
    sql = (Path(root) / 'docker/postgres/verify-gameintegration-runtime-role.sql').read_text(encoding='utf-8')
    return {
        'apiVersion': 'v1', 'kind': 'ConfigMap',
        'metadata': {'name': 'voice-gameintegration-db-verifier-sql', 'namespace': namespace},
        'immutable': True,
        'data': {'verify.sql': sql},
    }


def gameintegration_verifier_job(namespace='voice-dev'):
    if namespace not in ('voice-dev', 'voice-staging'):
        raise ValueError('GIS prerequisite jobs are not enabled for this namespace')
    image = GAMEINTEGRATION_IMAGES['postgres']
    resources = {'requests': {'cpu': '20m', 'memory': '64Mi'},
                 'limits': {'cpu': '250m', 'memory': '256Mi'}}
    return {
        'apiVersion': 'batch/v1', 'kind': 'Job',
        'metadata': {'name': GAMEINTEGRATION_JOBS['verify'], 'namespace': namespace},
        'spec': {'backoffLimit': 0, 'activeDeadlineSeconds': 300, 'ttlSecondsAfterFinished': 86400,
                 'template': {'metadata': {'labels': {'app': GAMEINTEGRATION_JOBS['verify']}},
                              'spec': {'restartPolicy': 'Never', 'automountServiceAccountToken': False,
                                       'initContainers': [{'name': 'wait-postgres', 'image': image,
                                                           'command': ['/bin/sh', '-ec',
                                                                       'until pg_isready -h voice-postgres -p 5432 -U "${POSTGRES_USER}"; do sleep 2; done'],
                                                           'env': [{'name': 'POSTGRES_USER', 'valueFrom': {'configMapKeyRef': {'name': 'voice-app-config', 'key': 'POSTGRES_USER'}}}],
                                                           'resources': resources}],
                                       'containers': [{'name': 'verify', 'image': image,
                                                       'command': ['/bin/sh', '-ec',
                                                                   'exec psql -X -qAt -v ON_ERROR_STOP=1 -h voice-postgres -p 5432 -U "${POSTGRES_USER}" -d game_integration_db -f /verify/verify.sql'],
                                                       'env': [{'name': 'POSTGRES_USER', 'valueFrom': {'configMapKeyRef': {'name': 'voice-app-config', 'key': 'POSTGRES_USER'}}},
                                                               {'name': 'PGPASSWORD', 'valueFrom': {'secretKeyRef': {'name': 'voice-app-secrets', 'key': 'POSTGRES_PASSWORD'}}}],
                                                       'resources': resources,
                                                       'volumeMounts': [{'name': 'verifier', 'mountPath': '/verify', 'readOnly': True}]}],
                                       'volumes': [{'name': 'verifier', 'configMap': {'name': 'voice-gameintegration-db-verifier-sql'}}]}}}}


def finalize_gameintegration(root, apply, run, get):
    """Reassert ledger protection after GIS provisioning and migration runs."""
    job_name = GAMEINTEGRATION_JOBS['verify']
    configmap = gameintegration_verifier_configmap(root, 'voice-dev')
    existing_job = next((item for item in get('jobs') if item['metadata']['name'] == job_name), None)
    if existing_job:
        if existing_job.get('status', {}).get('active', 0):
            raise ValueError('GIS verifier Job is already active')
        run('kubectl', '-n', 'voice-dev', 'delete', 'job', job_name, '--wait=true')
    existing_cm = next((item for item in get('configmaps')
                        if item['metadata']['name'] == configmap['metadata']['name']), None)
    if existing_cm and existing_cm.get('data') != configmap['data']:
        run('kubectl', '-n', 'voice-dev', 'delete', 'configmap', configmap['metadata']['name'], '--wait=true')
    apply([configmap, gameintegration_verifier_job()])
    run('kubectl', '-n', 'voice-dev', 'wait', '--for=condition=complete',
        'job/' + job_name, '--timeout=300s', timeout=330)
    print('DEV_MIGRATION_VERIFIED=gameintegration', flush=True)


def names(service):
    if service not in SERVICES:
        raise ValueError('unknown dev migration service')
    return (('voice-repair-auth-flyway', 'voice-auth-flyway-migrations') if service == 'auth'
            else ('voice-migrate-' + service + '-db', 'voice-' + service + '-db-migrations'))


def gameintegration_runtime_role_job(postgres_image):
    resources = {'requests': {'cpu': '20m', 'memory': '64Mi'}, 'limits': {'cpu': '500m', 'memory': '512Mi'}}
    return {'apiVersion': 'batch/v1', 'kind': 'Job',
            'metadata': {'name': 'voice-provision-gameintegration-runtime-role', 'namespace': 'voice-dev'},
            'spec': {'backoffLimit': 2, 'activeDeadlineSeconds': 600, 'ttlSecondsAfterFinished': 86400,
                     'template': {'metadata': {'labels': {'app': 'voice-provision-gameintegration-runtime-role'}},
                                  'spec': {'restartPolicy': 'OnFailure', 'automountServiceAccountToken': False,
                                           'initContainers': [{'name': 'wait-postgres', 'image': postgres_image,
                                                               'command': ['/bin/sh', '-ec', 'until pg_isready -h voice-postgres -p 5432 -U voice; do sleep 2; done'],
                                                               'resources': resources}],
                                           'containers': [{'name': 'provision', 'image': postgres_image,
                                                           'command': ['/bin/sh', '-ec',
                                                                       'exec psql -v ON_ERROR_STOP=1 -h voice-postgres -p 5432 -U voice -d postgres -f /provision/provision.sql'],
                                                           'env': [{'name': 'PGPASSWORD', 'valueFrom': {'secretKeyRef': {'name': 'voice-app-secrets', 'key': 'POSTGRES_PASSWORD'}}},
                                                                   {'name': 'GAME_INTEGRATION_DB_PASSWORD', 'valueFrom': {'secretKeyRef': {'name': 'voice-app-secrets', 'key': 'GAME_INTEGRATION_DB_PASSWORD'}}}],
                                                           'volumeMounts': [{'name': 'provisioner', 'mountPath': '/provision', 'readOnly': True}],
                                                           'resources': resources}],
                                           'volumes': [{'name': 'provisioner', 'configMap': {'name': 'voice-gameintegration-runtime-role-sql'}}]}}}}


def job(service, postgres_image):
    job_name, config_name = names(service)
    container = {
        'name': 'migrate', 'image': 'migrate/migrate:v4.18.1',
        'command': ['/bin/sh', '-ec', 'exec migrate -path=/migrations -database "$DATABASE_URL" up'],
        'env': [{'name': 'DATABASE_URL', 'valueFrom': {'secretKeyRef': {'name': 'voice-app-secrets', 'key': service.upper() + '_DATABASE_URL'}}}],
        'volumeMounts': [{'name': 'migrations', 'mountPath': '/migrations', 'readOnly': True}]}
    if service == 'auth':
        container = {
            'name': 'flyway', 'image': 'flyway/flyway:11.3.0', 'args': ['migrate'],
            'env': [{'name': 'FLYWAY_URL', 'value': 'jdbc:postgresql://voice-postgres:5432/auth_db'},
                    {'name': 'FLYWAY_USER', 'value': 'voice'},
                    {'name': 'FLYWAY_PASSWORD', 'valueFrom': {'secretKeyRef': {'name': 'voice-app-secrets', 'key': 'POSTGRES_PASSWORD'}}},
                    {'name': 'FLYWAY_LOCATIONS', 'value': 'filesystem:/flyway/sql'}],
            'volumeMounts': [{'name': 'migrations', 'mountPath': '/flyway/sql', 'readOnly': True}]}
    elif service == 'gameintegration':
        # Schema DDL runs with the existing Postgres migration authority. The
        # application still receives only GAME_INTEGRATION_DATABASE_URL.
        container = {
            'name': 'migrate', 'image': 'migrate/migrate:v4.18.1',
            'command': ['/bin/sh', '-ec',
                        'exec migrate -path=/migrations -database "postgres://${POSTGRES_USER}:${PGPASSWORD}@voice-postgres:5432/game_integration_db?sslmode=disable" up'],
            'env': [{'name': 'POSTGRES_USER', 'valueFrom': {'configMapKeyRef': {'name': 'voice-app-config', 'key': 'POSTGRES_USER'}}},
                    {'name': 'PGPASSWORD', 'valueFrom': {'secretKeyRef': {'name': 'voice-app-secrets', 'key': 'POSTGRES_PASSWORD'}}}],
            'volumeMounts': [{'name': 'migrations', 'mountPath': '/migrations', 'readOnly': True}]}
    elif service == 'space':
        container = {
            'name': 'migrate', 'image': space_image(),
            'command': ['/bin/sh', '-ec', 'if [ "${SPACE_NATIVE_INSTALLER_MODE:-provision}" = verify ]; then exec native-migrate --mode=verify --prior-evidence=/evidence/prior.jsonl; fi; exec native-migrate --mode=provision'],
            'env': [{'name': 'PGHOST', 'value': 'voice-postgres'},
                    {'name': 'PGUSER', 'valueFrom': {'configMapKeyRef': {'name': 'voice-app-config', 'key': 'POSTGRES_USER'}}},
                    {'name': 'PGPASSWORD', 'valueFrom': {'secretKeyRef': {'name': 'voice-app-secrets', 'key': 'POSTGRES_PASSWORD'}}},
                    {'name': 'PGSSLMODE', 'value': 'disable'}],
            'volumeMounts': [{'name': 'migrations', 'mountPath': '/migrations', 'readOnly': True},
                             {'name': 'native-evidence', 'mountPath': '/evidence', 'readOnly': True}]}
    resources = {'requests': {'cpu': '20m', 'memory': '64Mi'}, 'limits': {'cpu': '500m', 'memory': '512Mi'}}
    container['resources'] = resources
    result = {'apiVersion': 'batch/v1', 'kind': 'Job', 'metadata': {'name': job_name, 'namespace': 'voice-dev'},
            'spec': {'backoffLimit': 2, 'activeDeadlineSeconds': 600, 'ttlSecondsAfterFinished': 86400,
                     'template': {'metadata': {'labels': {'app': job_name}}, 'spec': {
                         'restartPolicy': 'OnFailure', 'automountServiceAccountToken': False,
                         'initContainers': [{'name': 'wait-postgres', 'image': postgres_image,
                                             'command': ['/bin/sh', '-ec', 'until pg_isready -h voice-postgres -p 5432 -U voice; do sleep 2; done'],
                                             'resources': resources}],
                         'containers': [container],
                         'volumes': [{'name': 'migrations', 'configMap': {'name': config_name}}]}}}}
    if service == 'space':
        result['spec']['backoffLimit'] = 0
        pod = result['spec']['template']['spec']
        pod['restartPolicy'] = 'Never'
        pod['volumes'].append({'name': 'native-evidence', 'configMap': {'name': SPACE_EVIDENCE_CM, 'optional': True}})
    return result


def apply_all(root, postgres_image, apply, run, get):
    if 'space' in SERVICES:
        space_image()  # Fail before any migration if release image custody is absent.
    for service in SERVICES:
        if service == 'space':
            apply_space(root, postgres_image, apply, run, get)
            continue
        job_name, config_name = names(service)
        directory = (root / 'src/backend/auth/src/main/resources/db/migration' if service == 'auth'
                     else root / 'src/backend/migrations' / MIGRATION_DATABASES.get(service, service + '_db'))
        data = {p.name: p.read_text(encoding='utf-8') for p in sorted(directory.glob('*.sql'))}
        if not data:
            raise ValueError('migration SQL missing for ' + service)
        if service == 'gameintegration':
            provision_name = 'voice-provision-gameintegration-runtime-role'
            existing_provision = next((i for i in get('jobs') if i['metadata']['name'] == provision_name), None)
            if existing_provision:
                if existing_provision.get('status', {}).get('active', 0):
                    raise ValueError('runtime role provision Job is already active: ' + provision_name)
                run('kubectl', '-n', 'voice-dev', 'delete', 'job', provision_name, '--wait=true')
            provision_sql = (root / 'docker/postgres/provision-gameintegration-runtime-role.sql').read_text(encoding='utf-8')
            provision_config = {'apiVersion': 'v1', 'kind': 'ConfigMap',
                                'metadata': {'name': 'voice-gameintegration-runtime-role-sql', 'namespace': 'voice-dev'},
                                'data': {'provision.sql': provision_sql}}
            apply([provision_config, gameintegration_runtime_role_job(postgres_image)])
            run('kubectl', '-n', 'voice-dev', 'wait', '--for=condition=complete',
                'job/' + provision_name, '--timeout=600s', timeout=660)
        digest = hashlib.sha256(json.dumps(data, sort_keys=True).encode()).hexdigest()
        existing_job = next((i for i in get('jobs') if i['metadata']['name'] == job_name), None)
        existing_cm = next((i for i in get('configmaps') if i['metadata']['name'] == config_name), None)
        if existing_job:
            succeeded = existing_job.get('status', {}).get('succeeded', 0)
            previous = (existing_cm or {}).get('metadata', {}).get('annotations', {}).get('voice.io/dev-migration-hash')
            if succeeded and previous == digest:
                print('DEV_MIGRATION_ALREADY_APPLIED=' + service, flush=True)
                if service == 'gameintegration':
                    finalize_gameintegration(root, apply, run, get)
                continue
            if existing_job.get('status', {}).get('active', 0):
                raise ValueError('migration job is already active: ' + job_name)
            run('kubectl', '-n', 'voice-dev', 'delete', 'job', job_name, '--wait=true')
        config = {'apiVersion': 'v1', 'kind': 'ConfigMap',
                  'metadata': {'name': config_name, 'namespace': 'voice-dev',
                               'annotations': {'voice.io/dev-migration-hash': digest}}, 'data': data}
        apply([config, job(service, postgres_image)])
        run('kubectl', '-n', 'voice-dev', 'wait', '--for=condition=complete', 'job/' + job_name, '--timeout=600s', timeout=660)
        print('DEV_MIGRATION_APPLIED=' + service, flush=True)
        if service == 'gameintegration':
            finalize_gameintegration(root, apply, run, get)


if __name__ == '__main__':
    import sys
    if len(sys.argv) == 3 and sys.argv[1] == 'native-space-source':
        print(space_source_digest(Path(sys.argv[2])))
    elif len(sys.argv) == 4 and sys.argv[1] == 'native-space-input':
        print(space_input_digest(Path(sys.argv[2]), sys.argv[3]))
    elif len(sys.argv) == 8 and sys.argv[1] == 'native-space-evidence':
        print(json.dumps(space_evidence(Path(sys.argv[2]), sys.argv[3], sys.argv[4], json.loads(Path(sys.argv[5]).read_text()), json.loads(Path(sys.argv[6]).read_text())['items'], Path(sys.argv[7]).read_bytes())))
    else:
        raise SystemExit('unsupported native installer adapter command')
