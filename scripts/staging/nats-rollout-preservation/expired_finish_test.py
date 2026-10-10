"""Real root records/finish capability/ledger; K8s/native observation adapters."""
import copy
from contextlib import ExitStack
from datetime import datetime,timedelta,timezone
from pathlib import Path
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import Mock,patch
import transaction
import expired_finish as finish
import expired_transport as transport
from expired_recovery_test import RecoveryAuthorityTests
from stage_runtime import HUB


class LegacyBoundaryTests(unittest.TestCase):
    def test_cold_attempt_nonce_and_release_intent_are_ordinary_history(self):
        state={'closed_preservation_attempt_nonce':'a'*64,
            'events':[{'kind':'cold_preservation_verified_release_intent'}]}
        before=copy.deepcopy(state)
        finish.legacy_finish_guard(state)
        self.assertEqual(state,before)

    def test_actual_legacy_history_is_rejected_without_start_or_release(self):
        for kind in ('closed_preservation_sealed','closed_seal_journal_head'):
            state={'events':[{'kind':kind}]};before=copy.deepcopy(state)
            with self.subTest(kind=kind),self.assertRaisesRegex(ValueError,'legacy_postseal_unsupported'):
                finish.legacy_finish_guard(state)
            self.assertEqual(state,before)

    def test_legacy_postseal_runtime_and_manifest_are_explicitly_unsupported(self):
        engine=object.__new__(finish.Engine)
        engine.observation={'schema':'voice-expired-finish-postseal-proof-v1'}
        for method in (engine.postseal_runtime,engine.current_seal_manifest):
            with self.subTest(method=method.__name__),self.assertRaisesRegex(ValueError,'legacy_postseal_unsupported'):
                method()


class ActiveNativeFinishTests(unittest.TestCase):
    def test_real_nonexpired_finish_guards_each_resume_and_final_release(self):
        for expire in (False,True):
            with self.subTest(expire=expire),TemporaryDirectory() as folder,ExitStack() as stack:
                fixture=FinishCompositionTests();fixture.setup_files(folder)
                state=fixture.state;state['phase']='PAUSED_APPLY';state['status']='WAITING'
                deadline=fixture.now+timedelta(seconds=40)
                state['authorization']['expires_at']=deadline.isoformat()
                state['expired_recovery_authority']['expires_at']=deadline.isoformat()
                state['authorization']['expired_recovery_authority_sha256']=transport.upload.digest(state['expired_recovery_authority'])
                fixture.adapters(stack)
                class Clock(datetime):
                    @classmethod
                    def now(cls,tz=None):return fixture.clock[0]
                stack.enter_context(patch('transaction.dt.datetime',Clock))
                stack.enter_context(patch('expired_recovery.runtime_authority',return_value=(state['expired_recovery_authority'],{})))
                stack.enter_context(patch.object(transaction,'DockerRuntime'))
                stack.enter_context(patch.object(transaction,'verify_post_apply',return_value={'verified':True}))
                def restart():
                    self.assertIn('finish_guard',fixture.stage.__dict__)
                    self.assertIn('finish_authority_guard',fixture.stage.__dict__)
                    fixture.stage.finish_guard('scale',HUB,1)
                    fixture.stage.finish_guard('ready',HUB,None)
                    fixture.stage.finish_guard('release',None,None)
                    if expire:fixture.clock[0]=deadline
                    return {'verified':True}
                fixture.stage.restart.side_effect=restart
                receipt=copy.deepcopy(state['authorization'])
                if expire:
                    with self.assertRaises(__import__('controller').Blocked):transaction.finish(None,fixture.base,state,receipt,'owned-claim',{})
                    self.assertEqual(state['status'],'BLOCKED');self.assertEqual(state['fence_status'],'VERIFIED')
                    fixture.stage.refence.assert_called_once()
                else:
                    result=transaction.finish(None,fixture.base,state,receipt,'owned-claim',{})
                    self.assertEqual(result['status'],'PASS')


class FinishProducerActionsTests(unittest.TestCase):
    def test_actual_actions_pause_producer_dual_export_and_final_drift_truth(self):
        import bridge_root
        import hashlib
        for drift in (False,True):
            with self.subTest(drift=drift),TemporaryDirectory() as folder,ExitStack() as stack:
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
                stage.verified.side_effect=lambda:stage.save({'kind':'rollout_verified'})
                def observe(runtime,actual_stage,actual_state,slot,validate,checksum,interval):
                    row={'schema':'voice-expired-finish-current-observation-v1','operation':state['operation'],
                        'original_cut_sha256':transport.upload.digest(state['cut']),
                        'post_apply_cut_sha256':transport.upload.digest(state['nats_migration']['cut']),
                        'selected_inventory_sha256':'a'*64,'census':{'full':'postapply'},'native_messages':{'all':6}}
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


class FinishCommitTests(unittest.TestCase):
    def test_root_dual_readback_mints_finish_only_and_rejects_bound_record_drift(self):
        for drift in (None,'observation','pause','readback','checkpoint'):
            with self.subTest(drift=drift),TemporaryDirectory() as folder:
                fixture=FinishCompositionTests();fixture.setup_files(folder)
                base,slot,state,execution=fixture.base,fixture.slot,fixture.state,fixture.execution
                (slot/'authority.json').unlink();(slot/'proof.json').unlink()
                current={'schema':'voice-expired-finish-current-observation-v1','operation':state['operation'],
                    'original_cut_sha256':transport.upload.digest(state['cut']),
                    'post_apply_cut_sha256':transport.upload.digest(state['nats_migration']['cut']),
                    'selected_inventory_sha256':fixture.proof['selected_inventory_sha256'],
                    'manifest':{'archive_sha256':'a'*64,'files':[]},
                    'raw_source_sha256':'b'*64,
                    'census':{'full':'consumer state'},'native_messages':{'full':'all six records'}}
                pause={'schema':'voice-expired-finish-owned-pause-v1','operation':state['operation'],
                    'checkpoint_sha256':transport.upload.digest(state),'workloads':fixture.proof['applied_workloads']}
                source={'execution':execution,'target':state['target']['tag'],'binding':'current helper'}
                producer={'schema':'voice-expired-finish-observation-producer-v1','operation':state['operation'],
                    'started_at':fixture.proof['started_at'],'completed_at':fixture.now.isoformat(),
                    'cipher':fixture.proof['observation_cipher_binding'],
                    'original_checkpoint_sha256':__import__('hashlib').sha256(transport.upload.private_read(base/'checkpoint.json')).hexdigest(),
                    'observation_sha256':transport.upload.digest(current),'pause_sha256':transport.upload.digest(pause),
                    'paused_record_sha256':transport.upload.digest(pause),'paused_checkpoint_sha256':transport.upload.digest(state),
                    'source_sha256':transport.upload.digest(source)}
                record={'execution':execution,'producer_sha256':transport.upload.digest(producer),
                    'original_cipher_binding':state['cipher_binding'],'observation_cipher_binding':producer['cipher'],
                    'artifact_bindings':fixture.proof['artifact_bindings'],
                    'checkpoint_sha256':producer['original_checkpoint_sha256']}
                for name,row in {'rollout-before-manifest.json':current,'pause.json':pause,'source.json':source,
                    'observation-cipher.json':producer['cipher'],
                    'producer.json':producer,'upload.json':record,'verified-pause.json':pause,
                    'paused-checkpoint.json':state}.items():transport.put(slot/name,row)
                if drift=='observation':current['native_messages']={'missing':'a record'}
                elif drift=='pause':pause['workloads'][HUB]['replicas']=1
                elif drift=='readback':
                    row=transport.upload.private_json(slot/'original-readback.json');row['run_id']+=1
                    transaction.save(slot/'original-readback.json',row)
                elif drift=='checkpoint':state['events'].append({'kind':'unexpected progress'})
                if drift=='observation':transaction.save(slot/'rollout-before-manifest.json',current)
                if drift=='pause':transaction.save(slot/'pause.json',pause)
                with patch('expired_finish.datetime') as clock:
                    clock.now.return_value=fixture.now
                    if drift is None:
                        import bridge_root
                        actions=object.__new__(bridge_root.Actions);actions.code=Path(folder)/'code';actions.binding={}
                        fixture.expire=False
                        with ExitStack() as stack:
                            fixture.adapters(stack)
                            stack.enter_context(patch.object(actions,'_finish_source_admission',return_value=
                                (base,state,fixture.stage,copy.deepcopy(execution),{}, {},lambda:None)))
                            stack.enter_context(patch('expired_transport.readback'))
                            decrypt=stack.enter_context(patch.object(actions,'_verify_decrypted_restore'))
                            stack.enter_context(patch('guard.protected_receipt',return_value=copy.deepcopy(state['authorization'])))
                            summary=actions._authorize_expired_finish({'operation':state['operation'],'nonce':'f'*64,
                                'upload_nonce':execution['nonce'],'token':'fixture-only','original_artifact_id':10,
                                'observation_artifact_id':11})
                        result=transport.upload.private_json(slot/'authority.json')
                        self.assertEqual(decrypt.call_count,2)
                        self.assertEqual(result['purpose'],'finish-only')
                        self.assertEqual(result['expires_at'],fixture.authority['expires_at'])
                        self.assertEqual(summary['status'],'PASS')
                        self.assertEqual(transport.upload.private_json(slot/'checkpoint.json')['status'],'BLOCKED')
                        self.assertEqual(state['authorization'],fixture.proof and transport.upload.private_json(slot/'checkpoint.json')['authorization'])
                    else:
                        with self.assertRaises(__import__('expired_recovery').RecoveryError):
                            finish.commit_authority(base,state,slot,execution,clock=lambda:fixture.now)
                        self.assertFalse((slot/'authority.json').exists())


class FinishCompositionTests(unittest.TestCase):
    def setup_files(self,folder):
        prior=RecoveryAuthorityTests();prior.setUp();self.execution=copy.deepcopy(prior.execution)
        self.now=datetime.now(timezone.utc);self.clock=[self.now];self.execution['nonce']='e'*64
        template={'metadata':{'annotations':{}},'spec':{'containers':[{'name':'fixture','image':'pinned'}]}}
        snapshots={name:{'metadata':{'uid':name+'-uid','generation':1},'spec':{'replicas':0,'template':copy.deepcopy(template)}} for name in (HUB,'voice-story')}
        target=copy.deepcopy(prior.state['target']);target['template_hashes']={name:transport.upload.digest(template) for name in snapshots};target['manifest_sha256']='a'*64
        old={'execution':{'nonce':'b'*64},'expires_at':(self.now-timedelta(hours=1)).isoformat()}
        receipt={'operation':prior.state['operation'],'target':target,'expired_recovery_authority_sha256':transport.upload.digest(old),'expires_at':old['expires_at']}
        self.state={**copy.deepcopy(prior.state),'target':target,'authorization':receipt,'phase':'RESTART','status':'BLOCKED',
            'fence_status':'VERIFIED','events':[{'kind':'actual-applied-target-fixture'}],'provenance':{},'contract':{},
            'context':{'snapshots':snapshots,'expected':{'namespace_uid':'fixture-namespace','source_claim_uid':'fixture-pvc'}},
            'expired_recovery_authority':old,'nats_migration':{'verified':True,'cut':{'post':'applied'}}}
        self.base=Path(folder)/('rollout-'+prior.state['operation']);self.base.mkdir(mode=0o700)
        (Path(folder)/'installed').mkdir(mode=0o700)
        parent=self.base/'expired-finish-proofs';parent.mkdir(mode=0o700);self.slot=parent/self.execution['nonce'];self.slot.mkdir(mode=0o700)
        cipher={**self.state['cipher_binding'],'cipher_bytes':21,'cipher_sha256':'c'*64}
        bindings={role:{**(self.state['cipher_binding'] if role=='original' else cipher),
            'run_id':self.execution['dispatcher_run_id'],'head_sha':self.execution['head_sha'],'challenge':role} for role in ('original','observation')}
        self.proof={'schema':'voice-expired-finish-proof-v1','operation':self.state['operation'],
            'started_at':(self.now-timedelta(seconds=20)).isoformat(),'completed_at':self.now.isoformat(),
            'checkpoint_sha256':transport.upload.digest(self.state),'applied_receipt_sha256':transport.upload.digest(receipt),
            'post_apply_cut_sha256':transport.upload.digest(self.state['nats_migration']['cut']),
            'selected_inventory_sha256':'a'*64,'census_sha256':'b'*64,'native_messages_sha256':'c'*64,'source_admission_sha256':'d'*64,
            'applied_workloads':{name:{'uid':row['metadata']['uid'],'template_sha256':transport.upload.digest(template),
                'replicas':0,'generation':1,'pause_proof_sha256':'d'*64} for name,row in snapshots.items()},
            'original_cipher_binding':self.state['cipher_binding'],'observation_cipher_binding':cipher,'artifact_bindings':bindings}
        readbacks={role:{'schema':'voice-nats-custody-v1','destination':'github:Poryadok/VoiceRoot',
            **{key:bindings[role][key] for key in ('operation','challenge','run_id','head_sha','cipher_sha256','cipher_bytes')},
            'artifact_id':number,'verified':True} for number,role in enumerate(('original','observation'),10)}
        self.authority=finish.issue(self.state,self.execution,self.proof,readbacks,now=self.now)
        rows={'checkpoint.json':self.state,'proof.json':self.proof,'authority.json':self.authority,
            **{role+'-readback.json':row for role,row in readbacks.items()}}
        for name,row in rows.items():transport.put(self.slot/name,row)
        transport.put(self.base/'checkpoint.json',self.state)
        self.stage=Mock();self.stage.snapshots=copy.deepcopy(snapshots);self.stage.marker={'metadata':{'resourceVersion':'fixture-rv'},'data':{'phase':'rollout-verified'}}
        self.stage.kube.get.side_effect=lambda kind,name:copy.deepcopy(self.stage.snapshots[name])
        def refence():
            for row in self.stage.snapshots.values():row['spec']['replicas']=0
            return {'verified':True}
        self.stage.refence.side_effect=refence;self.stage.final_path=Path('/selected-fixture')
        def restart(ledger):
            for name in (HUB,'voice-story'):
                ledger.guard('scale',name,1);row=self.stage.snapshots[name];row['spec']['replicas']=1
                row['status']={'observedGeneration':1,'readyReplicas':1,'updatedReplicas':1,'availableReplicas':1}
                ledger.guard('ready',name)
                if self.expire and name==HUB:self.clock[0]+=timedelta(seconds=600)
            ledger.guard('release');return {'verified':True}
        self.stage.restart_missing.side_effect=restart

    def adapters(self,stack):
        owner=self
        class Clock(datetime):
            @classmethod
            def now(cls,tz=None):return owner.clock[0]
        for adapter in (patch('guard.ROOT',self.base.parent),patch('root_cli.code_binding',return_value={}),
            patch('bridge_root.INSTALLED',self.base.parent/'installed'),
            patch('expired_recovery.verify_adopted_binding'),patch.object(transaction,'context',side_effect=lambda stage:copy.deepcopy(self.state['context'])),
            patch.object(transaction,'reconstruct',return_value=self.stage),patch.object(transaction,'revalidate_inputs'),
            patch('expired_proof.current_inventory',return_value='a'*64),patch('expired_finish.datetime',Clock)):
            stack.enter_context(adapter)

    def test_expired_apply_receipt_real_root_finish_capability_has_no_application_replay(self):
        with TemporaryDirectory() as folder,ExitStack() as stack:
            self.setup_files(folder);self.expire=False;self.adapters(stack);receipt=copy.deepcopy(self.state['authorization'])
            with patch.object(transaction,'authorize') as authorize,patch.object(transaction,'apply_contract') as migrate:
                result=transaction.finish(None,self.base,self.state,receipt,'unused',{},finish_authority=self.authority)
            self.assertEqual(result['status'],'PASS');self.assertEqual(result['authorization'],receipt)
            authorize.assert_not_called();migrate.assert_not_called();self.stage.adopt_applied_target.assert_not_called()
            self.stage.refence.assert_not_called();ledger=transport.upload.private_json(self.slot/'resume-ledger.json')
            self.assertEqual(ledger['pending'],{});self.assertEqual(set(ledger['completed']),{HUB,'voice-story'})
            self.assertEqual(sum(event['kind']=='expired_finish_resume_progress' for event in result['events']),4)

    def test_expiry_after_first_resume_truthfully_blocks_and_refences(self):
        with TemporaryDirectory() as folder,ExitStack() as stack:
            self.setup_files(folder);self.expire=True;self.adapters(stack)
            with self.assertRaises(ValueError):transaction.finish(None,self.base,self.state,self.state['authorization'],'unused',{},finish_authority=self.authority)
            self.assertEqual(self.state['status'],'BLOCKED');self.assertEqual(self.state['fence_status'],'VERIFIED')
            self.stage.refence.assert_called_once();ledger=transport.upload.private_json(self.slot/'resume-ledger.json')
            self.assertEqual(ledger['completed'],{});self.assertEqual(set(ledger['pending']),{HUB,'voice-story'})
            self.assertTrue(ledger['pending'][HUB]['requires_new_complete_proof'])
            self.assertTrue((self.slot/'safety-pause.json').exists())
            self.assertFalse((self.base/'apply-authorization.json').exists())
            self.assertEqual(self.stage.snapshots[HUB]['spec']['replicas'],0)
            self.assertTrue(any(event.get('kind')=='expired_finish_resume_progress' for event in self.state['events']))

    def test_proof_readback_drift_and_ordinary_expiry_reject_before_resume(self):
        for drift in ('proof','readback','ordinary'):
            with self.subTest(drift=drift),TemporaryDirectory() as folder,ExitStack() as stack:
                self.setup_files(folder);self.expire=False;self.adapters(stack)
                if drift!='ordinary':
                    path=self.slot/('proof.json' if drift=='proof' else 'original-readback.json');row=transport.upload.private_json(path)
                    row['native_messages_sha256' if drift=='proof' else 'cipher_sha256']='f'*64;path.unlink();transport.put(path,row)
                else:self.state.update(phase='PAUSED_APPLY',status='WAITING')
                with self.assertRaises((ValueError,transaction.Blocked)):
                    transaction.finish(None,self.base,self.state,self.state['authorization'],'unused',{},**({} if drift=='ordinary' else {'finish_authority':self.authority}))
                self.stage.restart_missing.assert_not_called();self.assertFalse((self.slot/'resume-ledger.json').exists())


if __name__=='__main__':unittest.main()


class MissingResumeSourceTests(unittest.TestCase):
    def fixture(self,completed=()):
        import runtime_stage
        import expired_recovery
        from stage_runtime import LEAVES
        names={HUB,'voice-gateway',*('voice-'+name for name in LEAVES),'voice-story'}
        stage=object.__new__(runtime_stage.RolloutStage)
        stage.operation=transport.upload.OPERATION;stage.expected={'source_claim':'pvc','generation':'generation'}
        stage.final_claim={'metadata':{'name':'pvc'}}
        stage.marker={'metadata':{'uid':'marker','resourceVersion':'1'},
            'data':{'phase':'rollout-verified','knownRolloutOperation':stage.operation,'dataPVC':'pvc'}}
        stage.snapshots={name:{'metadata':{'uid':name+'-uid','generation':1},
            'spec':{'replicas':int(name in completed),'template':{'metadata':{'annotations':{}},
                'spec':{'containers':[{'name':'user' if name=='voice-user' else 'service','env':[]}]}}},
            'status':{'observedGeneration':1,'readyReplicas':1,'updatedReplicas':1,'availableReplicas':1}}
            for name in names}
        rows=copy.deepcopy(stage.snapshots);marker=[copy.deepcopy(stage.marker)];self.patches=[];self.events=[];self.expired=False;self.closed_calls=0
        def get(kind,name):
            if kind=='deployment':return copy.deepcopy(rows[name])
            if name=='voice-app-config':return {'data':{'SPACE_GRPC_ADDR':'voice-space:50051'}}
            if name=='voice-nats-generation':return copy.deepcopy(marker[0])
            raise AssertionError((kind,name))
        def cas(kind,current,patches):
            self.patches.append((kind,current.get('metadata',{}).get('uid'),copy.deepcopy(patches)))
            row=copy.deepcopy(current)
            for patch_row in patches:
                parts=patch_row['path'].strip('/').split('/');parts=[s.replace('~1','/') for s in parts]
                parent=row
                for part in parts[:-1]:parent=parent[int(part)] if isinstance(parent,list) else parent[part]
                key=parts[-1];value=patch_row.get('value')
                if patch_row['op']=='test':self.assertEqual(parent[int(key)] if isinstance(parent,list) else parent[key],value)
                elif patch_row['op']=='remove':del parent[key]
                elif key=='-':parent.append(copy.deepcopy(value))
                else:parent[key]=copy.deepcopy(value)
            if kind=='deployment':rows[next(name for name,value in rows.items() if value['metadata']['uid']==row['metadata']['uid'])]=row
            else:row['metadata']['resourceVersion']=str(int(row['metadata']['resourceVersion'])+1);marker[0]=row
            return copy.deepcopy(row)
        stage.kube=Mock();stage.kube.get.side_effect=get;stage.kube.cas.side_effect=cas
        stage.save=self.events.append;stage.verify_selected_storage_identity=Mock()
        # This fixture tests actual resume/ledger/CAS behavior. Native/CMS
        # admission is independently exercised by the cold-route proof tests.
        stage.closed_preservation_producer=Mock()
        stage.closed_preservation_producer.seal_before_resume.return_value={'fixture_proof':'adapter'}
        stage.closed_preservation_runtime=Mock()
        stage.no_pods=Mock();stage.verify_closed=Mock()
        def authority():
            if self.expired:raise ValueError('expired')
        def closed():
            self.closed_calls+=1;self.assertTrue(all(row['spec']['replicas']==0 for row in rows.values()))
        def running(name):self.assertEqual(rows[name]['spec']['replicas'],1)
        obligations={name:{'uid':row['metadata']['uid'],'template_sha256':transport.upload.digest(row['spec']['template'])} for name,row in rows.items()}
        ledger=expired_recovery.FinishLedger({'schema':'voice-expired-finish-ledger-v1','operation':stage.operation,
            'authority_sha256':'a'*64,'pending':{name:row for name,row in obligations.items() if name not in completed},
            'completed':{name:row for name,row in obligations.items() if name in completed},'nats_started':HUB in completed},
            authority,closed,running,lambda row:self.events.append({'kind':'ledger','record':row}))
        stage.finish_guard=ledger.guard;stage.finish_ledger=ledger;stage.finish_authority_guard=authority
        return stage,ledger,rows,marker

    def test_actual_stage_cycle_restored_and_each_missing_workload_scaled_once(self):
        stage,ledger,rows,marker=self.fixture();templates={name:copy.deepcopy(row['spec']['template']) for name,row in rows.items()}
        result=stage.restart_missing(ledger)
        self.assertTrue(result['finish_only']);self.assertEqual(ledger.record['pending'],{});self.assertEqual(self.closed_calls,1)
        self.assertTrue(all(row['spec']['template']==templates[name] for name,row in rows.items()))
        scales=[uid for kind,uid,patches in self.patches if kind=='deployment' and any(p['path']=='/spec/replicas' and p['op']=='add' for p in patches)]
        self.assertEqual(len(scales),len(rows));self.assertEqual(len(set(scales)),len(rows))
        self.assertEqual(marker[0]['data']['phase'],'active')

    def test_running_nats_and_completed_user_observed_without_scale_or_override_replay(self):
        stage,ledger,rows,marker=self.fixture((HUB,'voice-user'));stage.restart_missing(ledger)
        scales=[uid for kind,uid,patches in self.patches if any(p['path']=='/spec/replicas' for p in patches)]
        self.assertNotIn(HUB+'-uid',scales);self.assertNotIn('voice-user-uid',scales)
        self.assertEqual(self.closed_calls,0)
        self.assertFalse(any(event.get('kind')=='user_cycle_override' for event in self.events))

    def test_deadline_after_release_cas_is_checked_and_not_reported_as_success(self):
        stage,ledger,rows,marker=self.fixture();original_cas=stage.kube.cas.side_effect
        def cas(kind,row,patches):
            result=original_cas(kind,row,patches)
            if kind=='configmap' and result['data']['phase']=='active':self.expired=True
            return result
        stage.kube.cas.side_effect=cas
        with self.assertRaises(ValueError):stage.restart_missing(ledger)
        self.assertEqual(marker[0]['data']['phase'],'active') # Truthful CAS fact retained before safety refence.
        self.assertTrue(any(event.get('kind')=='rollout_released' for event in self.events))
        self.assertTrue(stage.refence()['verified'])
        self.assertEqual(marker[0]['data']['phase'],'rollout-verified')
        self.assertTrue(all(row['spec']['replicas']==0 for row in rows.values()))
        self.assertTrue(any(event.get('kind')=='expired_finish_release_refenced' for event in self.events))

    def test_release_ownership_drift_prevents_reacquisition_or_any_scale(self):
        stage,ledger,rows,marker=self.fixture();stage.restart_missing(ledger)
        marker[0]['metadata']['resourceVersion']='unowned';count=len(self.patches)
        with self.assertRaises(transaction.Blocked):stage.refence()
        self.assertEqual(len(self.patches),count)


class FinishObservationTests(unittest.TestCase):
    def test_original_postapply_current_copies_share_full_record_proof_and_do_not_replace_cut(self):
        with TemporaryDirectory() as folder:
            base=Path(folder)/('rollout-'+transport.upload.OPERATION);base.mkdir(mode=0o700)
            slot=base/'expired-finish-proofs'/('a'*64);slot.mkdir(parents=True,mode=0o700)
            census={'account':'account','streams':[],'consumers':[]}
            manifest={'mechanism':'fixture','file_count':0,'bytes':0,'dirs':[],'files':[],'archive_sha256':'a'*64}
            cut={'manifest':manifest,'census':census,'census_sha256':transport.upload.digest(census),
                'manifest_sha256':transport.upload.digest(manifest),'archive':str(base/'post.tar')}
            state={'operation':transport.upload.OPERATION,'cut':copy.deepcopy(cut),'nats_migration':{'verified':True,'cut':copy.deepcopy(cut)}}
            original=copy.deepcopy(state);runtime=Mock();runtime.base=base;stage=Mock();stage.final_path=Path('/selected')
            observed={'census':census,'native_messages':{'sha256':'b'*64,'messages':6},'closed_durable_states':{'exact':'all pending fields'},'recovery':{'server_image':'pinned','started_at':'start','finished_at':'end'}}
            calls=[]
            def copy_observation(*args,**kwargs):
                self.assertEqual(kwargs,{'native_only':True})
                calls.append(args[1]);return copy.deepcopy(observed)
            with patch('native_store.verify_archive',return_value=True),patch('native_store.archive_closed_store',return_value=manifest),\
                 patch('expired_proof.observe_copy',side_effect=copy_observation),patch('preserve.recovered_census',side_effect=lambda before,after,*args:(after,[])),\
                 patch('expired_proof.current_inventory',return_value=transport.upload.digest({key:manifest[key] for key in ('mechanism','file_count','bytes','dirs','files')})):
                result=finish.observe_post_apply(runtime,stage,state,slot,lambda *args:None,lambda *args:b'',None)
            self.assertEqual(len(calls),3);self.assertEqual(state,original)
            self.assertEqual(result['native_messages'],observed['native_messages']);self.assertEqual(result['native_disposition']['branch'],'EXACT')
            self.assertTrue((slot/'rollout-before-manifest.json').exists())

    def test_changed_current_record_proof_rejects_without_observation_commit(self):
        with TemporaryDirectory() as folder:
            base=Path(folder)/('rollout-'+transport.upload.OPERATION);base.mkdir(mode=0o700)
            slot=base/'expired-finish-proofs'/('a'*64);slot.mkdir(parents=True,mode=0o700)
            census={'account':'account','streams':[],'consumers':[]};manifest={'mechanism':'fixture','file_count':0,'bytes':0,'dirs':[],'files':[],'archive_sha256':'a'*64}
            cut={'manifest':manifest,'census':census,'census_sha256':transport.upload.digest(census),'archive':str(base/'post.tar')}
            runtime=Mock();runtime.base=base;stage=Mock();stage.final_path=Path('/selected')
            state={'operation':transport.upload.OPERATION,'cut':cut,'nats_migration':{'verified':True,'cut':cut}}
            rows=[{'census':census,'native_messages':{'sha256':'a'*64},'closed_durable_states':{'exact':'all pending fields'},'recovery':{}} for _ in range(3)];rows[-1]['native_messages']={'sha256':'b'*64}
            with patch('native_store.verify_archive',return_value=True),patch('native_store.archive_closed_store',return_value=manifest),\
                 patch('expired_proof.observe_copy',side_effect=rows),patch('preserve.recovered_census',side_effect=lambda before,after,*args:(after,[])),self.assertRaises(ValueError):
                finish.observe_post_apply(runtime,stage,state,slot,lambda *args:None,lambda *args:b'',None)
            self.assertFalse((slot/'rollout-before-manifest.json').exists())


class FinishPauseSourceTests(unittest.TestCase):
    def test_actual_owned_runtime_pause_new_obligations_and_unowned_template_veto(self):
        for changed in (False,True):
            with self.subTest(changed=changed),TemporaryDirectory() as folder:
                fixture=MissingResumeSourceTests();stage,ledger,rows,marker=fixture.fixture()
                for row in rows.values():row['spec']['replicas']=1
                stage.snapshots=copy.deepcopy(rows)
                target={'tag':'73ca52699ddf6a9182e3407d5bbdedbc29c06dee',
                    'template_hashes':{name:transport.upload.digest(row['spec']['template']) for name,row in rows.items()}}
                state={'operation':transport.upload.OPERATION,'phase':'RESTART','status':'BLOCKED',
                    'target':target,'authorization':{'operation':transport.upload.OPERATION,'target':target},
                    'context':{'snapshots':copy.deepcopy(rows)},'events':[]}
                base=Path(folder)/('rollout-'+state['operation']);base.mkdir(mode=0o700)
                slot=base/'expired-finish-proofs'/('e'*64);slot.mkdir(parents=True,mode=0o700)
                transport.put(base/'checkpoint.json',state)
                def journal(event):state['events'].append(event)
                if changed:rows['voice-story']['spec']['template']['unknown']='unowned'
                if changed:
                    with self.assertRaises(ValueError):finish.pause_owned(stage,base,state,slot,journal,lambda:None)
                    self.assertEqual(fixture.patches,[])
                else:
                    with patch.object(transaction,'context',side_effect=lambda actual:{'snapshots':copy.deepcopy(actual.snapshots)}):
                        result=finish.pause_owned(stage,base,state,slot,journal,lambda:None)
                    self.assertTrue(all(row['spec']['replicas']==0 for row in rows.values()))
                    self.assertEqual(set(result['workloads']),set(rows))
                    self.assertEqual(state['authorization']['target'],target)
                    self.assertTrue((slot/'pause.json').exists())
                    self.assertEqual(state['events'][0]['kind'],'expired_finish_owned_pause_entered')

    def test_exact_owned_temporary_cycle_is_restored_only_after_zero_replica_pause(self):
        with TemporaryDirectory() as folder:
            fixture=MissingResumeSourceTests();stage,ledger,rows,marker=fixture.fixture()
            target={'tag':transport.upload.ORIGINAL_TARGET,'template_hashes':{name:transport.upload.digest(row['spec']['template']) for name,row in rows.items()}}
            original=copy.deepcopy(rows['voice-user']['spec']['template'])
            for row in rows.values():row['spec']['replicas']=1
            temporary=rows['voice-user']['spec']['template']
            temporary['metadata']['annotations']['voice.io/nats-user-space-bootstrap']=stage.operation
            temporary['spec']['containers'][0]['env'].append({'name':'SPACE_GRPC_ADDR','value':''})
            stage.snapshots=copy.deepcopy(rows)
            state={'operation':stage.operation,'phase':'RESTART','status':'BLOCKED','target':target,
                'authorization':{'operation':stage.operation,'target':target},'context':{'snapshots':copy.deepcopy(rows)},
                'events':[{'kind':'user_cycle_override','owner':stage.operation}]}
            base=Path(folder)/('rollout-'+state['operation']);base.mkdir(mode=0o700)
            slot=base/'expired-finish-proofs'/('e'*64);slot.mkdir(parents=True,mode=0o700);transport.put(base/'checkpoint.json',state)
            with patch.object(transaction,'context',side_effect=lambda actual:{'snapshots':copy.deepcopy(actual.snapshots)}):
                finish.pause_owned(stage,base,state,slot,state['events'].append,lambda:None)
            self.assertEqual(rows['voice-user']['spec']['template'],original)
            self.assertTrue(all(row['spec']['replicas']==0 for row in rows.values()))
            self.assertEqual(state['fence_status'],'VERIFIED')
            self.assertTrue(any(event['kind']=='expired_finish_owned_cycle_restored' for event in state['events']))


class NativeRestartCASTests(unittest.TestCase):
    def test_inherited_user_template_cas_each_checks_dedicated_deadline_and_restores_wrapper(self):
        for expire_at in (None,1,2):
            with self.subTest(expire_at=expire_at):
                helper=MissingResumeSourceTests();stage,ledger,rows,marker=helper.fixture()
                original_kube=stage.kube;get=original_kube.get.side_effect
                stage.final_claim['metadata']['uid']='owned-pvc'
                def read(kind,name):
                    if kind=='pvc':return {'metadata':{'uid':'owned-pvc'}}
                    return get(kind,name)
                original_kube.get.side_effect=read
                stage.scale=Mock();stage.wait_ready=Mock();stage.release_marker=Mock()
                stage.owned_marker=Mock(return_value=marker[0])
                calls=[]
                def deadline():
                    calls.append('check')
                    if len(calls)==expire_at:raise ValueError('synthetic deadline')
                stage.finish_authority_guard=deadline
                # Isolate the inherited template CAS deadline from the earlier
                # independently covered cold-proof/arm deadline boundary.
                stage.seal_before_business_resume=Mock(return_value={'fixture_proof':'adapter'})
                if expire_at is None:
                    result=stage.restart();self.assertTrue(result['user_cycle_restored'])
                    self.assertEqual(len(calls),2);self.assertEqual(len(helper.patches),2)
                    self.assertEqual(rows['voice-user']['spec']['template']['spec']['containers'][0]['env'],[])
                    stage.release_marker.assert_called_once()
                else:
                    with self.assertRaises(ValueError):stage.restart()
                    self.assertEqual(len(helper.patches),expire_at-1)
                    stage.release_marker.assert_not_called()
                    self.assertEqual(sum(event.get('kind')=='user_cycle_override' for event in helper.events),expire_at-1)
                self.assertIs(stage.kube,original_kube)
