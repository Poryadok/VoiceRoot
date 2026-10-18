"""Dev-only migration jobs with same-pod network readiness and SecretRefs."""
import hashlib
import json
from pathlib import Path

SERVICES = ('bot', 'story', 'moderation', 'subscription', 'user', 'search',
            'social', 'chat', 'messaging', 'file', 'space', 'role',
            'notification', 'matchmaking', 'voice', 'auth', 'gameintegration')

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
    resources = {'requests': {'cpu': '20m', 'memory': '64Mi'}, 'limits': {'cpu': '500m', 'memory': '512Mi'}}
    container['resources'] = resources
    return {'apiVersion': 'batch/v1', 'kind': 'Job', 'metadata': {'name': job_name, 'namespace': 'voice-dev'},
            'spec': {'backoffLimit': 2, 'activeDeadlineSeconds': 600, 'ttlSecondsAfterFinished': 86400,
                     'template': {'metadata': {'labels': {'app': job_name}}, 'spec': {
                         'restartPolicy': 'OnFailure', 'automountServiceAccountToken': False,
                         'initContainers': [{'name': 'wait-postgres', 'image': postgres_image,
                                             'command': ['/bin/sh', '-ec', 'until pg_isready -h voice-postgres -p 5432 -U voice; do sleep 2; done'],
                                             'resources': resources}],
                         'containers': [container],
                         'volumes': [{'name': 'migrations', 'configMap': {'name': config_name}}]}}}}


def apply_all(root, postgres_image, apply, run, get):
    for service in SERVICES:
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
