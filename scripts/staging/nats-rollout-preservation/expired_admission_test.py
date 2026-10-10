import copy
import hashlib
import json
from pathlib import Path
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch
import expired_admission as admission
import expired_recovery as recovery


class FinishAdmissionTests(unittest.TestCase):
    def test_completed_application_observation_adopts_only_exact_owned_target_without_checkpoint_write(self):
        from unittest.mock import Mock
        from contextlib import ExitStack
        from datetime import datetime,timezone
        import transaction
        from preserved_upload import digest
        for drift in (None,'uid','template','scale','marker','checkpoint'):
            with self.subTest(drift=drift),TemporaryDirectory() as folder,ExitStack() as stack:
                base=Path(folder);kernel=b'original kernel';manifests=b'actual root target manifests'
                (base/'kernel').write_bytes(kernel);(base/'apply-manifests.json').write_bytes(manifests)
                template={'spec':{'containers':[{'name':'fixture','image':'target@sha256:fixed'}]}}
                captured={'metadata':{'uid':'owned-workload','resourceVersion':'old'},'spec':{'replicas':0,'template':copy.deepcopy(template)}}
                actual=copy.deepcopy(captured);actual['metadata']['resourceVersion']='new';actual['spec']['replicas']=1
                old_marker={'metadata':{'uid':'marker-uid','resourceVersion':'prepared-rv'},'data':{'phase':'rollout-prepared'}}
                marker={'metadata':{'uid':'marker-uid','resourceVersion':'applying-rv'},'data':{'phase':'rollout-applying',
                    'knownRolloutOperation':'3340764a7d24','generation':'current','previousGeneration':'old','dataPVC':'owned-pvc'}}
                target={'manifest_sha256':hashlib.sha256(manifests).hexdigest(),'mode':'images-only',
                    'template_hashes':{'voice-story':digest(template)}}
                authority={'issued_at':datetime.now(timezone.utc).isoformat()}
                receipt={'operation':'3340764a7d24','target':target,'expired_recovery_authority_sha256':digest(authority),
                    'marker':{'uid':'marker-uid','resourceVersion':'prepared-rv','generation':'current','previousGeneration':'old','dataPVC':'owned-pvc'}}
                state={'operation':'3340764a7d24','phase':'PAUSED_APPLY','status':'WAITING','authorization':receipt,
                    'target':target,'expired_recovery_authority':authority,'nats_migration':{'verified':True},
                    'kernel_sha256':hashlib.sha256(kernel).hexdigest(),'contract':{},'provenance':{},
                    'service_actor_services':[],'service_actor_authority':{},'migrations':{},
                    'migration_secret_metadata':{'before':True},'migration_completed_metadata':{'after':True},
                    'nonnats':{},'nonnats_binding':{}}
                transaction.save(base/'checkpoint.json',state);before=(base/'checkpoint.json').read_bytes()
                stage=Mock();stage.marker=copy.deepcopy(old_marker);stage.snapshots={'voice-story':copy.deepcopy(captured)}
                kube=Mock();kube.get.side_effect=lambda kind,name:copy.deepcopy(marker if kind=='configmap' else actual)
                if drift=='uid':actual['metadata']['uid']='other'
                elif drift=='template':actual['spec']['template']['spec']['containers'][0]['image']='other'
                elif drift=='scale':actual['spec']['replicas']=2
                elif drift=='marker':marker['data']['knownRolloutOperation']='other'
                calls=[0]
                def fresh():
                    calls[0]+=1
                    if drift=='checkpoint' and calls[0]>1:(base/'checkpoint.json').write_bytes(b'drift')
                for adapter in (patch('expired_recovery.verify_adopted_binding'),patch('expired_recovery.runtime_authority'),
                    patch('guard.protected_receipt',return_value=receipt),patch.object(transaction,'reconstruct',return_value=stage),
                    patch.object(admission,'input_hashes',return_value={'bound':'inputs'}),
                    patch.object(admission,'get_permission_proof',return_value={'bound':'actual scratch GET'}),
                    patch('actor_root.revalidate'),patch('migrations.preflight',return_value={'after':True}),patch('nonnats_runtime.verify')):
                    stack.enter_context(adapter)
                if drift is None:
                    observed,proof=admission.admit_finish(kube,base,state,base,{},fresh)
                    self.assertIs(observed,stage);self.assertEqual(stage.marker,marker)
                    self.assertEqual(stage.snapshots['voice-story'],actual)
                    self.assertEqual((base/'checkpoint.json').read_bytes(),before)
                    self.assertEqual(proof['applied_receipt_sha256'],digest(receipt))
                else:
                    with self.assertRaises(recovery.RecoveryError):admission.admit_finish(kube,base,state,base,{},fresh)


class GetAdmissionTests(unittest.TestCase):
    def test_retained_identity_all_streams_including_empty_cache_and_denial(self):
        from bootstrap_root import hash_bytes,ACCOUNT_SHA
        from docker_runtime import NATS_IMAGE
        credentials=b'public-synthetic-credential-fixture'
        hashes={'inputs/bootstrap.creds':hash_bytes(credentials)}
        state={'operation':'3340764a7d24','cut':{'census':{'streams':[{'name':'empty'},{'name':'populated'}]}},
            'nats_migration':None,'bootstrap_enrollment':{'binding':{'generation':'fixture'},'secret':{'name':'retained'}}}
        from unittest.mock import Mock
        kube=Mock();fresh=Mock()
        def prove(creds,operator,unchanged,names):
            self.assertEqual(creds,credentials);self.assertEqual(names,['empty','populated']);unchanged()
            return {'server_image':NATS_IMAGE,'account_sha256':ACCOUNT_SHA,'get_permissions':{
                'schema':'voice-retained-get-permission-proof-v1','subjects_sha256':hash_bytes(b'["empty","populated"]'),
                'streams':2,'verified':True}}
        with patch('bootstrap_root.secret_bytes',return_value=credentials),patch('bootstrap_auth.prove',side_effect=prove) as actual:
            capability=admission.get_permission_proof(kube,state,hashes,fresh)
            self.assertEqual(capability['streams'],2)
            self.assertEqual(admission.get_permission_proof(kube,state,hashes,fresh,capability),capability)
            self.assertEqual(actual.call_count,1)
            with self.assertRaises(recovery.RecoveryError):
                admission.get_permission_proof(kube,state,{'inputs/bootstrap.creds':'0'*64},fresh,capability)
            changed=copy.deepcopy(state);changed['cut']['census']['streams'].append({'name':'new'})
            with self.assertRaises(recovery.RecoveryError):admission.get_permission_proof(kube,changed,hashes,fresh,capability)
        with patch('bootstrap_root.secret_bytes',return_value=credentials),patch('bootstrap_auth.prove',side_effect=ValueError('permission denied')):
            with self.assertRaises(ValueError):admission.get_permission_proof(kube,state,hashes,fresh)


class AdmissionTests(unittest.TestCase):
    def test_actual_actions_observation_keeps_original_checkpoint_and_rejects_final_drift(self):
        from unittest.mock import Mock
        import bridge_root
        from preserved_upload import OPERATION
        for drift in (False,True):
            with self.subTest(drift=drift),TemporaryDirectory() as folder:
                base=Path(folder)/('rollout-'+OPERATION);base.mkdir(mode=0o700)
                checkpoint=base/'checkpoint.json';checkpoint.write_bytes(b'original-checkpoint');checkpoint.chmod(0o600)
                actions=object.__new__(bridge_root.Actions);actions.code=Path(folder)/'code';actions.binding={}
                state={'operation':OPERATION,'target':{'tag':'a'*40},'bootstrap_enrollment':{},
                    'contract':{},'nats_contract':{},'nats_contract_binding':{},'nats_target_scripts':{}}
                stage=Mock();execution={'dispatcher_run_id':123,'head_sha':'b'*40}
                current={'verified':True,'source_files':{}};admission_row={'input_hashes':{},'target':{'bound':'original target'}}
                def fresh():
                    if drift:checkpoint.write_bytes(b'changed-checkpoint')
                observation={'schema':'fixture','full_records':{'messages':6}}
                with patch.object(actions,'_expired_source_admission',return_value=
                    (base,state,stage,execution,current,admission_row,fresh)),\
                     patch('expired_recovery.repair_inputs',return_value={'verified':True}),\
                     patch('expired_proof.produce',return_value=observation),\
                     patch('docker_runtime.DockerRuntime'),\
                     patch('encrypted_cut.encrypt_cut',return_value={'cipher_sha256':'c'*64,'cipher_bytes':123}),\
                     patch('preserved_upload.private_json',return_value={}):
                    request={'nonce':'d'*64}
                    if drift:
                        with self.assertRaises(bridge_root.Blocked):actions._produce_expired_observation(request)
                    else:
                        slot,actual,cipher=actions._produce_expired_observation(request)
                        self.assertEqual(checkpoint.read_bytes(),b'original-checkpoint')
                        self.assertEqual(actual,observation);self.assertEqual(cipher['run_id'],123)
                        self.assertEqual(json.loads((slot/'producer.json').read_text())['operation'],OPERATION)
                        with self.assertRaises(FileExistsError):actions._produce_expired_observation(request)

    def test_captured_config_and_credential_hashes_match_exact_original_capture_format(self):
        import root_main
        values={key:('synthetic-'+key).encode() for key in ('bootstrap.creds','social.creds',
            'realtime.creds','operator.jwt','account.jwt','system-account.jwt')}
        values.update({'account.public':b'A'+'A'.encode()*55,'system-account.public':b'A'+'B'.encode()*55})
        state={'contract':{},'provenance':{key:{'resource':'fixture','sha256':hashlib.sha256(raw).hexdigest()}
            for key,raw in values.items()}}
        class Kube:
            def get(self,*args):return {'fixture':'no actual Secret access'}
        with patch.object(admission,'revalidate_inputs') as inputs,\
             patch('bootstrap_root.secret_bytes',side_effect=lambda row,key:values[key]):
            expected=admission.input_hashes(Kube(),state)
            self.assertEqual(inputs.call_count,2)
            self.assertEqual(set(expected),recovery.INPUT_FILES)
            # Execute the same root capture config expression from its actual
            # AST, rather than a separately modeled serialization expectation.
            import ast
            tree=ast.parse(Path(root_main.__file__).read_text())
            capture=next(node for node in tree.body if isinstance(node,ast.FunctionDef) and node.name=='capture_inputs')
            expression=next(node.value for node in capture.body if isinstance(node,ast.Assign)
                and any(isinstance(target,ast.Name) and target.id=='config' for target in node.targets))
            tokens={key:raw.decode().strip() for key,raw in values.items()}
            config=eval(compile(ast.Expression(expression),'<actual-root-capture-config>','eval'),{'q':json.dumps,'tokens':tokens})
            self.assertEqual(expected['server.conf'],hashlib.sha256(config.encode()).hexdigest())
            for key in ('bootstrap.creds','social.creds','realtime.creds'):
                self.assertEqual(expected['inputs/'+key],hashlib.sha256(values[key]).hexdigest())
            changed=copy.deepcopy(state);changed['provenance']['bootstrap.creds']['sha256']='0'*64
            with self.assertRaises(ValueError):admission.input_hashes(Kube(),changed)


if __name__=='__main__':unittest.main()
