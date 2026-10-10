import copy
from datetime import datetime,timezone,timedelta
import hashlib
import json
import os
from pathlib import Path
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch
import expired_transport as transport
from expired_recovery_test import RecoveryAuthorityTests


class TransportTests(unittest.TestCase):
    def setup_files(self,folder):
        prior=RecoveryAuthorityTests();prior.setUp()
        self.state=prior.state;self.execution=prior.execution;self.now=datetime.now(timezone.utc)
        self.state['events']=[] # Synthetic adoption prefix; native producer proof is a separate fixture.
        self.state['contract']={} # Transaction is a declared external seam in this Actions orchestration control.
        self.state.update(custody={'verified':True},nats_contract={'schema':'trusted-test-plan'},
            migrations=[],migration_secret_metadata={},provenance={})
        self.state['target']['migration_sha256']=transport.upload.digest([])
        self.state['context']={'marker':{'data':{'phase':'rollout-capturing'}}}
        self.base=Path(folder)/('rollout-'+self.state['operation']);self.base.mkdir(mode=0o750)
        os.chown(self.base,0,1000)
        self.slot=self.base/'expired-recovery-proofs'/self.execution['nonce'];self.slot.mkdir(parents=True,mode=0o700)
        original=b'synthetic original encrypted bytes';observed=b'synthetic observation encrypted bytes'
        self.state['cipher_binding'].update(cipher_bytes=len(original),cipher_sha256=hashlib.sha256(original).hexdigest())
        self.cipher={**self.state['cipher_binding'],'cipher_bytes':len(observed),
            'cipher_sha256':hashlib.sha256(observed).hexdigest(),'run_id':self.execution['dispatcher_run_id'],
            'head_sha':self.execution['head_sha']}
        for file,row in ((self.base/'checkpoint.json',self.state),
                         (self.base/'expired-recovery-adopted-checkpoint.json',self.state)):
            transport.put(file,row)
        for file,raw in ((self.base/'rollout-backup.cms',original),(self.slot/'rollout-backup.cms',observed)):
            file.write_bytes(raw);file.chmod(0o600)
        observation={'selected_inventory_sha256':'a'*64,'native_disposition':{'branch':'EXACT'},
            'original_cut_sha256':transport.upload.digest(self.state['cut'])}
        transport.put(self.slot/'rollout-before-manifest.json',observation)
        transport.put(self.slot/'producer.json',{'cipher':self.cipher,'operation':self.state['operation'],
            'original_checkpoint_sha256':hashlib.sha256((self.base/'checkpoint.json').read_bytes()).hexdigest(),
            'observation_sha256':transport.upload.digest(observation),
            'started_at':(self.now-timedelta(seconds=30)).isoformat()})
        for name in ('admission.json','source.json'):transport.put(self.slot/name,{'bound':name})
        transport.put(self.base/'expired-recovery-input-repair.json',{'bound':'fixed-input-repair'})

    def test_dual_export_readback_new_authority_preserve_original_checkpoint_and_cipher(self):
        with TemporaryDirectory() as folder:
            self.setup_files(folder);checkpoint=(self.base/'checkpoint.json').read_bytes()
            record,paths=transport.export(self.base,self.state,self.slot,self.execution,self.cipher)
            self.assertEqual(set(paths),{'original','observation'})
            self.assertEqual(record['original_cipher_binding'],self.state['cipher_binding'])
            for role,path in paths.items():
                row=Path(path).lstat();self.assertEqual((row.st_uid,row.st_gid,row.st_mode&0o777),(0,1000,0o440))
                self.assertEqual(Path(path).read_bytes(),(self.base/'rollout-backup.cms' if role=='original'
                    else self.slot/'rollout-backup.cms').read_bytes())
            calls=[]
            def verify(token,binding,source,artifact):
                calls.append((source,artifact))
                return {'schema':'voice-nats-custody-v1','destination':'github:Poryadok/VoiceRoot',
                    **{key:binding[key] for key in ('operation','challenge','run_id','head_sha','cipher_sha256','cipher_bytes')},
                    'artifact_id':artifact,'verified':True}
            with patch('github_custody.verify_artifact',side_effect=verify):
                transport.readback('synthetic-token',self.base,self.state,self.slot,self.execution,
                    {'original':11,'observation':12},clock=lambda:self.now)
            self.assertEqual(calls,[(self.base/'rollout-backup.cms',11),(self.slot/'rollout-backup.cms',12)])
            authority=transport.commit_authority(self.base,self.state,self.slot,self.execution,clock=lambda:self.now)
            self.assertEqual(authority['original_expires_at'],self.state['nats_contract_expires_at'])
            self.assertEqual(authority['expires_at'],(self.now+timedelta(seconds=570)).isoformat())
            self.assertEqual((self.base/'checkpoint.json').read_bytes(),checkpoint)
            with self.assertRaises(FileExistsError):transport.export(self.base,self.state,self.slot,self.execution,self.cipher)

    def test_readback_identity_and_mid_download_deadline_veto_without_authority(self):
        for wrong in ('run','artifact','deadline'):
            with self.subTest(wrong=wrong),TemporaryDirectory() as folder:
                self.setup_files(folder)
                transport.export(self.base,self.state,self.slot,self.execution,self.cipher)
                execution=copy.deepcopy(self.execution);ids={'original':11,'observation':12};now=[self.now]
                if wrong=='run':execution['dispatcher_run_id']+=1
                if wrong=='artifact':ids['observation']=11
                def verify(*args):now[0]+=timedelta(seconds=600);return {'verified':True}
                with patch('github_custody.verify_artifact',side_effect=verify),self.assertRaises(ValueError):
                    transport.readback('synthetic-token',self.base,self.state,self.slot,execution,ids,clock=lambda:now[0])
                self.assertFalse((self.slot/'authority.json').exists())
                self.assertFalse((self.slot/'original-readback.json').exists())

    def test_actual_actions_readback_root_authority_and_transaction_consumer_binding(self):
        import bridge_root
        from unittest.mock import Mock
        with TemporaryDirectory() as folder:
            self.setup_files(folder)
            record,_=transport.export(self.base,self.state,self.slot,self.execution,self.cipher)
            actions=object.__new__(bridge_root.Actions);actions.binding={};actions.code=Path(folder)/'code'
            state=copy.deepcopy(self.state);calls=[]
            def verify(token,binding,source,artifact):
                return {'schema':'voice-nats-custody-v1','destination':'github:Poryadok/VoiceRoot',
                    **{key:binding[key] for key in ('operation','challenge','run_id','head_sha','cipher_sha256','cipher_bytes')},
                    'artifact_id':artifact,'verified':True}
            def authorize(kube,base,actual,archive,manifest,contract,**keywords):
                calls.append(keywords)
                self.assertEqual(actual['cipher_binding'],self.state['cipher_binding'])
                self.assertEqual(keywords['expired_authority']['execution'],self.execution)
                for name in ('apply-authorization.json','apply-manifests.json'):
                    (base/name).write_bytes(b'synthetic-transaction-output')
            request={'operation':state['operation'],'nonce':'e'*64,'upload_nonce':self.execution['nonce'],
                'token':'synthetic-token','original_artifact_id':11,'observation_artifact_id':12}
            source_tuple=(self.base,state,Mock(),copy.deepcopy(self.execution),{}, {},lambda:None)
            with patch.object(actions,'_expired_source_admission',return_value=source_tuple),\
                 patch('github_custody.verify_artifact',side_effect=verify),\
                 patch('guard.ROOT',self.base.parent),patch('root_cli.code_binding',return_value={}),\
                 patch('expired_recovery.verify_adopted_binding'),\
                 patch('transaction.authorize',side_effect=authorize),patch('bridge_root.grp.getgrnam',return_value=Mock(gr_gid=1000)):
                result=actions._authorize_expired_native(request)
            self.assertEqual(len(calls),1)
            self.assertEqual(result['operation'],self.state['operation'])
            self.assertEqual(calls[0]['authorization_deadline'].isoformat(),calls[0]['expired_authority']['expires_at'])


class TransactionCompositionTests(unittest.TestCase):
    setup_files=TransportTests.setup_files
    def test_actual_transaction_preserves_prospective_entry_and_refences_after_prepared_expiry(self):
        from contextlib import ExitStack
        from unittest.mock import Mock
        import transaction
        for expire in (False,True):
            with self.subTest(expire=expire),TemporaryDirectory() as folder,ExitStack() as stack:
                self.setup_files(folder)
                record,_=transport.export(self.base,self.state,self.slot,self.execution,self.cipher)
                def verify(token,binding,source,artifact):
                    return {'schema':'voice-nats-custody-v1','destination':'github:Poryadok/VoiceRoot',
                        **{key:binding[key] for key in ('operation','challenge','run_id','head_sha','cipher_sha256','cipher_bytes')},
                        'artifact_id':artifact,'verified':True}
                with patch('github_custody.verify_artifact',side_effect=verify):
                    transport.readback('synthetic-token',self.base,self.state,self.slot,self.execution,
                        {'original':11,'observation':12},clock=lambda:self.now)
                authority=transport.commit_authority(self.base,self.state,self.slot,self.execution,clock=lambda:self.now)
                stage=Mock();stage.final_path=Path('/selected-fixture-store')
                stage.expected={'namespace_uid':'namespace'}
                stage.marker={'metadata':{'uid':'marker','resourceVersion':'7'},
                    'data':{'generation':'generation','previousGeneration':'previous','dataPVC':'same-pvc'}}
                stage.final_claim={'metadata':{'name':'same-pvc','uid':'pvc'},'spec':{'volumeName':'same-pv'}}
                stage.final_pv={'metadata':{'uid':'pv'},'spec':{'local':{}}}
                stage.refence.return_value={'verified':True}
                current=[self.now];clock=self
                class Clock(datetime):
                    @classmethod
                    def now(cls,tz=None):return current[0]
                context=copy.deepcopy(self.state['context']);order=[]
                def migration(base,state,actual,journal,**kwargs):
                    kwargs['before_selected_start']();order.append('selected-start-authorized')
                    journal({'kind':'fixture-native-mutation-completed'})
                    return {'verified':True,'cut':copy.deepcopy(state['cut'])}
                def prepared():
                    order.append('prepared')
                    if expire:current[0]+=timedelta(seconds=600)
                stage.prepared.side_effect=prepared
                adapters=[patch('guard.ROOT',self.base.parent),patch('root_cli.code_binding',return_value={}),
                    patch('expired_recovery.verify_adopted_binding'),patch.object(transaction,'reconstruct',return_value=stage),
                    patch.object(transaction,'context',return_value=context),patch.object(transaction,'file_sha',return_value=self.state['cut']['manifest_sha256']),
                    patch.object(transaction,'verify_archive',return_value=True),patch.object(transaction,'revalidate_inputs'),
                    patch.object(transaction,'DockerRuntime'),patch('expired_proof.current_inventory',return_value='a'*64),
                    patch.object(transaction,'apply_contract',side_effect=migration),patch('nats_contract_custody.advance',return_value={}),
                    patch.object(transaction.migrations,'execute',return_value=[]),patch.object(transaction.space_safeguards,'before_migrate'),
                    patch.object(transaction.space_safeguards,'after_migrate',return_value=None),patch.object(transaction.dt,'datetime',Clock)]
                for adapter in adapters:stack.enter_context(adapter)
                installed=self.base.parent/'installed';installed.mkdir(mode=0o700)
                arguments=(None,self.base,self.state,self.state['cut']['manifest']['archive_sha256'],self.state['cut']['manifest_sha256'],{})
                keywords={'authorization_deadline':datetime.fromisoformat(authority['expires_at']),'expired_authority':authority}
                if expire:
                    with self.assertRaises(ValueError):transaction.authorize(*arguments,**keywords)
                    self.assertEqual(self.state['status'],'BLOCKED');self.assertEqual(self.state['fence_status'],'VERIFIED')
                    self.assertFalse((self.base/'apply-authorization.json').exists());stage.refence.assert_called_once()
                    self.assertIn('nats_migration',self.state)
                else:
                    result=transaction.authorize(*arguments,**keywords)
                    self.assertEqual(self.state['phase'],'PAUSED_APPLY');self.assertEqual(result['expires_at'],authority['expires_at'])
                    stage.refence.assert_not_called()
                self.assertEqual(order,['selected-start-authorized','prepared'])
                self.assertEqual(sum(event['kind']=='expired_recovery_transaction_entered' for event in self.state['events']),1)
                self.assertIn({'kind':'fixture-native-mutation-completed'},self.state['events'])
                self.assertEqual(self.state['nats_contract_expires_at'],authority['original_expires_at'])


if __name__=='__main__':unittest.main()
