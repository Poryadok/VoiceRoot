import copy,hashlib,tempfile,unittest
from pathlib import Path
from unittest.mock import patch
import bootstrap_selection
from bootstrap_selection_test import fixture as enrollment_fixture
from nats_contract_plan_test import live
from nats_contract_plan import DECLARATIONS
from rollout_census_test import CREATED
import nats_root_plan as module
from controller import Blocked

class Tests(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup);self.source=Path(self.tmp.name)
        self.kube,self.enrollment,self.rows,self.sha=enrollment_fixture()
        self.streams=live();self.tree={'streams':3,'consumers':0,'messages':14,'bytes':200,'total':1,'account_details':[{'id':'A'+'A'*55,'stream_detail':[]}]}
        for name,info in self.streams.items():
            state=dict(info['state'],consumer_count=0,bytes=100)
            self.tree['account_details'][0]['stream_detail'].append({'name':name,'created':CREATED,'config':info['config'],'state':state,'consumer_detail':[]})
        chat=self.streams['chat_events']['config']['subjects']+['chat.updated','chat.deleted','space.voice_room_access_invalidated']
        social=self.streams['social_events']['config']['subjects']
        role=self.streams['role_events']['config']['subjects']
        common='    stream chat_events '+' '.join(chat)+'\n'
        role_line='    stream role_events '+' '.join(role+['role.voice_policy_invalidated'])+'\n'
        declarations=''.join('    consumer '+stream+' '+config['name']+' '+config['filter_subject']+' '+config['deliver_subject']+'\n' for stream,config in DECLARATIONS)
        target=common+'    stream_with_duplicate_window social_events 86400000000000 '+' '.join(social)+'\n'+role_line+declarations
        self.files={}
        for role,text in [('realtime',target),('analytics-chat',common+role_line)]:
            p=self.source/('deploy/templates/nats-'+role+'-bootstrap.yaml');p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text)
            self.files[role]=hashlib.sha256(p.read_bytes()).hexdigest()
        self.contract={'scripts':[{'part':part} for part in ('realtime','analytics-chat','notification','search')]}
        normalized=[{'stream':stream,'durable':config['durable_name'],'config':copy.deepcopy(config)} for stream,config in DECLARATIONS]
        legacy=normalized[0]['config']
        from nats_contract_plan import digest
        self.legacy_auth=copy.deepcopy(self.enrollment['auth_proof'])
        self.fresh_auth=copy.deepcopy(self.legacy_auth)
        self.fresh_auth['normalized_consumers']=normalized
        self.fresh_auth['normalized_consumers_sha256']=digest(normalized)
    def decode(self,raw):
        text=raw.decode();part='realtime' if 'stream_with_duplicate_window' in text else 'analytics-chat'
        return {'kind':'ConfigMap','metadata':{'name':'voice-nats-'+part+'-bootstrap'},'data':{'bootstrap.sh':text}}
    def run_plan(self,auth=None):
        with patch.object(module,'TARGET_FILES',self.files),patch.object(module,'monitor',return_value=self.tree),patch.object(module,'census',return_value={'fixture_census':True}),\
             patch.object(bootstrap_selection,'ACCOUNT_SHA',self.sha),\
             patch('bootstrap_auth.prove',return_value=auth or copy.deepcopy(self.fresh_auth)) as prove:
            result=module.preflight(self.kube,self.source,self.enrollment,self.contract,self.decode)
            self.assertEqual(prove.call_args.args[0],b'renewed-private-creds')
            return result
    def test_root_producer_binds_source_existing_enrollment_server_and_plan(self):
        from nats_contract_plan import digest
        result=self.run_plan();self.assertEqual(len(result['plan']['actions']),6)
        self.assertEqual(result['binding']['plan_sha256'],digest(result['plan']))
        self.assertEqual(result['binding']['enrollment_sha256'],digest(self.enrollment))
        self.assertEqual(self.enrollment['auth_proof'],self.legacy_auth)
        self.assertEqual([(row['stream'],row['name']) for row in result['plan']['consumer_targets']],
            [(stream,config['durable_name']) for stream,config in DECLARATIONS])
        self.assertEqual([row['config_sha256'] for row in result['plan']['consumer_targets']],
            [digest(row['config']) for row in self.fresh_auth['normalized_consumers']])
        self.assertEqual(set(result['scripts']),{'realtime','analytics-chat'})
    def test_pending_records_can_advance_without_changing_config_intent(self):
        first=self.run_plan();stream=self.tree['account_details'][0]['stream_detail'][0]
        stream['state'].update(messages=8,last_seq=18);self.tree['messages']=15
        second=self.run_plan();self.assertEqual(first['plan'],second['plan']);self.assertEqual(first['binding'],second['binding'])
    def test_fresh_auth_default_or_source_change_refused(self):
        auth=copy.deepcopy(self.fresh_auth);auth['normalized_consumers'][1]['config']['max_ack_pending']=999
        with self.assertRaises(Blocked):self.run_plan(auth)
        self.files['realtime']='0'*64
        with self.assertRaises(Blocked):self.run_plan()
    def test_fresh_auth_must_preserve_exact_legacy_enrollment_proof(self):
        for field in ('server_image','account_sha256','reply_prefix','normalized_consumer_sha256'):
            auth=copy.deepcopy(self.fresh_auth);auth[field]='changed'
            with self.subTest(field=field),self.assertRaises(Blocked):self.run_plan(auth)
    def test_fresh_auth_v3_extension_is_exact_and_hash_bound(self):
        for mutate in (
            lambda row:row.update(unreviewed=True),
            lambda row:row.update(normalized_consumers_sha256='0'*64),
            lambda row:row.update(normalized_consumers=list(reversed(row['normalized_consumers']))),
            lambda row:row.update(normalized_consumers=row['normalized_consumers'][:-1]),
        ):
            auth=copy.deepcopy(self.fresh_auth);mutate(auth)
            with self.subTest(auth=auth.keys()),self.assertRaises(Blocked):self.run_plan(auth)
    def test_v3_enrollment_is_supported_only_when_fresh_proof_matches_exactly(self):
        self.enrollment['auth_proof']=copy.deepcopy(self.fresh_auth)
        result=self.run_plan()
        from nats_contract_plan import digest
        self.assertEqual(result['binding']['enrollment_sha256'],digest(self.enrollment))
        changed=copy.deepcopy(self.fresh_auth);changed['normalized_consumers'][2]['config']['ack_policy']='all'
        with self.assertRaises(Blocked):self.run_plan(changed)
if __name__=='__main__':unittest.main()
