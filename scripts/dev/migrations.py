"""Dev-only migration jobs with same-pod network readiness and SecretRefs."""
import hashlib
import json

SERVICES = ('bot', 'story', 'moderation', 'subscription', 'user', 'search',
            'social', 'chat', 'messaging', 'file', 'space', 'role',
            'notification', 'matchmaking', 'voice', 'auth')


def names(service):
    if service not in SERVICES:
        raise ValueError('unknown dev migration service')
    return (('voice-repair-auth-flyway', 'voice-auth-flyway-migrations') if service == 'auth'
            else ('voice-migrate-' + service + '-db', 'voice-' + service + '-db-migrations'))


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
                     else root / 'src/backend/migrations' / (service + '_db'))
        data = {p.name: p.read_text(encoding='utf-8') for p in sorted(directory.glob('*.sql'))}
        if not data:
            raise ValueError('migration SQL missing for ' + service)
        digest = hashlib.sha256(json.dumps(data, sort_keys=True).encode()).hexdigest()
        existing_job = next((i for i in get('jobs') if i['metadata']['name'] == job_name), None)
        existing_cm = next((i for i in get('configmaps') if i['metadata']['name'] == config_name), None)
        if existing_job:
            succeeded = existing_job.get('status', {}).get('succeeded', 0)
            previous = (existing_cm or {}).get('metadata', {}).get('annotations', {}).get('voice.io/dev-migration-hash')
            if succeeded and previous == digest:
                print('DEV_MIGRATION_ALREADY_APPLIED=' + service, flush=True)
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
