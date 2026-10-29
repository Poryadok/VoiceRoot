"""Post-backup Go consumer; external Kube/TT/source boundaries are synthetic."""

import copy
import hashlib
import json
from pathlib import Path
import subprocess
import unittest
from unittest import mock

import canonical_application_package as canonical
import cohort_restore_journal
import full_cohort_backup_phase_joint_test as backup_fixture
import full_cohort_go_migration as go
import full_cohort_image_only_baseline as baseline
import full_cohort_image_only_baseline_test as baseline_fixture
import full_cohort_release_activation as activation
import writer_cut_migration


class GoJointTest(unittest.TestCase):
    def test_real_backup_and_go_consumer(self):
        root = Path(__file__).resolve().parents[2]
        migrations = root / 'src/backend/migrations/voice_db'
        source = root / 'scripts/dev/migrations.py'
        fixture = backup_fixture.RealPreflightTest('test_synthetic_transport_readback_and_real_pg16_restore')
        self.addCleanup(fixture.doCleanups)
        environment = {}

        def seed(pg):
            pg.sql(pg.source, 'source_admin', 'postgres',
                   "CREATE ROLE voice_login LOGIN PASSWORD 'fixture-password'")
            pg.sql(pg.source, 'source_admin', 'postgres', 'CREATE DATABASE voice_db OWNER voice_login')
            for path in sorted(migrations.glob('*.up.sql')):
                if int(path.name[:6]) <= 14:
                    pg.sql(pg.source, 'source_admin', 'voice_db',
                           'SET ROLE voice_login;\n' + path.read_text(encoding='utf8'))
            pg.sql(pg.source, 'source_admin', 'voice_db',
                   'SET ROLE voice_login; CREATE TABLE public.schema_migrations(version bigint NOT NULL, dirty boolean NOT NULL); '
                   'INSERT INTO public.schema_migrations VALUES (14,false)')

        def files():
            paths = [source, *migrations.glob('*.sql')]
            return {str(path.relative_to(root)): hashlib.sha256(path.read_bytes()).hexdigest() for path in paths}

        def before_supervisor(work):
            base, fence = work.base, work.fence
            previous = copy.deepcopy(base.context)
            previous.update(generation=2, revision=1, operation_id='media-c1-' + 'b' * 32,
                            claim='33333333-3333-4333-8333-333333333333',
                            window='44444444-4444-4444-8444-444444444444',
                            commit='b' * 40, status='complete')
            previous['actual'] = copy.deepcopy(previous['images'])
            base.ledger.row = copy.deepcopy(previous)
            history = baseline_fixture.HistoryTests().history()
            event = history['events'][0]
            for card in (event['before'], event['after']):
                card.update(owner=previous['actor'], claim=previous['claim'], window=previous['window'],
                            generation=previous['generation'])
                card['candidate'].update(commit=previous['commit'], images=copy.deepcopy(previous['images']))
            event['after']['observed'].update(actor=previous['actor'], claim=previous['claim'],
                window=previous['window'], commit=previous['commit'], images=copy.deepcopy(previous['images']))
            observed = copy.deepcopy(event['after']['observed'])
            base.card.update(operation=3, observed=observed)
            prior = copy.deepcopy(event['after'])
            for revision in (2, 3):
                next_card = copy.deepcopy(prior); next_card['revision'] = revision
                history['events'].append({'id': 'r' + str(revision), 'kind': 'checkpoint',
                                          'before': prior, 'after': next_card})
                prior = next_card
            tip = copy.deepcopy(prior); tip.update(copy.deepcopy(base.card))
            history['events'].append({'id': 'r4', 'kind': 'claim', 'actor': base.context['actor'],
                                      'before': prior, 'after': tip})
            history['revision'] = 4
            with mock.patch.object(baseline.predecessor, 'TT_ENDPOINT', fence.check.endpoint), \
                 mock.patch.object(baseline.predecessor, '_no_tt_proxy'), \
                 mock.patch.object(baseline, '_read_history', return_value=history):
                fence.begin_image_only(work.client, base.lock, work.namespace_uid, work.journal)
            self.assertEqual(work.journal.state['steps']['image-only-admission']['status'], 'done')
            self.assertEqual(base.ledger.row['status'], 'active')

        def setup(state, enter):
            environment['enter'] = enter
            original_reader = go.schema.kube_key_reader
            original_popen = subprocess.Popen

            def reader(private):
                fallback = original_reader(private)
                def read(kind, name, key):
                    if (kind, name, key) == ('secret', 'voice-app-secrets', 'VOICE_DATABASE_URL'):
                        return {'uid': 'source-secret', 'resource_version': '1',
                                'value': 'postgres://voice_login:fixture-password@voice-postgres:5432/voice_db?sslmode=disable'}
                    return fallback(kind, name, key)
                return read

            def popen(args, *positional, **kwargs):
                if args and args[0] == 'kubectl' and go._LOGIN_SCRIPT in args:
                    tail = list(args[args.index(go._LOGIN_SCRIPT) + 1:])
                    if len(tail) != 6 or tail[0] != '--':
                        raise AssertionError('unexpected login command')
                    tail[-1] = '127.0.0.1'
                    args = ['docker', 'exec', '-i', state['pg'].source, 'sh', '-ec', go._LOGIN_SCRIPT, *tail]
                return original_popen(args, *positional, **kwargs)

            enter(mock.patch.object(go.schema, 'kube_key_reader', side_effect=reader))
            enter(mock.patch.object(subprocess, 'Popen', side_effect=popen))

        def after(work, state):
            enter = environment['enter']
            objects = {'Secret': {}, 'ConfigMap': {}, 'Job': {}}
            events = []
            client_type = type(work.owner.client)
            original_call = client_type.call
            original_read = work.source._read.side_effect

            def read(db, *args):
                if db == 'voice_db':
                    return json.loads(state['pg'].sql(state['pg'].source, 'source_admin', db,
                                                     go.metadata._query_for(db)))
                return original_read(db, *args)

            def private(args, body=None):
                if args[0] == 'create':
                    obj = copy.deepcopy(body)
                    if '--dry-run=server' not in args:
                        meta = obj['metadata']
                        meta.update(uid='uid-' + meta['name'], resourceVersion='7')
                        objects['Secret'][meta['name']] = copy.deepcopy(obj)
                    return obj
                if args[0] == 'get':
                    return copy.deepcopy(objects['Secret'].get(args[2], {}))
                name = args[1].rsplit('/', 1)[-1]
                actual = objects['Secret'][name]['metadata']
                self.assertEqual(body['preconditions'], {'uid': actual['uid'], 'resourceVersion': actual['resourceVersion']})
                del objects['Secret'][name]
                return {}

            def call(client, *args, body=None):
                if args[:2] == ('get', 'nodes'):
                    return {'items': [{'metadata': {'name': 'node', 'uid': 'uid'},
                                       'status': {'conditions': [{'type': 'Ready', 'status': 'True'}]}}]}
                if args[:2] in (('get', 'jobs'), ('get', 'configmaps')):
                    kind = 'Job' if args[1] == 'jobs' else 'ConfigMap'
                    return {'items': [{k: copy.deepcopy(v) for k, v in item.items() if k != '_pod'}
                                      for item in objects[kind].values()]}
                if args[:2] == ('get', 'pods') and objects['Job']:
                    existing = original_call(client, *args)
                    existing['items'].extend(copy.deepcopy(item['_pod']) for item in objects['Job'].values())
                    return existing
                if args[0] == 'get' and args[1] in ('job', 'configmap') and len(args) > 2:
                    kind = 'Job' if args[1] == 'job' else 'ConfigMap'
                    obj = objects[kind].get(args[2])
                    if obj is not None:
                        return {k: copy.deepcopy(v) for k, v in obj.items() if k != '_pod'}
                    if args[2].startswith('voice-go-'):
                        return {}
                if args[0] == 'create' and body is not None:
                    obj = copy.deepcopy(body)
                    kind, meta = obj['kind'], obj['metadata']
                    dry = '--dry-run=server' in args
                    if kind != 'Pod':
                        meta.update(uid='uid-' + meta['name'], resourceVersion='5')
                    if kind == 'Job':
                        uid, name = meta['uid'], meta['name']
                        generated = {'batch.kubernetes.io/controller-uid': uid, 'controller-uid': uid,
                                     'batch.kubernetes.io/job-name': name, 'job-name': name}
                        meta['labels'] = {'app': name, **generated}
                        obj['spec']['selector'] = {'matchLabels': {'batch.kubernetes.io/controller-uid': uid}}
                        obj['spec']['template']['metadata']['labels']['batch.kubernetes.io/controller-uid'] = uid
                        obj['status'] = {'conditions': [{'type': 'Complete', 'status': 'True'}]}
                        if not dry:
                            pg = state['pg']
                            script = (migrations / '000015_space_admission_ownership.up.sql').read_text(encoding='utf8')
                            pg.sql(pg.source, 'source_admin', 'voice_db', 'SET ROLE voice_login;\n' + script)
                            pg.sql(pg.source, 'source_admin', 'voice_db',
                                   'SET ROLE voice_login; UPDATE public.schema_migrations SET version=15')
                            template = obj['spec']['template']
                            spec = copy.deepcopy(template['spec']); spec['nodeName'] = 'node'
                            obj['_pod'] = {'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'name': name + '-pod',
                                'namespace': 'voice-dev', 'uid': 'pod-voice', 'resourceVersion': '6',
                                'labels': copy.deepcopy(template['metadata']['labels']),
                                'ownerReferences': [{'kind': 'Job', 'uid': uid, 'controller': True}]},
                                'spec': spec, 'status': {'phase': 'Succeeded', 'podIP': '10.1.2.3',
                                'containerStatuses': [{'name': 'migrate', 'containerID': 'containerd://fixture',
                                    'restartCount': 0, 'imageID': 'containerd://' + go.realtime_migration_job.PLATFORMS['amd64'],
                                    'state': {'terminated': {'exitCode': 0}}}]}}
                            events.append(name)
                    if not dry and kind != 'Pod':
                        objects[kind][meta['name']] = copy.deepcopy(obj)
                    return {k: copy.deepcopy(v) for k, v in obj.items() if k != '_pod'}
                if args[0] == 'delete' and body is not None:
                    kind = 'Job' if '/jobs/' in args[1] else 'ConfigMap'
                    name = args[1].rsplit('/', 1)[-1]; meta = objects[kind][name]['metadata']
                    self.assertEqual(body['preconditions'], {'uid': meta['uid'], 'resourceVersion': meta['resourceVersion']})
                    del objects[kind][name]
                    return {}
                return original_call(client, *args)

            def sessions(scope, identity, guard):
                guard()
                return {'scope': copy.deepcopy(scope), 'active_sessions': 0,
                        'active_transactions': 0, 'attributed_sessions': []}

            enter(mock.patch.object(work.source._read, 'side_effect', read))
            enter(mock.patch.object(client_type, 'call', call))
            enter(mock.patch.object(go, '_private_kube', side_effect=private))
            enter(mock.patch.object(writer_cut_migration.writer_cut_pg_probe, 'inspect_migration', side_effect=sessions))
            self.assertEqual(state['pg'].sql(state['pg'].source, 'source_admin', 'voice_db',
                                             'SELECT version,dirty FROM public.schema_migrations').strip(), '14|f')
            clone = cohort_restore_journal.verify_go_role(work.phase.cohort, work.source, 'fixture-token', 'voice_db')
            self.assertEqual((clone['database'], clone['role'], clone['from'], clone['to']),
                             ('voice_db', 'voice_login', 14, 15))
            result = activation._run_go(work.phase, 'fixture-token')
            self.assertEqual(result, ('voice_db',))
            self.assertEqual([item['phase'].database for item in work.cut.migration_history], ['voice_db'])
            self.assertEqual(go.verify_completed(work.cut.migration_history[0]['phase'])['to'], 15)
            self.assertEqual(len(events), 1)
            self.assertEqual(objects, {'Secret': {}, 'ConfigMap': {}, 'Job': {}})
            self.assertEqual(state['pg'].sql(state['pg'].source, 'source_admin', 'voice_db',
                                             'SELECT version,dirty FROM public.schema_migrations').strip(), '15|f')

        with mock.patch.object(canonical, 'ROOT', root):
            fixture.scenario(True, seed=seed, source_files=files, kube_setup=setup,
                             before_supervisor=before_supervisor, after_restore=after)


if __name__ == '__main__':
    unittest.main()
