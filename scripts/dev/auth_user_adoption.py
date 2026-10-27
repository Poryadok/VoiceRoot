#!/usr/bin/env python3
"""Existing DEV Auth/User credential adoption. Private journal primitives."""
import copy
import argparse
import base64
import fcntl
import hashlib
import json
import os
import pathlib
import re
import secrets
import shlex
import stat
import subprocess
import tempfile
import sys


class RecoveryRequired(RuntimeError):
    pass


def deployment_plan(original, service, image, secret):
    if service not in ('auth', 'user'):
        raise RecoveryRequired('only Auth/User admitted')
    meta = original.get('metadata', {})
    if (original.get('kind') != 'Deployment' or meta.get('namespace') != 'voice-dev'
            or meta.get('name') != 'voice-' + service
            or any(meta.get('labels', {}).get(k) != v for k, v in LABELS.items())
            or not meta.get('uid') or not meta.get('resourceVersion')
            or not re.fullmatch(r'[a-z0-9][a-z0-9./:_-]*@sha256:[a-f0-9]{64}', image)
            or not re.fullmatch(r'[a-z0-9][a-z0-9-]{0,62}', secret)):
        raise RecoveryRequired('deployment or candidate identity invalid')
    expected = copy.deepcopy(original)
    spec = expected['spec']['template']['spec']
    matches = [c for c in spec['containers'] if c['name'] == service]
    if len(matches) != 1:
        raise RecoveryRequired('exact application container required')
    main = matches[0]
    if any('secretRef' in source for source in main.get('envFrom', [])):
        raise RecoveryRequired('broad Secret environment not admitted')
    names = [e['name'] for e in main.get('env', [])]
    if len(names) != len(set(names)):
        raise RecoveryRequired('duplicate environment binding')
    if service == 'auth':
        if {'name': 'AUTH_FLYWAY_ENABLED', 'value': 'false'} not in main['env']:
            raise RecoveryRequired('runtime Flyway must remain disabled')
        urls=[e.get('value') for e in main['env'] if e['name']=='SPRING_DATASOURCE_URL']
        if urls!=['jdbc:postgresql://voice-postgres:5432/auth_db']:
            raise RecoveryRequired('Auth must retain its exact own database URL')
        forbidden={'POSTGRES_PASSWORD','USER_DB_PASSWORD','USER_DATABASE_URL'}
        for entry in main['env']:
            if entry['name'] in ('SPRING_DATASOURCE_USERNAME','SPRING_DATASOURCE_PASSWORD'):
                continue
            ref=entry.get('valueFrom',{}).get('secretKeyRef',{})
            if (entry['name'] in forbidden or ref.get('key') in forbidden
                    or 'user_db' in entry.get('value','').lower()):
                raise RecoveryRequired('Auth receives unrelated database credentials')
        for volume in spec.get('volumes',[]):
            ref=volume.get('secret')
            if ref and ref.get('secretName') in ('voice-app-secrets',secret):
                if 'items' not in ref or any(i['key'] in forbidden for i in ref['items']):
                    raise RecoveryRequired('Auth broad shared/runtime Secret projection refused')
        wanted = {'SPRING_DATASOURCE_USERNAME': 'AUTH_DB_USERNAME',
                  'SPRING_DATASOURCE_PASSWORD': 'AUTH_DB_PASSWORD'}
    else:
        wanted = {'DATABASE_URL': 'USER_DATABASE_URL'}
    if not set(wanted).issubset(names):
        raise RecoveryRequired('original datasource bindings missing')
    main['image'] = image
    main['env'] = [({'name': e['name'], 'valueFrom': {'secretKeyRef': {
        'name': secret, 'key': wanted[e['name']]}}} if e['name'] in wanted else e) for e in main['env']]
    patch = [{'op': 'test', 'path': '/metadata/uid', 'value': meta['uid']},
             {'op': 'test', 'path': '/metadata/resourceVersion', 'value': meta['resourceVersion']},
             {'op': 'test', 'path': '/spec', 'value': original['spec']},
             {'op': 'replace', 'path': '/spec', 'value': expected['spec']}]
    return patch, expected


def operation_secret(operation, credentials):
    if (not re.fullmatch(r'[a-z0-9][a-z0-9-]{0,30}', operation)
            or set(credentials) != {'auth', 'user'}
            or any(not re.fullmatch(r'[A-Za-z0-9_-]{32,128}', v) for v in credentials.values())
            or credentials['auth'] == credentials['user']):
        raise RecoveryRequired('operation credential identity invalid')
    data = {'AUTH_DB_USERNAME': 'auth_runtime', 'AUTH_DB_PASSWORD': credentials['auth'],
            'USER_DB_PASSWORD': credentials['user'], 'USER_DATABASE_URL':
            'postgres://user_runtime:' + credentials['user'] + '@voice-postgres:5432/user_db?sslmode=disable'}
    return {'apiVersion': 'v1', 'kind': 'Secret', 'type': 'Opaque', 'immutable': True,
            'metadata': {'namespace': 'voice-dev', 'name': 'voice-auth-user-' + operation,
                         'labels': dict(LABELS), 'annotations': {'voice.dev/adoption-operation': operation}},
            'data': {k: base64.b64encode(v.encode()).decode() for k, v in data.items()}}


def sql_phases(root):
    folder = pathlib.Path(root) / 'docker/postgres'
    text = (folder / 'provision-auth-user-runtime-roles.sql').read_text(encoding='utf-8')
    # Reuse the canonical grants bytewise; fail closed if its section format changes.
    parts = text.split('\\connect auth_db\n')
    if len(parts) != 2:
        raise RecoveryRequired('canonical Auth grants section changed')
    grants = parts[1].split('\\connect user_db\n')
    if len(grants) != 2 or any('ALTER ROLE' in p or '\\connect' in p for p in grants):
        raise RecoveryRequired('canonical User grants section changed')
    header = "\\set ON_ERROR_STOP on\nSET log_statement='none';\nSET log_min_error_statement='panic';\n"
    return {'roles': (folder / 'adopt-auth-user-runtime-roles.sql').read_text(encoding='utf-8'),
            'grants_auth': header + grants[0], 'grants_user': header + grants[1]}


LABELS = {'voice.environment': 'dev', 'app.kubernetes.io/managed-by': 'voice-dev'}


def terminal_job(job, expected_spec, operation, sessions, dirty):
    meta, status = job.get('metadata', {}), job.get('status', {})
    if (not meta.get('uid') or meta.get('annotations', {}).get('voice.dev/adoption-operation') != operation
            or job.get('spec') != expected_spec or meta.get('deletionTimestamp')):
        return {'status': 'conflict'}
    if sessions or status.get('active', 0):
        return {'status': 'running'}
    if dirty or status.get('failed', 0):
        return {'status': 'conflict'}
    terminal = any(c.get('type') == 'Complete' and c.get('status') == 'True'
                   for c in status.get('conditions', []))
    if not terminal or status.get('succeeded', 0) != 1:
        return {'status': 'running'}
    return {'status': 'complete', 'receipt': {'uid': meta['uid'],
            'spec_sha256': hashlib.sha256(json.dumps(expected_spec, sort_keys=True).encode()).hexdigest()}}


def old_incarnations_drained(original, pods, replicasets, sessions):
    old_pods = {p['uid'] for p in original['pods']}
    old_rs = {p['rs_uid'] for p in original['pods']}
    old_ips = {p['ip'] for p in original['pods']}
    identities = {(s['pid'], s['backend_start']) for s in original['sessions']}
    if any(p['uid'] in old_pods for p in pods):
        return False
    if any(r['uid'] in old_rs and r.get('replicas', 0) != 0 for r in replicasets):
        return False
    for session in sessions:
        if ((session.get('pid'), session.get('backend_start')) in identities
                or (session.get('role') == 'voice'
                    and (session.get('client_address') or '').split('/')[0] in old_ips)):
            return False
    return True


class Kube:
    def __init__(self, runner=subprocess.run):
        self.runner = runner

    def call(self, *args, body=None, input_text=None):
        command = ['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
                   '--namespace', 'voice-dev', '--request-timeout=15s', *args]
        if body is not None and input_text is not None:
            raise RecoveryRequired('ambiguous Kubernetes input')
        payload = input_text if body is None else json.dumps(body)
        patchfile = None
        try:
            if args and args[0] == 'patch':
                if not isinstance(body, list):
                    raise RecoveryRequired('JSON CAS patch required')
                fd, patchfile = tempfile.mkstemp(prefix='voice-adoption-patch-')
                with os.fdopen(fd, 'w') as stream:
                    stream.write(payload)
                command.append('--patch-file=' + patchfile)
                payload = None
            result = self.runner(command, input=payload, capture_output=True, text=True, timeout=20)
            if result.returncode:
                raise RecoveryRequired('Kubernetes effect failed or uncertain')
            return json.loads(result.stdout) if result.stdout.strip() else {}
        except BaseException:
            raise RecoveryRequired('Kubernetes effect failed or uncertain') from None
        finally:
            if patchfile:
                os.unlink(patchfile)

    def query(self, database, sql):
        if database not in ('postgres', 'auth_db', 'user_db'):
            raise RecoveryRequired('database outside adoption scope')
        return self.call('exec', '-i', 'voice-postgres-0', '--', 'sh', '-ec',
            'export PGPASSWORD="$POSTGRES_PASSWORD"; export PGAPPNAME=voice-adoption-observe; '
            'export PGOPTIONS="-c default_transaction_read_only=on -c statement_timeout=5000 -c lock_timeout=3000"; '
            'exec psql -h 127.0.0.1 -X -qAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$1"',
            '--', database, input_text=sql)

    def dump(self, database, output):
        if database not in ('auth_db', 'user_db', 'roles'):
            raise RecoveryRequired('database outside backup scope')
        script = ('export PGPASSWORD="$POSTGRES_PASSWORD"; export PGAPPNAME=voice-adoption-backup; '
                  'export PGOPTIONS="-c statement_timeout=60000 -c lock_timeout=5000"; ')
        script += ('exec pg_dumpall -h 127.0.0.1 -U "$POSTGRES_USER" --roles-only --no-role-passwords'
                   if database == 'roles' else
                   'exec pg_dump -h 127.0.0.1 -U "$POSTGRES_USER" --format=custom --lock-wait-timeout=5s -d "$1"')
        command = ['kubectl','--kubeconfig','/home/pmd/.kube/config','--context','default',
                   '--namespace','voice-dev','--request-timeout=15s','exec','voice-postgres-0','--','sh','-ec',script,'--',database]
        try:
            result = self.runner(command, stdout=output, stderr=subprocess.PIPE, timeout=90)
            if result.returncode:
                raise RecoveryRequired('backup stream failed')
        except BaseException:
            raise RecoveryRequired('backup stream uncertain; reconcile server activity') from None

    def runtime_query(self, role, database, password, sql, denied_database=None):
        if ((role,database) not in (('auth_runtime','auth_db'),('user_runtime','user_db'))
                or not re.fullmatch(r'[A-Za-z0-9_-]{32,128}',password)
                or denied_database not in (None,'auth_db','user_db') or denied_database==database):
            raise RecoveryRequired('invalid runtime credential probe')
        command = ('psql -h 127.0.0.1 -X -qAt -v ON_ERROR_STOP=1 -v VERBOSITY=verbose -U ' + role)
        script = ('set -eu\nexport PGPASSWORD='+shlex.quote(password)+'\nexport PGCONNECT_TIMEOUT=5\n'
                  'export PGAPPNAME=voice-adoption-runtime-probe\n'
                  'export PGOPTIONS="-c default_transaction_read_only=on -c statement_timeout=5000 -c lock_timeout=3000"\n'
                  +command+' -d '+database+' -c '+shlex.quote(sql)+'\n')
        if denied_database:
            script += ('if denied=$('+command+' -d '+denied_database+' -c "SELECT 1" 2>&1 >/dev/null); '
                       'then exit 19; fi\nprintf "%s" "$denied" | grep -q 42501\n')
        return self.call('exec','-i','voice-postgres-0','--','sh','-s',input_text=script)


STEPS = ('lock', 'backup', 'sources', 'secret', 'migrate_auth', 'migrate_user',
         'roles', 'grants_auth', 'grants_user', 'roll_auth', 'roll_user', 'verify')


class Operator:
    def __init__(self, journal, backend):
        self.journal, self.backend = journal, backend

    def run(self):
        state = self.journal.state
        try:
            if state.get('aborted') or state.get('finish',{}).get('status')=='done':
                raise RecoveryRequired('operation already closed')
            if set(state['steps']) - set(STEPS):
                raise RecoveryRequired('unknown journal step')
            for step in STEPS:
                self.backend.guard(state)
                saved = state['steps'].get(step)
                actual = self.backend.reconcile(step, state)
                if actual.get('status') == 'complete':
                    if not saved or saved.get('status') not in ('pending', 'done'):
                        raise RecoveryRequired('unowned completed effect')
                    if not actual.get('receipt'):
                        raise RecoveryRequired('terminal receipt missing')
                    if saved['status'] == 'done' and saved.get('receipt') != actual['receipt']:
                        raise RecoveryRequired('terminal receipt changed')
                elif actual.get('status') == 'not_started' and saved is None:
                    # Durable intent precedes every external effect. Any lost
                    # response leaves pending; only actual terminal proof can
                    # advance it. Absence after pending is never retry authority.
                    state['steps'][step] = {'status': 'pending'}
                    self.journal.save()
                    self.backend.guard(state)
                    self.backend.execute(step, state)
                    actual = self.backend.reconcile(step, state)
                    if actual.get('status') != 'complete' or not actual.get('receipt'):
                        raise RecoveryRequired('effect has no terminal receipt')
                else:
                    raise RecoveryRequired('effect unfinished or conflicting')
                self.backend.guard(state)
                state['steps'][step] = {'status': 'done', 'receipt': copy.deepcopy(actual['receipt'])}
                self.journal.save()
            self.backend.guard(state)
        except BaseException:
            # Transport/SQL errors may contain credentials. Never echo them or
            # interpret a client timeout as server-side completion/cancellation.
            raise RecoveryRequired('operation incomplete; reconcile private journal and owned effects') from None


class Journal:
    def __init__(self, directory, intent):
        self.directory = pathlib.Path(directory)
        self.dirfd = self.lockfd = None
        try:
            if any(parent.is_symlink() for parent in self.directory.parents):
                raise RecoveryRequired('symlink journal parent refused')
            self.directory.mkdir(mode=0o700, exist_ok=True)
            parentfd=os.open(self.directory.parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
            try:
                os.fsync(parentfd)
            finally:
                os.close(parentfd)
            self.dirfd = os.open(self.directory, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            self._private(os.fstat(self.dirfd), 0o700, directory=True)
            self.lockfd = os.open('lock', os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW,
                                  0o600, dir_fd=self.dirfd)
            self._private(os.fstat(self.lockfd), 0o600)
            fcntl.flock(self.lockfd, fcntl.LOCK_EX | fcntl.LOCK_NB)
            try:
                fd = os.open('state.json', os.O_RDONLY | os.O_NOFOLLOW, dir_fd=self.dirfd)
            except FileNotFoundError:
                if intent is None:
                    raise RecoveryRequired('existing operation journal required')
                self.state = {'format': 1, 'intent': copy.deepcopy(intent), 'credentials': {
                    'auth': secrets.token_urlsafe(32), 'user': secrets.token_urlsafe(32)}, 'steps': {}}
                self.save()
            else:
                with os.fdopen(fd) as stream:
                    self._private(os.fstat(stream.fileno()), 0o600)
                    self.state = json.load(stream)
                if (self.state.get('format') != 1 or not isinstance(self.state.get('intent'),dict)
                        or (intent is not None and self.state.get('intent') != intent)):
                    raise RecoveryRequired('operation intent changed')
                creds = self.state.get('credentials', {})
                if (set(creds) != {'auth', 'user'} or
                        any(not isinstance(v, str) or len(v) < 32 for v in creds.values()) or
                        creds['auth'] == creds['user'] or not isinstance(self.state.get('steps'), dict)):
                    raise RecoveryRequired('private journal invalid')
        except BaseException as exc:
            self.close()
            if isinstance(exc, RecoveryRequired):
                raise
            raise RecoveryRequired('private journal unavailable or unsafe') from None

    @staticmethod
    def _private(info, mode, directory=False):
        correct_type = stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode)
        if (not correct_type or stat.S_IMODE(info.st_mode) != mode or info.st_uid != os.geteuid()
                or (not directory and info.st_nlink != 1)):
            raise RecoveryRequired('private journal permissions or ownership invalid')

    def save(self):
        self.write_private('state.json', self.state)

    def read_private(self, name, optional=False):
        if not re.fullmatch(r'[a-z][a-z0-9-]*\.json', name):
            raise RecoveryRequired('invalid private journal member')
        try:
            fd = os.open(name, os.O_RDONLY|os.O_NOFOLLOW, dir_fd=self.dirfd)
        except FileNotFoundError:
            if optional:
                return None
            raise RecoveryRequired('private journal member missing') from None
        with os.fdopen(fd) as stream:
            self._private(os.fstat(stream.fileno()), 0o600)
            return json.load(stream)

    def write_private(self, target, value):
        if not re.fullmatch(r'[a-z][a-z0-9-]*\.json', target):
            raise RecoveryRequired('invalid private journal member')
        name = '.state-' + secrets.token_hex(12)
        fd = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW,
                     0o600, dir_fd=self.dirfd)
        try:
            with os.fdopen(fd, 'w') as stream:
                json.dump(value, stream, sort_keys=True, separators=(',', ':'))
                stream.flush()
                os.fsync(stream.fileno())
            os.replace(name, target, src_dir_fd=self.dirfd, dst_dir_fd=self.dirfd)
            os.fsync(self.dirfd)
        finally:
            try:
                os.unlink(name, dir_fd=self.dirfd)
            except FileNotFoundError:
                pass

    def close(self):
        for name in ('lockfd', 'dirfd'):
            fd = getattr(self, name, None)
            if fd is not None:
                os.close(fd)
                setattr(self, name, None)


def main(argv=None):
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action',choices=('prepare','run','observe','import-backup','finish','abort'))
    parser.add_argument('--journal',required=True)
    parser.add_argument('--manifest')
    parser.add_argument('--backup-report')
    args=parser.parse_args(argv)
    journal=None
    try:
        from auth_user_adoption_runtime import Runtime, prepare, verify_package, digest
        root=pathlib.Path(__file__).resolve().parents[2]
        if args.action=='prepare':
            if not args.manifest:
                raise RecoveryRequired('manifest required')
            journal=prepare(root,args.journal,args.manifest,Kube())
            print(json.dumps({'result':'PREPARED','operation':journal.state['intent']['operation'],
                              'intent_sha256':digest(journal.state['intent']),'live_mutations':False}))
            return 0
        journal=Journal(args.journal,None)
        verify_package(root,journal.state['intent']['manifest'])
        runtime=Runtime(journal,Kube(),root)
        if args.action=='run':
            Operator(journal,runtime).run()
            print(json.dumps({'result':'SETTLED_LOCK_RETAINED','operation':runtime.operation,
                              'next':'Record actual deployment/check under current resource guards; then guarded finish'}))
        elif args.action=='observe':
            runtime.guard(journal.state)
            print(json.dumps({'operation':runtime.operation,'steps':{s:runtime.reconcile(s,journal.state)['status']
                for s in journal.state['steps']},'finish':journal.state.get('finish',{}).get('status')}))
        elif args.action=='import-backup':
            if not args.backup_report:
                raise RecoveryRequired('off-node report required')
            from auth_user_adoption_backup import validate_restore
            report=json.loads(pathlib.Path(args.backup_report).read_text(encoding='utf-8'))
            manifest=journal.read_private('backup-manifest.json')
            validate_restore(manifest,report,runtime.operation)
            expected=__import__('hashlib').sha256((root/'scripts/dev/auth_user_adoption_restore.py').read_bytes()).hexdigest()
            if report.get('verifier_sha256')!=expected or report.get('scope')!='actual-existing-dev-backup':
                raise RecoveryRequired('actual frozen off-node verifier report required')
            existing=journal.read_private('backup-qualified.json',optional=True)
            if existing is not None and existing!=report:
                raise RecoveryRequired('backup qualification cannot be replaced')
            journal.write_private('backup-qualified.json',report)
            print(json.dumps({'result':'BACKUP_IMPORTED','operation':runtime.operation}))
        elif args.action=='finish':
            runtime.finish()
            print(json.dumps({'result':'OWNED_LOCK_RELEASED','operation':runtime.operation,
                              'task_or_resource_release_performed':False}))
        elif args.action=='abort':
            runtime.abort()
            print(json.dumps({'result':'UNCHANGED_DATA_ABORTED','operation':runtime.operation,
                              'task_or_resource_release_performed':False}))
        return 0
    except BaseException:
        # No stderr, traceback, SQL, token, Secret object or HTTP body is exposed.
        print(json.dumps({'result':'RECOVERY_REQUIRED','action':'Inspect private journal and reconcile current authority/owned effects; no automatic retry, rollback or unlock'}))
        return 1
    finally:
        if journal is not None:
            journal.close()


if __name__=='__main__':
    sys.exit(main())
