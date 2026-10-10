"""Actual plan/Actor and marker consumers; isolated wire/storage adapters."""
import copy,json,unittest
from pathlib import Path
from unittest.mock import Mock
from controller import Blocked
from nats_contract_actor import Actor,WRITE
from nats_contract_plan import execute,SCHEMA
from runtime_stage import RolloutStage

class Runtime:
    def inputs_directory(self):return self.base/'inputs'
    def __init__(self,clock,expire_at_create=False):
        self.clock=clock;self.expire_at_create=expire_at_create;self.base=Path('/synthetic-operation');self.owned=['broker'];self.commands={};self.started=[]
        self.config={'chat_events':{'name':'chat_events','subjects':['chat.old']},'social_events':{'name':'social_events','subjects':['social.old']}}
    def inspect(self,name):
        return {'Id':'b'*64,'State':{'Running':name=='broker'},'HostConfig':{'NetworkMode':'none','PortBindings':{}},'Mounts':[{'Destination':'/data','Type':'bind','Source':'/synthetic-store','RW':True}]}
    def create(self,suffix,image,mounts,command,broker):
        name='client-'+str(len(self.owned));self.owned.append(name);self.commands[name]=(command[-2],json.loads(command[-1]) if command[-1] else None)
        if command[-2] in WRITE and self.expire_at_create:self.clock[0]=1
        return name
    def run(self,args,**kwargs):
        kind,name=args;api,payload=self.commands[name]
        if kind=='start':
            if api in WRITE:self.started.append(api);self.config[WRITE[api]]=copy.deepcopy(payload)
            return ''
        if kind=='wait':return '0'
        if kind=='logs':
            config=self.config[api.rsplit('.',1)[-1]] if '.STREAM.INFO.' in api else self.config[WRITE[api]]
            return json.dumps({'config':config})
        raise AssertionError(args)

class MutationBoundaryTests(unittest.TestCase):
    def test_actual_plan_actor_rejects_expiry_after_issued_and_after_client_create(self):
        for where in ('issued','client-create'):
            with self.subTest(where=where):
                clock=[0];runtime=Runtime(clock,where=='client-create');actor=Actor(runtime,'broker','b'*64,'/synthetic-store',lambda:None)
                def authority():
                    if clock[0]:raise Blocked('expired_recovery_authority_expired')
                actor.mutation_guard=authority
                before=copy.deepcopy(runtime.config['chat_events']);after=copy.deepcopy(before);after['subjects'].append('chat.new')
                plan={'schema':SCHEMA,'migration_id':'space-chat-social-24h-friend-removed-v1','actions':[{'api':'$JS.API.STREAM.UPDATE.chat_events','object':'chat_events','before':before,'after':after}]}
                events=[]
                def journal(row):
                    events.append(row)
                    if where=='issued' and row['kind']=='nats_contract_mutation_issued':clock[0]=1
                with self.assertRaises(Blocked):execute(plan,actor,journal)
                self.assertEqual(runtime.started,[])
                self.assertEqual([row['kind'] for row in events],['nats_contract_intent','nats_contract_mutation_issued'])
                self.assertEqual(runtime.config['chat_events'],before)

    def test_actual_actor_readonly_and_ordinary_write_keep_their_boundary(self):
        runtime=Runtime([0]);actor=Actor(runtime,'broker','b'*64,'/synthetic-store',lambda:None)
        actor.mutation_guard=Mock(side_effect=Blocked('expired'))
        observed=actor._request('$JS.API.STREAM.INFO.chat_events','')
        self.assertEqual(observed['config'],runtime.config['chat_events']);actor.mutation_guard.assert_not_called()
        actor.mutation_guard=None
        after=copy.deepcopy(runtime.config['chat_events']);after['subjects'].append('normal-path')
        actor._request('$JS.API.STREAM.UPDATE.chat_events',after)
        self.assertEqual(runtime.config['chat_events'],after)
        self.assertEqual(runtime.started,['$JS.API.STREAM.UPDATE.chat_events'])

class MarkerObservationTests(unittest.TestCase):
    def stage(self,phase):
        marker={'metadata':{'uid':'marker','resourceVersion':'22'},'data':{'phase':phase,'knownRolloutOperation':'3340764a7d24','dataPVC':'claim'}}
        stage=object.__new__(RolloutStage);stage.marker=copy.deepcopy(marker);stage.operation='3340764a7d24';stage.expected={'source_claim':'claim'};stage.save=Mock();stage.verify_final_storage=Mock()
        stage.kube=Mock();stage.kube.get.return_value=copy.deepcopy(marker)
        def cas(kind,row,patch):
            result=copy.deepcopy(row);result['metadata']['resourceVersion']='23';result['data']['phase']='rollout-verified';stage.kube.get.return_value=result;return result
        stage.kube.cas.side_effect=cas
        return stage
    def test_actual_verified_marker_observation_is_idempotent_without_cas(self):
        stage=self.stage('rollout-verified');before=copy.deepcopy(stage.marker)
        stage.verified();stage.verified()
        self.assertEqual(stage.marker,before);stage.kube.cas.assert_not_called();stage.save.assert_not_called()
    def test_actual_applying_marker_advances_once_then_observes_without_replay(self):
        stage=self.stage('rollout-applying');stage.verified();stage.verified();self.assertEqual(stage.kube.cas.call_count,1)
    def test_actual_verified_marker_still_rejects_changed_owner_or_resource_version(self):
        for key,value in [('uid','other'),('resourceVersion','23')]:
            stage=self.stage('rollout-verified');stage.kube.get.return_value['metadata'][key]=value
            with self.assertRaises(Blocked):stage.verified()
            stage.kube.cas.assert_not_called()

    def test_actual_verified_observation_rejects_wrong_phase_and_storage(self):
        for phase in ('active','rollout-prepared','rollout-capturing'):
            stage=self.stage(phase)
            with self.subTest(phase=phase),self.assertRaises(Blocked):stage.verified()
            stage.kube.cas.assert_not_called();stage.save.assert_not_called()
        stage=self.stage('rollout-verified');stage.verify_final_storage.side_effect=Blocked('storage-changed')
        with self.assertRaises(Blocked):stage.verified()
        stage.kube.get.assert_not_called();stage.kube.cas.assert_not_called()

class FinishResponseBoundaryTests(unittest.TestCase):
    def test_actual_finish_prepare_response_survives_client_without_resetting_blocked_state(self):
        import tempfile
        import bridge_root,bridge_client
        from unittest.mock import patch
        with tempfile.TemporaryDirectory() as folder:
            base=Path(folder)/'rollout-3340764a7d24';slot=base/'expired-finish-proofs'/('a'*64);slot.mkdir(parents=True,mode=0o700)
            installed=Path(folder)/'installed';installed.mkdir(mode=0o750)
            for name in ('inbox','responses'):(installed/name).mkdir(mode=0o750)
            state={'operation':'3340764a7d24','status':'BLOCKED','phase':'RESTART','error':'old-error','fence_status':'VERIFIED','target':{'tag':'b'*40}}
            before=copy.deepcopy(state);execution={'nonce':'a'*64};cipher={'cipher':'bound'}
            request={'nonce':'c'*64};actions=object.__new__(bridge_root.Actions)
            actions._produce_finish_observation=Mock(return_value=(slot,state,cipher))
            bindings={'artifact_bindings':{'original':{'challenge':'d'*32},'observation':{'challenge':'e'*32}}}
            with patch('expired_transport.upload.private_json',return_value={'execution':execution}),patch('expired_transport.export',return_value=(bindings,{'original':'/original.cms','observation':'/observation.cms'})):
                result=actions._prepare_expired_finish(request)
            (installed/'responses'/(request['nonce']+'.json')).write_text(json.dumps(result))
            received=bridge_client.submit(request,installed,timeout=1)
            self.assertEqual(received['status'],'WAITING');self.assertEqual(state,before)
            self.assertEqual(received['original_cipher_path'],'/original.cms');self.assertEqual(received['observation_cipher_path'],'/observation.cms')
            self.assertEqual(received['operation_status'],'BLOCKED');self.assertEqual(received['operation_error'],'old-error')
            actions._produce_finish_observation.side_effect=Blocked('proof-rejected')
            with self.assertRaises(Blocked):actions._prepare_expired_finish(request)

class CustodyPublicationBoundaryTests(unittest.TestCase):
    def test_custody_cas_rechecks_after_own_intent_and_default_remains_normal(self):
        import hashlib
        from nats_contract_custody import advance
        for expired in (False,True):
            with self.subTest(expired=expired):
                old='original-script';new='target-script';sha=lambda text:hashlib.sha256(text.encode()).hexdigest()
                row={'metadata':{'uid':'cm','resourceVersion':'11'},'data':{'bootstrap.sh':old}}
                state={'operation':'3340764a7d24','nats_migration':{'verified':True,'old_record_files_verified':True,'proof':{}},
                    'contract':{'scripts':[{'part':'realtime','configMapUID':'cm','configMapResourceVersion':'11','sha256':sha(old)}],'consumer_pairs':[]},
                    'nats_target_scripts':{'realtime':{'script':new,'sha256':sha(new)}},'nats_contract':{'migration_id':'fixed','sources':{},'actions':[]}}
                kube=Mock();kube.get.return_value=row
                kube.cas.return_value={'metadata':{'uid':'cm','resourceVersion':'12'},'data':{'bootstrap.sh':new}}
                stage=Mock();stage.expected={'generation':'g','namespace_uid':'ns','source_claim_uid':'pvc'}
                events=[];clock=[False]
                def journal(event):events.append(event);clock[0]=True
                def authority():
                    if clock[0]:raise Blocked('expired')
                if expired:
                    with self.assertRaises(Blocked):advance(kube,state,stage,journal,mutation_guard=authority)
                    kube.cas.assert_not_called();self.assertEqual([e['kind'] for e in events],['nats_contract_cm_intent'])
                    self.assertEqual(state['contract']['scripts'][0]['configMapResourceVersion'],'11')
                else:
                    advance(kube,state,stage,journal);kube.cas.assert_called_once()
    def test_active_release_guard_is_at_file_publication_and_no_write_on_expiry(self):
        import bridge_root
        from unittest.mock import patch
        actions=object.__new__(bridge_root.Actions)
        state={'operation':'3340764a7d24','context':{'expected':{'namespace_uid':'ns','source_claim_uid':'pvc'}},'target':{'manifest_sha256':'a'*64}}
        guard=Mock(side_effect=Blocked('expired'))
        with patch('bridge_root.save') as save:
            with self.assertRaises(Blocked):actions._active_release(state,authority_guard=guard)
            save.assert_not_called();guard.assert_called_once()

class DedicatedApplyReceiptTests(unittest.TestCase):
    def test_dedicated_receipt_actual_guard_requires_root_bound_authority_projection(self):
        import tempfile,hashlib,datetime as dt,os
        import guard,expired_recovery,root_cli
        from guard_test import receipt
        from expired_recovery_test import RecoveryAuthorityTests
        from preserved_upload import digest
        from unittest.mock import patch
        fixture=RecoveryAuthorityTests();fixture.setUp();now=dt.datetime.now(dt.timezone.utc)
        fixture.proof['started_at']=(now-dt.timedelta(seconds=30)).isoformat();fixture.proof['completed_at']=now.isoformat()
        authority=expired_recovery.authority(fixture.state,fixture.execution,fixture.proof,now=now)
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder)/'authority';root.mkdir(mode=0o750);base=root/('rollout-'+fixture.state['operation']);base.mkdir(mode=0o750)
            source=Path(folder)/'source';(source/'scripts').mkdir(parents=True);(source/'scripts/fixed').write_text('bound')
            row=receipt();row.update(schema='nats-rollout-preservation-v1',operation=fixture.state['operation'],
                target={'registry':'registry','tag':fixture.state['target']['tag'],'mode':'images-only','changed_services':['story'],
                    'source_hashes':{'scripts/fixed':hashlib.sha256(b'bound').hexdigest()},'template_hashes':{'voice-story':'a'*64},
                    'input_template_hashes':{'voice-story':'a'*64},'images':{'voice-story/story':'registry/story@sha256:'+'b'*64},
                    'manifest_sha256':'c'*64,'migration_sha256':'d'*64,'nonnats_sha256':'e'*64},
                backup={'archive_sha256':'1'*64,'manifest_sha256':'2'*64,'census_sha256':'3'*64,'off_node_verified':True},
                expires_at=authority['expires_at'],expired_recovery_authority_sha256=digest(authority))
            fixture.proof['original_target_sha256']=digest(row['target']);authority['proof_sha256']=digest(fixture.proof)
            row['expired_recovery_authority_sha256']=digest(authority)
            projection={'schema':'voice-expired-native-apply-authority-v1','receipt_sha256':digest(row),
                'helper_binding_sha256':digest({'current':'helper'}),'authority':authority,'proof':fixture.proof}
            path=base/'apply-recovery-authority.json';path.write_text(json.dumps(projection));path.chmod(0o440)
            with patch.object(guard,'ROOT',root),patch.object(root_cli,'code_binding',return_value={'current':'helper'}):
                guard.validate(row,'registry',row['target']['tag'],'images-only',['story'],source)
                for key in ('receipt_sha256','helper_binding_sha256'):
                    changed=copy.deepcopy(projection);changed[key]='0'*64;path.chmod(0o600);path.write_text(json.dumps(changed))
                    with self.assertRaises(Blocked):guard.validate(row,'registry',row['target']['tag'],'images-only',['story'],source)
                path.chmod(0o600);path.write_text(json.dumps(projection));path.chmod(0o660)
                with self.assertRaises(Blocked):guard.validate(row,'registry',row['target']['tag'],'images-only',['story'],source)
                path.chmod(0o600);path.write_text(json.dumps(projection));path.chmod(0o440);changed=copy.deepcopy(row);changed['unknown']='extra'
                with self.assertRaises(Blocked):guard.validate(changed,'registry',row['target']['tag'],'images-only',['story'],source)
                # Rebind the enclosing hashes: these controls target semantic authority,
                # not merely a stale outer receipt digest.
                for case in ('finish-purpose','wrong-operation','wrong-workflow','old-attempt','wrong-target'):
                    changed=copy.deepcopy(projection);changed_row=copy.deepcopy(row)
                    if case=='finish-purpose':changed['authority']['purpose']='finish'
                    if case=='wrong-operation':changed['authority']['operation']='0'*12
                    if case=='wrong-workflow':changed['authority']['execution']['workflow_id']=1
                    if case=='old-attempt':changed['authority']['execution']['dispatcher_run_id']=37917461677
                    if case=='wrong-target':changed['proof']['original_target_sha256']='0'*64
                    changed['authority']['proof_sha256']=digest(changed['proof'])
                    changed_row['expired_recovery_authority_sha256']=digest(changed['authority'])
                    changed['receipt_sha256']=digest(changed_row)
                    path.chmod(0o600);path.write_text(json.dumps(changed));path.chmod(0o440)
                    with self.subTest(case=case),self.assertRaises(Blocked):
                        guard.validate(changed_row,'registry',row['target']['tag'],'images-only',['story'],source)
                path.chmod(0o600);path.write_text(json.dumps(projection));path.chmod(0o440)
                (source/'scripts/fixed').write_text('untrusted drift')
                with self.assertRaises(Blocked):guard.validate(row,'registry',row['target']['tag'],'images-only',['story'],source)
                (source/'scripts/fixed').write_text('bound')
                # The original exact nine-field route remains accepted without a
                # dedicated sibling file, while an unknown extra remains forbidden.
                ordinary=copy.deepcopy(row);ordinary.pop('expired_recovery_authority_sha256')
                path.unlink()
                guard.validate(ordinary,'registry',row['target']['tag'],'images-only',['story'],source)
                ordinary['unknown']='extra'
                with self.assertRaises(Blocked):guard.validate(ordinary,'registry',row['target']['tag'],'images-only',['story'],source)

class ApplyProducerCompositionTests(unittest.TestCase):
    def test_actual_transaction_receipt_root_projection_runner_claim_and_apply(self):
        import tempfile,hashlib,os,datetime as dt
        from contextlib import ExitStack
        from unittest.mock import patch
        import transaction,guard,runner,apply,expired_transport as transport
        from expired_transport_test import TransportTests
        from guard_test import ClaimTest
        for late in (None,'claim','apply','native-publication','projection-drift-claim','projection-drift-apply'):
            with self.subTest(late=late),tempfile.TemporaryDirectory() as folder,ExitStack() as stack:
                fixture=TransportTests();fixture.setup_files(folder);state=fixture.state
                kube=ClaimTest();kube.setUp();kube.marker['data']['knownRolloutOperation']=state['operation']
                source=Path(folder)/'source';(source/'scripts').mkdir(parents=True);(source/'scripts/fixed').write_text('bound')
                (source/'deploy/nats').mkdir(parents=True);(source/'deploy/nats/acl-intent.yaml').write_text('source-bound-acl')
                documents=[{'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':'fixed','namespace':guard.NS},'data':{'public':'bound'}}]
                raw=json.dumps(documents,sort_keys=True,separators=(',',':')).encode()
                state['target']={'registry':'registry','tag':transport.upload.ORIGINAL_TARGET,'mode':'images-only','changed_services':['story'],
                    'source_hashes':{'scripts/fixed':hashlib.sha256(b'bound').hexdigest()},'template_hashes':{'voice-story':'a'*64},
                    'input_template_hashes':{'voice-story':'a'*64},'images':{'voice-story/story':'registry/story@sha256:'+'b'*64},
                    'manifest_sha256':hashlib.sha256(raw).hexdigest(),'migration_sha256':transport.upload.digest([]),'nonnats_sha256':'e'*64}
                state['events']=[]
                state['context']['expected']={'namespace_uid':kube.r['namespace_uid'],'source_claim_uid':kube.r['pvc']['uid']}
                for path in (fixture.base/'checkpoint.json',fixture.base/'expired-recovery-adopted-checkpoint.json'):
                    transaction.save(path,state)
                producer=transport.upload.private_json(fixture.slot/'producer.json')
                producer['original_checkpoint_sha256']=hashlib.sha256((fixture.base/'checkpoint.json').read_bytes()).hexdigest()
                transaction.save(fixture.slot/'producer.json',producer)
                transport.export(fixture.base,state,fixture.slot,fixture.execution,fixture.cipher)
                def artifact(token,binding,path,artifact_id):
                    return {'schema':'voice-nats-custody-v1','destination':'github:Poryadok/VoiceRoot',**{k:binding[k] for k in ('operation','challenge','run_id','head_sha','cipher_sha256','cipher_bytes')},'artifact_id':artifact_id,'verified':True}
                with patch('github_custody.verify_artifact',side_effect=artifact):
                    transport.readback('synthetic',fixture.base,state,fixture.slot,fixture.execution,{'original':11,'observation':12})
                authority=transport.commit_authority(fixture.base,state,fixture.slot,fixture.execution)
                stage=Mock();stage.marker=copy.deepcopy(kube.marker);stage.final_claim=copy.deepcopy(kube.pvc);stage.final_pv=copy.deepcopy(kube.pv)
                stage.final_path=Path(kube.r['pv']['path']);stage.refence.return_value={'verified':True}
                stage.expected={'namespace_uid':kube.r['namespace_uid'],'generation':'r20260930a4','source_claim_uid':kube.r['pvc']['uid']}
                context=copy.deepcopy(state['context'])
                def migrate(base,current,actual,journal,**kwargs):
                    kwargs['before_selected_start']();kwargs['mutation_guard']();journal({'kind':'fixture-native-complete'})
                    return {'verified':True,'cut':copy.deepcopy(state['cut'])}
                for adapter in [patch('guard.ROOT',fixture.base.parent),patch('root_cli.code_binding',return_value={}),
                    patch('expired_recovery.verify_adopted_binding'),patch.object(transaction,'reconstruct',return_value=stage),
                    patch.object(transaction,'context',return_value=context),patch.object(transaction,'file_sha',return_value=state['cut']['manifest_sha256']),
                    patch.object(transaction,'verify_archive',return_value=True),patch.object(transaction,'revalidate_inputs'),
                    patch.object(transaction,'DockerRuntime'),patch('expired_proof.current_inventory',return_value='a'*64),
                    patch.object(transaction,'apply_contract',side_effect=migrate),patch('nats_contract_custody.advance',return_value={}),
                    patch.object(transaction.migrations,'execute',return_value=[]),patch.object(transaction.space_safeguards,'before_migrate'),
                    patch.object(transaction.space_safeguards,'after_migrate',return_value=None)]:stack.enter_context(adapter)
                (fixture.base.parent/'installed').mkdir(mode=0o700)
                receipt=transaction.authorize(None,fixture.base,state,state['cut']['manifest']['archive_sha256'],state['cut']['manifest_sha256'],{},
                    authorization_deadline=dt.datetime.fromisoformat(authority['expires_at']),expired_authority=authority)
                manifest=fixture.base/'apply-manifests.json';manifest.write_bytes(raw);manifest.chmod(0o440)
                receipt_path=fixture.base/'apply-authorization.json';receipt_path.chmod(0o440)
                env={'VOICE_NATS_PRESERVATION_RECEIPT':str(receipt_path),'VOICE_IMAGE_REGISTRY':'registry','VOICE_IMAGE_TAG':receipt['target']['tag'],
                    'DEPLOY_MODE':'images-only','CHANGED_SERVICES':'story','VOICE_NATS_ACL_PROOF_SHA':hashlib.sha256(b'source-bound-acl').hexdigest(),
                    'VOICE_NATS_ACL_PROOF_GENERATION':receipt['marker']['generation']}
                stack.enter_context(patch.dict(os.environ,env,clear=True));stack.enter_context(patch('guard.source_root',return_value=source))
                stack.enter_context(patch('runner.compatible_bootstrap'))
                captured_read=guard.protected_receipt
                stack.enter_context(patch('guard.protected_receipt',side_effect=lambda path,**kwargs:captured_read(path,root=fixture.base.parent,**{k:v for k,v in kwargs.items() if k!='root'})))
                current=[dt.datetime.now(dt.timezone.utc)];real_datetime=dt.datetime
                class Clock(real_datetime):
                    @classmethod
                    def now(cls,tz=None):return current[0]
                stack.enter_context(patch('guard.dt.datetime',Clock))
                applies=[]
                def wire(args,body=None):
                    if args[0]=='apply':applies.append(body);return b'configmap/fixed'
                    result=kube.kube(args,body)
                    if late=='claim' and args[:2]==['get','configmap']:current[0]=real_datetime.fromisoformat(authority['expires_at'])
                    if late=='apply' and args[:2]==['get','configmap'] and kube.marker['data']['phase']=='rollout-applying':current[0]=real_datetime.fromisoformat(authority['expires_at'])
                    if (late=='projection-drift-claim' and args[:2]==['get','configmap'] or
                        late=='projection-drift-apply' and args[:2]==['get','configmap'] and kube.marker['data']['phase']=='rollout-applying'):
                        projection_path=fixture.base/'apply-recovery-authority.json'
                        projection_path.chmod(0o600)
                        projected=json.loads(projection_path.read_text());projected['helper_binding_sha256']='0'*64
                        projection_path.write_text(json.dumps(projected));projection_path.chmod(0o440)
                    return result
                stack.enter_context(patch('guard.kubectl',side_effect=wire))
                if late in ('claim','projection-drift-claim'):
                    with self.assertRaises(Blocked):runner.preflight();guard.claim(receipt)
                    self.assertFalse(any(args[0]=='patch' for args,_ in kube.calls));self.assertEqual(applies,[])
                else:
                    self.assertEqual(runner.preflight(),receipt);rv=guard.claim(receipt)
                    os.environ['VOICE_NATS_ROLLOUT_CLAIM_RV']=rv
                    if late in ('apply','projection-drift-apply'):
                        with self.assertRaises(Blocked):apply.main()
                        self.assertEqual(applies,[])
                    else:
                        apply.main();self.assertEqual(len(applies),1);self.assertEqual(applies[0]['items'],documents)
                        if late=='native-publication':
                            import bridge_root
                            stack.enter_context(patch('bridge_root.INSTALLED',fixture.base.parent/'installed'))
                            stack.enter_context(patch.object(transaction,'verify_post_apply',return_value={'verified':True}))
                            actions=object.__new__(bridge_root.Actions);saved=bridge_root.save
                            def publish_save(path,value):
                                saved(path,value)
                                if Path(path).name=='active-release.json':current[0]=real_datetime.fromisoformat(authority['expires_at'])
                            stack.enter_context(patch('bridge_root.save',side_effect=publish_save))
                            def restart():
                                stage.finish_authority_guard();return {'verified':True}
                            stage.restart.side_effect=restart
                            with self.assertRaisesRegex(Blocked,'rollout_finish_authorization_expired'):
                                transaction.finish(None,fixture.base,state,receipt,rv,{},publication=actions._active_release)
                            self.assertEqual(state['status'],'BLOCKED');self.assertEqual(state['fence_status'],'VERIFIED');stage.refence.assert_called_once()
                            self.assertTrue((fixture.base.parent/'installed/active-release.json').exists())
                            actions.state=Mock(return_value=(fixture.base,state));actions._prepare=Mock()
                            with self.assertRaises(Blocked):actions.execute({'action':'prepare-rollback','operation':state['operation']})
                            actions._prepare.assert_not_called()

                self.assertEqual(state['nats_contract_expires_at'],authority['original_expires_at'])

class FinishPublicationCompositionTests(unittest.TestCase):
    def test_real_finish_engine_late_publication_blocks_refences_and_cannot_seed_rollback(self):
        import tempfile,datetime as dt
        from contextlib import ExitStack
        from unittest.mock import patch
        from expired_finish_test import FinishCompositionTests
        import bridge_root,transaction,expired_transport
        for boundary in ('before-save','after-save'):
            with self.subTest(boundary=boundary),tempfile.TemporaryDirectory() as folder,ExitStack() as stack:
                fixture=FinishCompositionTests();fixture.setup_files(folder);fixture.expire=False;fixture.adapters(stack)
                actions=object.__new__(bridge_root.Actions)
                deadline=dt.datetime.fromisoformat(fixture.authority['expires_at']);old_receipt=copy.deepcopy(fixture.state['authorization'])
                save=bridge_root.save
                def actual_save(path,row):
                    save(path,row)
                    if boundary=='after-save' and Path(path).name=='active-release.json':fixture.clock[0]=deadline
                stack.enter_context(patch('bridge_root.save',side_effect=actual_save))
                def publication(state,authority_guard):
                    if boundary=='before-save':fixture.clock[0]=deadline
                    actions._active_release(state,authority_guard=authority_guard)
                with self.assertRaises(ValueError):
                    transaction.finish(None,fixture.base,fixture.state,fixture.state['authorization'],'unused',{},
                        finish_authority=fixture.authority,publication=publication)
                self.assertEqual(fixture.state['status'],'BLOCKED');self.assertEqual(fixture.state['fence_status'],'VERIFIED')
                fixture.stage.refence.assert_called_once()
                ledger=expired_transport.upload.private_json(fixture.slot/'resume-ledger.json')
                self.assertEqual(ledger['completed'],{});self.assertTrue(all(row['requires_new_complete_proof'] for row in ledger['pending'].values()))
                self.assertTrue((fixture.slot/'safety-pause.json').exists())
                self.assertEqual(fixture.state['authorization'],old_receipt)
                active=fixture.base.parent/'installed/active-release.json'
                if boundary=='before-save':self.assertFalse(active.exists())
                else:
                    self.assertEqual(expired_transport.upload.private_json(active)['operation'],fixture.state['operation'])
                    actions.state=Mock(return_value=(fixture.base,fixture.state));actions._prepare=Mock()
                    with self.assertRaises(transaction.Blocked):actions.execute({'action':'prepare-rollback','operation':fixture.state['operation']})
                    actions._prepare.assert_not_called()

class RealStageFinishProducerTests(unittest.TestCase):
    def test_actual_actions_pause_producer_dual_export_and_final_drift_truth(self):
        import bridge_root,bridge_client,ast,os
        from expired_finish_test import FinishCompositionTests
        from tempfile import TemporaryDirectory
        from contextlib import ExitStack
        from unittest.mock import patch
        import transaction,expired_transport as transport
        import hashlib
        for phase,drift in (('rollout-applying',False),('rollout-verified',False),('rollout-verified',True)):
            with self.subTest(phase=phase,drift=drift),TemporaryDirectory() as folder,ExitStack() as stack:
                fixture=FinishCompositionTests();fixture.setup_files(folder)
                base,state,stage=fixture.base,fixture.state,fixture.stage
                execution=copy.deepcopy(fixture.execution);execution['nonce']='d'*64
                original=b'original immutable encrypted cut'
                state['cipher_binding']={**state['cipher_binding'],'cipher_bytes':len(original),'cipher_sha256':hashlib.sha256(original).hexdigest()}
                (base/'rollout-backup.cms').write_bytes(original);(base/'rollout-backup.cms').chmod(0o600)
                transaction.save(base/'checkpoint.json',state)
                actions=object.__new__(bridge_root.Actions);actions.code=Path(folder)/'code';actions.binding={}
                fixture.adapters(stack)
                encrypted=[False]
                def fresh():
                    if drift and encrypted[0]:(base/'checkpoint.json').write_bytes(b'late changed checkpoint')
                stack.enter_context(patch.object(actions,'_finish_source_admission',return_value=(base,state,stage,execution,{},
                    {'input_hashes':{'bound':'retained'},'get_permission_proof':{'bound':'actual scratch'}},fresh)))
                actual_marker=MarkerObservationTests().stage(phase)
                actual_marker.save=stage.save;actual_marker.verify_final_storage=stage.verify_final_storage
                stage.marker=copy.deepcopy(actual_marker.marker);stage.operation=state['operation']
                stage.owned_marker=actual_marker.owned_marker
                def verify_phase():
                    actual_marker.verified();stage.marker=copy.deepcopy(actual_marker.marker)
                stage.verified=verify_phase
                stack.enter_context(patch.object(transaction,'context',side_effect=lambda actual:{**copy.deepcopy(state['context']),'marker':copy.deepcopy(actual.marker)}))
                def observe(runtime,actual_stage,actual_state,slot,validate,checksum,interval):
                    row={'schema':'voice-expired-finish-current-observation-v1','operation':state['operation'],
                        'original_cut_sha256':transport.upload.digest(state['cut']),
                        'post_apply_cut_sha256':transport.upload.digest(state['nats_migration']['cut']),
                        'selected_inventory_sha256':'a'*64,'census':{'full':'postapply'},'full_records':{'all':6}}
                    transport.put(slot/'rollout-before-manifest.json',row)
                    transport.put(slot/'copy-checkpoint.json',{'bound':'closed copied proof'})
                    return row
                def encrypt(slot,recovery_keys):
                    raw=b'distinct current encrypted observation';(slot/'rollout-backup.cms').write_bytes(raw)
                    (slot/'rollout-backup.cms').chmod(0o600);encrypted[0]=True
                    return {'cipher_bytes':len(raw),'cipher_sha256':hashlib.sha256(raw).hexdigest()}
                stack.enter_context(patch('expired_finish.observe_post_apply',side_effect=observe))
                stack.enter_context(patch('encrypted_cut.encrypt_cut',side_effect=encrypt))
                request={'operation':state['operation'],'nonce':execution['nonce']}
                if drift:
                    with self.assertRaises(__import__('controller').Blocked):actions._prepare_expired_finish(request)
                    self.assertEqual(state['status'],'BLOCKED');self.assertEqual(state['fence_status'],'VERIFIED')
                    self.assertFalse((base/'expired-export'/execution['nonce']).exists())
                else:
                    result=actions._prepare_expired_finish(request)
                    slot=base/'expired-finish-proofs'/execution['nonce']
                    producer=transport.upload.private_json(slot/'producer.json')
                    record=transport.upload.private_json(slot/'upload.json')
                    self.assertEqual(record['producer_sha256'],transport.upload.digest(producer))
                    self.assertEqual(Path(result['original_cipher_path']).read_bytes(),original)
                    self.assertEqual(Path(result['observation_cipher_path']).read_bytes(),b'distinct current encrypted observation')
                    self.assertEqual(transport.upload.private_json(slot/'checkpoint.json'),state)
                    self.assertEqual(transport.upload.private_json(slot/'pause.json')['checkpoint_sha256'],
                        transport.upload.digest(transport.upload.private_json(slot/'paused-checkpoint.json')))
                    self.assertEqual(transport.upload.private_json(slot/'verified-pause.json')['checkpoint_sha256'],transport.upload.digest(state))
                    stage.restart.assert_not_called()
                    self.assertEqual(actual_marker.kube.cas.call_count,1 if phase=='rollout-applying' else 0)
                    self.assertEqual(result['status'],'WAITING');self.assertEqual(result['operation_status'],'BLOCKED')
                    installed=Path(folder)/'client';installed.mkdir(mode=0o750)
                    for name in ('inbox','responses'):(installed/name).mkdir(mode=0o750)
                    (installed/'responses'/(request['nonce']+'.json')).write_text(__import__('json').dumps(result))
                    received=bridge_client.submit(request,installed,timeout=1)
                    client_tree=ast.parse(Path(bridge_client.__file__).read_text())
                    main=next(row for row in client_tree.body if isinstance(row,ast.If))
                    output_if=next(row for row in main.body[0].body if isinstance(row,ast.If))
                    output=Path(folder)/'github-output'
                    with patch.dict(os.environ,{'GITHUB_OUTPUT':str(output)}):
                        exec(compile(ast.Module(body=[output_if],type_ignores=[]),'<actual-client-output>','exec'),{'os':os,'result':received})
                    lines=dict(line.split('=',1) for line in output.read_text().splitlines())
                    for key in ('original_cipher_path','observation_cipher_path','original_artifact_name','observation_artifact_name','upload_nonce','source_sha'):
                        self.assertEqual(lines[key],received[key])
                    self.assertEqual(state['status'],'BLOCKED')


if __name__=='__main__':unittest.main()
