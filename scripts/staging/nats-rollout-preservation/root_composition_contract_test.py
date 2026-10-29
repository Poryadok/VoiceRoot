"""Offline root composition contracts; external authorities remain fixtures."""
import copy
import contextlib
import hashlib
import json
import shutil
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock, patch

import bridge_root
import compiler
import frontend_createonly_contract_test as fixtures
import gis_first_install as gis
import normalize
import realtime_bootstrap
import resource_claim
import resource_claim_test
import root_cli
import runtime_stage
import source_authority
import source_plan
import source_plan_test
import transaction


class RootCompositionContract(unittest.TestCase):
    def test_bridge_first_install_c1_receipt_reaches_prepare_and_authorize(self):
        # C1 producer/loaders and external Kube/NATS/custody are controlled
        # boundaries; the typed receipt is synthetic, not a completed PASS.
        source = Path(source_plan_test.SOURCE)
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            installed = root / 'installed'
            (installed / 'sources').mkdir(parents=True)
            kube, params = source_plan_test.SourceTests().canonical_fixture('app-only')
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
            enrolled = {'voice-gateway', *('voice-' + name for name in runtime_stage.LEAVES),
                        'voice-web', 'voice-admin', 'voice-developer-portal', source_plan.PRINCIPAL_PUBLISHER}
            snapshots = {name: copy.deepcopy(obj) for (kind, name), obj in kube.objects.items()
                         if kind == 'Deployment' and name in enrolled}
            snapshots.pop('voice-nats-hub', None)
            old_images = {name + '/' + container['name']:
                'example.invalid/' + container['name'] + '@sha256:' + 'c' * 64
                for name, row in snapshots.items()
                for container in row['spec']['template']['spec'].get('initContainers', [])
                + row['spec']['template']['spec'].get('containers', [])}
            old_images['voice-p3-principal-jwks/jwks'] = 'docker.io/library/nginx@sha256:' + 'e' * 64
            marker = {'metadata': {'uid': 'fixture-marker', 'resourceVersion': '1'}, 'data': {
                'phase': 'active', 'generation': 'r20260930a4', 'previousGeneration': 'r20260930a3',
                'dataPVC': 'voice-nats-jsdata-d202610040049430b'}}
            stage = SimpleNamespace(snapshots=snapshots, original_snapshots=copy.deepcopy(snapshots),
                old_images=old_images, marker=marker, service={'metadata': {'uid': 'fixture-service'}},
                expected={'generation': marker['data']['generation'], 'source_claim': marker['data']['dataPVC'],
                    'namespace_uid': 'fixture-namespace', 'source_claim_uid': 'fixture-pvc', 'source_pv_uid': 'fixture-pv'},
                final_claim={'metadata': {'name': marker['data']['dataPVC'], 'uid': 'fixture-pvc'},
                             'spec': {'volumeName': 'fixture-pv'}},
                final_pv={'metadata': {'uid': 'fixture-pv'}, 'spec': {'local': {'path': '/fixture'}}},
                final_path=root / 'fixture-store')
            stage.fence = Mock()
            stage.verify_final_storage = Mock()
            stage.prepared = Mock()
            stage.refence = Mock(return_value={'verified': False})
            get = kube.get
            def get_with_stage(kind, name):
                if (kind, name) == ('configmap', 'voice-nats-generation'): return copy.deepcopy(marker)
                if kind == 'pvc' and name == marker['data']['dataPVC']:
                    return {'status': {'capacity': {'storage': '1Gi'}}}
                return get(kind, name)
            kube.get = get_with_stage
            code = root / 'code'
            (code / 'nats-known-baseline').mkdir(parents=True)
            (code / 'nats-known-baseline' / 'kernel').write_bytes(b'fixture')
            contract = {'scripts': []}
            for part in ('realtime', 'notification', 'search', 'analytics-chat'):
                raw = (source / ('deploy/templates/nats-' + part + '-bootstrap.yaml')).read_bytes().split(b'---')[0]
                script = source_plan_test.decoder(raw)['data']['bootstrap.sh']
                contract['scripts'].append({'part': part, 'sha256': hashlib.sha256(script.encode()).hexdigest()})
            (code / 'nats-known-baseline' / 'deployed-contract.json').write_text(json.dumps(contract))
            optional = {'s3_signing_endpoint', 'gateway_host', 'storage_host', 'livekit_host', 'web_host',
                'admin_host', 'developer_portal_host', 'gateway_tls_secret', 'storage_tls_secret',
                'image_pull_secret', 'apply_observability', 'minio_image', 'minio_mc_image',
                'minio_storage_class', 'minio_storage_size', 'web_origin', 'release_policy'}
            (installed / 'policy.json').write_text(json.dumps({k: v for k, v in params.items() if k in optional}))
            (installed / 'policy.json').chmod(0o600)
            prior = {'fixture': 'prior-enrollment'}
            for name, value in (('bootstrap-enrollment.json', prior), ('bootstrap-enrollment-v4.json', {
                    'schema': 'voice-nats-bootstrap-enrollment-v4', 'previous_enrollment': prior,
                    'previous_enrollment_sha256': resource_claim.digest(prior)})):
                (root / name).write_text(json.dumps(value))
                (root / name).chmod(0o600)
            approved = {'schema': 'voice-source-authority-v1', 'source_sha': params['tag'],
                'repository': source_authority.REPO, 'workflow_id': source_authority.WORKFLOW,
                'run_id': 17, 'run_attempt': 1, 'tree_sha': 'b' * 40, 'lock_artifact_id': 19,
                'lock_sha256': 'd' * 64, 'image_provenance': 'independently resolved trusted CI tag',
                'verified': True, 'source_files': {}, 'images': {
                    name: source_authority.REGISTRY + '/' + name + '@sha256:' + 'a' * 64
                    for name in source_authority.IMAGE_NAMES}}
            def capture_source(_token, _run, _sha, workspace):
                for relative in ('scripts', 'deploy', 'src/backend/migrations',
                                 'src/backend/auth/src/main/resources/db/migration', 'docker'):
                    shutil.copytree(source / relative, workspace / relative, dirs_exist_ok=True)
                approved['source_files'] = {str(path.relative_to(workspace)).replace('\\', '/'): hashlib.sha256(path.read_bytes()).hexdigest()
                    for path in workspace.rglob('*') if path.is_file()}
                return copy.deepcopy(approved)
            image = approved['images']['gameintegration']
            ctx = resource_claim_test.context()
            ctx.update(commit=approved['source_sha'], images={'voice-gameintegration/gameintegration': image})
            card = resource_claim_test.card(ctx)
            card['observed']['images'] = copy.deepcopy(old_images)
            journal = resource_claim_test.MemoryJournal()
            session = resource_claim.Session(ctx, 'a' * 12, resource_claim.digest({}),
                lambda: copy.deepcopy(card), journal)
            session.admit(start=True)
            c1_plan = {'secret': {'uid': 'fixture-secret', 'resourceVersion': '1', 'data_sha256': '1' * 64}}
            receipt = {'schema': 'voice-staging-realtime-bootstrap-receipt-v1', 'plan_sha256': '2' * 64,
                'steps_sha256': '3' * 64, 'role_oid': 13, 'database_oid': 29, 'owner_oid': 13,
                'secret': {'uid': 'fixture-secret', 'resourceVersion': '2', 'data_sha256': '4' * 64},
                'migration_pass': False}
            def c1_prepare(actions, request, authority, execution, workspace, captured):
                self.assertIs(actions.tt_session, session)
                self.assertEqual(request['nonce'][:12], session.operation)
                self.assertEqual(authority['source_sha'], ctx['commit'])
                self.assertEqual(execution['head_sha'], ctx['commit'])
                self.assertEqual(captured.old_images, old_images)
                self.assertTrue(workspace.is_dir())
                row = journal.read(); row.update(phase='realtime-bootstrap', target_sha256=resource_claim.digest(c1_plan))
                journal.save(row); session.expected_target = row['target_sha256']
                return copy.deepcopy(receipt)
            def c1_load(current, *, actual=None, rollout_target=None):
                self.assertIs(current, session)
                if actual is not None: self.assertEqual(actual, old_images)
                if rollout_target is not None:
                    self.assertEqual(rollout_target['realtime_bootstrap_sha256'], resource_claim.digest(receipt))
                return copy.deepcopy(receipt)
            actions = object.__new__(bridge_root.Actions)
            actions.code = code; actions.binding = {}; actions.tt_session = session
            execution = {'dispatcher_run_id': 17, 'head_sha': params['tag']}
            request = {'nonce': 'a' * 64, 'token': 'synthetic', 'mode': 'app-only',
                       'changed_services': [], 'gis_first_install': True}
            cut = {'manifest': {'archive_sha256': '6' * 64}, 'census': {'fixture': 'cut'}}
            cut['census_sha256'] = resource_claim.digest(cut['census'])
            import actor_root, nats_root_plan, nats_contract_custody, gis_prerequisite
            import datetime as dt
            gis_execute = Mock()
            with contextlib.ExitStack() as stack:
                for seam in (patch.object(bridge_root, 'INSTALLED', installed), patch.object(bridge_root.guard, 'ROOT', root),
                    patch.object(root_cli, 'ROOT', root), patch.object(bridge_root, 'Kube', return_value=kube),
                    patch.object(root_cli, 'Kube', return_value=kube), patch.object(actions, '_idle'),
                    patch.object(actions, '_dispatcher', return_value=(17, execution)),
                    patch.object(source_authority, 'capture_source', side_effect=capture_source),
                    patch.object(bridge_root, 'record_source_authority'),
                    patch.object(compiler, 'decode_yaml', side_effect=source_plan_test.decoder),
                    patch.object(compiler, 'ProducerKube', return_value=kube),
                    patch.object(runtime_stage.RolloutStage, 'capture', return_value=stage),
                    patch.object(actor_root, 'preflight', return_value=None), patch.object(actor_root, 'revalidate'),
                    patch.object(bridge_root, 'actor_after_realtime_bootstrap', return_value=None),
                    patch.object(nats_root_plan, 'preflight', return_value={'scripts': {}, 'plan': {'actions': []}, 'binding': {}}),
                    patch.object(realtime_bootstrap, 'run_prepare', side_effect=c1_prepare),
                    patch.object(realtime_bootstrap, 'load_completed', side_effect=c1_load),
                    patch.object(realtime_bootstrap, 'load_plan', return_value=c1_plan),
                    patch.object(source_authority, '_head'), patch.object(source_authority, '_headers', return_value={}),
                    patch.object(transaction.migrations, 'preflight', return_value={}),
                    patch.object(transaction.migrations, 'execute', return_value=[]),
                    patch.object(transaction.space_safeguards, 'preflight', return_value=None),
                    patch.object(transaction.space_safeguards, 'combined_capacity', return_value={}),
                    patch.object(transaction.space_safeguards, 'before_migrate'),
                    patch.object(transaction.space_safeguards, 'after_migrate', return_value={}),
                    patch.object(transaction.nonnats_runtime, 'preflight', return_value={}),
                    patch.object(transaction.nonnats_runtime, 'verify'), patch.object(transaction.nonnats_plan, 'revalidate'),
                    patch.object(source_plan, 'verify_db_init'), patch.object(transaction, 'native_preflight', return_value={}),
                    patch.object(transaction, 'capture_inputs', return_value={}),
                    patch.object(transaction, 'require_original_live_census', return_value={}),
                    patch.object(transaction, 'capture_cut', return_value=copy.deepcopy(cut)),
                    patch.object(transaction, 'DockerRuntime'), patch.object(transaction, 'verify_archive', return_value=True),
                    patch.object(transaction, 'revalidate_inputs'), patch.object(gis, 'preflight'),
                    patch.object(gis_prerequisite, 'preflight'), patch.object(transaction, 'apply_contract', return_value={}),
                    patch.object(nats_contract_custody, 'advance', return_value={}),
                    patch.object(transaction, 'execute_gis_first_install', new=gis_execute),
                    patch.object(actions, '_encrypt', side_effect=lambda _base, state, _execution: state),
                    patch('grp.getgrnam', return_value=SimpleNamespace(gr_gid=0))):
                    stack.enter_context(seam)
                with resource_claim.command_scope(session.check):
                    state = actions._prepare(request)
                    self.assertEqual(state['realtime_bootstrap'], receipt)
                    self.assertEqual(state['target']['realtime_bootstrap_sha256'], resource_claim.digest(receipt))
                    self.assertEqual(state['resource_fence'], session.binding())
                    self.assertEqual(session.binding()['target_sha256'], resource_claim.digest(state['target']))
                    self.assertIn('gis_first_install', state)
                    base = root / ('rollout-' + session.operation)
                    authorization = transaction.authorize(kube, base, state, '6' * 64,
                        state['cut']['manifest_sha256'], {})
                    self.assertEqual(authorization['resource_fence'], session.binding())
                    self.assertEqual(authorization['target'], state['target'])
                    self.assertEqual((state['phase'], state['status']), ('PAUSED_APPLY', 'WAITING'))
                    gis_execute.assert_called_once()

    def test_first_install_compiler_output_normalizes_without_generic_gis_apply(self):
        for mode in ('full', 'app-only'):
            with self.subTest(mode=mode):
                kube, pack, _ = fixtures.FrontendCreateOnlyContract().compile_absent(mode, first_install=True)
                self.assertEqual(pack['manifests'], [])
                source_plan = pack['gis_first_install']
                self.assertEqual(pack['target']['gis_first_install_sha256'], gis.digest(source_plan))
                deployment = next(row for row in source_plan['objects'] if row['kind'] == 'Deployment')
                image = next(row['image'] for row in deployment['spec']['template']['spec']['containers']
                             if row['name'] == 'gameintegration')
                self.assertEqual(pack['target']['images']['voice-gameintegration/gameintegration'], image)
                altered = copy.deepcopy(source_plan)
                changed = next(row for row in altered['objects'] if row['kind'] == 'Deployment')
                changed['spec']['template']['spec']['containers'][0]['image'] = 'example.invalid/foreign@sha256:' + 'f' * 64
                tampered = gis.bind_operation(altered, '123456abcdef', pack['target'])
                with tempfile.TemporaryDirectory() as tmp, patch.object(runtime_stage.RolloutStage, 'capture') as capture:
                    with self.assertRaisesRegex(transaction.Blocked, 'gis_first_install_target_binding_invalid'):
                        transaction.prepare(kube, Path(tmp), Path(tmp), pack['target'], pack['contract'],
                            '123456abcdef', {}, pack['migrations'], pack['nonnats'],
                            gis_first_install=tampered, gis_source_root=Path(tmp), gis_source_capture={})
                    capture.assert_not_called()
                stage = SimpleNamespace(expected={'source_claim': 'fixture'}, snapshots={})
                target = normalize.normalize_target(kube, stage, pack['manifests'], pack['target'])
                self.assertEqual(target['gis_first_install_sha256'], pack['target']['gis_first_install_sha256'])


if __name__ == '__main__':
    unittest.main()
