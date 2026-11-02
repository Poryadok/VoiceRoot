import copy
import base64
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import realtime_bootstrap_existing as existing


class ExistingFactsTests(unittest.TestCase):
    def setUp(self):
        self.role = {'oid': 13, 'login': True, 'superuser': False, 'createdb': False,
            'createrole': False, 'inherit': False, 'replication': False, 'bypassrls': False,
            'memberships': 0, 'connection_limit': -1, 'valid_until': None, 'config': None}
        self.catalog = {'role': self.role, 'database': {'oid': 29, 'owner_oid': 13}}
        self.receipt = {'role_oid': 13, 'database_oid': 29, 'owner_oid': 13}

    def test_exact_catalog_is_compatible(self):
        existing.validate_catalog(self.receipt, self.catalog)

    def test_recreated_database_or_role_is_rejected(self):
        for field in ('database_oid', 'role_oid', 'owner_oid'):
            receipt = dict(self.receipt, **{field: self.receipt[field] + 1})
            with self.assertRaises(Exception): existing.validate_catalog(receipt, self.catalog)

    def test_role_privilege_and_membership_drift_is_rejected(self):
        for field, value in [('superuser', True), ('createdb', True), ('createrole', True),
                             ('replication', True), ('bypassrls', True), ('inherit', True),
                             ('login', False), ('memberships', 1), ('config', ['search_path=evil'])]:
            catalog = copy.deepcopy(self.catalog); catalog['role'][field] = value
            with self.assertRaises(Exception): existing.validate_catalog(self.receipt, catalog)

    def test_missing_database_or_wrong_owner_is_rejected(self):
        for database in (None, {'oid': 29, 'owner_oid': 14}, {'oid': 29, 'owner_oid': 13, 'extra': 1}):
            catalog = copy.deepcopy(self.catalog); catalog['database'] = database
            with self.assertRaises(Exception): existing.validate_catalog(self.receipt, catalog)

    def test_secret_full_data_and_runtime_credential_are_bound(self):
        password = b'a' * 32
        dsn = b'postgres://realtime:' + password + b'@voice-postgres:5432/realtime_db?sslmode=disable'
        data = {'REALTIME_DATABASE_URL': base64.b64encode(dsn).decode(), 'other': 'Yg=='}
        secret = {'metadata': {'uid': 'uid', 'resourceVersion': '3', 'name': 'voice-app-secrets',
                               'namespace': 'voice-staging'}, 'type': 'Opaque', 'data': data}
        anchor = {'uid': 'uid', 'resourceVersion': '2', 'data_sha256': existing.digest(data)}
        result = existing.validate_secret(anchor, secret, password)
        self.assertEqual(result['resourceVersion'], '3')
        for bad in (b'b' * 32, b'bad'):
            with self.assertRaises(Exception): existing.validate_secret(anchor, secret, bad)
        altered = copy.deepcopy(secret); altered['data']['other'] = 'Yw=='
        self.assertNotEqual(existing.validate_secret(anchor, altered, password)['data_sha256'], result['data_sha256'])
        altered = copy.deepcopy(secret); altered['metadata']['uid'] = 'recreated'
        with self.assertRaises(Exception): existing.validate_secret(anchor, altered, password)
        altered = copy.deepcopy(secret); altered['metadata']['deletionTimestamp'] = '2026-01-01T00:00:00Z'
        with self.assertRaises(Exception): existing.validate_secret(anchor, altered, password)

    def test_engine_stable_identity_allows_pod_restart_but_rejects_volume_or_system_change(self):
        original = {'system_identifier': '12345', 'statefulset': {'uid': 'sts', 'resourceVersion': '1'},
                    'service': {'uid': 'svc', 'resourceVersion': '1', 'cluster_ip': '10.0.0.1'},
                    'pvc': {'name': 'pgdata', 'uid': 'pvc', 'resourceVersion': '1'},
                    'pv': {'name': 'pv', 'uid': 'pv', 'resourceVersion': '1'},
                    'pod': {'uid': 'old', 'resourceVersion': '1', 'container_id': 'ctr', 'restart_count': 0},
                    'endpoints': {'uid': 'ep', 'resourceVersion': '1', 'pod_ip': '10.0.0.2'},
                    'slice': {'uid': 'slice', 'resourceVersion': '1'}}
        current = copy.deepcopy(original); current['pod']['uid'] = 'new'
        existing.validate_engine(original, current)
        for component, field, value in [('pvc', 'uid', 'other'), ('pv', 'uid', 'other'),
                                         ('service', 'uid', 'other'), ('statefulset', 'uid', 'other')]:
            bad = copy.deepcopy(current); bad[component][field] = value
            with self.assertRaises(Exception): existing.validate_engine(original, bad)
        bad = copy.deepcopy(current); bad['system_identifier'] = '999'
        with self.assertRaises(Exception): existing.validate_engine(original, bad)

    def test_real_session_rejects_lost_claim_and_late_phase(self):
        import commands
        import resource_claim
        from resource_claim_test import context, card, MemoryJournal
        ctx = context(); state = card(ctx); journal = MemoryJournal()
        session = resource_claim.Session(ctx, 'a' * 12, 'b' * 64, lambda: copy.deepcopy(state), journal)
        session.admit(start=True)
        with patch.object(commands, 'authority_guard', session.check):
            self.assertEqual(existing._session(session)['phase'], 'prepare')
            journal.row['phase'] = 'apply-ready'
            with self.assertRaises(Exception): existing._session(session)
            journal.row['phase'] = 'prepare'; state['revision'] += 1
            with self.assertRaises(Exception): existing._session(session)

    @unittest.skipUnless(importlib.util.find_spec('realtime_bootstrap'), 'C1 bootstrap module not integrated')
    def test_actual_session_guard_accepts_only_canonical_source_decoder(self):
        import compiler
        import resource_claim
        from resource_claim_test import context, card, MemoryJournal
        ctx = context(); state = card(ctx); journal = MemoryJournal()
        session = resource_claim.Session(ctx, 'a' * 12, 'b' * 64, lambda: copy.deepcopy(state), journal)
        session.admit(start=True)
        def capture(args, **kwargs):
            session.check(args)
            return b'{"kind":"Deployment"}'
        with patch.object(compiler, 'capture', side_effect=capture):
            self.assertEqual(compiler.decode_yaml(b'kind: Deployment\n'), {'kind': 'Deployment'})
        with self.assertRaises(Exception):
            session.check(['kubectl', 'create', '--dry-run=client', '-f', 'arbitrary.yaml'])

    def test_forged_source_arguments_without_protected_capture_fail(self):
        import commands
        import guard
        import resource_claim
        from resource_claim_test import context, card, MemoryJournal
        ctx = context(); state = card(ctx); journal = MemoryJournal()
        session = resource_claim.Session(ctx, 'a' * 12, 'b' * 64, lambda: copy.deepcopy(state), journal)
        session.admit(start=True)
        approved = {'schema': 'voice-source-authority-v1', 'verified': True, 'source_sha': ctx['commit']}
        with tempfile.TemporaryDirectory(dir='/root') as directory, patch.object(guard, 'ROOT', Path(directory)), \
             patch.object(commands, 'authority_guard', session.check):
            with self.assertRaises(Exception):
                existing._source(session, approved, {'head_sha': ctx['commit']})

    @unittest.skipUnless(importlib.util.find_spec('realtime_bootstrap'), 'C1 bootstrap module not integrated')
    def test_candidate_c1_helper_mismatch_is_rejected(self):
        import commands
        import gis_existing_update
        import guard
        import realtime_bootstrap as c1
        import resource_claim
        from resource_claim_test import context, card, MemoryJournal
        ctx = context(); state = card(ctx); journal = MemoryJournal()
        session = resource_claim.Session(ctx, 'a' * 12, 'b' * 64, lambda: copy.deepcopy(state), journal)
        session.admit(start=True)
        with tempfile.TemporaryDirectory(dir='/root') as temporary:
            root = Path(temporary); source = root / 'installed/sources' / session.operation
            modules = {}; hashes = {}
            for name, relative in existing.SOURCE_HELPERS.items():
                data = ('fixture-' + name).encode()
                installed = root / 'code' / relative; installed.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
                installed.write_bytes(data); installed.chmod(0o600)
                captured = source / 'scripts/staging' / relative; captured.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
                captured.write_bytes(data); captured.chmod(0o600)
                modules[name] = SimpleNamespace(__file__=str(installed))
                hashes['scripts/staging/' + relative] = hashlib.sha256(data).hexdigest()
            approved = {'schema': 'voice-source-authority-v1', 'verified': True, 'source_sha': ctx['commit'], 'source_files': hashes}
            execution = {'head_sha': ctx['commit']}
            receipt_path = root / 'installed/source-authority' / (session.operation + '.json')
            receipt_path.parent.mkdir(parents=True, mode=0o700)
            def save_provenance():
                value = {'schema': 'voice-staging-source-capture-v2', 'installation_sha256': session.installation.sha256, 'operation': session.operation,
                         'context': ctx, 'helper_sha256': session.helper,
                         'approved_sha256': existing.digest(approved), 'execution_sha256': existing.digest(execution)}
                receipt_path.write_text(json.dumps(value)); receipt_path.chmod(0o600)
            save_provenance()
            with patch.object(guard, 'ROOT', root), patch.object(commands, 'authority_guard', session.check), \
                 patch.object(existing.importlib, 'import_module', side_effect=lambda name: modules[name]), \
                 patch.object(gis_existing_update, 'validate_source_authority'), patch.object(c1, '_source_contract'):
                existing._source(session, approved, execution)
                approved['source_files']['scripts/staging/nats-rollout-preservation/realtime_bootstrap.py'] = '0' * 64
                save_provenance()
                with self.assertRaisesRegex(Exception, 'realtime_existing_source_changed'):
                    existing._source(session, approved, execution)

    def test_stage_generation_and_termination_are_checked(self):
        stage = SimpleNamespace(snapshots={'voice-space': {'metadata': {'uid': 'u', 'generation': 2},
                                                          'spec': {'replicas': 1}}},
                                marker={'metadata': {'uid': 'm'}}, old_images={'voice-space/space': 'sha'})
        view = existing._stage_view(stage)
        class Kube:
            def __init__(self): self.deployment = copy.deepcopy(stage.snapshots['voice-space'])
            def get(self, kind, name): return stage.marker if kind == 'configmap' else self.deployment
        kube = Kube(); existing._check_stage(kube, view)
        kube.deployment['metadata']['generation'] = 3
        with self.assertRaises(Exception): existing._check_stage(kube, view)
        kube.deployment['metadata']['generation'] = 2
        kube.deployment['metadata']['deletionTimestamp'] = '2026-01-01T00:00:00Z'
        with self.assertRaises(Exception): existing._check_stage(kube, view)


@unittest.skipUnless(importlib.util.find_spec('realtime_bootstrap'), 'C1 bootstrap module not integrated')
class ProtectedAnchorTests(unittest.TestCase):
    def setUp(self):
        import guard
        import realtime_bootstrap as c1
        from resource_claim_test import context
        self.c1 = c1; self.temp = tempfile.TemporaryDirectory(dir='/root'); self.root = Path(self.temp.name)
        self.guard = patch.object(guard, 'ROOT', self.root); self.guard.start()
        self.op = 'a' * 12; ctx = context(); self.ctx = ctx
        code = {'nats-rollout-preservation/resource_claim.py': '1' * 64}
        helper = existing.digest(code); password = b'a' * 32
        before = {'metadata': {'name': 'voice-app-secrets', 'namespace': 'voice-staging',
                               'uid': 'secret', 'resourceVersion': '1'}, 'type': 'Opaque',
                  'data': {'POSTGRES_PASSWORD': base64.b64encode(b'pw').decode()}}
        approved = {'source_files': {'x': 'y'}}; execution = {'head_sha': ctx['commit']}
        stage = {'marker': {'fixture': 1}}
        role = {'oid': 13, 'login': True, 'superuser': False, 'createdb': False, 'createrole': False,
                'inherit': False, 'replication': False, 'bypassrls': False, 'memberships': 0,
                'connection_limit': -1, 'valid_until': None, 'config': None}
        role_row = {'role': role, 'database': None}; final_row = {'role': role, 'database': {'oid': 29, 'owner_oid': 13}}
        after_data = copy.deepcopy(before['data'])
        after_data['REALTIME_DATABASE_URL'] = c1.secret_patch(before, password)[-1]['value']
        secret = {'uid': 'secret', 'resourceVersion': '2', 'data_sha256': existing.digest(after_data)}
        pg = {'statefulset': {'uid': 'sts', 'resourceVersion': '1'},
              'pod': {'uid': 'pod', 'resourceVersion': '1', 'container_id': 'ctr', 'restart_count': 0},
              'pvc': {'name': 'pgdata-voice-postgres-0', 'uid': 'pvc', 'resourceVersion': '1'},
              'pv': {'name': 'pv', 'uid': 'pv', 'resourceVersion': '1'},
              'service': {'uid': 'svc', 'resourceVersion': '1', 'cluster_ip': '10.0.0.1'},
              'endpoints': {'uid': 'ep', 'resourceVersion': '1', 'pod_ip': '10.0.0.2'},
              'slice': {'uid': 'sl', 'resourceVersion': '1'}, 'system_identifier': '12345'}
        plan = {'schema': 'voice-staging-realtime-bootstrap-v1', 'operation': self.op, 'context': ctx,
                'helper_sha256': helper, 'source_sha': ctx['commit'],
                'source_files_sha256': existing.digest(approved['source_files']),
                'source_authority_sha256': existing.digest(approved), 'execution_sha256': existing.digest(execution),
                'stage_sha256': existing.digest(stage), 'credential_sha256': hashlib.sha256(password).hexdigest(),
                'actual_images': ctx['images'], 'namespace_uid': 'ns', 'postgres': pg,
                'secret': c1._secret_identity(before), 'active_marker_sha256': existing.digest(stage['marker'])}
        def step(before_hash, request_hash, readback):
            return {'status': 'DONE', 'before_sha256': before_hash, 'request_sha256': request_hash, 'readback': readback}
        steps = {'role': step(existing.digest({'role': None, 'database': None}),
                              existing.digest({'kind': 'role', 'credential_sha256': plan['credential_sha256']}), role_row),
                 'database': step(existing.digest(role_row), existing.digest({'kind': 'database', 'owner_oid': 13}), final_row),
                 'secret': step(existing.digest(plan['secret']), existing.digest({'kind': 'secret', 'plan_sha256': existing.digest(plan)}), secret)}
        receipt = {'schema': 'voice-staging-realtime-bootstrap-receipt-v1', 'plan_sha256': existing.digest(plan),
                   'steps_sha256': existing.digest(steps), 'role_oid': 13, 'database_oid': 29,
                   'owner_oid': 13, 'secret': secret, 'migration_pass': False}
        target = {'tag': ctx['commit'], 'images': ctx['images'], 'manifest_sha256': 'd' * 64,
                  'realtime_bootstrap_sha256': existing.digest(receipt)}
        state = {'schema':'nats-rollout-root-v1','operation': self.op, 'status': 'PASS', 'phase': 'POST_RESTART', 'fence_status': 'RELEASED',
                 'context': {'expected': {'namespace_uid': 'ns', 'source_claim_uid': 'nats-pvc'}},
                 'target': target, 'code_capture': code, 'realtime_bootstrap': receipt,
                 'resource_fence': {'schema': 'voice-staging-resource-fence-v1', 'operation': self.op,
                    'context': ctx, 'helper_sha256': helper, 'target_sha256': existing.digest(target)}}
        checkpoint = {'schema': 'voice-staging-realtime-bootstrap-checkpoint-v1', 'plan': plan, 'status': 'COMPLETE',
                      'resource_fence': {'schema': 'voice-staging-resource-fence-v1', 'operation': self.op,
                         'context': ctx, 'helper_sha256': helper, 'target_sha256': existing.digest(plan)},
                      'steps': steps, 'receipt': receipt}
        self.active = {'operation': self.op, 'namespace_uid': 'ns', 'pvc_uid': 'nats-pvc', 'target_sha256': 'd' * 64}
        self.state = state; self.checkpoint = checkpoint; self.before = before; self.authority = {'approved': approved, 'execution': execution}
        self.stage = stage; self.password = password; self.receipt = receipt
        self._save()

    def tearDown(self): self.guard.stop(); self.temp.cleanup()

    def _write(self, path, value):
        path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        path.write_bytes(value if isinstance(value, bytes) else (json.dumps(value) + '\n').encode())
        path.chmod(0o600)

    def _save(self):
        self._write(self.root / 'installed/active-release.json', self.active)
        self._write(self.root / ('rollout-' + self.op) / 'checkpoint.json', self.state)
        base = self.root / 'installed/realtime-bootstrap' / self.op
        for name, value in [('checkpoint.json', self.checkpoint), ('before-secret.json', self.before),
                            ('authority.json', self.authority), ('stage.json', self.stage), ('credential', self.password)]:
            self._write(base / name, value)
        plan = self.checkpoint['plan']
        self._write(self.root / 'installed/source-authority' / (self.op + '.json'),
                    {'schema': 'voice-staging-source-capture-v1', 'operation': self.op, 'context': plan['context'],
                     'helper_sha256': plan['helper_sha256'], 'approved_sha256': plan['source_authority_sha256'],
                     'execution_sha256': plan['execution_sha256']})

    def test_real_c1_anchor_and_private_credential(self):
        ancestry, plan, receipt, directory = existing._anchor()
        self.assertEqual(ancestry['anchor'], {'operation': self.op, 'receipt_sha256': existing.digest(self.receipt)})
        self.assertEqual(existing._original_credential(directory, plan, receipt), self.password)

    def test_scoped_c1_checkpoint_preserves_domain_and_rejects_mixed_versions(self):
        import resource_claim
        authority=resource_claim.InstallationAuthority('owned-isolated-v1','11111111-1111-4111-8111-111111111111',
            'r20260930a4','a'*64,'b'*64,'c'*64)
        state=copy.deepcopy(self.checkpoint);plan=state['plan']
        plan.update(schema='voice-staging-realtime-bootstrap-v3',installation_sha256=authority.sha256)
        state['steps']['secret']['request_sha256']=existing.digest({'kind':'secret','plan_sha256':existing.digest(plan)})
        state['receipt'].update(schema='voice-staging-realtime-bootstrap-receipt-v2',installation_sha256=authority.sha256,
            plan_sha256=existing.digest(plan),steps_sha256=existing.digest(state['steps']))
        state.update(schema='voice-staging-realtime-bootstrap-checkpoint-v2',installation_sha256=authority.sha256)
        state['resource_fence'].update(schema='voice-staging-resource-fence-v2',installation_sha256=authority.sha256,
                                       target_sha256=existing.digest(plan))
        session=SimpleNamespace(operation=self.op,context=self.ctx,helper=plan['helper_sha256'],installation=authority)
        result=self.c1._records(state,session)
        self.assertEqual(result[0],plan);self.assertEqual(result[1],state['receipt'])
        for key in ('plan','receipt','resource_fence'):
            bad=copy.deepcopy(state);bad[key]['installation_sha256']='f'*64
            with self.subTest(key=key),self.assertRaises(Exception):self.c1._records(bad,session)
        bad=copy.deepcopy(state);bad['schema']='voice-staging-realtime-bootstrap-checkpoint-v1';del bad['installation_sha256']
        with self.assertRaises(Exception):self.c1._records(bad,session)

    def test_scoped_c1_anchor_and_original_credential_keep_same_domain(self):
        import resource_claim
        authority=resource_claim.InstallationAuthority('owned-isolated-v1','11111111-1111-4111-8111-111111111111',
            'r20260930a4','a'*64,'b'*64,'c'*64)
        self.stage['marker']={'data':{'generation':authority.generation}}
        plan=self.checkpoint['plan'];plan.update(schema='voice-staging-realtime-bootstrap-v3',
            installation_sha256=authority.sha256,stage_sha256=existing.digest(self.stage),
            active_marker_sha256=existing.digest(self.stage['marker']))
        self.checkpoint['steps']['secret']['request_sha256']=existing.digest({'kind':'secret','plan_sha256':existing.digest(plan)})
        self.receipt.update(schema='voice-staging-realtime-bootstrap-receipt-v2',installation_sha256=authority.sha256,
            plan_sha256=existing.digest(plan),steps_sha256=existing.digest(self.checkpoint['steps']))
        self.checkpoint.update(schema='voice-staging-realtime-bootstrap-checkpoint-v2',installation_sha256=authority.sha256)
        self.checkpoint['resource_fence'].update(schema='voice-staging-resource-fence-v2',installation_sha256=authority.sha256,
                                                 target_sha256=existing.digest(plan))
        target=self.state['target'];target.update(registry='registry',mode='full',changed_services=[],
            source_hashes={'source':'1'*64},template_hashes={'voice-user':'2'*64},input_template_hashes={'voice-user':'3'*64},
            migration_sha256='4'*64,nonnats_sha256='5'*64,installation_sha256=authority.sha256,
            realtime_bootstrap_sha256=existing.digest(self.receipt))
        self.state.update(schema='nats-rollout-root-v2',installation_sha256=authority.sha256)
        self.state['resource_fence'].update(schema='voice-staging-resource-fence-v2',installation_sha256=authority.sha256,
                                            target_sha256=existing.digest(target))
        self.active.update(schema='voice-active-release-v1',installation_sha256=authority.sha256)
        self._save()
        self._write(self.root/'installed/source-authority'/(self.op+'.json'),
            {'schema':'voice-staging-source-capture-v2','installation_sha256':authority.sha256,'operation':self.op,
             'context':plan['context'],'helper_sha256':plan['helper_sha256'],
             'approved_sha256':plan['source_authority_sha256'],'execution_sha256':plan['execution_sha256']})
        ancestry,loaded,receipt,directory=existing._anchor(installation=authority)
        self.assertEqual(ancestry['anchor']['receipt_sha256'],existing.digest(self.receipt))
        self.assertEqual(existing._original_credential(directory,loaded,receipt,installation=authority),self.password)
        with self.assertRaises(Exception):existing._anchor()
        with patch.object(self.c1,'_read_private',side_effect=AssertionError('credential read')) as read:
            with self.assertRaises(Exception):existing._original_credential(directory,loaded,receipt)
            read.assert_not_called()

    def test_resealed_original_authority_without_capture_receipt_is_rejected(self):
        _, plan, receipt, directory = existing._anchor()
        self.authority['approved']['source_files']['x'] = 'different'
        self._write(directory / 'authority.json', self.authority)
        plan['source_authority_sha256'] = existing.digest(self.authority['approved'])
        plan['source_files_sha256'] = existing.digest(self.authority['approved']['source_files'])
        with self.assertRaisesRegex(Exception, 'realtime_existing_anchor_invalid'):
            existing._original_credential(directory, plan, receipt)

    def test_pass_without_bootstrap_or_with_pending_step_is_not_adopted(self):
        self.state.pop('realtime_bootstrap'); self._save()
        with self.assertRaises(Exception): existing._anchor()
        self.state['realtime_bootstrap'] = self.receipt
        self.checkpoint['steps']['secret']['status'] = 'PENDING'; self._save()
        with self.assertRaises(Exception): existing._anchor()

    def test_rolled_back_or_unreleased_fence_is_not_adopted(self):
        for key, value in [('status', 'ROLLED_BACK'), ('fence_status', 'UNKNOWN'), ('phase', 'PREPARE')]:
            previous = self.state[key]; self.state[key] = value; self._save()
            with self.assertRaises(Exception): existing._anchor()
            self.state[key] = previous

    def test_successor_follows_protected_original_anchor(self):
        ancestry, plan, receipt, _ = existing._anchor()
        op2 = 'b' * 12
        secret = dict(receipt['secret'], realtime_value_sha256='f' * 64)
        record = {'schema': 'voice-staging-realtime-bootstrap-existing-v1', 'operation': op2,
                  'context': self.ctx, 'helper_sha256': self.state['resource_fence']['helper_sha256'],
                  'source_authority_sha256': 'a' * 64, 'execution_sha256': 'b' * 64, 'stage_sha256': 'c' * 64,
                  'actual_images': self.ctx['images'], 'namespace_uid': 'ns', 'anchor': ancestry['anchor'],
                  'active_release': ancestry['active_release'],
                  'live': {'postgres': plan['postgres'], 'role_oid': 13, 'database_oid': 29, 'owner_oid': 13, 'secret': secret}}
        parented={**record,'schema':'voice-staging-realtime-bootstrap-existing-v2','prerequisites_sha256':'4'*64}
        existing._validate_record(parented)
        for invalid in ({**parented,'prerequisites_sha256':'bad'},
                        {**record,'prerequisites_sha256':'4'*64},
                        {k:v for k,v in parented.items() if k!='prerequisites_sha256'}):
            with self.assertRaises(existing.Blocked):existing._validate_record(invalid)
        target = {'tag': self.ctx['commit'], 'images': self.ctx['images'], 'manifest_sha256': 'e' * 64,
                  'realtime_bootstrap_sha256': existing.digest(record)}
        state = {'schema': 'nats-rollout-root-v1', 'operation': op2, 'status': 'PASS', 'phase': 'POST_RESTART', 'fence_status': 'RELEASED',
                 'context': self.state['context'], 'target': target,
                 'code_capture': self.state['code_capture'], 'realtime_bootstrap': record,
                 'resource_fence': {'schema': 'voice-staging-resource-fence-v1', 'operation': op2,
                    'context': self.ctx, 'helper_sha256': self.state['resource_fence']['helper_sha256'],
                    'target_sha256': existing.digest(target)}}
        self._write(self.root / ('rollout-' + op2) / 'checkpoint.json', state)
        self._write(self.root / 'installed/realtime-bootstrap' / op2 / 'existing.json', record)
        self._write(self.root / 'installed/active-release.json',
                    {'operation': op2, 'namespace_uid': 'ns', 'pvc_uid': 'nats-pvc', 'target_sha256': 'e' * 64})
        self.assertEqual(existing._anchor()[0]['anchor'], ancestry['anchor'])
        import sys
        from unittest.mock import Mock
        parent={'fixture':'root historical capture'};loader=Mock();loader.validate_historical_capture.return_value=parent
        parented=copy.deepcopy(record);parented.update(schema='voice-staging-realtime-bootstrap-existing-v2',
            prerequisites_sha256=existing.digest(parent))
        parent_state=copy.deepcopy(state);parent_state['realtime_bootstrap']=parented
        parent_state['target']['realtime_bootstrap_sha256']=existing.digest(parented)
        parent_state['target']['prerequisites_sha256']=existing.digest(parent)
        parent_state['resource_fence']['target_sha256']=existing.digest(parent_state['target'])
        self._write(self.root / ('rollout-' + op2) / 'checkpoint.json', parent_state)
        self._write(self.root / 'installed/realtime-bootstrap' / op2 / 'existing.json', parented)
        with patch.dict(sys.modules,{'staging_prerequisites':loader}):
            self.assertEqual(existing._anchor()[0]['anchor'],ancestry['anchor'])
            loader.validate_historical_capture.assert_called_with(op2,installation=__import__('resource_claim').production_installation())
            loader.validate_historical_capture.return_value={'changed':True}
            with self.assertRaisesRegex(existing.Blocked,'anchor_changed'):existing._anchor()
        self._write(self.root / ('rollout-' + op2) / 'checkpoint.json', state)
        self._write(self.root / 'installed/realtime-bootstrap' / op2 / 'existing.json', record)
        record['anchor']['receipt_sha256'] = '0' * 64
        state['realtime_bootstrap'] = record
        state['target']['realtime_bootstrap_sha256'] = existing.digest(record)
        state['resource_fence']['target_sha256'] = existing.digest(state['target'])
        self._write(self.root / ('rollout-' + op2) / 'checkpoint.json', state)
        self._write(self.root / 'installed/realtime-bootstrap' / op2 / 'existing.json', record)
        with self.assertRaises(Exception): existing._anchor()

    def test_observe_rechecks_engine_catalog_and_secret_with_real_session(self):
        import resource_claim
        from resource_claim_test import card, MemoryJournal
        state = card(self.ctx); journal = MemoryJournal()
        session = resource_claim.Session(self.ctx, 'b' * 12, 'b' * 64, lambda: copy.deepcopy(state), journal)
        session.admit(start=True)
        _, plan, receipt, _ = existing._anchor()
        secret = copy.deepcopy(self.before)
        secret['metadata']['resourceVersion'] = '2'
        secret['data']['REALTIME_DATABASE_URL'] = self.c1.secret_patch(self.before, self.password)[-1]['value']
        class Kube:
            def get(self, kind, name): return copy.deepcopy(secret)
        pg = copy.deepcopy(plan['postgres']); pg.pop('system_identifier')
        catalog = self.checkpoint['steps']['database']['readback']
        with patch.object(self.c1, 'engine_snapshot', return_value=('ns', copy.deepcopy(pg))), \
             patch.object(self.c1, '_system_identifier', return_value='12345'), \
             patch.object(self.c1, '_catalog', return_value=copy.deepcopy(catalog)), \
             patch.object(self.c1, '_verify_admin'), patch.object(self.c1, '_verify_runtime_auth') as auth:
            live = existing._observe(session, Kube(), plan, receipt, self.password)
            self.assertEqual(live['database_oid'], 29)
            auth.assert_called_once_with(self.password)
        changed = copy.deepcopy(pg); changed['pod']['restart_count'] += 1
        with patch.object(self.c1, 'engine_snapshot', side_effect=[('ns', copy.deepcopy(pg)), ('ns', changed)]), \
             patch.object(self.c1, '_system_identifier', return_value='12345'), \
             patch.object(self.c1, '_catalog', return_value=copy.deepcopy(catalog)), \
             patch.object(self.c1, '_verify_admin'), patch.object(self.c1, '_verify_runtime_auth'):
            with self.assertRaisesRegex(Exception, 'realtime_existing_live_changed'):
                existing._observe(session, Kube(), plan, receipt, self.password)

    def test_root_journal_revalidates_existing_before_and_after_target_binding(self):
        import commands
        import resource_claim
        import stage_runtime
        from resource_claim_test import card
        state = card(self.ctx); journal = resource_claim.RootJournal(self.root)
        operation = 'c' * 12
        session = resource_claim.Session(self.ctx, operation, 'b' * 64, lambda: copy.deepcopy(state), journal)
        marker = {'metadata': {'name': 'voice-nats-generation', 'uid': 'marker'}}
        deploy = {'metadata': {'uid': 'deploy', 'generation': 1}, 'spec': {'replicas': 1}}
        view = {'marker': marker, 'snapshots': {'voice-space': {'uid': 'deploy', 'generation': 1, 'spec': deploy['spec']}},
                'actual_images': state['observed']['images']}
        class Kube:
            def get(self, kind, name): return copy.deepcopy(marker if kind == 'configmap' else deploy)
        ancestry, plan, receipt, _ = existing._anchor()
        live = {'postgres': plan['postgres'], 'role_oid': 13, 'database_oid': 29, 'owner_oid': 13,
                'secret': dict(receipt['secret'], realtime_value_sha256='f' * 64)}
        authority = {'approved': {'fixture': 1}, 'execution': {'fixture': 2}}
        record = {'schema': 'voice-staging-realtime-bootstrap-existing-v1', 'operation': operation,
                  'context': self.ctx, 'helper_sha256': session.helper,
                  'source_authority_sha256': existing.digest(authority['approved']),
                  'execution_sha256': existing.digest(authority['execution']), 'stage_sha256': existing.digest(view),
                  'actual_images': view['actual_images'], 'namespace_uid': 'ns', 'anchor': ancestry['anchor'],
                  'active_release': ancestry['active_release'], 'live': live}
        base = self.root / 'installed/realtime-bootstrap' / operation
        for name, value in [('existing.json', record), ('authority.json', authority), ('stage.json', view)]:
            self._write(base / name, value)
        with patch('grp.getgrnam', return_value=SimpleNamespace(gr_gid=0)), \
             patch.object(commands, 'authority_guard', session.check), patch.object(stage_runtime, 'Kube', return_value=Kube()), \
             patch.object(existing, '_source'), patch.object(existing, '_observe', return_value=copy.deepcopy(live)):
            session.admit(start=True)
            self.assertEqual(existing.load_existing(session), record)
            target = {'tag': self.ctx['commit'], 'images': self.ctx['images'],
                      'realtime_bootstrap_sha256': existing.digest(record)}
            session.bind_target(target['tag'], target['images'], view['actual_images'], existing.digest(target))
            self.assertEqual(existing.load_existing(session, actual=view['actual_images'], rollout_target=target), record)
            with self.assertRaises(Exception): existing.load_existing(session, actual={'wrong': 'image'}, rollout_target=target)
            deploy['metadata']['generation'] += 1
            with self.assertRaises(Exception): existing.load_existing(session, actual=view['actual_images'], rollout_target=target)

    def test_run_existing_uses_protected_capture_and_real_session(self):
        import bridge_root
        import commands
        import gis_existing_update
        import resource_claim
        import runtime_stage
        import stage_runtime
        from resource_claim_test import card
        state = card(self.ctx); journal = resource_claim.RootJournal(self.root); operation = 'c' * 12
        modules, hashes = {}, {}
        source = self.root / 'installed/sources' / operation
        for name, relative in existing.SOURCE_HELPERS.items():
            data = Path(existing.__file__).read_bytes() if name == 'realtime_bootstrap_existing' else ('fixture-' + name).encode()
            installed = self.root / 'code' / relative; installed.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            installed.write_bytes(data); installed.chmod(0o600)
            captured = source / 'scripts/staging' / relative; captured.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            captured.write_bytes(data); captured.chmod(0o600)
            modules[name] = SimpleNamespace(__file__=str(installed))
            hashes['scripts/staging/' + relative] = hashlib.sha256(data).hexdigest()
        binding = {'nats-rollout-preservation/realtime_bootstrap_existing.py':
                   hashes['scripts/staging/nats-rollout-preservation/realtime_bootstrap_existing.py']}
        session = resource_claim.Session(self.ctx, operation, existing.digest(binding), lambda: copy.deepcopy(state), journal)
        actions = object.__new__(bridge_root.Actions); actions.tt_session = session; actions.binding = binding; actions._installation = session.installation
        marker = {'metadata': {'name': 'voice-nats-generation', 'uid': 'marker'}}
        deploy = {'metadata': {'uid': 'deploy', 'generation': 1}, 'spec': {'replicas': 1}}
        stage = object.__new__(runtime_stage.RolloutStage)
        stage.marker = marker; stage.snapshots = {'voice-space': deploy}; stage.old_images = state['observed']['images']
        class Kube:
            def get(self, kind, name): return copy.deepcopy(marker if kind == 'configmap' else deploy)
        approved = {'schema': 'voice-source-authority-v1', 'verified': True, 'source_sha': self.ctx['commit'], 'source_files': hashes}
        execution = {'head_sha': self.ctx['commit']}
        self._write(self.root / 'installed/source-authority' / (operation + '.json'),
                    {'schema': 'voice-staging-source-capture-v2', 'installation_sha256': session.installation.sha256, 'operation': operation, 'context': self.ctx,
                     'helper_sha256': session.helper, 'approved_sha256': existing.digest(approved),
                     'execution_sha256': existing.digest(execution)})
        _, plan, receipt, _ = existing._anchor()
        live = {'postgres': plan['postgres'], 'role_oid': 13, 'database_oid': 29, 'owner_oid': 13,
                'secret': dict(receipt['secret'], realtime_value_sha256='f' * 64)}
        request = {'action': 'prepare', 'mode': 'full', 'nonce': operation + '0' * 20}
        with patch('grp.getgrnam', return_value=SimpleNamespace(gr_gid=0)), \
             patch.object(commands, 'authority_guard', session.check), patch.object(stage_runtime, 'Kube', return_value=Kube()), \
             patch.object(existing.importlib, 'import_module', side_effect=lambda name: modules[name]), \
             patch.object(gis_existing_update, 'validate_source_authority'), patch.object(self.c1, '_source_contract'), \
             patch.object(existing, '_observe', return_value=copy.deepcopy(live)):
            session.admit(start=True)
            result = existing.run_existing(actions, request, approved, execution, source, stage)
            self.assertEqual(result['anchor']['operation'], self.op)
            target = {'tag': self.ctx['commit'], 'images': self.ctx['images'],
                      'realtime_bootstrap_sha256': existing.digest(result)}
            session.bind_target(target['tag'], target['images'], stage.old_images, existing.digest(target))
            self.assertEqual(existing.load_existing(session, actual=stage.old_images, rollout_target=target), result)


if __name__ == '__main__': unittest.main()
