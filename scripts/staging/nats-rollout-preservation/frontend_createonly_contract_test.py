"""Offline compiler and TT contract for absent frontend runtime ConfigMaps."""
import copy
import contextlib
import datetime as dt
import hashlib
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock, patch

import compiler
import nonnats_plan
import nonnats_runtime
import resource_claim
import resource_claim_test
import source_plan
import source_plan_test
import transaction
import commands


NAMES = {'voice-web-runtime-config', 'voice-admin-runtime-config',
         'voice-developer-portal-runtime-config'}
SOURCE = Path(__file__).resolve().parents[3]


class FrontendCreateOnlyContract(unittest.TestCase):
    def compile_absent(self, mode, first_install=False):
        kube, params = source_plan_test.SourceTests().canonical_fixture(mode)
        for name in NAMES:
            del kube.objects[('ConfigMap', name)]
        if first_install:
            params['gis_first_install'] = True
            for kind in ('Service', 'Deployment'):
                del kube.objects[(kind, 'voice-gameintegration')]
            run = kube.run
            def inventory(args, body=None, timeout=30):
                if args[:2] == ['get', 'services,deployments']:
                    return {'apiVersion': 'v1', 'kind': 'List', 'metadata': {},
                            'items': [copy.deepcopy(obj) for (kind, _), obj in kube.objects.items()
                                      if kind in ('Service', 'Deployment')]}
                return run(args, body=body, timeout=timeout)
            kube.run = inventory
        params.update(generation='r20260930a4', dataPVC='voice-nats-jsdata-d202610040049430b',
                      changed_services=[], enrolled=[], contract={'scripts': []})
        for part in ('realtime', 'notification', 'search', 'analytics-chat'):
            raw = (SOURCE / ('deploy/templates/nats-' + part + '-bootstrap.yaml')).read_bytes().split(b'---')[0]
            script = source_plan_test.decoder(raw)['data']['bootstrap.sh']
            params['contract']['scripts'].append({'part': part, 'sha256': hashlib.sha256(script.encode()).hexdigest()})
        rows = compiler.rendered_source(SOURCE, params, source_plan_test.decoder)
        deployments = [row for row in rows if row.get('kind') == 'Deployment']
        params['enrolled'] = sorted(row['metadata']['name'] for row in deployments)
        params['images'] = {
            row['metadata']['name'] + '/' + container['name']:
                ('docker.io/library/nginx@sha256:' + 'e' * 64
                 if row['metadata']['name'] == source_plan.PRINCIPAL_PUBLISHER
                 else 'registry/' + container['name'] + '@sha256:' + 'a' * 64)
            for row in deployments
            for container in (row['spec']['template']['spec'].get('initContainers', [])
                              + row['spec']['template']['spec'].get('containers', []))}
        with tempfile.TemporaryDirectory() as tmp:
            parameter = Path(tmp) / 'params.json'
            output = Path(tmp) / ('target-' + params['tag'] + '.json')
            parameter.write_text(json.dumps(params), encoding='utf-8')
            with patch.object(compiler, 'decode_yaml', side_effect=source_plan_test.decoder):
                compiler.main([str(SOURCE), str(parameter), str(output)], producer=kube)
            return kube, json.loads(output.read_text(encoding='utf-8')), rows

    def test_missing_frontend_configs_are_bound_without_generic_apply(self):
        for mode in ('full', 'app-only'):
            with self.subTest(mode=mode):
                kube, pack, _ = self.compile_absent(mode)
                frontends = next(row for row in pack['nonnats']['checks'] if row['id'] == 'frontends')
                created = [row for row in frontends['objects'] if row.get('disposition') == 'create']
                self.assertEqual({row['name'] for row in created}, NAMES)
                self.assertTrue(all(row['kind'] == 'ConfigMap' and row['namespace'] == 'voice-staging'
                                    for row in created))
                self.assertFalse(any(row['kind'] == 'ConfigMap' and row['metadata']['name'] in NAMES
                                     for row in pack['manifests']))
                self.assertFalse(any(action['manifest']['metadata']['name'] in NAMES
                                     for action in pack['nonnats']['actions']))
                self.assertEqual(pack['target']['nonnats_sha256'], nonnats_plan.digest(pack['nonnats']))
                context = resource_claim_test.context()
                context.update(commit=pack['target']['tag'], images=copy.deepcopy(pack['target']['images']))
                card = resource_claim_test.card(context)
                card['observed']['images'] = {}
                session = resource_claim.Session(context, '123456abcdef', 'a' * 64,
                    lambda: copy.deepcopy(card), resource_claim_test.MemoryJournal())
                session.admit(start=True)
                session.bind_target(pack['target']['tag'], pack['target']['images'], {},
                                    resource_claim.digest(pack['target']))
                self.assertEqual(session.binding()['target_sha256'], resource_claim.digest(pack['target']))
                self.assertEqual(session.check()['target_sha256'], resource_claim.digest(pack['target']))

    def test_first_install_defers_frontend_creation_and_runtime_fails_closed(self):
        for mode in ('full', 'app-only'):
            with self.subTest(mode=mode):
                _, first_pack, _ = self.compile_absent(mode, first_install=True)
                self.assertEqual({(row['kind'], row['metadata']['name']) for row in first_pack['gis_first_install']['objects']},
                                 {('Service', 'voice-gameintegration'), ('Deployment', 'voice-gameintegration')})
                self.assertFalse(any(item.get('disposition') == 'create'
                                     for check in first_pack['nonnats']['checks'] for item in check['objects']))
                self.assertFalse(first_pack['nonnats']['actions'])
                self.assertFalse(any(row['metadata']['name'] in NAMES for row in first_pack['manifests']))
        kube, pack, _ = self.compile_absent('full')
        binding = nonnats_plan.preflight(kube, pack['nonnats'], 'full')
        kube.run = Mock(wraps=kube.run)
        with self.assertRaisesRegex(nonnats_runtime.Blocked, 'frontend_bootstrap_gis_first_install_forbidden'):
            nonnats_runtime.create_frontends(kube, pack['nonnats'], binding,
                {'gis_first_install': {'fixture': True}}, Mock(), Mock(), Mock())
        kube.run.assert_not_called()

    # Synthetic admission isolates the frontend transaction; real admission is tested separately.
    @patch('migrations.require_preserving_admission', new=lambda *args: None)
    def test_authorize_frontend_create_failure_never_publishes_authorization(self):
        real_datetime = dt.datetime
        now = real_datetime.now(dt.timezone.utc)
        class FixtureClock(real_datetime):
            expired = False
            @classmethod
            def now(cls, tz=None):
                value = now + (dt.timedelta(minutes=6) if cls.expired else dt.timedelta())
                return cls.fromtimestamp(value.timestamp(), tz=tz)
        for fault in ('deadline', 'response_lost', 'checkpoint_failure', 'positive'):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as tmp:
                FixtureClock.expired = False
                kube, pack, _ = self.compile_absent('full')
                plan = pack['nonnats']
                binding = nonnats_plan.preflight(kube, plan, 'full')
                target = copy.deepcopy(pack['target'])
                target['migration_sha256'] = resource_claim.digest([])
                target['installation_sha256'] = resource_claim.production_installation().sha256
                context = resource_claim_test.context()
                context.update(commit=target['tag'], images=copy.deepcopy(target['images']))
                card = resource_claim_test.card(context)
                card['observed']['images'] = {}
                session = resource_claim.Session(context, '123456abcdef', 'a' * 64,
                    lambda: copy.deepcopy(card), resource_claim_test.MemoryJournal())
                session.admit(start=True)
                session.bind_target(target['tag'], target['images'], {}, resource_claim.digest(target))
                state = {'schema': 'nats-rollout-root-v2', 'installation_sha256': session.installation.sha256,
                    'operation': '123456abcdef', 'phase': 'AWAITING_OFF_NODE', 'status': 'WAITING',
                    'target': target, 'context': {}, 'events': [], 'provenance': {}, 'migrations': [],
                    'migration_secret_metadata': {}, 'nonnats': plan, 'nonnats_binding': binding,
                    'resource_fence': session.binding(),
                    'nats_contract_expires_at': (now + dt.timedelta(minutes=10)).isoformat(),
                    'cut': {'manifest': {'archive_sha256': '6' * 64},
                        'manifest_sha256': '7' * 64, 'census_sha256': '8' * 64}}
                base = Path(tmp)
                stage = Mock()
                stage.refence.return_value = {'verified': False}
                stage.snapshots = {}
                stage.expected = {'namespace_uid': 'fixture-namespace'}
                stage.marker = {'metadata': {'uid': 'fixture-marker', 'resourceVersion': '2'},
                    'data': {'generation': 'r20260930a4', 'previousGeneration': 'r20260930a3', 'dataPVC': 'fixture'}}
                stage.final_claim = {'metadata': {'name': 'fixture', 'uid': 'fixture-pvc'}, 'spec': {'volumeName': 'fixture-pv'}}
                stage.final_pv = {'metadata': {'uid': 'fixture-pv-uid'}, 'spec': {'local': {'path': '/fixture'}}}
                stage.final_path = Path('/fixture')
                calls = []
                run = kube.run
                def simulated_run(args, body=None, timeout=30):
                    if args[0] == 'create' and '--dry-run=server' not in args:
                        commands.authority_guard(args)
                        calls.append(body['metadata']['name'])
                        created = copy.deepcopy(body)
                        created['metadata'].update(uid=body['metadata']['name'] + '-uid', resourceVersion='9')
                        kube.objects[('ConfigMap', body['metadata']['name'])] = created
                        if fault == 'response_lost':
                            raise OSError('simulated lost response')
                        return copy.deepcopy(created)
                    return run(args, body=body, timeout=timeout)
                kube.run = simulated_run
                actual_save = transaction.save
                failed = False
                def save(path, row):
                    nonlocal failed
                    pending = any(event.get('kind') == 'frontend_bootstrap_pending' for event in row.get('events', []))
                    if fault == 'checkpoint_failure' and pending and not failed:
                        failed = True
                        raise OSError('simulated checkpoint failure')
                    actual_save(path, row)
                    if fault == 'deadline' and pending:
                        FixtureClock.expired = True
                deadline = FixtureClock.fromtimestamp((now + dt.timedelta(minutes=5)).timestamp(), tz=dt.timezone.utc)
                with contextlib.ExitStack() as stack:
                    for seam in (patch.object(transaction.dt, 'datetime', FixtureClock),
                        patch.object(transaction, 'save', side_effect=save),
                        patch.object(transaction, 'file_sha', return_value='7' * 64),
                        patch.object(transaction, 'verify_archive', return_value=True),
                        patch.object(transaction, 'reconstruct', return_value=stage),
                        patch.object(transaction, 'context', return_value={}),
                        patch.object(transaction, 'revalidate_inputs'),
                        patch.object(transaction.space_safeguards, 'before_migrate'),
                        patch.object(transaction.space_safeguards, 'after_migrate', return_value={}),
                        patch.object(transaction.migrations, 'execute', return_value=[])):
                        stack.enter_context(seam)
                    admission = object()
                    issue = stack.enter_context(patch('staging_cohort_preservation.issue_final_admission', return_value=admission))
                    complete = stack.enter_context(patch('staging_cohort_preservation.complete_authorization'))
                    stack.enter_context(resource_claim.command_scope(session.check))
                    if fault == 'positive':
                        authorization = transaction.authorize(kube, base, state, '6' * 64, '7' * 64, {},
                            authorization_deadline=deadline, resource_session=session, cohort_request={})
                    else:
                        expected = (OSError if fault == 'checkpoint_failure' else nonnats_runtime.Blocked)
                        reason = {'deadline': 'preserved_upload_authorization_expired',
                            'checkpoint_failure': 'simulated checkpoint failure',
                            'response_lost': 'frontend_bootstrap_outcome_unknown'}[fault]
                        with self.assertRaisesRegex(expected, reason):
                            transaction.authorize(kube, base, state, '6' * 64, '7' * 64, {},
                                                  authorization_deadline=deadline, resource_session=session, cohort_request={})
                saved = json.loads((base / 'checkpoint.json').read_text(encoding='utf-8'))
                if fault == 'positive':
                    issue.assert_called_once_with(session, base, state, {})
                    complete.assert_called_once_with(admission, session, base, state)
                    self.assertEqual((saved['phase'], saved['status']), ('PAUSED_APPLY', 'WAITING'))
                    self.assertEqual(json.loads((base / 'apply-authorization.json').read_text()), authorization)
                    self.assertEqual(authorization['resource_fence'], session.binding())
                    self.assertEqual(authorization['target'], target)
                else:
                    complete.assert_not_called()
                    self.assertEqual(saved['status'], 'BLOCKED')
                    self.assertFalse((base / 'apply-authorization.json').exists())
                    self.assertNotIn('authorization', saved)
                    pending = [event for event in saved['events'] if event['kind'] == 'frontend_bootstrap_pending']
                    self.assertEqual(len(pending), 1)
                    self.assertEqual(saved['frontend_bootstrap'][pending[0]['name']]['status'], 'pending')
                    if fault == 'checkpoint_failure': self.assertTrue(failed)
                self.assertEqual(len(calls), 0 if fault in ('deadline', 'checkpoint_failure')
                                 else 1 if fault == 'response_lost' else 3)
                if fault == 'response_lost':
                    self.assertIn(('ConfigMap', calls[0]), kube.objects)


if __name__ == '__main__':
    unittest.main()
