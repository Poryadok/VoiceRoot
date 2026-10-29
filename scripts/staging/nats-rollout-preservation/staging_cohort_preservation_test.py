"""Protected root preservation tests; external Kubernetes responses are fixtures."""
import copy
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
import sys
from types import SimpleNamespace
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'nats-known-baseline'))
import resource_claim
from resource_claim_test import context, card
import guard
import commands
import staging_cohort_preservation as preservation


class CutEffectTests(unittest.TestCase):
    def test_large_binary_transport_refuses_tamper_and_lost_reply_replay(self):
        import hashlib
        import subprocess
        import staging_cohort_preservation_runtime as runtime
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                session,_=self.fixture(root,{'spec':{'replicas':1}})
                with resource_claim.command_scope(session.check):
                    nonce=preservation._begin_restore(session,'local');base=preservation._base(session)
                    body=b'x'*((2<<20)+1);cid='d'*64
                    preservation._write(base/'restore-tools.json',{'images':{},'ids':{},'migrate_binary_sha256':hashlib.sha256(body).hexdigest()})
                    resources=base/'restore'/nonce/'resources.json'
                    preservation.save(resources,{'operation':session.operation,'attempt':nonce,'resources':{
                        'owned':{'kind':'postgres','cid':cid,'status':'ACTIVE'}}})
                    argv=runtime.DOCKER+['exec','-i',cid,'sh','-ec','cat > /tmp/voice-migrate && chmod 0555 /tmp/voice-migrate']
                    with patch.object(subprocess,'Popen') as start:
                        for args,raw in ((argv,body[:-1]+b'y'),(argv[:-1]+['cat > /tmp/foreign'],body)):
                            with self.assertRaisesRegex(Exception,'cohort_restore_binary_copy_invalid'):
                                runtime.capture_migrate_binary(session,args,raw,timeout=120)
                        start.assert_not_called()
                    with patch.object(subprocess,'Popen',side_effect=RuntimeError('terminal_reply_lost')) as start:
                        with self.assertRaisesRegex(RuntimeError,'terminal_reply_lost'):
                            preservation.restore_command(session,argv,body=body)
                        with self.assertRaisesRegex(Exception,'cohort_reconciliation_required'):
                            preservation.restore_command(session,argv,body=body)
                        self.assertEqual(start.call_count,1)

    def test_store_projection_uses_qualified_candidate_policy_without_committing_it(self):
        import hashlib
        import staging_release_policy_prepare as policy
        parent=preservation.private
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                session,_=self.fixture(root,{'spec':{'replicas':1}})
                approved={'source_sha':session.context['commit'],'source_files':{}}
                old=parent._bytes({'release_policy':policy.staging_descriptor('d'*40),'unrelated':'retained'})
                parent._private_write(root/'installed'/'policy.json',old)
                plan={k:'c'*64 for k in parent.PLAN_FIELDS}
                plan.update(schema='voice-staging-prerequisites-plan-v1',operation=session.operation,context=session.context,
                    helper_sha256=session.helper,approved_sha256=preservation.digest(approved),namespace_uid='ns',
                    actual_before={'app/image':'old'},children={'release_policy':{'descriptor':policy.staging_descriptor(approved['source_sha'])},'p3':{},'gis_material':{}},
                    policy_before_sha256=hashlib.sha256(old).hexdigest(),auxiliary_images={},renderer_authority_sha256=None,previous_active_sha256=None)
                capture={'schema':'voice-staging-source-capture-v1','operation':session.operation,'context':session.context,
                    'helper_sha256':session.helper,'approved_sha256':plan['approved_sha256'],'execution_sha256':plan['execution_sha256']}
                plan['source_capture_sha256']=preservation.digest(capture)
                parent._private_directory(root/'installed'/'source-authority')
                preservation._write(root/'installed'/'source-authority'/(session.operation+'.json'),capture)
                base=root/'installed'/'prerequisites'/session.operation
                parent._private_directory(base.parent);parent._private_directory(base)
                prepared={'schema':'voice-staging-prerequisites-prepared-v1','plan':plan,'request':{},'gis_authority_sha256':'f'*64}
                preservation._write(base/'prepared.json',prepared)
                method=getattr(preservation,'_store_policy',None);self.assertTrue(callable(method))
                with resource_claim.command_scope(session.check):
                    result=method(session,approved,preservation.digest(prepared))
                    self.assertEqual(result['release_policy'],plan['children']['release_policy']['descriptor'])
                    self.assertEqual(result['unrelated'],'retained')
                    self.assertEqual((root/'installed'/'policy.json').read_bytes(),old)
                    with self.assertRaisesRegex(Exception,'cohort_prepared_changed'):method(session,approved,'0'*64)
                    (root/'installed'/'policy.json').write_bytes(old+b' ')
                    with self.assertRaisesRegex(Exception,'cohort_store_policy_changed'):method(session,approved,preservation.digest(prepared))

    def fixture(self, root, before,namespace_uid='namespace',preflight=None):
        c=context(); current=card(c)
        journal=resource_claim.RootJournal(root)
        session=resource_claim.Session(c,'a'*12,'b'*64,lambda:copy.deepcopy(current),journal)
        session.admit(start=True)
        path=root/'installed'/'cohort-preservation'/session.operation
        preservation.private._private_directory(path.parent)
        preservation.private._private_directory(path)
        precut={'context':{'original_snapshots':{'voice-user':before},'marker':{}}}
        plan={key:'c'*64 for key in preservation.PLAN_FIELDS}
        plan.update(schema=preservation.SCHEMA,operation=session.operation,context=c,
            helper_sha256=session.helper,namespace_uid=namespace_uid,actual_before=current['observed']['images'],
            candidate={'commit':c['commit'],'images':c['images']},precut_capture_sha256=preservation.digest(precut))
        if preflight is not None:
            plan['preflight_sha256']=preservation.digest(preflight)
            preservation._write(path/'preflight.json',preflight)
        tool={'schema':'voice-staging-cohort-space-tool-v1','operation':session.operation,
            'source_capture_sha256':plan['source_capture_sha256'],'approved_sha256':plan['approved_sha256'],
            'selected':False,'authenticated_publication':None}
        plan['space_tool_sha256']=preservation.digest(tool);preservation._write(path/'space-tool.json',tool)
        preservation._write(path/'precut.json',precut)
        preservation._write(path/'checkpoint.json',{'schema':'voice-staging-cohort-preservation-checkpoint-v1','plan':plan})
        session.bind_cohort_preservation(plan,plan['actual_before'])
        return session,current

    def test_response_uid_generation_or_spec_drift_remains_pending(self):
        for fault in ('uid','generation','image'):
            with self.subTest(fault=fault),tempfile.TemporaryDirectory(dir='/root') as directory:
                root=Path(directory);(root/'installed').mkdir(mode=0o700)
                with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                    before={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-user','namespace':'voice-staging',
                        'uid':'user','resourceVersion':'3','generation':1},'spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'user','image':'old'}]}}}}
                    session,_=self.fixture(root,before)
                    after=copy.deepcopy(before);after['spec']['replicas']=0;after['metadata'].update(resourceVersion='4',generation=2)
                    if fault=='uid':after['metadata']['uid']='foreign'
                    elif fault=='generation':after['metadata']['generation']=3
                    else:after['spec']['template']['spec']['containers'][0]['image']='foreign'
                    def submit(argv,**kwargs):
                        commands.authority_guard(argv)
                        return json.dumps(after).encode()
                    with resource_claim.command_scope(session.check),patch.object(commands,'capture',side_effect=submit):
                        with self.assertRaisesRegex(Exception,'cohort_cut_response_changed'):
                            preservation.cut_cas(session,'deployment',before,[{'op':'replace','path':'/spec/replicas','value':0}])
                        with self.assertRaisesRegex(Exception,'reconciliation_required'):
                            preservation.version_read(session)
                    record=preservation._read(root/'installed'/'cohort-preservation'/session.operation/'effects'/'cut-deployment-voice-user.json')
                    self.assertEqual(record['status'],'PENDING')

    def test_foreign_snapshot_and_tampered_request_refused_before_api(self):
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                before={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-user','namespace':'voice-staging',
                    'uid':'user','resourceVersion':'3','generation':1},'spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'user','image':'old'}]}}}}
                session,_=self.fixture(root,before)
                with resource_claim.command_scope(session.check),patch.object(commands,'capture') as submit:
                    foreign=copy.deepcopy(before);foreign['metadata']['uid']='other'
                    with self.assertRaisesRegex(Exception,'cohort_cut_scope_changed'):
                        preservation.cut_cas(session,'deployment',foreign,[{'op':'replace','path':'/spec/replicas','value':0}])
                    path=preservation._base(session)/'effects';preservation.private._private_directory(path)
                    argv=preservation.PREFIX+['patch','deployment','voice-user','--type=json','--patch-file=/dev/stdin','-o','json']
                    preservation._write(path/'cut-test.json',{'status':'PENDING','operation':session.operation,
                        'effect_id':'cut-test','binding':session.binding(),'argv':argv,'body_sha256':'f'*64})
                    with self.assertRaisesRegex(Exception,'cohort_effect_changed'):
                        preservation._send(session,'cut-test',argv,b'[]')
                    submit.assert_not_called()

    def test_lost_cut_response_retains_pending_and_forbids_replay(self):
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                before={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-user','namespace':'voice-staging',
                    'uid':'user','resourceVersion':'3','generation':1},'spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'user','image':'registry/user@sha256:'+'b'*64}]}}}}
                session,current=self.fixture(root,before)
                after=copy.deepcopy(before);after['spec']['replicas']=0;after['metadata'].update(resourceVersion='4',generation=2)
                path=root/'installed'/'cohort-preservation'/session.operation
                calls=[]
                def submit(argv,*,body,**kwargs):
                    commands.authority_guard(argv)
                    record=json.loads((path/'effects'/'cut-deployment-voice-user.json').read_bytes())
                    self.assertEqual(record['status'],'PENDING');calls.append((argv,body))
                    raise RuntimeError('applied_response_lost')
                with resource_claim.command_scope(session.check), patch.object(commands,'capture',side_effect=submit):
                    with self.assertRaisesRegex(RuntimeError,'applied_response_lost'):
                        preservation.cut_cas(session,'deployment',before,[{'op':'replace','path':'/spec/replicas','value':0}])
                    with self.assertRaisesRegex(Exception,'reconciliation_required'):
                        preservation.cut_cas(session,'deployment',before,[{'op':'replace','path':'/spec/replicas','value':0}])
                self.assertEqual(len(calls),1)
                record=json.loads((path/'effects'/'cut-deployment-voice-user.json').read_bytes())
                self.assertEqual(record['status'],'PENDING')

    def test_exact_cut_response_requires_same_uid_and_replica_only_delta(self):
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                before={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-user','namespace':'voice-staging',
                    'uid':'user','resourceVersion':'3','generation':1},'spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'user','image':'registry/user@sha256:'+'b'*64}]}}}}
                session,current=self.fixture(root,before)
                after=copy.deepcopy(before);after['spec']['replicas']=0;after['metadata'].update(resourceVersion='4',generation=2)
                def submit(argv,*,body,**kwargs):
                    commands.authority_guard(argv)
                    patch_body=json.loads(body)
                    self.assertEqual(patch_body[:2],[{'op':'test','path':'/metadata/uid','value':'user'},
                        {'op':'test','path':'/metadata/resourceVersion','value':'3'}])
                    return json.dumps(after).encode()
                with resource_claim.command_scope(session.check),patch.object(commands,'capture',side_effect=submit):
                    result=preservation.cut_cas(session,'deployment',before,[{'op':'replace','path':'/spec/replicas','value':0}])
                self.assertEqual(result,after)
                record=json.loads((root/'installed'/'cohort-preservation'/session.operation/'effects'/'cut-deployment-voice-user.json').read_bytes())
                self.assertEqual(record['status'],'DONE');self.assertEqual(record['response'],after)

    def test_isolated_clone_forbids_host_mount_and_lost_create_replay(self):
        import staging_cohort_preservation_runtime as runtime
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                before={'metadata':{'name':'voice-user','namespace':'voice-staging','uid':'user','resourceVersion':'3','generation':1},'spec':{'replicas':1}}
                session,_=self.fixture(root,before)
                with resource_claim.command_scope(session.check):
                    preservation._begin_restore(session,'tooling')
                    preservation._write(preservation._base(session)/'restore-tools.json',{
                        'images':{'postgres':'postgres@sha256:'+'a'*64,'migrate':'migrate@sha256:'+'b'*64},
                        'ids':{'postgres':'sha256:'+'c'*64,'migrate':'sha256:'+'d'*64}})
                    journal=preservation.restore_journal(session);name='voice-media-restore-'+'e'*32
                    journal.begin('migrate',name,{'voice.test.owner':name})
                    argv=runtime.DOCKER+['create','--name',name,'--network','none','--label','voice.test.owner='+name,
                        '--entrypoint','/bin/true','sha256:'+'d'*64]
                    with patch.object(commands,'capture') as capture:
                        foreign=argv[:-1]+['--mount','type=bind,source=/,target=/host',argv[-1]]
                        with self.assertRaisesRegex(Exception,'cohort_restore_create_invalid'):
                            preservation.restore_command(session,foreign)
                        capture.assert_not_called()
                    calls=[]
                    def lost(args,**kwargs):
                        commands.authority_guard(args);calls.append(args)
                        raise RuntimeError('created_reply_lost')
                    with patch.object(commands,'capture',side_effect=lost):
                        with self.assertRaisesRegex(RuntimeError,'created_reply_lost'):
                            preservation.restore_command(session,argv)
                        with self.assertRaisesRegex(Exception,'cohort_reconciliation_required'):
                            preservation.restore_command(session,argv)
                    self.assertEqual(len(calls),1)

    def test_auxiliary_resume_requires_offnode_and_retains_lost_response(self):
        import staging_cohort_preservation_auxiliary as auxiliary
        import staging_p3_prerequisites as p3
        for fault in ('missing_offnode','lost_reply','wrong_image','success'):
            with self.subTest(fault=fault),tempfile.TemporaryDirectory(dir='/root') as directory:
                root=Path(directory);(root/'installed').mkdir(mode=0o700)
                with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                    original={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':auxiliary.NAME,'namespace':'voice-staging',
                        'uid':'publisher','resourceVersion':'3','generation':1},'spec':{'replicas':1,
                        'template':{'spec':{'containers':[{'name':'jwks','image':'nginx@sha256:'+'c'*64}]}}}}
                    session,_=self.fixture(root,original)
                    state=preservation._read(preservation._base(session)/'checkpoint.json');state['status']='AWAITING_OFF_NODE'
                    preservation.save(preservation._base(session)/'checkpoint.json',state)
                    paused=copy.deepcopy(original);paused['spec']['replicas']=0;paused['metadata'].update(resourceVersion='4',generation=2)
                    response=copy.deepcopy(original);response['metadata'].update(resourceVersion='5',generation=3)
                    if fault=='wrong_image':response['spec']['template']['spec']['containers'][0]['image']='foreign'
                    child={'preserved':{auxiliary.IDENT:{'observed':{'semantic_sha256':preservation.digest(p3.private_semantic(original))}}}}
                    stage=SimpleNamespace(original_snapshots={auxiliary.NAME:original},snapshots={auxiliary.NAME:paused},
                        kube=SimpleNamespace(get=lambda *args:copy.deepcopy(paused)))
                    calls=[]
                    def send(argv,**kwargs):
                        commands.authority_guard(argv);calls.append(argv)
                        self.assertEqual(json.loads(kwargs['body'])[-1],{'op':'replace','path':'/spec/replicas','value':1})
                        if fault=='lost_reply':raise RuntimeError('resume_applied_reply_lost')
                        return json.dumps(response).encode()
                    with resource_claim.command_scope(session.check),patch.object(auxiliary,'_inputs',return_value=(child,)),\
                        patch.object(preservation,'_paused_stage',return_value=stage),patch.object(auxiliary,'_health',return_value={'owned_pod_fixture':True}),\
                        patch.object(commands,'capture',side_effect=send):
                        if fault=='missing_offnode':
                            with self.assertRaises(FileNotFoundError):auxiliary.resume(session)
                            self.assertEqual(calls,[]);continue
                        with patch.object(preservation,'_offnode_records'):
                            if fault=='success':
                                value=auxiliary.resume(session);self.assertEqual(value['response'],response)
                            else:
                                with self.assertRaises(Exception):auxiliary.resume(session)
                                with self.assertRaisesRegex(Exception,'cohort_reconciliation_required'):auxiliary.resume(session)
                            self.assertEqual(len(calls),1)

    def test_native_readiness_is_one_owned_bounded_operation(self):
        import staging_cohort_preservation_runtime as runtime
        for lost in (False,True):
            with self.subTest(lost=lost),tempfile.TemporaryDirectory(dir='/root') as directory:
                root=Path(directory);(root/'installed').mkdir(mode=0o700)
                with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                    before={'metadata':{'name':'voice-user','namespace':'voice-staging','uid':'user',
                        'resourceVersion':'3','generation':1},'spec':{'replicas':1}}
                    session,_=self.fixture(root,before)
                    preservation._runtime_base(session).mkdir(mode=0o700)
                    native=runtime.NativeRuntime(session);name='voice-known-'+session.operation+'-copy'
                    native.captured[name]={'name':name,'id':'d'*64};calls=[]
                    argv=['exec',name,'/bin/busybox','wget','-q','-O','-','http://127.0.0.1:8222/healthz']
                    def terminal(args,**kwargs):
                        commands.authority_guard(args);calls.append(args)
                        self.assertEqual(args[:7],runtime.DOCKER+['exec',name,'/bin/busybox','sh','-ec'])
                        self.assertIn('"$i" -lt 120',args[-1]);self.assertEqual(kwargs['timeout'],160)
                        if lost:raise RuntimeError('terminal_reply_lost')
                        return b'{"status":"ok"}'
                    with resource_claim.command_scope(session.check),patch.object(preservation,'revalidate_paused'),\
                        patch.object(commands,'capture',side_effect=terminal):
                        foreign=argv.copy();foreign[1]='unowned'
                        with self.assertRaisesRegex(Exception,'cohort_native_command_invalid'):native._run(foreign)
                        self.assertEqual(calls,[])
                        if lost:
                            with self.assertRaisesRegex(RuntimeError,'terminal_reply_lost'):native._run(argv)
                            with self.assertRaisesRegex(Exception,'cohort_reconciliation_required'):native._run(argv)
                        else:self.assertEqual(native._run(argv),'{"status":"ok"}')
                        self.assertEqual(len(calls),1)

    def test_final_capture_adds_only_completed_publisher_to_paused_stage(self):
        import staging_p3_prerequisites as p3
        name='voice-p3-principal-jwks';ident='Deployment/'+name;image='nginx@sha256:'+'e'*64
        for fault in ('success','status_only','foreign_uid','pending','extra_image'):
            with self.subTest(fault=fault),tempfile.TemporaryDirectory(dir='/root') as directory:
                root=Path(directory);(root/'installed').mkdir(mode=0o700)
                with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                    before={'metadata':{'name':'voice-user','namespace':'voice-staging','uid':'user',
                        'resourceVersion':'3','generation':1},'spec':{'replicas':1}}
                    session,current=self.fixture(root,before);old=current['observed']['images']
                    publisher={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':name,'namespace':'voice-staging',
                        'uid':'publisher','resourceVersion':'7','generation':1},'spec':{'replicas':1,
                            'template':{'spec':{'containers':[{'name':'jwks','image':image}]}}}}
                    paused=copy.deepcopy(before);paused['spec']['replicas']=0
                    stage=SimpleNamespace(snapshots={'voice-user':paused},original_snapshots={'voice-user':before},
                        expected={'deployment_uids':{'voice-user':'user'}},old_images=copy.deepcopy(old),
                        kube=SimpleNamespace(get=lambda *args:copy.deepcopy(publisher)))
                    qualified={'root_qualified_fixture':True};child={'preserved':{}}
                    observation={'uid':'publisher','resourceVersion':'7','generation':1,
                        'semantic_sha256':preservation.digest(p3.private_semantic(publisher))}
                    original_publisher=copy.deepcopy(publisher)
                    import hashlib
                    attempts=root/'installed'/'prerequisites'/session.operation/'attempts'
                    attempts.mkdir(parents=True,mode=0o700)
                    preservation._write(attempts/(hashlib.sha256(('p3/'+ident).encode()).hexdigest()+'.response.json'),
                        {'object':original_publisher,'observation':observation})
                    state={'plan':{'schema':'voice-staging-prerequisites-plan-v2','preservation_sha256':preservation.digest(qualified),
                        'auxiliary_images':{name+'/jwks':image},'children':{'p3':child}},
                        'steps':{'p3':{ident:{'status':'DONE','observation':observation}}},
                        'receipt':{'publisher_identity':observation,'actual_after':{**old,name+'/jwks':image}}}
                    proof={'deployment':{'uid':'publisher','generation':1,'semantic_sha256':preservation.digest(p3.private_semantic(publisher))}}
                    ready={'proof':proof,'receipt':{'operation':session.operation,'plan_sha256':preservation.digest(child),
                        'publisher_ready_sha256':preservation.digest(proof)}}
                    if fault=='foreign_uid':publisher['metadata']['uid']='foreign'
                    if fault=='pending':state['steps']['p3'][ident]['status']='PENDING'
                    if fault=='extra_image':state['plan']['auxiliary_images']['voice-other/x']=image
                    if fault=='status_only':publisher['metadata']['resourceVersion']='8';publisher['status']={'readyReplicas':1}
                    with resource_claim.command_scope(session.check),patch.object(preservation,'load_qualified',return_value=qualified),\
                        patch.object(preservation,'capture_stage',return_value=stage),patch.object(preservation.private,'_completed',return_value=state),\
                        patch.object(preservation.private,'_checkpoint',return_value=state),patch.object(preservation,'revalidate_paused'),\
                        patch.object(preservation.private,'load_done',return_value={ident:observation}),\
                        patch.object(p3,'verify_complete',return_value=ready):
                        if fault not in ('success','status_only'):
                            with self.assertRaisesRegex(Exception,'cohort_final_'):preservation.capture_final_stage(session)
                        else:
                            result=preservation.capture_final_stage(session)
                            self.assertEqual(result.snapshots['voice-user'],paused)
                            self.assertEqual(result.original_snapshots['voice-user'],before)
                            self.assertEqual(result.snapshots[name],original_publisher)
                            self.assertEqual(set(result.snapshots),{'voice-user',name})
                            self.assertEqual(result.old_images,state['receipt']['actual_after'])


class CatalogTests(unittest.TestCase):
    def test_selected_space_proofs_are_required_and_hash_bound(self):
        import staging_space_native as inspector
        import staging_cohort_preservation_space as native
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                session,_=CutEffectTests().fixture(root,{'spec':{'replicas':1}})
                with resource_claim.command_scope(session.check):
                    method=getattr(preservation,'_space_record_hashes',None);self.assertTrue(callable(method))
                    base=preservation._base(session);plan=preservation.load_plan(session)
                    self.assertEqual(method(base,plan),{'space_tool_available_sha256':None,'space_inspection_sha256':None})
                    selected=preservation._read(base/'space-tool.json');selected.update(selected=True,authenticated_publication={'publication':{}})
                    plan={**plan,'space_tool_sha256':preservation.digest(selected)};preservation.save(base/'space-tool.json',selected)
                    with self.assertRaises(Exception):method(base,plan)
                    available={'actual_loader_fixture':'available'};inspection={'actual_loader_fixture':'inspection'}
                    preservation._write(base/'space-tool-available.json',available);preservation._write(base/'space-inspection.json',inspection)
                    expected={'space_tool_available_sha256':preservation.digest(available),'space_inspection_sha256':preservation.digest(inspection)}
                    self.assertEqual(method(base,plan),expected)
                    with patch.object(preservation,'load_plan',return_value=plan),patch.object(native,'load_available_tool',return_value=available),\
                        patch.object(inspector,'load_retained_inspection',return_value=inspection):
                        self.assertEqual(preservation._verify_space_proofs(session),expected)
                        preservation.save(base/'space-inspection.json',{'tampered':True})
                        with self.assertRaisesRegex(Exception,'cohort_space_proof_changed'):preservation._verify_space_proofs(session)

    def test_space_identity_rechecks_original_oid_owner_and_schema(self):
        import staging_cohort_preservation_pg as pg
        from stage_runtime import Kube
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root=Path(directory);(root/'installed').mkdir(mode=0o700)
            with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                session,_=CutEffectTests().fixture(root,{'spec':{'replicas':1}})
                with resource_claim.command_scope(session.check):
                    plan=preservation.load_plan(session);base=preservation._base(session)
                    catalog={'schema':'voice-staging-cohort-catalog-v1','operation':session.operation,
                        'plan_sha256':preservation.digest(plan),'engine':{'system_identifier':'123'},
                        'server_start':'start','databases':{'space_db':42},'properties':{'space_db':{'owner':'original_owner'}}}
                    metadata={'schema':'voice-staging-cohort-metadata-v1','operation':session.operation,
                        'plan_sha256':preservation.digest(plan),'catalog_sha256':preservation.digest(catalog),
                        'source':{'catalog':{'space_db':{'database_owner':'original_owner','schema_owner':'original_owner'}}}}
                    preservation._write(base/'catalog.json',catalog);preservation._write(base/'metadata.json',metadata)
                    preservation._write(base/'archive-record.json',{'catalog_ref':preservation._reference(session,catalog),
                        'metadata_ref':preservation._reference(session,metadata)})
                    engine={'engine':catalog['engine'],'server_start':'start'}
                    current={'database_name':'space_db','database_oid':42,'database_owner':'original_owner',
                        'schema_owner':'original_owner','installer_role':'original_owner'}
                    method=getattr(pg,'guard_space_identity',None);self.assertTrue(callable(method))
                    with patch.object(pg,'guard_identity',return_value=engine) as physical,patch.object(Kube,'_run',return_value=current) as query:
                        self.assertEqual(method(session),{**engine,**current})
                        self.assertEqual(query.call_args.args[0],pg.SPACE_IDENTITY_ARGS)
                        self.assertEqual(physical.call_count,2)
                        for key,value in (('database_oid',43),('database_owner','foreign'),('schema_owner','foreign'),('installer_role','foreign')):
                            with self.subTest(key=key):
                                query.return_value={**current,key:value}
                                with self.assertRaisesRegex(Exception,'cohort_space_identity_changed'):method(session)

    def test_guard_postgres_uses_exact_read_and_rejects_engine_drift(self):
        import realtime_bootstrap
        import staging_cohort_preservation_pg as pg
        from stage_runtime import Kube
        for fault in ('none','system','restart','after'):
            with self.subTest(fault=fault),tempfile.TemporaryDirectory(dir='/root') as directory:
                root=Path(directory);(root/'installed').mkdir(mode=0o700)
                with patch.object(guard,'ROOT',root),patch.object(preservation,'_source'):
                    session,_=CutEffectTests().fixture(root,{'metadata':{'uid':'user'},'spec':{'replicas':1}})
                    with resource_claim.command_scope(session.check):
                        plan=preservation.load_plan(session)
                        engine={'pod':{'uid':'pod','container_id':'container','restart_count':0},'pvc':{'uid':'pvc'},'pv':{'uid':'pv'}}
                        current={'namespace_uid':'namespace','pod_uid':'pod','container_id':'container','restart_count':0,
                            'pvc_uid':'pvc','pv_uid':'pv','system_identifier':'123'}
                        old={'schema':'voice-staging-cohort-catalog-v1','operation':session.operation,
                            'plan_sha256':preservation.digest(plan),'engine':current,'server_start':'start'}
                        base=preservation._base(session)
                        preservation._write(base/'catalog.json',old)
                        preservation._write(base/'archive-record.json',{'catalog_ref':preservation._reference(session,old)})
                        value={'system_identifier':'wrong' if fault=='system' else '123','server_start':'start'}
                        if fault=='restart':engine['pod']['restart_count']=1
                        after=copy.deepcopy(engine)
                        if fault=='after':after['pod']['uid']='foreign'
                        def query(args):
                            self.assertEqual(args,pg.IDENTITY_ARGS);commands.authority_guard(preservation.PREFIX+args)
                            return copy.deepcopy(value)
                        with patch.object(Kube,'_run',side_effect=query),patch.object(realtime_bootstrap,'engine_snapshot',
                            side_effect=[('namespace',engine),('namespace',after)]):
                            if fault=='none':self.assertEqual(preservation.guard_postgres(session),{'engine':current,'server_start':'start'})
                            else:
                                with self.assertRaisesRegex(Exception,'cohort_postgres_changed'):
                                    preservation.guard_postgres(session)
                        changed=pg.IDENTITY_ARGS.copy();changed[-1]+='; SELECT 1'
                        with self.assertRaises(Exception):session.check(preservation.PREFIX+changed)

    def test_closed_copy_requalifies_exact_frozen_authority_not_live_census(self):
        import nats_root_plan
        original={'plan':{'source':'exact'},'binding':{'source':'exact'},'scripts':{},'live_census_sha256':'precut'}
        frozen={'nats':original,'enrollment':{},'contract':{}}
        current={**copy.deepcopy(original),'live_census_sha256':'closed-copy'}
        session=SimpleNamespace(operation='a'*12)
        with patch.object(preservation,'load_preflight',return_value=frozen),patch.object(preservation,'revalidate_paused'),\
            patch.object(nats_root_plan,'closed_copy_preflight',return_value=current) as qualify:
            self.assertEqual(preservation._closed_copy_authority(session,'runtime','broker',{'tree':1}),current)
            self.assertEqual(qualify.call_args.args[-3:],('runtime','broker',{'tree':1}))
            current['binding']={'source':'foreign'}
            with self.assertRaisesRegex(Exception,'cohort_closed_copy_authority_changed'):
                preservation._closed_copy_authority(session,'runtime','broker',{'tree':1})

    def test_full_catalog_refuses_active_clients_prepared_or_unreadable_database(self):
        import staging_cohort_preservation_pg as pg
        engine={'pod':{'uid':'pod','container_id':'containerd://'+'a'*64,'restart_count':0},'pvc':{'uid':'pvc'},'pv':{'uid':'pv'}}
        row={'name':'postgres','oid':5,'allowconn':True,'template':False,'encoding':'UTF8','collate':'C','ctype':'C',
            'locale_provider':'c','icu_locale':None,'connection_limit':-1,'owner':'postgres','acl':None,'connect':True,'sessions':0,'prepared':0}
        value={'system_identifier':'123','server_start':'2026-10-29','can_read':True,'databases':[row]}
        self.assertEqual(pg.validate_catalog(value,engine,'namespace','a'*12,'b'*64)['databases'],{'postgres':5})
        for field,changed in (('sessions',1),('prepared',1),('connect',False)):
            with self.subTest(field=field):
                bad=copy.deepcopy(value);bad['databases'][0][field]=changed
                with self.assertRaisesRegex(Exception,'cohort_catalog_invalid'):
                    pg.validate_catalog(bad,engine,'namespace','a'*12,'b'*64)


if __name__=='__main__':unittest.main()
