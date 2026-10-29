"""C2 fixed Job routing; source/history and Kubernetes responses are fixtures."""
import copy
import json
import tempfile
import unittest
import uuid
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import commands
import guard
import resource_claim
import staging_cohort_preservation as owner
import staging_space_native as native
from staging_cohort_preservation_test import CutEffectTests


class SuccessorTests(unittest.TestCase):
    def test_transition_choice_is_derived_from_predecessor(self):
        session=object()
        with patch.object(native,'historical_native_predecessor',return_value=None), \
             patch.object(native,'inspect_source',return_value={'legacy':'fixture'}) as legacy, \
             patch.object(native,'verify_native_successor') as successor:
            self.assertEqual(native.prepare_native_transition(session),{'legacy':'fixture'})
            legacy.assert_called_once_with(session);successor.assert_not_called()
        with patch.object(native,'historical_native_predecessor',return_value={'native':'fixture'}), \
             patch.object(native,'inspect_source') as legacy, \
             patch.object(native,'verify_native_successor',return_value={'successor':'fixture'}) as successor:
            self.assertEqual(native.prepare_native_transition(session),{'successor':'fixture'})
            successor.assert_called_once_with(session);legacy.assert_not_called()

    def test_fixed_verify_job_and_lost_create_no_replay(self):
        for lost in (False,True):
            with self.subTest(lost=lost),tempfile.TemporaryDirectory(dir='/root') as directory:
                root=Path(directory);(root/'installed').mkdir(mode=0o700)
                with patch.object(guard,'ROOT',root),patch.object(owner,'_source'):
                    session,_=CutEffectTests().fixture(root,{'spec':{'replicas':1}})
                    tool={'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@sha256:'+'c'*64,
                          'source_inventory':{'build_source_sha256':'d'*64}}
                    fake=SimpleNamespace(operation=session.operation,context=session.context)
                    job=native._live_documents(fake,tool,'node-a','local-path','snapshot')['verify']
                    name='voice-space-native-verify-'+session.operation
                    job['metadata']['name']=name;job['metadata']['labels']={'voice-space-native-successor':session.operation}
                    job['spec']['template']['metadata']['labels']=copy.deepcopy(job['metadata']['labels'])
                    hashes={'/evidence/origin.json':'1'*64,'/evidence/completed.jsonl':'2'*64}
                    plan={'schema':'voice-staging-space-native-successor-plan-v1','operation':session.operation,
                          'predecessor':{'fixture':'modeled'},'job':job,'node':'node-a','expected_hashes':hashes,
                          'tool_platform_digest':'sha256:'+'c'*64,'tool_config_digest':'sha256:'+'e'*64}
                    job_uid=str(uuid.uuid4());pod_uid=str(uuid.uuid4());live={};calls=[];polls=[0]
                    def pod():
                        spec=copy.deepcopy(job['spec']['template']['spec']);spec['nodeName']='node-a'
                        return {'metadata':{'name':name+'-pod','namespace':'voice-staging','uid':pod_uid,'resourceVersion':'9',
                            'ownerReferences':[{'controller':True,'kind':'Job','uid':job_uid}]},'spec':spec,
                            'status':{'containerStatuses':[{'name':'native','imageID':'fixture@'+plan['tool_platform_digest'],
                                'state':{'terminated':{'exitCode':0}}}]}}
                    class Kube:
                        def run(self,args,body=None):
                            if args==['get','pods','-o','json']:
                                return {'apiVersion':'v1','kind':'PodList','metadata':{'resourceVersion':'1'},'items':[pod()] if live else []}
                            return copy.deepcopy(body)
                        def get(self,kind,key):
                            if kind=='pod':return copy.deepcopy(pod())
                            value=copy.deepcopy(live[key]);polls[0]+=1
                            if polls[0]==1:value['status']={'active':1}
                            return value
                    def capture(argv,body=b'',**kwargs):
                        commands.authority_guard(argv);tail=argv[len(owner.PREFIX):];calls.append(tail[0])
                        if tail[0]=='create':
                            value=json.loads(body);value['metadata'].update(uid=job_uid,resourceVersion='7')
                            value['status']={'succeeded':1};live[name]=value
                            if lost:raise RuntimeError('lost create response')
                            return json.dumps(value).encode()
                        if tail[0]=='delete':
                            del live[name];return json.dumps({'apiVersion':'v1','kind':'Status','status':'Success'}).encode()
                        if tail[0]=='logs':return (hashes['/evidence/origin.json']+'  /evidence/origin.json\n'+hashes['/evidence/completed.jsonl']+'  /evidence/completed.jsonl\nverification=ok\n').encode()
                        if tail[0]=='get' and tail[1]=='pods':
                            return json.dumps({'apiVersion':'v1','kind':'PodList','metadata':{'resourceVersion':'1'},'items':[pod()] if live else []}).encode()
                        if tail[0]=='get' and tail[1]=='--raw':
                            return json.dumps({'apiVersion':'batch/v1','kind':'JobList','metadata':{'resourceVersion':'1'},'items':[]}).encode()
                        raise AssertionError(tail)
                    with resource_claim.command_scope(session.check),patch.object(native,'Kube',return_value=Kube()), \
                         patch.object(native,'_successor_context',return_value=plan) as context,patch.object(native,'_successor_no_sql'), \
                         patch.object(native.time,'sleep'),patch.object(commands,'capture',side_effect=capture):
                        if lost:
                            with self.assertRaisesRegex(RuntimeError,'lost create response'):native.verify_native_successor(session)
                            with self.assertRaisesRegex(Exception,'cohort_reconciliation_required'):native.verify_native_successor(session)
                            self.assertEqual(calls.count('create'),1)
                            self.assertEqual(owner._read(owner._base(session)/'effects'/'space-native-successor-create.json')['status'],'PENDING')
                        else:
                            receipt=native.verify_native_successor(session)
                            self.assertEqual(receipt['hashes'],hashes);self.assertEqual(receipt['cleanup'],'verified')
                            self.assertEqual(calls.count('create'),1);self.assertEqual(calls.count('delete'),1)
                            self.assertGreaterEqual(polls[0],2);self.assertEqual(context.call_count,5)
                            tampered=copy.deepcopy(receipt);tampered['hashes']['/evidence/origin.json']='f'*64
                            owner.save(owner._base(session)/'space-native-successor.json',tampered)
                            with self.assertRaisesRegex(Exception,'space_native_successor_changed'):
                                native.load_native_successor(session)


if __name__=='__main__':unittest.main()
