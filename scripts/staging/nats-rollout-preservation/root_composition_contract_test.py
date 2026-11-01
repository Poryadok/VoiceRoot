"""Offline root composition contracts; external authorities remain fixtures."""
import copy
import contextlib
import hashlib
import json
import shutil
import tempfile
import unittest
import uuid
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock, patch

import bridge_root
import compiler
import frontend_createonly_contract_test as fixtures
import gis_first_install as gis
import gis_existing_update_test as gis_fixtures
import guard_test
import normalize
import nonnats_plan
import realtime_bootstrap
import resource_claim
import resource_claim_test
import root_cli
import runtime_stage
import stage_runtime
import source_authority
import source_plan
import source_plan_test
import transaction


def with_gis_server_defaults(kube):
    run = kube.run
    def server(args, body=None, timeout=30):
        if args[:2] == ['create', '--dry-run=server'] and body and body.get('metadata', {}).get('name') == 'voice-gameintegration':
            value = copy.deepcopy(body)
            if value['kind'] == 'Service':
                value['spec']['clusterIP'] = '10.43.1.99'
                value['spec']['clusterIPs'] = ['10.43.1.99']
            elif value['kind'] == 'Deployment':
                value['spec']['template']['spec']['dnsPolicy'] = 'ClusterFirst'
            return value
        return run(args, body=body, timeout=timeout)
    kube.run = server


class RootCompositionContract(unittest.TestCase):
    def test_c2_protected_anchor_revalidates_actual_compiler_target_after_tt_bind(self):
        # Author fixture models a historical PASS; live PG/Kubernetes
        # observations and current source attestation are controlled seams.
        import realtime_bootstrap_existing as c2
        import realtime_bootstrap_existing_test as c2_fixtures
        owner = c2_fixtures.ProtectedAnchorTests()
        owner.setUp()
        self.addCleanup(owner.tearDown)
        _kube, pack, _ = fixtures.FrontendCreateOnlyContract().compile_absent('app-only')
        ctx = resource_claim_test.context()
        ctx.update(commit=pack['target']['tag'], images=copy.deepcopy(pack['target']['images']))
        card = resource_claim_test.card(ctx)
        old_images = {'voice-space/space': 'example.invalid/space@sha256:' + 'c' * 64}
        card['observed']['images'] = copy.deepcopy(old_images)
        binding = {'nats-rollout-preservation/realtime_bootstrap_existing.py':
                   hashlib.sha256(Path(c2.__file__).read_bytes()).hexdigest()}
        journal = resource_claim.RootJournal(owner.root)
        session = resource_claim.Session(ctx, 'b' * 12, resource_claim.digest(binding),
            lambda: copy.deepcopy(card), journal)
        marker = {'metadata': {'name': 'voice-nats-generation', 'uid': 'marker'}}
        deployment = {'metadata': {'uid': 'deploy', 'generation': 1}, 'spec': {'replicas': 1}}
        stage = object.__new__(runtime_stage.RolloutStage)
        stage.marker = marker
        stage.snapshots = {'voice-space': deployment}
        stage.old_images = old_images
        class Kube:
            def get(self, kind, name):
                return copy.deepcopy(marker if kind == 'configmap' else deployment)
        ancestry, plan, first_receipt, _ = c2._anchor()
        live = {'postgres': plan['postgres'], 'role_oid': 13, 'database_oid': 29, 'owner_oid': 13,
                'secret': dict(first_receipt['secret'], realtime_value_sha256='f' * 64)}
        actions = object.__new__(bridge_root.Actions)
        actions.tt_session = session
        actions._installation = session.installation
        actions.binding = binding
        request = {'action': 'prepare', 'mode': 'app-only', 'nonce': session.operation + 'c' * 52}
        approved = {'fixture': 'controlled-source'}
        execution = {'fixture': 'controlled-execution'}
        source = owner.root / 'installed' / 'sources' / session.operation
        source.mkdir(parents=True, mode=0o700)
        with patch('grp.getgrnam', return_value=SimpleNamespace(gr_gid=0)), \
                patch.object(stage_runtime, 'Kube', return_value=Kube()), \
                patch.object(c2, '_source'), patch.object(c2, '_observe', return_value=copy.deepcopy(live)):
            session.admit(start=True)
            with resource_claim.command_scope(session.check):
                record = c2.run_existing(actions, request, approved, execution, source, stage)
                self.assertEqual(record['anchor'], ancestry['anchor'])
                target = copy.deepcopy(pack['target'])
                target['realtime_bootstrap_sha256'] = resource_claim.digest(record)
                session.bind_target(target['tag'], target['images'], old_images, resource_claim.digest(target))
                self.assertEqual(c2.load_existing(session, actual=old_images, rollout_target=target), record)
                with self.assertRaises(c2.Blocked):
                    c2.load_existing(session, actual={'voice-space/space': 'foreign'}, rollout_target=target)

    def test_first_install_receipt_binds_exact_private_baseline_snapshots(self):
        # Synthetic cluster identities: verify receipt binding, not a live PASS.
        _kube, state, original = gis_fixtures.first_install()
        plan = state['gis_first_install']
        snapshots = {name: {'kind': 'Deployment', 'metadata': {
            'name': name, 'namespace': 'voice-staging', 'uid': str(uuid.UUID(int=index + 1))},
            'spec': {'template': {'fixture': name}}}
            for index, name in enumerate(sorted(gis.baseline_workload_names(include_hub=True)))}
        self.assertEqual(len(snapshots), 24)
        plan['baseline_snapshots'] = copy.deepcopy(snapshots)
        plan['custody']['baseline_workload_count'] = 24
        plan['custody']['baseline_snapshots_sha256'] = gis.digest(snapshots)
        plan['custody_binding_sha256'] = gis.digest(plan['custody'])
        plan['prerequisite_receipt']['custody_binding_sha256'] = plan['custody_binding_sha256']
        state['target']['gis_first_install_sha256'] = plan['source_plan_sha256']
        state['target']['gis_first_install_normalized_sha256'] = gis.normalized_binding(plan)
        state['context'] = {'original_snapshots': copy.deepcopy(snapshots), 'snapshots': {}}
        receipt = gis.transition_receipt(plan, original['ready'], source_ci=plan['source_ci'],
                                         custody=plan['custody'], target_mode='app-only')
        self.assertTrue(gis.validate_transition_receipt(receipt, state))
        state['context']['snapshots'] = {'later-state': 'changed'}
        self.assertTrue(gis.validate_transition_receipt(receipt, state))
        for fault in ('tamper', 'missing', 'extra', 'duplicate_uid'):
            with self.subTest(fault=fault):
                altered = copy.deepcopy(state)
                rows = altered['gis_first_install']['baseline_snapshots']
                first, second = sorted(rows)[:2]
                if fault == 'tamper': rows[first]['spec']['template']['fixture'] = 'changed'
                elif fault == 'missing': del rows[first]
                elif fault == 'extra': rows['voice-foreign'] = copy.deepcopy(rows[first])
                else: rows[second]['metadata']['uid'] = rows[first]['metadata']['uid']
                expected = ('gis_first_install_transition_custody_invalid' if fault == 'tamper'
                            else 'gis_first_install_baseline_workload_count_invalid')
                with self.assertRaisesRegex(gis.Blocked, expected):
                    gis.validate_transition_receipt(receipt, altered)

    def _exercise_bridge(self, ordinary_fault=None):
        # C1 producer/loaders and external Kube/NATS/custody are controlled
        # boundaries; the typed receipt is synthetic, not a completed PASS.
        source = Path(source_plan_test.SOURCE)
        c2_mode = ordinary_fault == 'c2'
        with tempfile.TemporaryDirectory(dir='/root' if c2_mode else None) as tmp:
            root = Path(tmp)
            if c2_mode:
                import realtime_bootstrap_existing as c2
                import realtime_bootstrap_existing_test as c2_fixtures
                anchor = c2_fixtures.ProtectedAnchorTests()
                anchor.setUp()
                shutil.copytree(anchor.root, root, dirs_exist_ok=True)
                anchor.tearDown()
            installed = root / 'installed'
            (installed / 'sources').mkdir(parents=True, exist_ok=True)
            kube, params = source_plan_test.SourceTests().canonical_fixture('app-only')
            ordinary = ordinary_fault is not None
            if c2_mode:
                for index, obj in enumerate(kube.objects.values()):
                    obj.setdefault('metadata', {})['uid'] = str(uuid.UUID(int=index + 1))
            if not ordinary:
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
            with_gis_server_defaults(kube)
            enrolled = {'voice-gateway', *('voice-' + name for name in runtime_stage.LEAVES),
                        'voice-web', 'voice-admin', 'voice-developer-portal', source_plan.PRINCIPAL_PUBLISHER}
            if ordinary: enrolled.add('voice-gameintegration')
            snapshots = {name: copy.deepcopy(obj) for (kind, name), obj in kube.objects.items()
                         if kind == 'Deployment' and name in enrolled}
            snapshots[runtime_stage.HUB] = {'metadata': {'name': runtime_stage.HUB, 'uid': 'fixture-hub'},
                'spec': {'replicas': 1, 'template': {'spec': {'containers': [
                    {'name': 'nats', 'image': 'example.invalid/nats@sha256:' + 'c' * 64}]}}}}
            if c2_mode:
                for row in snapshots.values(): row['metadata']['generation'] = 1
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
            if c2_mode:
                real_stage = object.__new__(runtime_stage.RolloutStage)
                vars(real_stage).update(vars(stage))
                stage = real_stage
            operation = ('b' if c2_mode else 'a') * 12
            stage.operation = operation
            stage.fence = Mock()
            stage.verify_final_storage = Mock()
            stage.prepared = Mock()
            stage.refence = Mock(return_value={'verified': False})
            get = kube.get
            def get_with_stage(kind, name):
                if (kind, name) == ('configmap', 'voice-nats-generation'): return copy.deepcopy(marker)
                if c2_mode and kind == 'deployment' and name in snapshots: return copy.deepcopy(snapshots[name])
                if kind == 'pvc' and name == marker['data']['dataPVC']:
                    return {'status': {'capacity': {'storage': '1Gi'}}}
                if c2_mode and kind and kind[0].islower():
                    kind = next((key[0] for key in kube.objects if key[0].lower() == kind and key[1] == name), kind)
                return get(kind, name)
            kube.get = get_with_stage
            if c2_mode:
                base_run = kube.run
                def normalized_apply(args, body=None, timeout=30):
                    value = base_run(args, body=body, timeout=timeout)
                    if args[:2] == ['apply', '--dry-run=server'] and body:
                        current = get(body['kind'], body['metadata']['name'])
                        value.setdefault('metadata', {})['uid'] = current['metadata']['uid']
                    return value
                kube.run = normalized_apply
            code = root / 'code'
            (code / 'nats-known-baseline').mkdir(parents=True)
            (code / 'nats-known-baseline' / 'kernel').write_bytes(b'fixture')
            binding = {}
            if c2_mode:
                c2_code = Path(c2.__file__).read_bytes()
                helper_path = code / 'nats-rollout-preservation' / 'realtime_bootstrap_existing.py'
                helper_path.parent.mkdir()
                helper_path.write_bytes(c2_code)
                binding['nats-rollout-preservation/realtime_bootstrap_existing.py'] = hashlib.sha256(c2_code).hexdigest()
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
            if ordinary:
                compile_params = {k: v for k, v in params.items() if k in optional}
                compile_params.update(registry=source_authority.REGISTRY, tag=approved['source_sha'], mode='app-only',
                    changed_services=[], images={key: value for key, value in old_images.items()
                                                if not key.startswith(runtime_stage.HUB + '/')},
                    contract=contract, enrolled=list(stage.snapshots), generation=marker['data']['generation'],
                    dataPVC=marker['data']['dataPVC'])
                parameter = root / 'expected-parameters.json'
                output = root / ('target-' + approved['source_sha'] + '.json')
                parameter.write_text(json.dumps(compile_params))
                with patch.object(compiler, 'decode_yaml', side_effect=source_plan_test.decoder):
                    compiler.main([str(source), str(parameter), str(output)], producer=kube)
                expected_images = json.loads(output.read_text())['target']['images']
                candidate_images = copy.deepcopy(expected_images)
                if ordinary_fault == 'missing_secret':
                    for plan_mode in ('full', 'app-only'):
                        source_plan.compile_plan(source, dict(compile_params, mode=plan_mode), source_plan_test.decoder, kube)
                    del kube.objects[('Secret', 'voice-app-secrets')]['data']['REALTIME_DATABASE_URL']
                    for plan_mode in ('full', 'app-only'):
                        with self.assertRaisesRegex(nonnats_plan.PlanError, 'required_secret_key_invalid|canonical_app_secret_key_missing'):
                            source_plan.compile_plan(source, dict(compile_params, mode=plan_mode), source_plan_test.decoder, kube)
                if ordinary_fault == 'backend': del candidate_images['voice-user/user']
                if ordinary_fault == 'frontend': del candidate_images['voice-web/web']
                if ordinary_fault == 'backend_admission':
                    existing_run = kube.run
                    def admission(args, body=None, timeout=30):
                        value = existing_run(args, body=body, timeout=timeout)
                        if (args[:2] == ['apply', '--dry-run=server'] and body
                            and body.get('kind') == 'Deployment'
                            and body.get('metadata', {}).get('name') == 'voice-user'):
                            value['spec']['template']['spec']['containers'].append({
                                'name': 'admission-sidecar', 'image': 'example.invalid/sidecar@sha256:' + 'f' * 64})
                        return value
                    kube.run = admission
            else:
                candidate_images = {'voice-gameintegration/gameintegration': image}
            ctx.update(commit=approved['source_sha'], images=candidate_images)
            card = resource_claim_test.card(ctx)
            card['observed']['images'] = copy.deepcopy(old_images)
            journal = resource_claim_test.MemoryJournal()
            session = resource_claim.Session(ctx, operation, resource_claim.digest(binding),
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
            actions.code = code; actions.binding = binding; actions.tt_session = session
            execution = {'dispatcher_run_id': 17, 'head_sha': params['tag']}
            request = {'nonce': ('b' if c2_mode else 'a') * 64, 'token': 'synthetic', 'mode': 'app-only',
                       'changed_services': [], 'gis_first_install': True}
            request['action'] = 'prepare'
            if ordinary: del request['gis_first_install']
            cut = {'manifest': {'archive_sha256': '6' * 64}, 'census': {'fixture': 'cut'}}
            cut['census_sha256'] = resource_claim.digest(cut['census'])
            import actor_root, nats_root_plan, nats_contract_custody, gis_prerequisite
            import gis_existing_update
            import datetime as dt
            gis_execute = Mock()
            captures = Mock(return_value=stage)
            class ReachedC1(Exception):
                pass
            c1 = Mock(side_effect=(c2.run_existing if c2_mode else ReachedC1 if ordinary else c1_prepare))
            if c2_mode:
                first_receipt = anchor.receipt
                live = {'postgres': anchor.checkpoint['plan']['postgres'], 'role_oid': 13, 'database_oid': 29,
                    'owner_oid': 13, 'secret': dict(first_receipt['secret'], realtime_value_sha256='f' * 64)}
            with contextlib.ExitStack() as stack:
                for seam in (patch.object(bridge_root, 'INSTALLED', installed), patch.object(bridge_root.guard, 'ROOT', root),
                    patch.object(root_cli, 'ROOT', root), patch.object(bridge_root, 'Kube', return_value=kube),
                    patch.object(root_cli, 'Kube', return_value=kube), patch.object(actions, '_idle'),
                    patch.object(gis_existing_update, 'load_authority', return_value=None),
                    patch.object(actions, '_previous_gis_transition', return_value=None),
                    patch.object(actions, '_dispatcher', return_value=(17, execution)),
                    patch.object(source_authority, 'capture_source', side_effect=capture_source),
                    patch.object(bridge_root, 'record_source_authority'),
                    patch.object(compiler, 'decode_yaml', side_effect=source_plan_test.decoder),
                    patch.object(compiler, 'ProducerKube', return_value=kube),
                    patch.object(runtime_stage.RolloutStage, 'capture', new=captures),
                    patch.object(actor_root, 'preflight', return_value=None), patch.object(actor_root, 'revalidate'),
                    patch.object(bridge_root, 'actor_after_realtime_bootstrap', return_value=None),
                    patch.object(nats_root_plan, 'preflight', return_value={'scripts': {}, 'plan': {'actions': []}, 'binding': {}}),
                    patch.object(realtime_bootstrap, 'run_prepare', new=c1),
                    patch.object(realtime_bootstrap, 'load_completed', side_effect=c1_load),
                    patch.object(realtime_bootstrap, 'load_plan', return_value=c1_plan),
                    patch.object(source_authority, '_head'), patch.object(source_authority, '_headers', return_value={}),
                    patch.object(transaction.migrations, 'preflight', return_value={}),
                    patch.object(transaction.migrations, 'execute', return_value=[]),
                    patch.object(transaction.migrations.bot_migration, 'required', return_value=False),
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
                    patch.object(transaction, 'revalidate_inputs'), patch.object(transaction, 'reconstruct', return_value=stage),
                    patch.object(gis_prerequisite, 'preflight'), patch.object(transaction, 'apply_contract', return_value={}),
                    patch.object(nats_contract_custody, 'advance', return_value={}),
                    patch.object(transaction, 'execute_gis_first_install', new=gis_execute),
                    patch.object(actions, '_encrypt', side_effect=lambda _base, state, _execution: state),
                    patch('grp.getgrnam', return_value=SimpleNamespace(gr_gid=0))):
                    stack.enter_context(seam)
                if c2_mode:
                    stack.enter_context(patch.object(c2, '_source'))
                    stack.enter_context(patch.object(c2, '_observe', return_value=copy.deepcopy(live)))
                    stack.enter_context(patch.object(stage_runtime, 'Kube', return_value=kube))
                with resource_claim.command_scope(session.check):
                    if ordinary and not c2_mode:
                        if ordinary_fault in ('none', 'missing_secret'):
                            with self.assertRaises(ReachedC1): actions._prepare(request)
                            c1.assert_called_once()
                        else:
                            with self.assertRaisesRegex(bridge_root.Blocked, 'realtime_bootstrap_candidate_changed'):
                                actions._prepare(request)
                            c1.assert_not_called()
                        return
                    state = actions._prepare(request)
                    self.assertEqual(captures.call_count, 2)
                    if c2_mode:
                        record = state['realtime_bootstrap']
                        self.assertEqual(record['schema'], 'voice-staging-realtime-bootstrap-existing-v1')
                        self.assertEqual(record['anchor'], {'operation': anchor.op, 'receipt_sha256': c2.digest(first_receipt)})
                        self.assertEqual(state['target']['realtime_bootstrap_sha256'], c2.digest(record))
                        self.assertEqual(state['resource_fence'], session.binding())
                        base = root / ('rollout-' + operation)
                        authorization = transaction.authorize(kube, base, state, '6' * 64,
                            state['cut']['manifest_sha256'], {})
                        self.assertEqual(authorization['target'], state['target'])
                        self.assertEqual(authorization['resource_fence'], session.binding())
                        self.assertEqual((state['phase'], state['status']), ('PAUSED_APPLY', 'WAITING'))
                        gis_execute.assert_not_called()
                        return
                    self.assertEqual(set(stage.snapshots), gis.baseline_workload_names(include_hub=True))
                    self.assertNotIn('voice-gameintegration', stage.snapshots)
                    self.assertEqual(state['realtime_bootstrap'], receipt)
                    self.assertEqual(state['target']['realtime_bootstrap_sha256'], resource_claim.digest(receipt))
                    self.assertEqual(state['resource_fence'], session.binding())
                    self.assertEqual(session.binding()['target_sha256'], resource_claim.digest(state['target']))
                    self.assertIn('gis_first_install', state)
                    base = root / ('rollout-' + session.operation)
                    self.assertEqual(state['target']['gis_first_install_normalized_sha256'],
                        gis.normalized_binding(state['gis_first_install']))
                    for fault in ('service', 'deployment', 'allocation'):
                        with self.subTest(normalized_tamper=fault):
                            altered = copy.deepcopy(state)
                            plan = altered['gis_first_install']
                            if fault == 'allocation': plan['service_allocation'] = 'foreign'
                            elif fault == 'service':
                                service = next(row for row in plan['objects'] if row['kind'] == 'Service')
                                service['spec']['ports'][0]['port'] += 1
                            else:
                                deployment = next(row for row in plan['objects'] if row['kind'] == 'Deployment')
                                deployment['spec']['template']['spec']['containers'][0]['image'] = 'example.invalid/foreign@sha256:' + 'f' * 64
                            with self.assertRaisesRegex(gis.Blocked, 'gis_first_install_normalized_binding_changed'):
                                transaction.authorize(kube, base, altered, '6' * 64,
                                    altered['cut']['manifest_sha256'], {})
                            gis_execute.assert_not_called()
                            self.assertFalse((base / 'apply-authorization.json').exists())
                    authorization = transaction.authorize(kube, base, state, '6' * 64,
                        state['cut']['manifest_sha256'], {})
                    self.assertEqual(authorization['resource_fence'], session.binding())
                    self.assertEqual(authorization['target'], state['target'])
                    self.assertEqual((state['phase'], state['status']), ('PAUSED_APPLY', 'WAITING'))
                    gis_execute.assert_called_once()
                    guard_receipt = guard_test.receipt()
                    guard_receipt.update(schema='nats-rollout-preservation-v1', operation=session.operation,
                        target=copy.deepcopy(authorization['target']), backup=copy.deepcopy(authorization['backup']),
                        expires_at=authorization['expires_at'], resource_fence=copy.deepcopy(authorization['resource_fence']))
                    workspace = installed / 'sources' / session.operation
                    args = (guard_receipt['target']['registry'], guard_receipt['target']['tag'], 'app-only', [], workspace)
                    bridge_root.guard.validate(guard_receipt, *args)
                    for field in ('gis_first_install_sha256', 'gis_first_install_normalized_sha256'):
                        bad = copy.deepcopy(guard_receipt)
                        del bad['target'][field]
                        bad['resource_fence']['target_sha256'] = resource_claim.digest(bad['target'])
                        with self.assertRaisesRegex(bridge_root.guard.Blocked, 'gis_first_install_target_binding_invalid'):
                            bridge_root.guard.validate(bad, *args)

    def test_bridge_first_install_c1_receipt_reaches_prepare_and_authorize(self):
        self._exercise_bridge()

    def test_bridge_existing_c2_anchor_reaches_prepare_and_authorize(self):
        self._exercise_bridge('c2')

    def test_ordinary_candidate_inventory_is_exact_before_c1(self):
        for fault in ('none', 'backend', 'frontend', 'backend_admission', 'missing_secret'):
            with self.subTest(fault=fault): self._exercise_bridge(fault)

    def test_first_install_compiler_output_normalizes_without_generic_gis_apply(self):
        for mode in ('full', 'app-only'):
            with self.subTest(mode=mode):
                kube, pack, _ = fixtures.FrontendCreateOnlyContract().compile_absent(mode, first_install=True)
                with_gis_server_defaults(kube)
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
                stage = SimpleNamespace(expected={'source_claim': 'fixture'}, snapshots={}, operation='123456abcdef')
                original = gis.bind_operation(source_plan, stage.operation, pack['target'])
                target, normalized_plan = normalize.normalize_first_install(kube, stage, pack['manifests'], pack['target'], original)
                self.assertEqual(target['gis_first_install_sha256'], pack['target']['gis_first_install_sha256'])
                normalized_deployment = next(row for row in normalized_plan['objects'] if row['kind'] == 'Deployment')
                normalized_service = next(row for row in normalized_plan['objects'] if row['kind'] == 'Service')
                self.assertEqual(target['template_hashes']['voice-gameintegration'],
                    gis.digest(normalized_deployment['spec']['template']))
                self.assertEqual(target['input_template_hashes']['voice-gameintegration'],
                    pack['target']['template_hashes']['voice-gameintegration'])
                self.assertEqual(normalized_plan['service_allocation'], 'server')
                self.assertNotIn('clusterIP', normalized_service['spec'])


if __name__ == '__main__':
    unittest.main()
