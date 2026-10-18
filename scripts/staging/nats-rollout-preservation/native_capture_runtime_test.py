"""Closed fake-Kubernetes proof for the native PREPARE census consumer."""
import copy
import hashlib
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import kube_json_api
import preserve
import rollout_census
import tokenless_job
import verification_job
from closed_archive import Archive
from native_store import archive_closed_store


OP='a'*12
SOURCE_UID='11111111-1111-4111-8111-111111111111'
SOURCE_PV_UID='22222222-2222-4222-8222-222222222222'
TARGET_UID='33333333-3333-4333-8333-333333333333'
TARGET_PV_UID='44444444-4444-4444-8444-444444444444'
JOB_UID='55555555-5555-4555-8555-555555555555'
POD_UID='66666666-6666-4666-8666-666666666666'
ACCOUNT='A'+'B'*55
IMAGE='ghcr.io/poryadok/voice-nats-rollout-runner@sha256:'+'a'*64


def valid_census():
    row={'schema':rollout_census.SCHEMA,'account':ACCOUNT,
        'streams':[{'name':'chat_events','created':'2026-10-05T10:00:00Z','config_sha256':'a'*64,
            'state':{'messages':7,'bytes':70,'first_seq':1,'last_seq':7,'consumer_count':1},
            'observations':{}}],
        'consumers':[{'stream':'chat_events','name':'rt_realtime1','durable':'rt_realtime1',
            'created':'2026-10-05T10:00:00Z','config_sha256':'b'*64,'inactive_threshold':0,
            'delivered':{'consumer_seq':2,'stream_seq':7},'ack_floor':{'consumer_seq':1,'stream_seq':2},
            'num_ack_pending':1,'num_redelivered':0,'num_pending':3,'observations':{'num_waiting':0}}]}
    return rollout_census.validate_snapshot(row)


class CensusAPI:
    def __init__(self, failure=None):
        self.calls=[]; self.failure=failure; self.pod_reads=0
        self.pvc={'apiVersion':'v1','kind':'PersistentVolumeClaim',
            'metadata':{'name':'voice-nats-verify-'+OP,'namespace':'voice-staging','uid':TARGET_UID,
                'labels':{'voice.io/nats-operation':OP,'voice.io/nats-job-nonce':'f'*32,
                    'voice.io/nats-pvc-role':'verification','voice.io/nats-verification-attempt':'00000000'}},
            'spec':{'accessModes':['ReadWriteOnce'],'volumeMode':'Filesystem','storageClassName':'fast',
                'resources':{'requests':{'storage':str(48<<30)}},'volumeName':'target-pv'},
            'status':{'phase':'Bound','capacity':{'storage':'48Gi'}}}
        self.pv={'metadata':{'name':'target-pv','uid':TARGET_PV_UID},'status':{'phase':'Bound'},
            'spec':{'claimRef':{'namespace':'voice-staging','name':self.pvc['metadata']['name'],'uid':TARGET_UID},
                'storageClassName':'fast','accessModes':['ReadWriteOnce'],'volumeMode':'Filesystem',
                'capacity':{'storage':'48Gi'},'nodeAffinity':{'required':{'nodeSelectorTerms':[
                    {'matchExpressions':[{'key':'kubernetes.io/hostname','operator':'In','values':['worker.internal']}]}]}}}}
        self.node={'metadata':{'name':'worker-1','uid':'77777777-7777-4777-8777-777777777777',
            'labels':{'kubernetes.io/hostname':'worker.internal'}},'status':{'conditions':[{'type':'Ready','status':'True'}]}}
        self.job=None; self.pod=None

    def get_storage_class(self,name):
        self.calls.append('get-storage-class')
        return {'metadata':{'name':name,'uid':'88888888-8888-4888-8888-888888888888','resourceVersion':'1'},
            'provisioner':'example.test/csi','volumeBindingMode':'Immediate'}
    def get_pvc(self,namespace,name):
        self.calls.append('get-pvc')
        if 'create-verification-pvc' not in self.calls: raise kube_json_api.KubeNotFound('missing')
        return copy.deepcopy(self.pvc)
    def create_verification_pvc(self,operation,nonce,size_bytes,storage_class,*,attempt):
        self.calls.append('create-verification-pvc')
        self.pvc['metadata']['name']='voice-nats-verify-'+operation+'-'+attempt
        self.pvc['metadata']['labels']['voice.io/nats-operation']=operation
        self.pvc['metadata']['labels']['voice.io/nats-job-nonce']=nonce
        self.pvc['metadata']['labels']['voice.io/nats-verification-attempt']=attempt
        return copy.deepcopy(self.pvc)
    def get_pv(self,name): self.calls.append('get-pv'); return copy.deepcopy(self.pv)
    def list_nodes_by_hostname(self,hostname): self.calls.append('list-node'); return [copy.deepcopy(self.node)]
    def get_job(self,namespace,name):
        self.calls.append('get-job')
        if self.job is None: raise kube_json_api.KubeNotFound('missing')
        return copy.deepcopy(self.job)
    def create_job(self,namespace,wanted):
        self.calls.append('create-job'); self.job=copy.deepcopy(wanted)
        self.job['metadata']['uid']=JOB_UID
        return copy.deepcopy(self.job)
    def wait_job_pod(self,namespace,name,uid,timeout):
        self.calls.append('wait-pod')
        template=self.job['spec']['template']
        metadata=copy.deepcopy(template['metadata']); metadata.update({'name':name+'-abcde',
            'namespace':namespace,'uid':POD_UID,'ownerReferences':[{'apiVersion':'batch/v1','kind':'Job',
            'name':name,'uid':uid,'controller':True,'blockOwnerDeletion':True}]})
        labels=metadata['labels']; labels.update({'batch.kubernetes.io/controller-uid':uid,
            'controller-uid':uid,'batch.kubernetes.io/job-name':name,'job-name':name})
        spec=copy.deepcopy(template['spec']); spec['nodeName']='worker-1'
        self.pod={'apiVersion':'v1','kind':'Pod','metadata':metadata,'spec':spec,
            'status':{'phase':'Running','containerStatuses':[{'name':'nats-phase',
                'containerID':'containerd://'+'9'*64,'restartCount':0,'ready':True}]}}
        return copy.deepcopy(self.pod)
    def get_pod(self,namespace,name):
        self.calls.append('get-pod'); self.pod_reads+=1
        row=copy.deepcopy(self.pod)
        if self.failure=='pod-replaced-after-load' and self.pod_reads>=2:
            row['metadata']['uid']='99999999-9999-4999-8999-999999999999'
        return row
    def exec_pod_verification_input(self,*args): self.calls.append('load-inputs'); return self.input_frame
    def read_census_input_receipt(self,*args,**kwargs):
        self.calls.append('read-input-receipt')
        if self.failure=='missing-input-receipt': raise ValueError('missing')
        return self.input_receipt
    def execute_census_input_consumption(self,*args,**kwargs):
        self.calls.append('consume-inputs')
        return {'schema':'voice-nats-census-consumption-receipt-v1','phase':'census-consumption',
            'purpose':'verification','operation':OP,'nonce':self.request['nonce'],'pod_uid':POD_UID,
            'input_pod_uid':POD_UID,'source_pvc_uid':SOURCE_UID,'source_pv_uid':SOURCE_PV_UID,
            'verification_pvc_uid':TARGET_UID,'verification_pv_uid':TARGET_PV_UID,
            'archive_sha256':self.archive['sha256'],'archive_bytes':self.archive['bytes'],
            'session_attempt':self.inputs['attempt'],'input_bundle_sha256':self.inputs['sha256'],
            'input_bundle_bytes':self.inputs['bytes'],'kernel_bundle_sha256':'c'*64,
            'server_config_sha256':'d'*64,'status':'COMPLETE'}
    def exec_pod_census_input(self,*args): self.calls.append('receive-archive')
    def read_verification_census(self,*args):
        self.calls.append('read-census')
        if self.failure=='partial-census-stream': raise ValueError('truncated')
        return {'receipt':{'census_sha256':'e'*64},'census':valid_census()}
    def delete_job(self,namespace,name,uid): self.calls.append('delete-job'); return True


class NativeCaptureRuntimeTests(unittest.TestCase):
    def setUp(self):
        anchor=os.environ.get('NATIVE_PRIVATE_TEST_ANCHOR') if os.geteuid()==65532 and os.getegid()==65532 else None
        self.temp=tempfile.TemporaryDirectory(dir=anchor)
        self.base=Path(self.temp.name)
        os.chmod(self.base,0o700)
        (self.base/'inputs').mkdir(mode=0o700)
        for path,data in ((self.base/'inputs'/'bootstrap.creds',b'bootstrap-secret-fixture'),
                          (self.base/'server.conf',b'accounts { SYS { users = [] } }')):
            path.write_bytes(data); os.chmod(path,0o440)
        self.archive=self.base/'rollout-before.tar'; self.archive.write_bytes(b'private-archive-fixture')
        os.chmod(self.archive,0o600)
        self.manifest={'mechanism':'closed-jetstream-store-tar-v1','file_count':0,'bytes':0,
            'dirs':[],'files':[]}
        self.api=CensusAPI()
        self.runtime=tokenless_job.PvcArchiveRuntime(self.api,self.base,
            {'claim_name':'voice-nats-jsdata-d20261017a','claim_uid':SOURCE_UID,
                'pv_name':'source-pv','pv_uid':SOURCE_PV_UID,'node_name':'worker-1',
                'node_hostname':'worker.internal'},IMAGE,OP,'b'*64,persist_intent=lambda row:None,
            persist_result=lambda row:None,account_id=ACCOUNT,
            verification_pvc_size_bytes=48<<30,verification_storage_class='fast')
        self.runtime._archive_result={'archive_sha256':hashlib.sha256(self.archive.read_bytes()).hexdigest(),
            'bytes':self.archive.stat().st_size}
        self._configure_inputs()

    def _configure_inputs(self):
        request={'operation':OP,'nonce':hashlib.sha256((self.runtime.request['nonce']+':prepare-census').encode()).hexdigest()[:32],
            'phase':'census'}
        self.api.request=request; self.api.archive={'sha256':self.runtime._archive_result['archive_sha256'],
            'bytes':self.archive.stat().st_size}
        files={'bootstrap.creds':b'bootstrap-secret-fixture','server.conf':b'accounts { SYS { users = [] } }'}
        bundle={'schema':'voice-nats-census-inputs-v1','operation':OP,'nonce':request['nonce'],
            'pod_uid':POD_UID,'files':{key:__import__('base64').b64encode(value).decode() for key,value in files.items()}}
        bundle_bytes=json.dumps(bundle,sort_keys=True,separators=(',',':'),ensure_ascii=False).encode()
        self.api.inputs={'attempt':hashlib.sha256((OP+':'+self.runtime.request['nonce']+':census').encode()).hexdigest()[:8],
            'sha256':hashlib.sha256(bundle_bytes).hexdigest(),'bytes':len(bundle_bytes)}
        self.api.input_receipt={'schema':'voice-nats-census-input-receipt-v1','phase':'census-inputs',
            'purpose':'verification','operation':OP,'nonce':request['nonce'],'pod_uid':POD_UID,
            'source_pvc_uid':SOURCE_UID,'source_pv_uid':SOURCE_PV_UID,'verification_pvc_uid':TARGET_UID,
            'verification_pv_uid':TARGET_PV_UID,'archive_sha256':self.api.archive['sha256'],
            'archive_bytes':self.api.archive['bytes'],'session_attempt':self.api.inputs['attempt'],
            'input_bundle_sha256':self.api.inputs['sha256'],'input_bundle_bytes':self.api.inputs['bytes'],
            'status':'COMPLETE','file_count':2,'kernel_bundle_sha256':'c'*64,'server_config_sha256':'d'*64}
        self.api.input_frame=(json.dumps(self.api.input_receipt,sort_keys=True,separators=(',',':'),
            ensure_ascii=False).encode()+b'\n')

    def tearDown(self): self.temp.cleanup()

    def test_complete_prepare_census_runs_one_target_only_pod_and_reads_valid_receipt(self):
        with patch('native_store.verify_archive',return_value=True):
            census=self.runtime.capture_census(self.archive,self.manifest,'rollout-before')
        self.assertEqual(census['account'],ACCOUNT)
        self.assertIn('create-verification-pvc',self.api.calls)
        self.assertEqual(self.api.calls.count('create-job'),1)
        self.assertEqual(self.api.calls.count('read-census'),1)
        self.assertEqual(self.api.calls[-1],'delete-job')
        self.assertLess(self.api.calls.index('load-inputs'),self.api.calls.index('consume-inputs'))
        self.assertLess(self.api.calls.index('consume-inputs'),self.api.calls.index('receive-archive'))
        created=next(call for call in self.api.calls if call=='create-job')
        self.assertIsNotNone(created)
        self.assertNotIn('delete-pvc',self.api.calls)

    def test_lost_input_receipt_leaves_job_and_never_retries_or_cleans_storage(self):
        self.api.failure='missing-input-receipt'
        with patch('native_store.verify_archive',return_value=True), self.assertRaises(ValueError):
            self.runtime.capture_census(self.archive,self.manifest,'rollout-before')
        self.assertEqual(self.api.calls.count('create-job'),1)
        self.assertNotIn('receive-archive',self.api.calls)
        self.assertNotIn('delete-job',self.api.calls)
        self.assertNotIn('delete-pvc',self.api.calls)

    def test_replaced_pod_uid_blocks_before_consumption_and_archive_ingest(self):
        self.api.failure='pod-replaced-after-load'
        with patch('native_store.verify_archive',return_value=True), self.assertRaises(ValueError):
            self.runtime.capture_census(self.archive,self.manifest,'rollout-before')
        self.assertIn('load-inputs',self.api.calls)
        self.assertNotIn('consume-inputs',self.api.calls)
        self.assertNotIn('receive-archive',self.api.calls)
        self.assertNotIn('delete-job',self.api.calls)

    def test_partial_census_readback_does_not_delete_job_or_retry(self):
        self.api.failure='partial-census-stream'
        with patch('native_store.verify_archive',return_value=True), self.assertRaises(ValueError):
            self.runtime.capture_census(self.archive,self.manifest,'rollout-before')
        self.assertEqual(self.api.calls.count('create-job'),1)
        self.assertEqual(self.api.calls.count('receive-archive'),1)
        self.assertNotIn('delete-job',self.api.calls)
        self.assertNotIn('delete-pvc',self.api.calls)

    def test_capture_cut_derives_catalog_and_closed_states_from_authenticated_archive(self):
        if (os.geteuid()!=65532 or os.getegid()!=65532
            or self.base.parent!=Archive.PRIVATE_ANCHOR):
            self.skipTest('native private archive proof requires runner UID/GID 65532')
        source=self.base/'source';meta=source/'jetstream'/ACCOUNT/'streams'/'chat_events'
        meta.mkdir(parents=True,mode=0o700)
        (meta/'meta.inf').write_text(json.dumps({'name':'chat_events','storage':'file'}),encoding='utf-8')
        (meta/'meta.sum').write_bytes(b'closed-stream-metadata')
        source_archive=self.base/'canonical-source.tar'
        manifest=archive_closed_store(source,source_archive)
        self.runtime._archive_result={'archive_sha256':manifest['archive_sha256'],
            'bytes':source_archive.stat().st_size}
        self.runtime.no_operation_containers=lambda *,running_only:None
        def archive_selected_store(destination):
            destination=Path(destination)
            destination.write_bytes(source_archive.read_bytes())
            self.runtime._archive_result={'archive_sha256':manifest['archive_sha256'],
                'bytes':destination.stat().st_size}
            return manifest
        self.runtime.archive_selected_store=archive_selected_store
        snapshot=valid_census();snapshot['streams'][0]['state']['consumer_count']=0;snapshot['consumers']=[]
        self.api.read_verification_census=lambda *args: {'receipt':{'census_sha256':'e'*64},'census':snapshot}
        fences=[]
        cut=preserve.capture_cut(self.runtime,None,lambda:fences.append('verified'),
            live_catalog={'account':ACCOUNT,'streams':[{'name':'chat_events'}],'consumers':[]})
        self.assertEqual(cut['manifest']['archive_sha256'],manifest['archive_sha256'])
        self.assertEqual(cut['census'],snapshot)
        self.assertEqual(cut['closed_durable_states'],{})
        self.assertEqual(cut['native_catalog_sha256'],preserve.canonical({
            'account':ACCOUNT,'streams':['chat_events'],'consumers':[]}))
        self.assertGreaterEqual(len(fences),4)
        self.assertIn('load-inputs',self.api.calls)
        self.assertIn('receive-archive',self.api.calls)
        self.assertEqual(self.api.calls[-1],'delete-job')

    def test_native_private_archive_rejects_mode_symlink_partial_and_identity_drift(self):
        if (os.geteuid()!=65532 or os.getegid()!=65532
            or self.base.parent!=Archive.PRIVATE_ANCHOR):
            self.skipTest('native private archive proof requires the installed runner anchor')
        source=self.base/'empty-source';source.mkdir(mode=0o700)
        archive=self.base/'private.tar';manifest=archive_closed_store(source,archive)
        archive.chmod(0o600)
        reader=Archive(self.base,archive,manifest,lambda:None,private_native=True)
        reader.verify()
        archive.chmod(0o644)
        with self.assertRaises(Exception):
            Archive(self.base,archive,manifest,lambda:None,private_native=True)
        archive.chmod(0o600)
        alias=self.base/'alias.tar';alias.symlink_to(archive)
        with self.assertRaises(Exception):
            Archive(self.base,alias,manifest,lambda:None,private_native=True)
        alias.unlink()
        archive.write_bytes(b'x'*archive.stat().st_size)
        with self.assertRaises(Exception):reader.verify()

    def test_selected_archive_allows_only_private_expired_proof_destinations(self):
        if (os.geteuid()!=65532 or os.getegid()!=65532
            or self.base.parent!=Archive.PRIVATE_ANCHOR):
            self.skipTest('native private archive proof requires the installed runner anchor')
        freshness=self.base/'expired-freshness-test'
        recovery=self.base/'expired-recovery-proofs'/'cut-test'
        finish=self.base/'expired-finish-proofs'/'cut-test'
        for directory in (freshness,recovery,finish):
            current=self.base
            for part in directory.relative_to(self.base).parts:
                current=current/part
                current.mkdir(mode=0o700,exist_ok=True)
                current.chmod(0o700)
        allowed=(self.base/'rollout-before.tar',self.base/'rollout-after.tar',
            freshness/'current.tar',recovery/'rollout-before.tar',finish/'rollout-before.tar')
        for path in allowed:
            self.assertEqual(self.runtime._archive_artifact_relative(path),path.relative_to(self.base))
        rejected=(self.base/'unrelated.tar',self.base/'expired-freshness-test'/'other.tar',
            self.base/'expired-recovery-proofs'/'cut-test'/'other.tar',
            self.base.parent/'outside'/'rollout-before.tar')
        for path in rejected:
            with self.subTest(path=path):self.assertIsNone(self.runtime._archive_artifact_relative(path))
        wrong_mode=self.base/'expired-recovery-proofs'/'loose'
        wrong_mode.mkdir(mode=0o755);wrong_mode.chmod(0o755)
        self.assertIsNone(self.runtime._archive_artifact_relative(wrong_mode/'rollout-before.tar'))
        link=self.base/'expired-freshness-link'
        link.symlink_to(freshness,target_is_directory=True)
        self.assertIsNone(self.runtime._archive_artifact_relative(link/'current.tar'))

    def test_native_private_archive_reader_rejects_wrong_effective_uid(self):
        if os.geteuid()==65532:
            self.skipTest('wrong-UID negative is exercised by the UID1000 suite')
        with self.assertRaises(Exception):
            Archive(self.base,self.archive,self.manifest,lambda:None,private_native=True)


if __name__=='__main__': unittest.main()
