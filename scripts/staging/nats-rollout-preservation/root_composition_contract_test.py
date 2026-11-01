"""Offline root composition contracts; external authorities remain fixtures."""
import copy
import contextlib
import hashlib
import json
import os
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
    def _exercise_public_realtime(self, c2_mode=False):
        """Public continuation with synthetic P2 and external infrastructure."""
        import bridge_composition_test
        import staging_cohort_preservation as cohort
        import nats_contract_custody_test as nats_fixture
        import nats_contract_plan
        import datetime as dt
        owner=self
        class Deep:
            is_c2=c2_mode

            def candidate(self, helper, source, mode):
                kube,parameters,targets,renderer,old,catalog=helper._compiler_catalog_stage(mode)
                auxiliary=Path(bridge_composition_test.__file__).parent/'fixtures'/'qualified-nginx'/'manifest.json'
                images={key:value for key,value in old.items() if not key.startswith(bridge_root.HUB+'/')}
                images['voice-p3-principal-jwks/jwks']='docker.io/library/nginx@sha256:'+hashlib.sha256(auxiliary.read_bytes()).hexdigest()
                kube.objects[('ConfigMap','voice-p3-principal-public')]={'apiVersion':'v1','kind':'ConfigMap',
                    'metadata':{'name':'voice-p3-principal-public','namespace':'voice-staging','uid':'fixture-public','resourceVersion':'1'},
                    'immutable':True,'data':{'realtime.json':'{}'}}
                parameters.update(registry=source_authority.REGISTRY,tag=source.TAG,mode=mode,changed_services=[],
                    images=images,enrolled=list(catalog.snapshots),contract={'scripts':[]})
                parameters['release_policy']['source_sha']=source.TAG
                with patch.object(compiler,'decode_yaml',side_effect=source.decoder):
                    return compiler.prebootstrap_image_inventory(source.SOURCE,parameters,kube)

            def approve(self, approved):
                approved.update(schema='voice-source-authority-v1', repository=source_authority.REPO,
                    workflow_id=source_authority.WORKFLOW, run_id=17, run_attempt=1, tree_sha='b'*40,
                    lock_artifact_id=19, lock_sha256='d'*64,
                    image_provenance='independently resolved trusted CI tag', verified=True)

            def contract(self, contract, code):
                import nats_active_contract
                _,_,nats,_=nats_fixture.fixture()
                self.nats_plan=copy.deepcopy(nats['nats_contract'])
                self.nats_plan.update(actions=[], sources={name:'c'*64 for name in nats_active_contract.SOURCE_PATHS})
                return json.loads((code/'nats-known-baseline'/'deployed-contract.json').read_bytes())

            def plan(self, plan, stage, source, request, prepared, contract):
                from docker_runtime import NATS_IMAGE
                names=[name.removeprefix('voice-') for name in stage.snapshots if name!=bridge_root.HUB]
                names.append('p3-principal-jwks')
                self.enrollment={'fixture':'synthetic-enrollment'}
                self.nats={'scripts':{},'plan':self.nats_plan,'binding':{
                    'plan_sha256':cohort.digest(self.nats_plan),'server_image':NATS_IMAGE,
                    'enrollment_sha256':cohort.digest(self.enrollment),'sources':self.nats_plan['sources']}}
                self.preflight={'enrollment':self.enrollment,'nats':self.nats,'contract':contract,'services':names,'actor':None}
                return {**plan,'preflight_sha256':cohort.digest(self.preflight),
                    'prerequisite_inputs_sha256':prepared['inputs_sha256']}

            def bootstrap(self, session, stage, stack, prereq, cohort, actions, workspace, kube, approved, execution, contract, p2_plan):
                import stage_runtime
                import nats_root_plan
                self.session=session;self.stage=stage;self.prereq=prereq;self.stack=stack
                self.cohort=cohort;self.kube=kube;self.contract_value=contract;self.approved=approved
                self.operation=session.operation;root=bridge_root.guard.ROOT;self.root=root
                base=root/'installed'/'cohort-preservation'/session.operation;base.mkdir(parents=True,mode=0o700)
                cohort._write(base/'checkpoint.json',{'schema':'voice-staging-cohort-preservation-checkpoint-v1','plan':p2_plan})
                cohort._write(base/'preflight.json',self.preflight)
                request_path=cohort._request_record(session);request_path.parent.mkdir(parents=True,mode=0o700)
                source_request={'mode':'app-only','changed_services':[],
                    'nonce':session.operation+'a'*52,'resource_context':session.context}
                if not self.is_c2:source_request['gis_first_install']=True
                cohort._write(request_path,source_request)
                stack.enter_context(patch.object(cohort,'_base',return_value=base))
                stack.enter_context(patch.object(cohort,'load_plan',return_value=p2_plan))
                stack.enter_context(patch.object(cohort,'validate_plan'))
                stack.enter_context(patch.object(cohort,'_source'))
                stack.enter_context(patch.object(cohort,'_preflight_shape'))
                stack.enter_context(patch.object(cohort,'load_preflight',return_value=self.preflight))
                stack.enter_context(patch.object(nats_root_plan,'preflight',return_value=self.nats))
                stack.enter_context(patch.object(cohort,'_offnode_records',return_value={'artifact_id':42,'verified':True}))
                stack.enter_context(patch.object(realtime_bootstrap,'validate_plan'))
                stack.enter_context(patch.object(realtime_bootstrap,'load_plan',side_effect=lambda *a,**k:self.realtime_plan))
                stack.enter_context(patch.object(realtime_bootstrap,'load_completed',side_effect=lambda *a,**k:copy.deepcopy(self.receipt)))
                stack.enter_context(patch.object(bridge_root,'actor_after_realtime_bootstrap',return_value=None))
                capture=stage_runtime.capture
                digest=hashlib.sha256((Path(bridge_composition_test.__file__).parent/'fixtures'/'qualified-nginx'/'manifest.json').read_bytes()).hexdigest()
                def kube_capture(argv,**kwargs):
                    raw=capture(argv,**kwargs)
                    if argv[-4:]==['get','pods','-o','json']:
                        value=json.loads(raw)
                        for pod in value.get('items',[]):
                            for status in pod.get('status',{}).get('containerStatuses',[]):
                                if status.get('name')=='jwks':status['imageID']='docker-pullable://nginx@sha256:'+digest
                        return json.dumps(value).encode()
                    return raw
                stack.enter_context(patch.object(stage_runtime,'capture',side_effect=kube_capture))
                if self.is_c2:
                    import realtime_bootstrap_existing as existing
                    import realtime_bootstrap_existing_test as c2_fixtures
                    anchor=c2_fixtures.ProtectedAnchorTests();anchor.setUp();anchor.guard.stop()
                    authority=session.installation
                    anchor.stage['marker']={'data':{'generation':authority.generation}}
                    plan=anchor.checkpoint['plan'];plan.update(schema='voice-staging-realtime-bootstrap-v3',
                        installation_sha256=authority.sha256,stage_sha256=existing.digest(anchor.stage),
                        active_marker_sha256=existing.digest(anchor.stage['marker']))
                    anchor.checkpoint['steps']['secret']['request_sha256']=existing.digest({'kind':'secret','plan_sha256':existing.digest(plan)})
                    anchor.receipt.update(schema='voice-staging-realtime-bootstrap-receipt-v2',installation_sha256=authority.sha256,
                        plan_sha256=existing.digest(plan),steps_sha256=existing.digest(anchor.checkpoint['steps']))
                    anchor.checkpoint.update(schema='voice-staging-realtime-bootstrap-checkpoint-v2',installation_sha256=authority.sha256)
                    anchor.checkpoint['resource_fence'].update(schema='voice-staging-resource-fence-v2',installation_sha256=authority.sha256,
                        target_sha256=existing.digest(plan))
                    target=anchor.state['target'];target.update(registry='registry',mode='full',changed_services=[],
                        source_hashes={'source':'1'*64},template_hashes={'voice-user':'2'*64},input_template_hashes={'voice-user':'3'*64},
                        migration_sha256='4'*64,nonnats_sha256='5'*64,installation_sha256=authority.sha256,
                        realtime_bootstrap_sha256=existing.digest(anchor.receipt))
                    anchor.state.update(schema='nats-rollout-root-v2',installation_sha256=authority.sha256)
                    anchor.state['resource_fence'].update(schema='voice-staging-resource-fence-v2',installation_sha256=authority.sha256,
                        target_sha256=existing.digest(target))
                    anchor.active.update(schema='voice-active-release-v1',installation_sha256=authority.sha256)
                    anchor._save()
                    anchor._write(anchor.root/'installed/source-authority'/(anchor.op+'.json'),
                        {'schema':'voice-staging-source-capture-v2','installation_sha256':authority.sha256,'operation':anchor.op,
                         'context':plan['context'],'helper_sha256':plan['helper_sha256'],
                         'approved_sha256':plan['source_authority_sha256'],'execution_sha256':plan['execution_sha256']})
                    self.anchor_operation=anchor.op;self.anchor_receipt=copy.deepcopy(anchor.receipt)
                    live={'postgres':plan['postgres'],'role_oid':13,'database_oid':29,'owner_oid':13,
                        'secret':dict(anchor.receipt['secret'],realtime_value_sha256='f'*64)}
                    shutil.copytree(anchor.root,root,dirs_exist_ok=True);anchor.tearDown()
                    stack.enter_context(patch.object(existing,'_source'))
                    stack.enter_context(patch.object(existing,'_observe',return_value=live))
                    stack.enter_context(patch.object(actions,'_previous_gis_transition',return_value=None))
                    self.receipt=None
                else:
                    self.receipt={'schema':'voice-staging-realtime-bootstrap-receipt-v1',
                        'secret':{'uid':'fixture','resourceVersion':'1','data_sha256':'1'*64},'fixture':'synthetic-c1'}
                return self.receipt

            def run_bootstrap(self, actions, request, approved, execution, workspace, stage):
                if self.is_c2:
                    import realtime_bootstrap_existing as existing
                    self.receipt=existing.run_existing(actions,request,approved,execution,workspace,stage)
                    return copy.deepcopy(self.receipt)
                capture=self.prereq.load_capture(self.session,actual=stage.old_images)
                self.realtime_plan={'actual_images':stage.old_images,'prerequisites_sha256':cohort.digest(capture),
                    'secret':copy.deepcopy(self.receipt['secret'])}
                realtime=self.root/'installed'/'realtime-bootstrap'/self.session.operation
                realtime.mkdir(parents=True,mode=0o700)
                cohort._write(realtime/'checkpoint.json',{'plan':self.realtime_plan})
                self.session.bind_realtime_bootstrap(self.realtime_plan,stage.old_images)
                return copy.deepcopy(self.receipt)

            def prepare(self,value,args,kwargs,session,stage,kube,workspace,qualified,contract,public,stack):
                import actor_root, nats_root_plan, gis_prerequisite
                import nats_migration, nats_active_contract
                captures=cohort.capture_final_stage
                before_prepare=captures.call_count
                owner.assertGreater(before_prepare,0)
                runtime=self.root/('rollout-'+session.operation);runtime.mkdir(mode=0o700)
                shutil.copyfile(Path(args[0])/'nats-known-baseline'/'kernel',runtime/'kernel')
                manifest={'archive_sha256':'6'*64};manifest_file=runtime/'rollout-before-manifest.json'
                cohort._write(manifest_file,manifest)
                cut={'manifest':manifest,'manifest_sha256':transaction.file_sha(manifest_file),'census_sha256':'7'*64,'census':{}}
                preserved={'cut':cut,'custody':{'artifact_id':42,'verified':True},'provenance':{},
                    'contract':contract,'events':[]}
                migration_id=self.nats_plan['migration_id']
                proof={'schema':nats_active_contract.VERSIONS[migration_id][0], 'migration_id':migration_id,
                    'before_sha256':'d'*64,'after_sha256':'e'*64,'messages':1,
                    'old_consumers_preserved':1,'created_consumers':0}
                bot=next(row for row in value['migrations'] if row['database']=='bot')
                bot_version=max(int(name.split('_',1)[0]) for name in bot['files'] if name.endswith('.up.sql'))
                preflight_calls=[]
                def migration_preflight(*unused):
                    preflight_calls.append(None)
                    return {'bot_prerequisite':{'observation':{'version':bot_version if len(preflight_calls)>=4 else 0}}}
                stage.service={'metadata':{'uid':'fixture-service'}}
                original_get=kube.get
                if self.is_c2:
                    kube.objects[('Deployment','voice-p3-principal-jwks')]=copy.deepcopy(stage.snapshots['voice-p3-principal-jwks'])
                def get(kind,name):
                    if (kind,name)==('pvc',stage.expected['source_claim']):return {'status':{'capacity':{'storage':'1Gi'}}}
                    if self.is_c2 and kind and kind[0].islower():
                        kind=next((identity[0] for identity in kube.objects if identity[0].lower()==kind and identity[1]==name),kind)
                    return original_get(kind,name)
                kube.get=get
                if not self.is_c2:
                    for kind in ('Service','Deployment'):
                        kube.objects.pop((kind,'voice-gameintegration'),None)
                original_run=kube.run
                def run(args,body=None,timeout=30):
                    if args[:2]==['get','services,deployments']:
                        return {'apiVersion':'v1','kind':'List','metadata':{},
                            'items':[copy.deepcopy(row) for (kind,_),row in kube.objects.items()
                                     if kind in ('Service','Deployment')]}
                    result=original_run(args,body=body,timeout=timeout)
                    if self.is_c2 and args[:2]==['apply','--dry-run=server'] and body:
                        result.setdefault('metadata',{})['uid']=original_get(body['kind'],body['metadata']['name'])['metadata']['uid']
                    return result
                kube.run=run
                with_gis_server_defaults(kube)
                for seam in (patch.object(root_cli,'ROOT',self.root),patch.object(root_cli,'Kube',return_value=kube),
                    patch.object(bridge_root,'Kube',return_value=kube),
                    patch.object(cohort,'load_rollout_continuation',return_value=preserved),
                    patch.object(actor_root,'revalidate'),
                    patch.object(transaction.migrations,'preflight',side_effect=migration_preflight),
                    # Database job transport is synthetic; execute's admission guard remains real.
                    patch.object(transaction.migrations,'legacy_credentials',return_value=None),
                    patch.object(transaction.migrations,'documents',return_value=[]),
                    patch.object(transaction,'verify_archive',return_value=True),
                    patch.object(transaction,'reconstruct',return_value=stage),
                    patch.object(transaction,'revalidate_inputs'),
                    patch.object(transaction.space_safeguards,'preflight',return_value=None),
                    patch.object(transaction.space_safeguards,'combined_capacity',return_value={}),
                    patch.object(transaction.space_safeguards,'before_migrate'),
                    patch.object(transaction.space_safeguards,'after_migrate',return_value={}),
                    patch.object(transaction.nonnats_runtime,'preflight',return_value={}),
                    patch.object(transaction.nonnats_runtime,'verify'),
                    patch.object(transaction.nonnats_runtime,'create_frontends'),
                    patch.object(transaction.nonnats_plan,'revalidate'),
                    patch.object(source_plan,'verify_db_init'),patch.object(transaction,'native_preflight',return_value={}),
                    # Native NATS readback and its census are external synthetic boundaries.
                    patch.object(nats_migration,'verify_post_apply',return_value={}),
                    patch.object(nats_migration,'verify_census',return_value=proof),
                    patch.object(gis_prerequisite,'preflight'),patch.object(transaction,'execute_gis_first_install')):
                    stack.enter_context(seam)
                cohort._write(self.root/'bootstrap-enrollment.json',self.enrollment)
                actual_kwargs=dict(kwargs);ref=actual_kwargs.pop('preservation_ref')
                state=root_cli.prepare_value(value,*args,_preservation_ref=ref,**actual_kwargs)
                owner.assertGreater(captures.call_count,before_prepare)
                self.state=state
                self.cut=cut
                stage.verify_final_storage=Mock();stage.owned_marker=Mock();stage.prepared=Mock()
                stage.refence=Mock(return_value={'verified':True})
                execute_gis=transaction.execute_gis_first_install
                original_authorize=transaction.authorize
                def authorize(kube,base,current,*authorize_args,**authorize_kwargs):
                    if self.is_c2:
                        owner.assertEqual(current['realtime_bootstrap']['schema'],'voice-staging-realtime-bootstrap-existing-v4')
                        owner.assertEqual(current['realtime_bootstrap']['anchor'],{'operation':self.anchor_operation,
                            'receipt_sha256':cohort.digest(self.anchor_receipt)})
                        owner.assertEqual(current['target']['realtime_bootstrap_sha256'],cohort.digest(current['realtime_bootstrap']))
                        owner.assertEqual(current['resource_fence'],session.binding())
                        result=original_authorize(kube,base,current,*authorize_args,**authorize_kwargs)
                        owner.assertEqual(result['target'],current['target'])
                        owner.assertEqual(result['resource_fence'],session.binding())
                        owner.assertEqual((current['phase'],current['status']),('PAUSED_APPLY','WAITING'))
                        execute_gis.assert_not_called()
                        self.authorization=result
                        return result
                    owner.assertEqual(set(stage.snapshots),gis.baseline_workload_names(include_hub=True))
                    owner.assertNotIn('voice-gameintegration',stage.snapshots)
                    owner.assertEqual(current['realtime_bootstrap'],self.receipt)
                    owner.assertEqual(current['target']['realtime_bootstrap_sha256'],resource_claim.digest(self.receipt))
                    owner.assertEqual(current['resource_fence'],session.binding())
                    owner.assertEqual(session.binding()['target_sha256'],resource_claim.digest(current['target']))
                    owner.assertEqual(current['target']['gis_first_install_normalized_sha256'],
                        gis.normalized_binding(current['gis_first_install']))
                    for fault in ('service','deployment','allocation'):
                        with owner.subTest(normalized_tamper=fault):
                            altered=copy.deepcopy(current);plan=altered['gis_first_install']
                            if fault=='allocation':plan['service_allocation']='foreign'
                            elif fault=='service':
                                next(row for row in plan['objects'] if row['kind']=='Service')['spec']['ports'][0]['port']+=1
                            else:
                                next(row for row in plan['objects'] if row['kind']=='Deployment')['spec']['template']['spec']['containers'][0]['image']='example.invalid/foreign@sha256:'+'f'*64
                            transaction.save(Path(base)/'checkpoint.json',altered)
                            with owner.assertRaisesRegex(gis.Blocked,'gis_first_install_normalized_binding_changed'):
                                original_authorize(kube,base,altered,*authorize_args,**authorize_kwargs)
                            execute_gis.assert_not_called()
                            owner.assertFalse((Path(base)/'apply-authorization.json').exists())
                            (cohort._base(session)/'authorizing.json').unlink()
                    transaction.save(Path(base)/'checkpoint.json',current)
                    result=original_authorize(kube,base,current,*authorize_args,**authorize_kwargs)
                    owner.assertEqual(result['resource_fence'],session.binding())
                    owner.assertEqual(result['target'],current['target'])
                    owner.assertEqual((current['phase'],current['status']),('PAUSED_APPLY','WAITING'))
                    execute_gis.assert_called_once()
                    guard_receipt=guard_test.receipt()
                    guard_receipt.update(schema='nats-rollout-preservation-v2',operation=session.operation,
                        installation_sha256=session.installation.sha256,target=copy.deepcopy(result['target']),
                        backup=copy.deepcopy(result['backup']),expires_at=result['expires_at'],
                        resource_fence=copy.deepcopy(result['resource_fence']))
                    guard_args=(guard_receipt['target']['registry'],guard_receipt['target']['tag'],'app-only',[],workspace)
                    bridge_root.guard.validate(guard_receipt,*guard_args,installation=session.installation)
                    for field in ('gis_first_install_sha256','gis_first_install_normalized_sha256'):
                        bad=copy.deepcopy(guard_receipt);del bad['target'][field]
                        bad['resource_fence']['target_sha256']=resource_claim.digest(bad['target'])
                        with owner.assertRaisesRegex(bridge_root.guard.Blocked,'gis_first_install_target_binding_invalid'):
                            bridge_root.guard.validate(bad,*guard_args,installation=session.installation)
                    self.authorization=result
                    return result
                stack.enter_context(patch.object(transaction,'authorize',side_effect=authorize))
                return state
        bridge_composition_test.CompositionTests()._exercise_catalog_compiler('app-only',deep=Deep())

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
        with tempfile.TemporaryDirectory(dir='/root' if ordinary_fault is not None else None) as tmp:
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
            actions._installation = session.installation
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
                        # Public continuation with a synthetic P2/prerequisite
                        # boundary. Session, durable lineage and the candidate
                        # inventory check below remain real; this is not a P2 proof.
                        import staging_cohort_preservation as cohort
                        import staging_prerequisites as prereq
                        workspace = installed / 'sources' / operation
                        workspace.mkdir()
                        capture_source('fixture', 17, ctx['commit'], workspace)
                        retained_request = dict(request, source_sha=ctx['commit'], resource_context=ctx)
                        retained_request.pop('token')
                        request_path = installed / 'cohort-requests' / (operation + '.json')
                        request_path.parent.mkdir(mode=0o700)
                        cohort._write(request_path, retained_request)
                        p2_plan = dict.fromkeys(cohort.PLAN_FIELDS)
                        p2_plan.update(schema=cohort.SCHEMA, operation=operation, context=ctx,
                            helper_sha256=session.helper, actual_before=old_images,
                            candidate={'commit':ctx['commit'], 'images':ctx['images']})
                        checkpoint = installed / 'cohort-preservation' / operation / 'checkpoint.json'
                        checkpoint.parent.mkdir(mode=0o700, parents=True)
                        cohort._write(checkpoint, {'plan':p2_plan})
                        qualified = {'fixture':'synthetic-qualified-p2'}
                        loaded = {'request':{'mode':request['mode'], 'changed_services':request['changed_services'],
                            'gis_first_install':False}, 'approved':approved, 'execution':execution,
                            'workspace':str(workspace)}
                        parent = {'fixture':'synthetic-prerequisites-capture'}
                        preflight = {'contract':contract, 'enrollment':{}, 'services':[], 'actor':None,
                            'nats':{'scripts':{}, 'plan':{'actions':[]}, 'binding':{}}}
                        original_read = cohort._read
                        app = copy.deepcopy(kube.objects[('Secret','voice-app-secrets')])
                        def read_boundary(path, *args, **kwargs):
                            if path.name == 'stores.json':return {'app_before':copy.deepcopy(app)}
                            return original_read(path, *args, **kwargs)
                        prior_get = kube.get
                        def candidate_get(kind, name):
                            if (kind,name)==('secret','voice-app-secrets'):return copy.deepcopy(app)
                            return prior_get(kind,name)
                        kube.get = candidate_get
                        def prepare_parent(*args, **kwargs):
                            plan = {'schema':'voice-staging-prerequisites-plan-v4',
                                'preservation_sha256':resource_claim.digest(qualified), 'actual_before':old_images}
                            session.bind_prerequisites(plan, old_images)
                        for seam in (patch.object(cohort, 'validate_plan'),
                            patch.object(cohort, 'load_qualified', return_value=qualified),
                            patch.object(cohort, 'load_prepared', return_value=loaded),
                            patch.object(cohort, 'qualify_offnode'),
                            patch.object(cohort, 'begin_continuation', return_value=(qualified,loaded)),
                            patch.object(cohort, 'require_continuation'),
                            patch.object(cohort, 'capture_final_stage', return_value=stage),
                            patch.object(cohort, 'load_preflight', return_value=preflight),
                            patch.object(cohort, '_read', side_effect=read_boundary),
                            patch.object(cohort, '_app_successor'), patch.object(cohort, 'revalidate_paused'),
                            patch.object(bridge_root, '_cohort_story_composition', return_value=None),
                            patch.object(prereq, 'validate_plan'),
                            patch.object(prereq, 'run_prepare', side_effect=prepare_parent),
                            patch.object(prereq, 'capture', return_value=parent),
                            patch.object(prereq, 'load_capture', return_value=parent),
                            patch.object(prereq, 'load_gis_authority', return_value=None)):
                            stack.enter_context(seam)
                        session.bind_cohort_preservation(p2_plan,old_images)
                        authorization = {'action':'authorize', 'operation':operation, 'artifact_id':42,
                            'token':'fixture', 'resource_context':ctx}
                        if ordinary_fault in ('none', 'missing_secret'):
                            with self.assertRaises(ReachedC1): actions.execute(authorization)
                            c1.assert_called_once()
                        else:
                            with self.assertRaisesRegex(bridge_root.Blocked, 'realtime_bootstrap_candidate_changed'):
                                actions.execute(authorization)
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
        self._exercise_public_realtime()

    def test_bridge_existing_c2_anchor_reaches_prepare_and_authorize(self):
        self._exercise_public_realtime(True)

    @unittest.skipUnless(hasattr(os, 'geteuid') and os.geteuid() == 0, 'root-owned cohort lineage')
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
