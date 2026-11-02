import copy
import hashlib
import json
import tempfile
import unittest
import sys
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import guard
import resource_claim
import resource_claim_test
import staging_prerequisites as prereq
from root_main import save


def bind_fixture_prerequisites(session, plan):
    """Exercise the real session transitions with a deliberately mocked P2 receipt."""
    import staging_cohort_preservation as cohort
    receipt={'fixture':'qualified-p2'}
    p2={'actual_before':plan['actual_before'],
        'candidate':{'commit':session.context['commit'],'images':session.context['images']}}
    plan.update(schema='voice-staging-prerequisites-plan-v4',installation_sha256=session.installation.sha256,
                preservation_sha256=prereq.digest(receipt))
    with patch.object(cohort,'validate_plan'),patch.object(cohort,'load_qualified',return_value=receipt):
        session.bind_cohort_preservation(p2,plan['actual_before'])
        return session.bind_prerequisites(plan,plan['actual_before'])


class PrivateCustodyTests(unittest.TestCase):
    def test_paused_gis_authority_requires_protected_prepared_hash_and_active_anchor(self):
        import staging_cohort_preservation as cohort
        import gis_existing_update
        from stage_runtime import Kube
        binding={'operation':'d'*12,'anchor_sha256':'e'*64}
        active={'gis_first_install_transition':{'operation':binding['operation'],'receipt_sha256':binding['anchor_sha256']}}
        record={'binding':binding,'active':active}
        prepared={'schema':'voice-staging-prerequisites-prepared-v1','plan':{},'request':{},'gis_authority_sha256':prereq.digest(record)}
        receipt={'prerequisite_inputs_sha256':prereq.digest(prepared)};session=SimpleNamespace(check=lambda:None)
        def read(path,**kwargs):
            values={'prepared.json':prepared,'gis-authority.json':record,'active-release.json':active}
            return json.dumps(values[path.name]).encode()
        with patch.object(cohort,'load_qualified',return_value=receipt),patch.object(prereq,'_base',return_value=Path('/fixture')), \
             patch.object(prereq,'_read_private',side_effect=read),patch.object(gis_existing_update,'verify_phase') as verify,patch.object(Kube,'__init__',return_value=None):
            self.assertEqual(prereq.load_gis_authority(session),binding)
            self.assertEqual(verify.call_args.args[2],'paused-old')
            record['binding']['operation']='f'*12
            with self.assertRaisesRegex(prereq.Blocked,'staging_prerequisites_gis_authority_changed'):
                prereq.load_gis_authority(session)

    def test_v2_capture_binds_producer_paused_stage_and_rejects_drift(self):
        import realtime_bootstrap
        import staging_cohort_preservation as cohort
        from runtime_stage import RolloutStage
        session=SimpleNamespace(operation='a'*12,context={'claim':'fixture'},helper='b'*64)
        stage=object.__new__(RolloutStage);stage.operation=session.operation
        stage.expected={'deployment_uids':{'voice-user':'uid'}};stage.old_images={'voice-user/user':'image'}
        stage.marker={'metadata':{'name':'marker'},'data':{'phase':'capturing'}}
        stage.snapshots={'voice-user':{'metadata':{'uid':'uid','generation':2},'spec':{'replicas':0}}}
        stage.original_snapshots={'voice-user':{'metadata':{'uid':'uid','generation':1},'spec':{'replicas':1}}}
        state={'plan':{'schema':'voice-staging-prerequisites-plan-v2','source_capture_sha256':'c'*64,'namespace_uid':'namespace'},
               'receipt':{'actual_before':stage.old_images,'actual_after':stage.old_images}}
        with patch.object(prereq,'_completed',return_value=state),patch.object(prereq,'_private_write'), \
             patch.object(prereq,'_base',return_value=Path('/fixed')),patch.object(cohort,'capture_final_stage',return_value=stage):
            result=prereq.capture(session,stage)
            self.assertEqual(result['stage']['workloads'],realtime_bootstrap._stage_view(stage))
            altered=copy.deepcopy(stage);altered.snapshots['voice-user']['spec']['replicas']=1
            with self.assertRaisesRegex(prereq.Blocked,'staging_prerequisites_capture_changed'):prereq.capture(session,altered)

    def test_preserved_publisher_accepts_only_bound_auxiliary_metadata(self):
        import staging_cohort_preservation as cohort
        response={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-p3-principal-jwks','namespace':'voice-staging',
                  'uid':'publisher-uid','resourceVersion':'7','generation':3},'spec':{'replicas':1,'template':{'spec':{'containers':[]}}}}
        observed={'uid':'publisher-uid','resourceVersion':'2','generation':1,'semantic_sha256':prereq.digest(prereq._semantic(response))}
        plan={'children':{'p3':{'preserved':{'Deployment/voice-p3-principal-jwks':{'observed':observed}}}}}
        with patch.object(cohort,'load_auxiliary_successor',return_value={'response':response}):
            prereq._bind_auxiliary_successor(object(),plan)
        self.assertEqual((observed['generation'],observed['resourceVersion']),(3,'7'))
        changed=copy.deepcopy(response);changed['spec']['replicas']=2
        with patch.object(cohort,'load_auxiliary_successor',return_value={'response':changed}):
            with self.assertRaisesRegex(prereq.Blocked,'staging_prerequisites_auxiliary_changed'):
                prereq._bind_auxiliary_successor(object(),plan)
        with patch.object(cohort,'load_auxiliary_successor',return_value=None):
            with self.assertRaisesRegex(prereq.Blocked,'staging_prerequisites_auxiliary_missing'):
                prereq._bind_auxiliary_successor(object(),plan)

    def test_auxiliary_module_executes_exact_source_not_timestamp_cached_bytecode(self):
        import os, py_compile
        with tempfile.TemporaryDirectory(dir='/root') as folder:
            source=Path(folder);path=source/'scripts/deploy/staging_p3_auxiliary_image.py';path.parent.mkdir(parents=True,mode=0o700)
            good=b"VALUE = 'approved'\n";bad=b"VALUE = 'injected'\n"
            path.write_bytes(bad);path.chmod(0o600);timestamp=int(path.stat().st_mtime);os.utime(path,(timestamp,timestamp))
            py_compile.compile(str(path),doraise=True,invalidation_mode=py_compile.PycInvalidationMode.TIMESTAMP)
            path.write_bytes(good);os.utime(path,(timestamp,timestamp))
            approved={'source_files':{'scripts/deploy/staging_p3_auxiliary_image.py':hashlib.sha256(good).hexdigest()}}
            self.assertEqual(prereq._source_module(approved,source).VALUE,'approved')

    def test_crypto_helper_is_required(self):
        self.assertIn('nats-rollout-preservation/staging_p3_prerequisites_crypto.py',prereq.REQUIRED_HELPERS)
        for name in ('gis_prerequisite','source_plan','compiler'):
            self.assertIn('nats-rollout-preservation/'+name+'.py',prereq.REQUIRED_HELPERS)

    def test_initial_capture_requires_real_session_and_exclusive_operation(self):
        import bridge_root
        with tempfile.TemporaryDirectory(dir='/root') as folder:
            root=Path(folder);installed=root/'installed'
            for path in (installed/'source-authority',installed/'prerequisites',installed/'code'/'helper',
                         installed/'sources'/('a'*12)/'scripts'/'staging'/'helper'):
                path.mkdir(parents=True,mode=0o700)
            raw=b'fixture';name='helper/fixture.py';wanted=hashlib.sha256(raw).hexdigest()
            for path in (installed/'code'/name,installed/'sources'/('a'*12)/'scripts'/'staging'/name):
                path.write_bytes(raw);path.chmod(0o600)
            binding={name:wanted};save(installed/'code'/'capture-manifest.json',binding)
            context=resource_claim_test.context();card=resource_claim_test.card(context)
            session=resource_claim.Session(context,'a'*12,prereq.digest(binding),lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal());session.admit(start=True)
            approved={'source_sha':context['commit'],'source_files':{'scripts/staging/'+name:wanted}}
            execution={'fixture':'execution'}
            receipt={'schema':'voice-staging-source-capture-v2','installation_sha256':session.installation.sha256,'operation':session.operation,'context':context,
                     'helper_sha256':session.helper,'approved_sha256':prereq.digest(approved),'execution_sha256':prereq.digest(execution)}
            save(installed/'source-authority'/('a'*12+'.json'),receipt)
            actions=bridge_root.Actions(installed/'code');actions.tt_session=session
            with patch.object(guard,'ROOT',root),patch.object(prereq,'REQUIRED_HELPERS',frozenset((name,))),patch.object(prereq,'BINARY_PINS',{}):
                with self.assertRaises(prereq.Blocked):prereq._capture_authority(actions,session,approved,execution)
                with resource_claim.command_scope(session.check):
                    (installed/'prerequisites').chmod(0o750)
                    with self.assertRaises(prereq.Blocked):prereq._capture_authority(actions,session,approved,execution)
                    (installed/'prerequisites').chmod(0o700)
                    self.assertEqual(prereq._capture_authority(actions,session,approved,execution),prereq.digest(receipt))
                    with self.assertRaises(FileExistsError):prereq._capture_authority(actions,session,approved,execution)

    def test_effect_order_is_closed_and_allows_preserved_only_slots(self):
        plan={'children':{slot:{'effects':[]} for slot in prereq.SLOTS}}
        plan['children']['p3']['effects']=[{'id':prereq.EFFECT_IDS['p3'][0]},
                                           {'id':prereq.EFFECT_IDS['p3'][-1]}]
        self.assertEqual(prereq._effect_order(plan),[('p3',prereq.EFFECT_IDS['p3'][0]),
                                                     ('p3',prereq.EFFECT_IDS['p3'][-1])])
        for ids in ([prereq.EFFECT_IDS['p3'][0]]*2,
                    [prereq.EFFECT_IDS['p3'][-1],prereq.EFFECT_IDS['p3'][0]],
                    ['Secret/../../foreign']):
            plan['children']['p3']['effects']=[{'id':ident} for ident in ids]
            with self.assertRaises(prereq.Blocked):prereq._effect_order(plan)

    def test_root_source_and_exact_payload_are_bound_to_session(self):
        with tempfile.TemporaryDirectory(dir='/root') as folder:
            root=Path(folder);installed=root/'installed';(installed/'source-authority').mkdir(parents=True,mode=0o700)
            base=installed/'prerequisites'/('a'*12);payloads=base/'payloads'/'gis_material';payloads.mkdir(parents=True,mode=0o700)
            context=resource_claim_test.context();card=resource_claim_test.card(context)
            session=resource_claim.Session(context,'a'*12,'b'*64,lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal())
            session.admit(start=True)
            source={'schema':'voice-staging-source-capture-v2','installation_sha256':session.installation.sha256,'operation':session.operation,
                'context':context,'helper_sha256':session.helper,'approved_sha256':'c'*64,'execution_sha256':'d'*64}
            save(installed/'source-authority'/('a'*12+'.json'),source)
            raw=b'{"fixture":true}';payload=payloads/(hashlib.sha256(b'gis-signer').hexdigest()+'.json');payload.write_bytes(raw);payload.chmod(0o600)
            effect={'id':'gis-signer','payload_sha256':hashlib.sha256(raw).hexdigest()}
            plan={'schema':'voice-staging-prerequisites-plan-v1','operation':session.operation,'context':context,
                'helper_sha256':session.helper,'source_capture_sha256':prereq.digest(source),
                'approved_sha256':'c'*64,'execution_sha256':'d'*64,'namespace_uid':'namespace',
                'marker_sha256':'e'*64,'baseline_sha256':'f'*64,'actual_before':card['observed']['images'],
                'policy_before_sha256':'1'*64,'children':{'release_policy':{},'p3':{},'gis_material':{'effects':[effect]}},
                'auxiliary_images':{},'renderer_authority_sha256':None}
            with patch.object(guard,'ROOT',root):

                plan.setdefault('previous_active_sha256',None)
                for item in plan['children'].values():
                    for key in ('operation','namespace_uid','source_capture_sha256'):item.setdefault(key,plan[key])
                binding=bind_fixture_prerequisites(session,plan)
                state={'schema':'voice-staging-prerequisites-checkpoint-v1','plan':plan,'initial_resource_fence':binding,
                    'status':'RUNNING','steps':{},'receipt':None,'preflight_sha256':None}
                save(base/'checkpoint.json',state)
                self.assertEqual(prereq.load_payload(session,'gis_material','gis-signer'),raw)
                payload.write_bytes(b'{"fixture":false}')
                with self.assertRaisesRegex(prereq.Blocked,'payload_changed'):
                    prereq.load_payload(session,'gis_material','gis-signer')
                with self.assertRaises(prereq.Blocked):prereq.load_payload(session,'foreign','gis-signer')
                source['approved_sha256']='0'*64;save(installed/'source-authority'/('a'*12+'.json'),source)
                with self.assertRaisesRegex(prereq.Blocked,'plan_invalid'):prereq.require_child(session,'gis_material')

    def test_source_authority_loader_requires_exact_protected_capture(self):
        with tempfile.TemporaryDirectory(dir='/root') as folder:
            root=Path(folder);installed=root/'installed';(installed/'source-authority').mkdir(parents=True,mode=0o700)
            base=installed/'prerequisites'/('a'*12);base.mkdir(parents=True,mode=0o700)
            workspace=installed/'sources'/('a'*12);workspace.mkdir(parents=True,mode=0o700)
            code=installed/'code';(code/'nats-rollout-preservation').mkdir(parents=True,mode=0o700)
            source_file=workspace/'scripts'/'staging'/'nats-rollout-preservation'/'fixture.py'
            source_file.parent.mkdir(parents=True,mode=0o700)
            source_file.write_bytes(b'fixture');source_file.chmod(0o600)
            helper=code/'nats-rollout-preservation'/'fixture.py';helper.write_bytes(b'fixture');helper.chmod(0o600)
            name='nats-rollout-preservation/fixture.py';manifest={name:hashlib.sha256(b'fixture').hexdigest()}
            save(code/'capture-manifest.json',manifest)
            code_stat=code.stat();save(base/'code-custody.json',{'root':str(code),'manifest':manifest,
                'directory':{'dev':code_stat.st_dev,'ino':code_stat.st_ino}})
            context=resource_claim_test.context();card=resource_claim_test.card(context)
            session=resource_claim.Session(context,'a'*12,prereq.digest(manifest),lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal());session.admit(start=True)
            approved={'source_sha':context['commit'],'source_files':{'scripts/staging/'+name:manifest[name]},'images':{'fixture':'g'}}
            execution={'fixture':'execution'}
            source={'schema':'voice-staging-source-capture-v2','installation_sha256':session.installation.sha256,'operation':session.operation,'context':context,
                    'helper_sha256':session.helper,'approved_sha256':prereq.digest(approved),'execution_sha256':prereq.digest(execution)}
            save(installed/'source-authority'/('a'*12+'.json'),source);save(base/'approved.json',approved);save(base/'execution.json',execution)
            plan={'schema':'voice-staging-prerequisites-plan-v1','operation':session.operation,'context':context,'helper_sha256':session.helper,
                'source_capture_sha256':prereq.digest(source),'approved_sha256':prereq.digest(approved),'execution_sha256':prereq.digest(execution),
                'namespace_uid':'namespace','marker_sha256':'e'*64,'baseline_sha256':'f'*64,'actual_before':card['observed']['images'],
                'policy_before_sha256':'1'*64,'children':{slot:{'effects':[{'id':slot.replace('_','-')+'-effect'}]} for slot in prereq.SLOTS},
                'auxiliary_images':{},'renderer_authority_sha256':None}
            with patch.object(guard,'ROOT',root),patch.object(prereq,'REQUIRED_HELPERS',frozenset((name,))),patch.object(prereq,'BINARY_PINS',{}):

                plan.setdefault('previous_active_sha256',None)
                for item in plan['children'].values():
                    for key in ('operation','namespace_uid','source_capture_sha256'):item.setdefault(key,plan[key])
                binding=bind_fixture_prerequisites(session,plan)
                save(base/'checkpoint.json',{'schema':'voice-staging-prerequisites-checkpoint-v1','plan':plan,'initial_resource_fence':binding,
                    'status':'RUNNING','steps':{},'receipt':None,'preflight_sha256':None})
                value=prereq.load_authority(session)
                self.assertEqual(value,{'approved':approved,'execution':execution,'source':str(workspace)})
                crypto='nats-rollout-preservation/staging_p3_prerequisites_crypto.py'
                with patch.object(prereq,'REQUIRED_HELPERS',frozenset((name,crypto))):
                    with self.assertRaises(prereq.Blocked):prereq.load_authority(session)
                source_file.write_bytes(b'drift')
                with self.assertRaises(prereq.Blocked):prereq.load_authority(session)
                source_file.write_bytes(b'fixture')
                helper.write_bytes(b'drift')
                with self.assertRaises(prereq.Blocked):prereq.load_authority(session)
                helper.write_bytes(b'fixture')
                approved['source_sha']='0'*40;save(base/'approved.json',approved)
                with self.assertRaises(prereq.Blocked):prereq.load_authority(session)

    def test_fixed_preflight_and_effect_order_use_real_session_and_durable_intent(self):
        with tempfile.TemporaryDirectory(dir='/root') as folder:
            root=Path(folder);installed=root/'installed';(installed/'source-authority').mkdir(parents=True,mode=0o700)
            base=installed/'prerequisites'/('a'*12);base.mkdir(parents=True,mode=0o700)
            context=resource_claim_test.context();card=resource_claim_test.card(context)
            session=resource_claim.Session(context,'a'*12,'b'*64,lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal());session.admit(start=True)
            source={'schema':'voice-staging-source-capture-v2','installation_sha256':session.installation.sha256,'operation':session.operation,'context':context,
                    'helper_sha256':session.helper,'approved_sha256':'c'*64,'execution_sha256':'d'*64}
            save(installed/'source-authority'/('a'*12+'.json'),source)
            plan={'schema':'voice-staging-prerequisites-plan-v1','operation':session.operation,'context':context,'helper_sha256':session.helper,
                'source_capture_sha256':prereq.digest(source),'approved_sha256':'c'*64,'execution_sha256':'d'*64,
                'namespace_uid':'namespace','marker_sha256':'e'*64,'baseline_sha256':'f'*64,'actual_before':card['observed']['images'],
                'policy_before_sha256':'1'*64,'children':{slot:{'effects':[{'id':prereq.EFFECT_IDS[slot][0]}]} for slot in prereq.SLOTS},
                'auxiliary_images':{},'renderer_authority_sha256':None}
            calls=[]
            def child(slot):
                return SimpleNamespace(preflight_payloads=lambda session:calls.append(('preflight',slot)),
                    verify_effect=lambda session,effect,obs:(calls.append(('verify',slot,effect)) or copy.deepcopy(obs)))
            modules={name:child(slot) for slot,name in prereq.CHILD_MODULES.items()}
            with patch.object(guard,'ROOT',root),patch.dict(sys.modules,modules):

                plan.setdefault('previous_active_sha256',None)
                for item in plan['children'].values():
                    for key in ('operation','namespace_uid','source_capture_sha256'):item.setdefault(key,plan[key])
                binding=bind_fixture_prerequisites(session,plan)
                state={'schema':'voice-staging-prerequisites-checkpoint-v1','plan':plan,'initial_resource_fence':binding,
                    'status':'RUNNING','steps':{},'receipt':None,'preflight_sha256':None};save(base/'checkpoint.json',state)
                with self.assertRaises(prereq.Blocked):prereq.record_pending(session,'release_policy','create-policy-configmap')
                prereq.preflight_all(session)
                self.assertEqual(calls,[('preflight',s) for s in prereq.SLOTS])
                with self.assertRaises(prereq.Blocked):prereq.record_pending(session,'p3','Secret/voice-realtime-principal-signing')
                prereq.record_pending(session,'release_policy','create-policy-configmap')
                with self.assertRaises(prereq.Blocked):prereq.record_pending(session,'release_policy','create-policy-configmap')
                observed={'uid':'fixture'}
                with self.assertRaisesRegex(prereq.Blocked,'submit_response_required'):
                    prereq.record_done(session,'release_policy','create-policy-configmap',observed)
                state=json.loads((base/'checkpoint.json').read_bytes())
                self.assertEqual(state['steps']['release_policy']['create-policy-configmap'],{'status':'PENDING'})
                with self.assertRaises(prereq.Blocked):prereq.record_pending(session,'p3','Secret/voice-realtime-principal-signing')


class SubmitTests(unittest.TestCase):
    def test_only_fixed_successful_submit_can_complete_and_lost_reply_is_not_replayed(self):
        import commands
        import staging_release_policy_prepare as policy
        from contextlib import ExitStack
        for fault in ('none','lost_reply','changed_response','lost_claim'):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory(dir='/root') as folder, ExitStack() as stack:
                root=Path(folder);installed=root/'installed'
                (installed/'source-authority').mkdir(parents=True,mode=0o700)
                base=installed/'prerequisites'/('a'*12)
                (base/'payloads'/'release_policy').mkdir(parents=True,mode=0o700)
                context=resource_claim_test.context();card=resource_claim_test.card(context)
                session=resource_claim.Session(context,'a'*12,'b'*64,lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal())
                session.admit(start=True)
                source={'schema':'voice-staging-source-capture-v2','installation_sha256':session.installation.sha256,'operation':session.operation,'context':context,
                    'helper_sha256':session.helper,'approved_sha256':'c'*64,'execution_sha256':'d'*64}
                save(installed/'source-authority'/('a'*12+'.json'),source)
                value=policy.staging_descriptor(context['commit']);obj=value['policy_object']
                raw=policy._canonical(obj);sha=hashlib.sha256(raw).hexdigest()
                effect={'id':'create-policy-configmap','kind':'ConfigMap','name':obj['metadata']['name'],
                    'namespace':'voice-staging','before':{'absent':True,'list_rv':'1'},'desired_sha256':sha,'payload_sha256':sha}
                child={'schema':'voice-staging-release-policy-plan-v1','operation':session.operation,'namespace_uid':'ns',
                    'source_capture_sha256':prereq.digest(source),'source_sha':context['commit'],'descriptor':value,
                    'policy_before_sha256':'1'*64,'policy_after_sha256':'2'*64,'effects':[effect,
                    {'id':'commit-root-policy','kind':'RootPolicy','name':'installed-policy','namespace':'voice-staging',
                     'before':{'sha256':'1'*64},'desired_sha256':'2'*64,'payload_sha256':'2'*64}]}
                plan={'schema':'voice-staging-prerequisites-plan-v1','operation':session.operation,'context':context,
                    'helper_sha256':session.helper,'source_capture_sha256':prereq.digest(source),'approved_sha256':'c'*64,
                    'execution_sha256':'d'*64,'namespace_uid':'ns','marker_sha256':'e'*64,'baseline_sha256':'f'*64,
                    'actual_before':card['observed']['images'],'policy_before_sha256':'1'*64,
                    'children':{'release_policy':child,'p3':{'effects':[]},'gis_material':{'effects':[]}},
                    'auxiliary_images':{},'renderer_authority_sha256':None}
                stack.enter_context(patch.object(guard,'ROOT',root))

                plan.setdefault('previous_active_sha256',None)
                for item in plan['children'].values():
                    for key in ('operation','namespace_uid','source_capture_sha256'):item.setdefault(key,plan[key])
                binding=bind_fixture_prerequisites(session,plan)
                state={'schema':'voice-staging-prerequisites-checkpoint-v1','plan':plan,'initial_resource_fence':binding,
                    'status':'RUNNING','steps':{},'receipt':None,'preflight_sha256':prereq.digest(plan)}
                save(base/'checkpoint.json',state)
                path=base/'payloads'/'release_policy'/(hashlib.sha256(effect['id'].encode()).hexdigest()+'.json')
                path.write_bytes(raw);path.chmod(0o600)
                live={};writes=[]
                def get(kind,name):
                    if kind=='namespace':return {'metadata':{'name':'voice-staging','uid':'ns'}}
                    return copy.deepcopy(live[name])
                def capture(argv,*,body=b'',**kwargs):
                    commands.authority_guard(argv)
                    if '--raw' in argv:return b'{"apiVersion":"v1","kind":"ConfigMapList","metadata":{"resourceVersion":"2"},"items":[]}'
                    self.assertEqual(argv,['/usr/local/bin/k3s','kubectl','--namespace','voice-staging','create','-f','/dev/stdin','-o','json'])
                    self.assertEqual(body,raw)
                    self.assertEqual(json.loads((base/'checkpoint.json').read_bytes())['steps']['release_policy'][effect['id']],{'status':'PENDING'})
                    writes.append(body);row=json.loads(body);row['metadata'].update(uid='uid',resourceVersion='3')
                    live[effect['name']]=copy.deepcopy(row)
                    if fault=='lost_reply':raise RuntimeError('lost reply')
                    if fault=='lost_claim':card['claim']='foreign'
                    if fault=='changed_response':row['data']={'foreign':'data'}
                    return json.dumps(row).encode()
                stack.enter_context(patch.object(commands,'capture',capture))
                stack.enter_context(patch('stage_runtime.Kube.get',side_effect=get))
                stack.enter_context(patch.object(prereq,'load_authority',return_value={'fixture':'external-source-boundary'}))
                stack.enter_context(patch.object(prereq,'_recheck_baseline',return_value=None,create=True))
                stack.enter_context(resource_claim.command_scope(session.check))
                prereq.record_pending(session,'release_policy',effect['id'])
                observation={'kind':'ConfigMap','name':effect['name'],'namespace':'voice-staging','uid':'uid','resourceVersion':'3','semantic_sha256':sha}
                forged_live=copy.deepcopy(obj);forged_live['metadata'].update(uid='uid',resourceVersion='3')
                live[effect['name']]=forged_live
                with self.assertRaises(prereq.Blocked):prereq.record_done(session,'release_policy',effect['id'],observation)
                live.clear()
                if fault=='none':
                    result=prereq.submit_effect(session,'release_policy',effect['id'])
                    self.assertEqual(result,observation)
                    prereq.record_done(session,'release_policy',effect['id'],result)
                    self.assertEqual(json.loads((base/'checkpoint.json').read_bytes())['steps']['release_policy'][effect['id']]['status'],'DONE')
                else:
                    with self.assertRaises(Exception):prereq.submit_effect(session,'release_policy',effect['id'])
                    self.assertEqual(json.loads((base/'checkpoint.json').read_bytes())['steps']['release_policy'][effect['id']],{'status':'PENDING'})
                    with self.assertRaises(Exception):prereq.submit_effect(session,'release_policy',effect['id'])
                self.assertEqual(len(writes),1)
                self.assertIsNone(session._prerequisite_permit)


class PrepareCompositionTests(unittest.TestCase):
    """Real parent/children/Session; Kubernetes, baseline and registry are fixtures."""
    def test_complete_fixture_source_receipt_includes_realtime_before_admission(self):
        import os
        if not os.environ.get('VOICE_QUALIFIED_BINARIES'):self.skipTest('qualified fixture binaries required')
        def ready(actions,request,approved,execution,workspace,*_):
            session=actions.tt_session
            receipt=json.loads((workspace.parents[1]/'source-authority'/(session.operation+'.json')).read_bytes())
            self.assertEqual(receipt['approved_sha256'],prereq.digest(approved))
            self.assertEqual(approved['images']['realtime'],
                'ghcr.io/poryadok/voiceroot/realtime@sha256:'+'d'*64)
            self.assertEqual(set(session.context['images']),{'voice-gameintegration/gameintegration'})
        self._cold_prepare(on_ready=ready,full_source=True,strict_helpers=True,
            realtime_source=True,fixture_complete_candidate=True)

    def test_complete_fixture_storage_identity_reaches_compiler_endpoint_guard(self):
        import copy,os,compiler,stage_runtime
        if not os.environ.get('VOICE_QUALIFIED_BINARIES'):self.skipTest('qualified fixture binaries required')
        def ready(actions,request,approved,execution,workspace,stage,*_):
            claim=stage.expected['source_claim'];generation=stage.expected['generation']
            with patch.object(compiler.release_policy_wiring,'source_input',return_value=None):
                with self.assertRaisesRegex(compiler.Blocked,'rollout_compile_endpoint_invalid'):
                    compiler.rendered_source(workspace,{'generation':generation,'dataPVC':claim,'s3_signing_endpoint':'invalid endpoint'},None)
                with self.assertRaisesRegex(compiler.Blocked,'rollout_compile_identity_invalid'):
                    compiler.rendered_source(workspace,{'generation':generation,'dataPVC':'fixture','s3_signing_endpoint':'invalid endpoint'},None)
            self.assertEqual(stage.marker['data']['dataPVC'],claim)
            self.assertEqual(stage.marker['data']['generation'],generation)
            self.assertEqual(stage.final_claim['metadata']['name'],claim)
            self.assertEqual(stage.final_pv['spec']['claimRef']['name'],claim)
            self.assertTrue(str(stage.final_path).endswith('_voice-staging_'+claim))
            claim_uid=stage.expected['source_claim_uid'];pv_uid=stage.expected['source_pv_uid']
            self.assertEqual(stage_runtime.pv_storage_path(stage.final_pv,claim_uid,claim,pv_uid),str(stage.final_path))
            changed=copy.deepcopy(stage.final_pv);changed['metadata']['uid']='00000000-0000-0000-0000-000000000000'
            with self.assertRaisesRegex(stage_runtime.Blocked,'pv_storage_identity_changed'):
                stage_runtime.pv_storage_path(changed,claim_uid,claim,pv_uid)
            changed=copy.deepcopy(stage.final_pv);changed['spec']['local']['path']=str(stage.final_path).replace('_voice-staging_','_other-namespace_')
            with self.assertRaisesRegex(stage_runtime.Blocked,'pv_storage_path_unsupported'):
                stage_runtime.pv_storage_path(changed,claim_uid,claim,pv_uid)
        self._cold_prepare(on_ready=ready,full_source=True,strict_helpers=True,
            realtime_source=True,fixture_complete_candidate=True)

    def test_fixture_compiler_producer_reads_live_secret_metadata(self):
        import compiler
        def ready(actions,request,approved,execution,workspace,stage,live,transport,*_):
            expected=live[('secret','voice-app-secrets')]['metadata']
            self.assertEqual(compiler.ProducerKube().secret_meta('voice-app-secrets'),
                {key:expected[key] for key in ('uid','resourceVersion')})
            self.assertEqual(compiler.ProducerKube().get('Secret','voice-app-secrets'),
                live[('secret','voice-app-secrets')])
            with self.assertRaises((AssertionError,RuntimeError)):
                compiler.capture(['kubectl','--namespace','another-namespace','get','secret','voice-app-secrets','-o','json'])
            with self.assertRaises((AssertionError,RuntimeError)):
                compiler.capture(['kubectl','--namespace','voice-staging','delete','secret','voice-app-secrets','-o','json'])
        self._cold_prepare(on_ready=ready)

    def test_fixture_accepts_owned_installation_before_session_admission(self):
        import bootstrap_selection_test
        installation=bootstrap_selection_test.owned_fixture()[3]
        def ready(actions,*args):
            self.assertIs(actions.tt_session.installation,installation)
            self.assertEqual(actions.tt_session.check()['phase'],'prepare')
        self._cold_prepare(on_ready=ready,fixture_installation=installation)

    def test_fixture_accepts_distinct_operation_before_session_admission(self):
        def ready(actions,*args):
            self.assertEqual(actions.tt_session.operation,'b'*12)
            self.assertEqual(actions.tt_session.check()['operation'],'b'*12)
        self._cold_prepare(on_ready=ready,fixture_operation='b'*12)

    def test_strict_fixture_binds_dispatcher_execution(self):
        def ready(actions,request,approved,execution,workspace,*args):
            self.assertEqual(execution['dispatcher_run_id'],17)
            self.assertEqual(execution['head_sha'],actions.tt_session.context['commit'])
            self.assertEqual(actions.tt_session.context['images'],candidate)
            receipt=json.loads((workspace.parents[1]/'source-authority'/
                                (actions.tt_session.operation+'.json')).read_bytes())
            self.assertEqual(receipt['execution_sha256'],prereq.digest(execution))
        candidate={'voice-user/user':'example.invalid/user@sha256:'+'a'*64}
        self._cold_prepare(on_ready=ready,full_source=True,strict_helpers=True,fixture_candidate_images=candidate)

    def test_strict_fixture_binds_user_and_new_gis_candidate(self):
        import bridge_root
        registry=bridge_root.source_authority.REGISTRY
        def ready(actions,request,approved,execution,workspace,stage,live,transport,get,card):
            user=registry+'/user@sha256:'+'c'*64
            gis=registry+'/gameintegration@sha256:'+'a'*64
            realtime=registry+'/realtime@sha256:'+'d'*64
            self.assertEqual(approved['images'],{'user':user,'gameintegration':gis,'realtime':realtime})
            self.assertEqual(actions.tt_session.context['images'],{'voice-gameintegration/gameintegration':gis})
            self.assertEqual(stage.old_images['voice-user/user'],registry+'/user@sha256:'+'b'*64)
            auxiliary={'image_ref':'docker.io/library/nginx@sha256:'+'d'*64}
            prereq._candidate_images(actions.tt_session,request,approved,workspace,stage,{},auxiliary,None,actions.code,{})
            from types import SimpleNamespace
            altered=SimpleNamespace(context={'images':{**actions.tt_session.context['images'],'voice-user/user':user}})
            with self.assertRaisesRegex(bridge_root.Blocked,'realtime_bootstrap_candidate_changed'):
                prereq._candidate_images(altered,request,approved,workspace,stage,{},auxiliary,None,actions.code,{})
            source=json.loads((workspace.parents[1]/'source-authority'/
                (actions.tt_session.operation+'.json')).read_bytes())
            self.assertEqual(source['approved_sha256'],prereq.digest(approved))
        self._cold_prepare(on_ready=ready,full_source=True,strict_helpers=True,fixture_complete_candidate=True)

    def test_cold_prepare_executes_all_children_before_typed_capture(self):
        self._cold_prepare()

    def test_publisher_ready_binds_actual_manifest_and_rejects_image_drift(self):
        import staging_p3_prerequisites as p3
        fixture=Path(__file__).parent/'fixtures'/'qualified-nginx'/'manifest.json'
        manifest=hashlib.sha256(fixture.read_bytes()).hexdigest()
        self.assertNotEqual(manifest,'d'*64)
        self._cold_prepare(fixture_manifest=manifest)
        with self.assertRaisesRegex(p3.PrerequisiteError,'staging P3 completion not qualified'):
            self._cold_prepare(fixture_manifest=manifest,fixture_publisher_drift=True)

    def test_lost_app_cas_reply_never_completes_or_replays(self):
        self._cold_prepare('lost-app-reply')

    def test_missing_operator_input_prevents_every_effect(self):
        self._cold_prepare('missing-issuer')

    def test_existing_inventory_requires_protected_pass_and_runs_without_writes(self):
        self._cold_prepare('c2')

    def test_operator_trust_expansion_is_rejected_before_effects(self):
        self._cold_prepare('expanded-trust')

    def test_completed_realtime_addition_requires_exact_private_custody(self):
        self._cold_prepare('rt-extension')

    def test_existing_inventory_new_candidate_creates_only_release_policy(self):
        self._cold_prepare('c2-new-candidate')

    def _cold_prepare(self,fault=None,*,realtime_source=False,bridge_missing=None,on_ready=None,
                      context_commit=None,full_source=False,strict_helpers=False,source_overrides=None,workspace_context=None,
                      fixture_installation=None,fixture_operation=None,fixture_candidate_images=None,
                      fixture_manifest='d'*64,fixture_publisher_drift=False,fixture_complete_candidate=False):
        import os, shutil, subprocess, base64, yaml, uuid, tarfile
        from contextlib import ExitStack
        import bridge_root, commands, compiler, runtime_stage, stage_runtime
        import staging_gis_material as gis
        import staging_p3_prerequisites as p3
        import staging_p3_prerequisites_crypto as crypto
        import staging_release_policy_prepare as policy
        if strict_helpers and not full_source:raise ValueError('strict fixture requires the captured source tree')
        source_root=Path(os.environ.get('VOICE_TEST_SOURCE_ROOT',Path(__file__).resolve().parents[3]))
        if not (source_root/'deploy/staging/services.yaml').is_file():self.skipTest('requires actual source fixture')
        with (workspace_context or tempfile.TemporaryDirectory(dir='/root')) as folder, ExitStack() as stack:
            root=Path(folder);installed=root/'installed';operation=fixture_operation if fixture_operation is not None else 'a'*12
            workspace=installed/'sources'/operation;code=installed/'code';operator=installed/'prerequisite-inputs'
            for directory in (workspace,code/'fixture',installed/'source-authority',operator):directory.mkdir(parents=True,mode=0o700)
            relative=['deploy/staging/services.yaml','scripts/dev/migrations.py',
                'docker/postgres/provision-gameintegration-runtime-role.sql','docker/postgres/verify-gameintegration-runtime-role.sql',
                'deploy/templates/provision-gameintegration-runtime-role-job.yaml','deploy/templates/migrate-game-integration-db-job.yaml',
                'deploy/templates/verify-game-integration-db-job.yaml']
            relative += [str(path.relative_to(source_root)) for path in (source_root/'src/backend/migrations/game_integration_db').glob('*.sql')]
            if realtime_source:relative += ['deploy/templates/migrate-realtime-db-job.yaml','src/backend/migrations/realtime_db/000001_admission.up.sql']
            if full_source:
                for prefix in ('deploy','docker','scripts/staging','scripts/storage','scripts/dev','scripts/deploy','src/backend/migrations',
                               'src/backend/auth/src/main/resources/db/migration'):
                    relative.extend(str(path.relative_to(source_root)) for path in (source_root/prefix).rglob('*')
                        if path.is_file() and '__pycache__' not in path.parts and path.suffix!='.pyc')
                relative=list(dict.fromkeys(relative))
            for name in relative:
                dest=workspace/name;dest.parent.mkdir(parents=True,exist_ok=True,mode=0o700);shutil.copyfile(source_root/name,dest);dest.chmod(0o600)
            for name,path in (source_overrides or {}).items():
                if name not in relative:raise ValueError('override outside captured source')
                shutil.copyfile(path,workspace/name)
            helper='fixture/identity.py';helper_raw=b'# fixture installed helper boundary\n'
            if not strict_helpers:
                for path in (code/helper,workspace/'scripts/staging'/helper):
                    path.parent.mkdir(parents=True,exist_ok=True,mode=0o700);path.write_bytes(helper_raw);path.chmod(0o600)
            resource_helper='nats-rollout-preservation/resource_claim.py'
            if not strict_helpers:
                for path in (code/resource_helper,workspace/'scripts/staging'/resource_helper):
                    path.parent.mkdir(parents=True,exist_ok=True,mode=0o700);path.write_bytes(helper_raw);path.chmod(0o600)
            manifest={name:hashlib.sha256(helper_raw).hexdigest() for name in (helper,resource_helper)}
            if realtime_source:
                name='nats-rollout-preservation/realtime_bootstrap.py'
                raw=Path(__file__).with_name('realtime_bootstrap.py').read_bytes()
                for path in (code/name,workspace/'scripts/staging'/name):
                    path.parent.mkdir(parents=True,exist_ok=True,mode=0o700);path.write_bytes(raw);path.chmod(0o600)
                manifest[name]=hashlib.sha256(raw).hexdigest()
            if strict_helpers:
                import bundle
                qualified=Path(os.environ.get('VOICE_QUALIFIED_BINARIES','/fixture/qualified-binaries'))
                output=root/'qualified-bundle'
                bundle.build(qualified/'kernel',output,qualified/'bootstrap-renewer',qualified/'story-current4-probe',
                    story_target8=qualified/'story-restored8-probe')
                with tarfile.open(output/'rollout-bundle.tar') as archive:
                    archive.extractall(code,filter='data')
                manifest=json.loads((code/'capture-manifest.json').read_bytes())
            save(code/'capture-manifest.json',manifest)
            context=resource_claim_test.context()
            if context_commit is not None:context['commit']=context_commit
            if strict_helpers:
                game_image=bridge_root.source_authority.REGISTRY+'/gameintegration@sha256:'+'a'*64
                user_image=bridge_root.source_authority.REGISTRY+'/user@sha256:'+'c'*64
                context['images']=(copy.deepcopy(fixture_candidate_images) if fixture_candidate_images is not None
                                   else {'voice-gameintegration/gameintegration':game_image})
            card=resource_claim_test.card(context)
            if fixture_complete_candidate:
                if not strict_helpers or fixture_candidate_images is not None:raise ValueError('invalid complete candidate fixture')
                card['observed']['images']['voice-user/user']=bridge_root.source_authority.REGISTRY+'/user@sha256:'+'b'*64
            if strict_helpers:
                import bootstrap_selection_test
                installation=fixture_installation if fixture_installation is not None else bootstrap_selection_test.owned_fixture()[3]
                journal=resource_claim.RootJournal(root)
            else:
                installation=fixture_installation;journal=resource_claim_test.MemoryJournal()
            session=resource_claim.Session(context,operation,prereq.digest(manifest),lambda:copy.deepcopy(card),journal,installation=installation)
            session.admit(start=True)
            approved={'source_sha':context['commit'],'source_files':{name:hashlib.sha256((workspace/name).read_bytes()).hexdigest() for name in relative},'images':{}}
            if strict_helpers:approved['images']['gameintegration']=game_image
            if fixture_complete_candidate:
                approved['images'].update(user=user_image,
                    realtime=bridge_root.source_authority.REGISTRY+'/realtime@sha256:'+'d'*64)
            if not strict_helpers:
                approved['source_files']['scripts/staging/'+helper]=manifest[helper]
                approved['source_files']['scripts/staging/'+resource_helper]=manifest[resource_helper]
            if realtime_source:
                approved.update(schema='voice-source-authority-v1',verified=True)
                approved['source_files']['scripts/staging/nats-rollout-preservation/realtime_bootstrap.py']=manifest['nats-rollout-preservation/realtime_bootstrap.py']
            if not strict_helpers:approved['source_files']['scripts/deploy/staging_p3_auxiliary_image.py']='e'*64
            execution={'fixture':'source selected by real bridge capture, modeled here'}
            if strict_helpers:execution.update(dispatcher_run_id=17,head_sha=context['commit'])
            if realtime_source:execution['head_sha']=context['commit']
            source={'schema':'voice-staging-source-capture-v2','installation_sha256':session.installation.sha256,'operation':operation,'context':context,'helper_sha256':session.helper,
                'approved_sha256':prereq.digest(approved),'execution_sha256':prereq.digest(execution)}
            save(installed/'source-authority'/(operation+'.json'),source);save(installed/'policy.json',{'unrelated':'preserve'})
            actions=bridge_root.Actions(code,installation=installation);actions.tt_session=session
            def openssl(*args):subprocess.run(['/usr/bin/openssl',*map(str,args)],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
            key=root/'ca.key';cert=root/'ca.crt';peerkey=root/'peer.key';csr=root/'peer.csr';peer=root/'peer.crt';ext=root/'ext'
            openssl('req','-x509','-newkey','rsa:2048','-nodes','-keyout',key,'-out',cert,'-days','180','-subj','/CN=synthetic-root',
                '-addext','basicConstraints=critical,CA:TRUE','-addext','keyUsage=critical,keyCertSign,cRLSign')
            openssl('req','-newkey','rsa:2048','-nodes','-keyout',peerkey,'-out',csr,'-subj','/CN=voice-space')
            ext.write_text('basicConstraints=critical,CA:FALSE\nsubjectAltName=DNS:voice-space.voice-staging.svc\nextendedKeyUsage=serverAuth,clientAuth\n')
            openssl('x509','-req','-in',csr,'-CA',cert,'-CAkey',key,'-set_serial','1234','-days','150','-extfile',ext,'-out',peer)
            ca=cert.read_bytes();peer_bytes=peer.read_bytes();ns='00000000-0000-0000-0000-000000000001'
            b64=lambda b:base64.b64encode(b).decode()
            def secret(name,data):return {'apiVersion':'v1','kind':'Secret','metadata':{'name':name,'namespace':'voice-staging','uid':str(uuid.uuid5(uuid.NAMESPACE_DNS,name)),'resourceVersion':'1'},'type':'Opaque','data':{k:b64(v) for k,v in data.items()}}
            live={('secret','voice-media-ca'):secret('voice-media-ca',{'ca.crt':ca,'jwks-ca.crt':ca}),
                ('secret','voice-space-media-client-tls'):secret('voice-space-media-client-tls',{'tls.crt':peer_bytes,'tls.key':peerkey.read_bytes()}),
                ('secret','voice-space-media-server-tls'):secret('voice-space-media-server-tls',{'tls.crt':peer_bytes,'tls.key':peerkey.read_bytes()}),
                ('secret','voice-app-secrets'):secret('voice-app-secrets',{'POSTGRES_PASSWORD':b'synthetic-unused'})}
            gis_issuer={'schema':'voice-staging-gis-issuer-input-v1','receipt':{},'ca_certificate_pem':ca.decode(),
                'ca_private_key_pem':key.read_text(),'jwks_ca_bundle_pem':ca.decode(),'ca_secret':{}}
            p3_issuer={'schema':'voice-staging-p3-issuer-input-v1','receipt':{},'ca_certificate_pem':ca.decode(),
                'ca_private_key_pem':key.read_text(),'trust':{name:ca.decode() for name in crypto.TRUST_KEYS},
                'space_client_certificate_pem':peer_bytes.decode(),'space_jwks_certificate_pem':peer_bytes.decode(),
                'space_server_ca_pem':ca.decode(),'replay_password_b64':b64(b'synthetic-replay-password')}
            save(operator/'gis-issuer.json',gis_issuer);save(operator/'p3-issuer.json',p3_issuer)
            if fault=='missing-issuer':(operator/'p3-issuer.json').unlink()
            if fault=='expanded-trust':
                extra=root/'extra.crt';extra_key=root/'extra.key'
                openssl('req','-x509','-newkey','rsa:2048','-nodes','-keyout',extra_key,'-out',extra,'-days','180','-subj','/CN=unrelated-root','-addext','basicConstraints=critical,CA:TRUE')
                p3_issuer['trust']['space-client-ca.crt']+=extra.read_text();save(operator/'p3-issuer.json',p3_issuer)
            claim_name='voice-nats-jsdata-d20260930a4' if fixture_complete_candidate else 'fixture'
            marker={'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':'voice-nats-generation','namespace':'voice-staging','uid':'marker','resourceVersion':'1'},
                'data':{'phase':'active','generation':session.installation.generation if fixture_complete_candidate else 'fixture','dataPVC':claim_name}}
            deployment={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-user','namespace':'voice-staging','uid':'user','resourceVersion':'1','generation':1},
                'spec':{'replicas':1,'template':{'metadata':{'labels':{'app':'voice-user'}},'spec':{'containers':[{'name':'user','image':card['observed']['images']['voice-user/user']}]}}}}
            live[('configmap','voice-nats-generation')]=marker;live[('deployment','voice-user')]=deployment
            claim_uid='00000000-0000-0000-0000-000000000002';pv_uid='00000000-0000-0000-0000-000000000003'
            storage_path='/var/lib/rancher/k3s/storage/pvc-'+claim_uid+'_voice-staging_'+claim_name
            claim={'metadata':{'name':claim_name,'uid':claim_uid},'spec':{'volumeName':'fixture-pv'}}
            claim_ref={'uid':claim_uid}
            if fixture_complete_candidate:
                claim['metadata']['namespace']='voice-staging'
                claim_ref.update(name=claim_name,namespace='voice-staging')
            pv={'metadata':{'name':'fixture-pv','uid':pv_uid},'spec':{'claimRef':claim_ref,'local':{'path':storage_path},
                'nodeAffinity':{'required':{'nodeSelectorTerms':[{'matchExpressions':[{'key':'kubernetes.io/hostname','operator':'In','values':['pmdebook']}]}]}}}}
            live[('pvc',claim_name)]=claim;live[('pv','fixture-pv')]=pv
            writes=[];events=[];rv=[10];ready={}
            def get(kind,name):
                commands.authority_guard(prereq.KUBE_PREFIX+['get',kind,name,'-o','json'])
                if kind=='namespace':return {'apiVersion':'v1','kind':'Namespace','metadata':{'name':'voice-staging','uid':ns}}
                if kind=='node':return {'metadata':{'name':'pmdebook','uid':'node'},'status':{'nodeInfo':{'operatingSystem':'linux','architecture':'amd64'},'conditions':[{'type':'Ready','status':'True'}]}}
                if kind=='serviceaccount':return {'metadata':{'name':'default','namespace':'voice-staging','uid':'00000000-0000-0000-0000-000000000101','resourceVersion':'1'}}
                return copy.deepcopy(live[(kind,name)])
            def normalize(obj):
                result=copy.deepcopy(obj)
                if result['kind']=='Service':result['spec'].update(type='ClusterIP',clusterIP='10.2.0.1',clusterIPs=['10.2.0.1'],ipFamilies=['IPv4'])
                return result
            def ready_objects(dep):
                template=copy.deepcopy(dep['spec']['template']);template['metadata']['labels']['pod-template-hash']='abc12'
                ready['replicasets']=[{'metadata':{'namespace':'voice-staging','uid':'rs','annotations':{'deployment.kubernetes.io/revision':'1'},'ownerReferences':[{'kind':'Deployment','uid':dep['metadata']['uid'],'controller':True}]},
                    'spec':{'replicas':1,'selector':{'matchLabels':{'app':'voice-p3-principal-jwks','pod-template-hash':'abc12'}},'template':template}}]
                ready['replicaset']=ready['replicasets']
                spec=copy.deepcopy(dep['spec']['template']['spec']);spec.update(nodeName='pmdebook',serviceAccountName='default')
                manifest=spec['containers'][0]['image'].rsplit('@',1)[-1]
                image_id='sha256:'+'0'*64 if fixture_publisher_drift else manifest
                spec['volumes'].append({'name':'kube-api-access-abc12','projected':{'defaultMode':420,'sources':[{'serviceAccountToken':{'expirationSeconds':3607,'path':'token'}},
                    {'configMap':{'name':'kube-root-ca.crt','items':[{'key':'ca.crt','path':'ca.crt'}]}},{'downwardAPI':{'items':[{'path':'namespace','fieldRef':{'apiVersion':'v1','fieldPath':'metadata.namespace'}}]}}]}})
                spec['containers'][0]['volumeMounts'].append({'name':'kube-api-access-abc12','mountPath':'/var/run/secrets/kubernetes.io/serviceaccount','readOnly':True})
                ready['pods']=[{'metadata':{'namespace':'voice-staging','uid':'pod','labels':{'app':'voice-p3-principal-jwks'},'ownerReferences':[{'kind':'ReplicaSet','uid':'rs','controller':True}]},'spec':spec,
                    'status':{'phase':'Running','podIP':'10.2.0.8','conditions':[{'type':'Ready','status':'True'}],'containerStatuses':[{'name':'jwks','ready':True,'state':{'running':{}},'imageID':'docker-pullable://nginx@'+image_id}]}}]
                ready['endpointslices']=[{'metadata':{'uid':'slice','namespace':'voice-staging','labels':{'kubernetes.io/service-name':'voice-p3-principal-jwks'},'ownerReferences':[{'kind':'Service','uid':live[('service','voice-p3-principal-jwks')]['metadata']['uid'],'controller':True}]},
                    'ports':[{'name':'https','port':8443,'protocol':'TCP'}],'endpoints':[{'addresses':['10.2.0.8'],'conditions':{'ready':True},'targetRef':{'kind':'Pod','uid':'pod','namespace':'voice-staging'}}]}]
            def transport(argv,*,body=b'',**kwargs):
                commands.authority_guard(argv)
                if argv==['kubectl','create','--dry-run=client','--validate=false','-f','/dev/stdin','-o','json']:return prereq._bytes(yaml.safe_load(body))
                self.assertEqual(argv[:4],prereq.KUBE_PREFIX);args=argv[4:]
                if args[0]=='get':
                    if '--raw' in args:
                        path=args[-1];plural=path.split('/')[5].split('?')[0];name=path.split('%3D')[1].split('&')[0]
                        kind={'secrets':'Secret','configmaps':'ConfigMap','services':'Service'}[plural]
                        items=[copy.deepcopy(row) for (k,n),row in live.items() if k==kind.lower() and n==name]
                    elif args[1] in ready:kind={'replicaset':'ReplicaSet','replicasets':'ReplicaSet','pods':'Pod','endpointslices':'EndpointSlice'}[args[1]];items=copy.deepcopy(ready[args[1]])
                    else:
                        kind={'deployments':'Deployment','secret':'Secret','configmap':'ConfigMap','service':'Service','deployment':'Deployment'}[args[1]]
                        items=[copy.deepcopy(row) for (k,n),row in live.items() if k==kind.lower()]
                    return prereq._bytes({'apiVersion':'apps/v1' if kind in ('Deployment','ReplicaSet') else 'v1','kind':kind+'List','metadata':{'resourceVersion':str(rv[0])},'items':items})
                if '--dry-run=server' in args:return prereq._bytes(normalize(json.loads(body)))
                state=json.loads((prereq._base(session)/'checkpoint.json').read_bytes())
                self.assertEqual(state['preflight_sha256'],prereq.digest(state['plan']))
                self.assertEqual(sum(v=={'status':'PENDING'} for group in state['steps'].values() for v in group.values()),1)
                rv[0]+=1;writes.append(args[:]);events.append('write')
                if args[0]=='create':
                    row=normalize(json.loads(body));identity=(row['kind'].lower(),row['metadata']['name']);self.assertNotIn(identity,live)
                    row['metadata'].update(uid=str(uuid.uuid5(uuid.NAMESPACE_DNS,identity[1])),resourceVersion=str(rv[0]))
                    if row['kind']=='Deployment':row['metadata']['generation']=1
                    live[identity]=copy.deepcopy(row)
                    if row['kind']=='Deployment':
                        live[identity]['metadata']['annotations']['deployment.kubernetes.io/revision']='1'
                        live[identity]['status']={'observedGeneration':1,'readyReplicas':1,'availableReplicas':1};ready_objects(live[identity])
                    return prereq._bytes(row)
                self.assertEqual(args,['patch','secret','voice-app-secrets','--type=json','--patch-file=/dev/stdin','-o','json'])
                row=live[('secret','voice-app-secrets')];changes=json.loads(body)
                self.assertEqual(changes[:3],[{'op':'test','path':'/metadata/uid','value':row['metadata']['uid']},{'op':'test','path':'/metadata/resourceVersion','value':row['metadata']['resourceVersion']},{'op':'test','path':'/data','value':row['data']}])
                for change in changes[3:]:self.assertEqual(change['op'],'add');row['data'][change['path'].split('/')[-1]]=change['value']
                row['metadata']['resourceVersion']=str(rv[0])
                if fault=='lost-app-reply':raise RuntimeError('synthetic_lost_app_response')
                return prereq._bytes(row)
            def compiler_transport(argv,*,body=b'',**kwargs):
                if argv[:3]!=['kubectl','--namespace','voice-staging']:
                    return transport(argv,body=body,**kwargs)
                commands.authority_guard(argv)
                args=argv[3:]
                self.assertEqual(len(args),5)
                self.assertEqual(args[0],'get');self.assertEqual(args[3],'-o')
                row=copy.deepcopy(live[(args[1].lower(),args[2])])
                if args[4]=='json':return prereq._bytes(row)
                self.assertEqual(args[4],"jsonpath-as-json={.metadata['uid','resourceVersion']}")
                return prereq._bytes([row['metadata'][key] for key in ('uid','resourceVersion')])
            kube=stage_runtime.Kube()
            stage=object.__new__(runtime_stage.RolloutStage);stage.kube=kube;stage.operation=operation;stage.marker=copy.deepcopy(marker)
            stage.snapshots={'voice-user':copy.deepcopy(deployment)};stage.original_snapshots=copy.deepcopy(stage.snapshots)
            stage.old_images=copy.deepcopy(card['observed']['images']);stage.expected={'namespace_uid':ns,'deployment_uids':{'voice-user':'user'},
                'source_claim':claim_name,'source_claim_uid':claim_uid,'source_pv_uid':pv_uid,'generation':session.installation.generation}
            stage.final_claim=copy.deepcopy(claim);stage.final_pv=copy.deepcopy(pv);stage.final_path=Path(storage_path)
            collector=SimpleNamespace(collect=lambda:{k:b'{}' for k in ('index','manifest','config')},validate=lambda *a:{'source_ref':'nginx:stable-alpine','repository':'docker.io/library/nginx',
                'index_sha256':'e'*64,'manifest_sha256':fixture_manifest,'config_sha256':'f'*64,'platform':{'os':'linux','architecture':'amd64','variant':None},'image_ref':'docker.io/library/nginx@sha256:'+fixture_manifest})
            stack.enter_context(patch.object(guard,'ROOT',root))
            if not strict_helpers:
                stack.enter_context(patch.object(prereq,'REQUIRED_HELPERS',frozenset(manifest)))
                stack.enter_context(patch.object(prereq,'BINARY_PINS',{}))
            if strict_helpers:
                original_collector=prereq._source_module
                metadata_dir=Path(__file__).parent/'fixtures'/'qualified-nginx'
                def offline_collector(authority,source):
                    module=original_collector(authority,source)
                    module.collect=lambda:{name:(metadata_dir/(name+'.json')).read_bytes() for name in ('index','manifest','config')}
                    return module
                stack.enter_context(patch.object(prereq,'_source_module',side_effect=offline_collector))
            else:stack.enter_context(patch.object(prereq,'_source_module',return_value=collector))
            stack.enter_context(patch.object(runtime_stage.RolloutStage,'capture',return_value=stage))
            if not strict_helpers:
                stack.enter_context(patch.object(prereq,'_candidate_images',side_effect=lambda *a:events.append('candidate')))
                stack.enter_context(patch.object(policy,'validate_source',side_effect=lambda approved,source,value:copy.deepcopy(value)))
            stack.enter_context(patch.object(stage_runtime.Kube,'get',side_effect=get));stack.enter_context(patch.object(commands,'capture',side_effect=transport))
            stack.enter_context(patch.object(compiler,'capture',side_effect=compiler_transport));stack.enter_context(patch.object(stage_runtime,'capture',side_effect=transport))
            stack.enter_context(patch.object(p3,'probe_jwks',return_value=None));stack.enter_context(resource_claim.command_scope(session.check))
            for slot in prereq.SLOTS:
                module=__import__(prereq.CHILD_MODULES[slot]);original=module.preflight_payloads
                def checked(s,original=original,slot=slot):
                    self.assertEqual(writes,[]);result=original(s);events.append('preflight:'+slot);return result
                stack.enter_context(patch.object(module,'preflight_payloads',side_effect=checked))
            request={'mode':'app-only','changed_services':['gameintegration'],'gis_first_install':True}
            if on_ready is not None:
                return on_ready(actions,request,approved,execution,workspace,stage,live,transport,get,card)
            if fault=='missing-issuer':
                if bridge_missing is None:
                    with self.assertRaises(FileNotFoundError):prereq.prepare_inputs(actions,request,approved,execution,workspace)
                else:bridge_missing(actions,request,approved,execution,workspace)
                self.assertEqual(writes,[]);self.assertEqual(session.check()['phase'],'prepare');return
            if fault=='expanded-trust':
                with self.assertRaisesRegex(prereq.Blocked,'issuer_trust_changed'):prereq.prepare_inputs(actions,request,approved,execution,workspace)
                self.assertEqual(writes,[]);self.assertEqual(session.check()['phase'],'prepare');return
            prepared=prereq.prepare_inputs(actions,request,approved,execution,workspace)
            # These tests exercise prerequisite effect ordering, not the P2
            # producer. The receipt and paused stage are explicit fixtures.
            import staging_cohort_preservation as cohort
            qualified={'prerequisite_inputs_sha256':prepared['inputs_sha256']}
            p2_plan={'actual_before':stage.old_images,'candidate':{'commit':context['commit'],'images':context['images']}}
            qualified_loader=stack.enter_context(patch.object(cohort,'load_qualified',return_value=qualified))
            stack.enter_context(patch.object(cohort,'capture_stage',return_value=stage))
            stack.enter_context(patch.object(cohort,'capture_final_stage',return_value=stage))
            auxiliary_loader=stack.enter_context(patch.object(cohort,'load_auxiliary_successor',return_value=None))
            stack.enter_context(patch.object(cohort,'revalidate_paused'))
            with patch.object(cohort,'validate_plan'):
                session.bind_cohort_preservation(p2_plan,stage.old_images)
            if fault=='lost-app-reply':
                with self.assertRaisesRegex(RuntimeError,'synthetic_lost_app_response'):prereq.run_prepare(actions,request,approved,execution,workspace,preservation_ref=qualified)
                self.assertIn('GAME_INTEGRATION_DATABASE_URL',live[('secret','voice-app-secrets')]['data'])
                state=prereq._checkpoint(session)
                self.assertEqual(state['steps']['gis_material']['gis-app-credentials'],{'status':'PENDING'})
                self.assertEqual(state['status'],'RUNNING');self.assertIsNone(state['receipt'])
                with self.assertRaises(Exception):prereq.submit_effect(session,'gis_material','gis-app-credentials')
                with self.assertRaises(Exception):prereq._complete(session)
                self.assertEqual(len(writes),15);return
            receipt=prereq.run_prepare(actions,request,approved,execution,workspace,preservation_ref=qualified)
            self.assertEqual(events[:4],['candidate',*('preflight:'+slot for slot in prereq.SLOTS)])
            self.assertEqual(len(writes),15);self.assertEqual(prereq._checkpoint(session)['status'],'COMPLETE')
            self.assertEqual(json.loads((installed/'policy.json').read_bytes())['unrelated'],'preserve')
            self.assertEqual(receipt['actual_after'],{**stage.old_images,'voice-p3-principal-jwks/jwks':collector.validate()['image_ref']})
            stage.snapshots['voice-p3-principal-jwks']=copy.deepcopy(live[('deployment','voice-p3-principal-jwks')]);stage.original_snapshots=copy.deepcopy(stage.snapshots)
            stage.expected['deployment_uids']['voice-p3-principal-jwks']=stage.snapshots['voice-p3-principal-jwks']['metadata']['uid'];stage.old_images=receipt['actual_after']
            binding=session.capture_prerequisites(stage)
            self.assertEqual(session.check()['phase'],'prerequisites-captured')
            captured=prereq.load_capture(session,actual=receipt['actual_after'])
            self.assertEqual(binding['target_sha256'],prereq.digest(captured))
            with self.assertRaises(Exception):prereq.submit_effect(session,'gis_material','gis-app-credentials')
            self.assertEqual(len(writes),15)
            if fault=='rt-extension':
                # SQL execution is not claimed here: construct its protected
                # COMPLETE records, then run the actual parent/RT validators.
                import realtime_bootstrap as rt
                before=copy.deepcopy(live[('secret','voice-app-secrets')]);password=b'r'*43
                after=copy.deepcopy(before);after['metadata']['resourceVersion']='901'
                after['data']['REALTIME_DATABASE_URL']=rt.secret_patch(before,password)[-1]['value']
                pg={'statefulset':{'uid':'sts','resourceVersion':'1'},
                    'pod':{'uid':'pgpod','resourceVersion':'1','container_id':'ctr','restart_count':0},
                    'pvc':{'name':'pgdata-voice-postgres-0','uid':'pgpvc','resourceVersion':'1'},
                    'pv':{'name':'pgpv','uid':'pgpv','resourceVersion':'1'},
                    'service':{'uid':'pgsvc','resourceVersion':'1','cluster_ip':'10.0.0.1'},
                    'endpoints':{'uid':'pgep','resourceVersion':'1','pod_ip':'10.0.0.2'},
                    'slice':{'uid':'pgslice','resourceVersion':'1'},'system_identifier':'12345'}
                view=rt._stage_view(stage)
                plan={'schema':'voice-staging-realtime-bootstrap-v4','installation_sha256':session.installation.sha256,'operation':session.operation,
                    'context':session.context,'helper_sha256':session.helper,'source_sha':context['commit'],
                    'source_files_sha256':prereq.digest(approved['source_files']),
                    'source_authority_sha256':prereq.digest(approved),'execution_sha256':prereq.digest(execution),
                    'stage_sha256':prereq.digest(view),'credential_sha256':hashlib.sha256(password).hexdigest(),
                    'actual_images':stage.old_images,'namespace_uid':ns,'postgres':pg,
                    'secret':rt._secret_identity(before),'active_marker_sha256':prereq.digest(stage.marker),
                    'prerequisites_sha256':prereq.digest(captured)}
                role={'oid':13,'login':True,'superuser':False,'createdb':False,'createrole':False,
                    'inherit':False,'replication':False,'bypassrls':False,'memberships':0,
                    'connection_limit':-1,'valid_until':None,'config':None}
                role_row={'role':role,'database':None};final={'role':role,'database':{'oid':29,'owner_oid':13}}
                def done(old,request,value):return {'status':'DONE','before_sha256':prereq.digest(old),
                    'request_sha256':prereq.digest(request),'readback':value}
                steps={'role':done({'role':None,'database':None},{'kind':'role','credential_sha256':plan['credential_sha256']},role_row),
                    'database':done(role_row,{'kind':'database','owner_oid':13},final),
                    'secret':done(plan['secret'],{'kind':'secret','plan_sha256':prereq.digest(plan)},rt._secret_identity(after))}
                rt_receipt={'schema':'voice-staging-realtime-bootstrap-receipt-v2','installation_sha256':session.installation.sha256,'plan_sha256':prereq.digest(plan),
                    'steps_sha256':prereq.digest(steps),'role_oid':13,'database_oid':29,'owner_oid':13,
                    'secret':rt._secret_identity(after),'migration_pass':False}
                checkpoint={'schema':'voice-staging-realtime-bootstrap-checkpoint-v2','installation_sha256':session.installation.sha256,'plan':plan,'status':'COMPLETE',
                    'resource_fence':{**session.binding(),'target_sha256':prereq.digest(plan)},'steps':steps,'receipt':rt_receipt}
                rtbase=installed/'realtime-bootstrap'/session.operation;rtbase.mkdir(parents=True,mode=0o700)
                for name,value in [('checkpoint.json',checkpoint),('before-secret.json',before),
                    ('authority.json',{'approved':approved,'execution':execution}),('stage.json',view)]:save(rtbase/name,value)
                (rtbase/'credential').write_bytes(password);(rtbase/'credential').chmod(0o600)
                live[('secret','voice-app-secrets')]=copy.deepcopy(after)
                self.assertEqual(prereq.load_capture(session),captured)
                legacy_plan=copy.deepcopy(plan);legacy_plan['schema']='voice-staging-realtime-bootstrap-v2'
                del legacy_plan['installation_sha256']
                legacy_steps=copy.deepcopy(steps)
                legacy_steps['secret']['request_sha256']=prereq.digest({'kind':'secret','plan_sha256':prereq.digest(legacy_plan)})
                legacy_receipt={**rt_receipt,'schema':'voice-staging-realtime-bootstrap-receipt-v1',
                    'plan_sha256':prereq.digest(legacy_plan),'steps_sha256':prereq.digest(legacy_steps)}
                del legacy_receipt['installation_sha256']
                legacy_fence={**checkpoint['resource_fence'],'schema':'voice-staging-resource-fence-v1',
                              'target_sha256':prereq.digest(legacy_plan)}
                del legacy_fence['installation_sha256']
                legacy={'schema':'voice-staging-realtime-bootstrap-checkpoint-v1','plan':legacy_plan,'status':'COMPLETE',
                        'resource_fence':legacy_fence,'steps':legacy_steps,'receipt':legacy_receipt}
                save(rtbase/'checkpoint.json',legacy)
                with self.assertRaisesRegex(prereq.Blocked,'realtime_custody_changed'):prereq.load_capture(session)
                save(rtbase/'checkpoint.json',checkpoint)
                checkpoint['status']='RUNNING';save(rtbase/'checkpoint.json',checkpoint)
                with self.assertRaises(Exception):prereq.load_capture(session)
                checkpoint['status']='COMPLETE';save(rtbase/'checkpoint.json',checkpoint)
                for field in ('uid','resourceVersion'):
                    altered=copy.deepcopy(before);altered['metadata'][field]='foreign';save(rtbase/'before-secret.json',altered)
                    with self.assertRaises(Exception):prereq.load_capture(session)
                save(rtbase/'before-secret.json',before)
                live[('secret','voice-app-secrets')]['data']['UNRELATED']='Zm9yZWlnbg=='
                with self.assertRaises(Exception):prereq.load_capture(session)
                live[('secret','voice-app-secrets')]=copy.deepcopy(after)
                self.assertEqual(prereq.load_capture(session),captured)
                save(rtbase/'authority.json',{'approved':{**approved,'forged':True},'execution':execution})
                with self.assertRaises(Exception):prereq.load_capture(session)
                self.assertEqual(len(writes),15);return
            if fault in ('c2','c2-new-candidate'):
                # A durable rollout PASS is modeled here; this is not a claim
                # that the transaction/runtime boundary ran in this fixture.
                target={'registry':'registry','tag':context['commit'],'mode':'app-only','changed_services':['gameintegration'],
                    'images':context['images'],'manifest_sha256':'1'*64,'migration_sha256':'2'*64,'nonnats_sha256':'3'*64,
                    'source_hashes':{'fixture':'4'*64},'template_hashes':{'fixture':'5'*64},'input_template_hashes':{'fixture':'6'*64},
                    'installation_sha256':session.installation.sha256,'cohort_preservation_sha256':prereq.digest(qualified),
                    'realtime_bootstrap_sha256':'7'*64,'prerequisites_sha256':prereq.digest(captured)}
                saved={'schema':'nats-rollout-root-v2','installation_sha256':session.installation.sha256,
                    'operation':operation,'status':'PASS','phase':'POST_RESTART','fence_status':'RELEASED',
                    'code_capture':manifest,'target':target,'prerequisites':captured,
                    'resource_fence':{**session.binding(),'target_sha256':prereq.digest(target)},
                    'context':{'expected':stage.expected,'snapshots':stage.snapshots}}
                rollout=root/('rollout-'+operation);rollout.mkdir(mode=0o700);save(rollout/'checkpoint.json',saved)
                active={'schema':'voice-active-release-v1','installation_sha256':session.installation.sha256,
                        'operation':operation,'namespace_uid':ns,'pvc_uid':claim_uid,'target_sha256':'1'*64}
                # The historical P2 producer is outside this prerequisite unit;
                # its qualified receipt is the explicit preservation fixture.
                historical_receipts={operation:qualified}
                stack.enter_context(patch.object(cohort,'validate_historical_capture',
                    side_effect=lambda op,**kwargs:historical_receipts[op]))
                save(installed/'active-release.json',active)
                self.assertEqual(prereq.validate_historical_capture(operation),captured)
                attempt=prereq._base(session)/'attempts'/(hashlib.sha256(b'gis_material/gis-app-credentials').hexdigest()+'.json')
                original_attempt=json.loads(attempt.read_bytes());save(attempt,{**original_attempt,'body_sha256':'0'*64})
                with self.assertRaisesRegex(prereq.Blocked,'history_effect_changed'):prereq.validate_historical_capture(operation)
                save(attempt,original_attempt)
                saved['status']='ROLLED_BACK';save(rollout/'checkpoint.json',saved)
                with self.assertRaisesRegex(prereq.Blocked,'history_not_pass'):prereq.validate_historical_capture(operation)
                saved['status']='PASS';save(rollout/'checkpoint.json',saved)
                operation='b'*12;workspace2=installed/'sources'/operation;shutil.copytree(workspace,workspace2)
                context2=copy.deepcopy(context);context2['operation_id']='stg-run-18'
                card2=resource_claim_test.card(context2);card2['observed']['images']=copy.deepcopy(stage.old_images)
                session=resource_claim.Session(context2,operation,prereq.digest(manifest),lambda:copy.deepcopy(card2),resource_claim_test.MemoryJournal())
                session.admit(start=True);actions.tt_session=session;stage.operation=operation
                save(installed/'source-authority'/(operation+'.json'),{**source,'operation':operation,'context':context2})
                writes.clear();events.clear()
                # GIS workload admission belongs to the normal Stage caller;
                # this external baseline seam remains explicit in the fixture.
                anchor={'operation':'d'*12,'receipt_sha256':'e'*64}
                active['gis_first_install_transition']=anchor;save(installed/'active-release.json',active)
                stack.enter_context(patch('gis_existing_update.load_authority',return_value={
                    'operation':anchor['operation'],'anchor_sha256':anchor['receipt_sha256']}))
                with resource_claim.command_scope(session.check):
                    request2={'mode':'app-only','changed_services':['gameintegration']}
                    prepared2=prereq.prepare_inputs(actions,request2,approved,execution,workspace2)
                    qualified2={'prerequisite_inputs_sha256':prepared2['inputs_sha256']}
                    historical_receipts[operation]=qualified2
                    qualified_loader.return_value=qualified2
                    auxiliary_loader.return_value={'response':copy.deepcopy(live[('deployment','voice-p3-principal-jwks')])}
                    p2_plan2={'actual_before':stage.old_images,'candidate':{'commit':context2['commit'],'images':context2['images']}}
                    with patch.object(cohort,'validate_plan'):
                        session.bind_cohort_preservation(p2_plan2,stage.old_images)
                    receipt2=prereq.run_prepare(actions,request2,approved,execution,workspace2,preservation_ref=qualified2)
                    self.assertEqual(writes,[])
                    self.assertEqual(receipt2['actual_before'],receipt2['actual_after'])
                    binding2=session.capture_prerequisites(stage)
                    captured2=prereq.load_capture(session)
                    self.assertEqual(binding2['target_sha256'],prereq.digest(captured2))
                    live[('deployment','voice-p3-principal-jwks')]['metadata']['resourceVersion']='900'
                    self.assertEqual(prereq.load_capture(session),captured2)
                    old=live[('secret','voice-gameintegration-workload-credentials')]['metadata']['uid']
                    live[('secret','voice-gameintegration-workload-credentials')]['metadata']['uid']='foreign'
                    with self.assertRaises(Exception):prereq.load_capture(session)
                    live[('secret','voice-gameintegration-workload-credentials')]['metadata']['uid']=old
                    save(installed/'active-release.json',{**active,'target_sha256':'2'*64})
                    with self.assertRaisesRegex(prereq.Blocked,'active_changed'):prereq.load_preserved(session,'p3','Secret/voice-realtime-principal-signing')
                    save(installed/'active-release.json',active)
                    card2['generation']+=1
                    with self.assertRaises(Exception):prereq.load_preserved(session,'p3','Secret/voice-realtime-principal-signing')
                    card2['generation']-=1
                    self.assertEqual(prereq.load_capture(session),captured2)
                    # Exercise the actual source comparator with a separately
                    # hash-qualified changed publisher blob, not stale bytes.
                    source_path=workspace2/'deploy/staging/services.yaml';source_bytes=source_path.read_bytes()
                    changed=source_bytes.replace(b'nginx:stable-alpine',b'nginx:alpine')
                    self.assertNotEqual(changed,source_bytes);source_path.write_bytes(changed)
                    changed_approved=copy.deepcopy(approved)
                    changed_approved['source_files']['deploy/staging/services.yaml']=hashlib.sha256(changed).hexdigest()
                    prior,_,prior_rollout=prereq._historical(active['operation'],set())
                    with self.assertRaisesRegex(prereq.Blocked,'preserved_source_changed'):
                        prereq._qualified_preserved(session,'p3','Deployment/voice-p3-principal-jwks',prior,prior_rollout,changed_approved)
                    source_path.write_bytes(source_bytes)
                target2={**target,'cohort_preservation_sha256':prereq.digest(qualified2),'prerequisites_sha256':prereq.digest(captured2)}
                saved2={**saved,'operation':operation,'target':target2,'prerequisites':captured2,
                    'resource_fence':{**session.binding(),'target_sha256':prereq.digest(target2)}}
                rollout2=root/('rollout-'+operation);rollout2.mkdir(mode=0o700);save(rollout2/'checkpoint.json',saved2)
                active2={**active,'operation':operation};save(installed/'active-release.json',active2)
                self.assertEqual(prereq.validate_historical_capture(operation),captured2)
                # Re-seal all local digest links around a foreign origin. The
                # immediate protected predecessor must still reject transfer.
                bbase=prereq._base(session);bstate=json.loads((bbase/'checkpoint.json').read_bytes())
                bready=json.loads((bbase/'publisher-ready.json').read_bytes())
                forged=copy.deepcopy(bstate);forged_ready=copy.deepcopy(bready)
                forged['plan']['children']['p3']['preserved']['Secret/voice-realtime-principal-signing']['authority']={
                    'operation':'f'*12,'receipt_sha256':'f'*64}
                forged['initial_resource_fence']['target_sha256']=prereq.digest(forged['plan'])
                forged['preflight_sha256']=prereq.digest(forged['plan'])
                forged_ready['receipt']['plan_sha256']=prereq.digest(forged['plan']['children']['p3'])
                forged['receipt']['children_receipts']['p3']=forged_ready['receipt']
                forged['receipt']['plan_sha256']=prereq.digest(forged['plan'])
                forged_capture={**captured2,'plan_sha256':prereq.digest(forged['plan']),'receipt_sha256':prereq.digest(forged['receipt'])}
                forged_rollout=copy.deepcopy(saved2);forged_rollout['prerequisites']=forged_capture
                forged_rollout['target']['prerequisites_sha256']=prereq.digest(forged_capture)
                forged_rollout['resource_fence']['target_sha256']=prereq.digest(forged_rollout['target'])
                save(bbase/'checkpoint.json',forged);save(bbase/'publisher-ready.json',forged_ready)
                save(bbase/'capture.json',forged_capture);save(rollout2/'checkpoint.json',forged_rollout)
                with self.assertRaisesRegex(prereq.Blocked,'history_predecessor_changed'):prereq.validate_historical_capture(operation)
                save(bbase/'checkpoint.json',bstate);save(bbase/'publisher-ready.json',bready)
                save(bbase/'capture.json',captured2);save(rollout2/'checkpoint.json',saved2)
                operation='d'*12;workspace3=installed/'sources'/operation;shutil.copytree(workspace,workspace3)
                context3={**context,'operation_id':'stg-run-19'};approved3=copy.deepcopy(approved)
                if fault=='c2-new-candidate':context3['commit']='e'*40;approved3['source_sha']=context3['commit']
                card3=resource_claim_test.card(context3);card3['observed']['images']=copy.deepcopy(stage.old_images)
                session=resource_claim.Session(context3,operation,prereq.digest(manifest),lambda:copy.deepcopy(card3),resource_claim_test.MemoryJournal())
                session.admit(start=True);actions.tt_session=session;stage.operation=operation
                save(installed/'source-authority'/(operation+'.json'),{**source,'operation':operation,'context':context3,'approved_sha256':prereq.digest(approved3)})
                with resource_claim.command_scope(session.check):
                    request3={'mode':'app-only','changed_services':['gameintegration']}
                    prepared3=prereq.prepare_inputs(actions,request3,approved3,execution,workspace3)
                    qualified3={'prerequisite_inputs_sha256':prepared3['inputs_sha256']};qualified_loader.return_value=qualified3
                    p2_plan3={'actual_before':stage.old_images,'candidate':{'commit':context3['commit'],'images':context3['images']}}
                    with patch.object(cohort,'validate_plan'):
                        session.bind_cohort_preservation(p2_plan3,stage.old_images)
                    receipt3=prereq.run_prepare(actions,request3,approved3,execution,workspace3,preservation_ref=qualified3)
                    self.assertEqual(len(writes),1 if fault=='c2-new-candidate' else 0)
                    if fault=='c2-new-candidate':
                        plan3=prereq._checkpoint(session)['plan'];descriptor3=policy.staging_descriptor(context3['commit'])
                        self.assertEqual([row['id'] for row in plan3['children']['release_policy']['effects']],list(prereq.EFFECT_IDS['release_policy']))
                        self.assertEqual(plan3['children']['p3']['effects'],[]);self.assertEqual(plan3['children']['gis_material']['effects'],[])
                        self.assertIn(('configmap',descriptor3['policy_object']['metadata']['name']),live)
                        self.assertEqual(json.loads((installed/'policy.json').read_bytes())['release_policy'],descriptor3)
                    self.assertEqual(receipt3['actual_before'],receipt3['actual_after'])
                    self.assertEqual(prereq.require_child(session,'p3')['preserved']['Secret/voice-realtime-principal-signing']['authority']['operation'],'a'*12)
                    self.assertEqual(prereq._checkpoint(session)['plan']['previous_active_sha256'],prereq.digest(active2))


if __name__=='__main__':unittest.main()
