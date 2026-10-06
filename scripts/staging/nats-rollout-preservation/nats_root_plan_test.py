import copy,hashlib,tempfile,unittest
from pathlib import Path
from unittest.mock import patch
import bootstrap_selection
from bootstrap_selection_test import fixture as enrollment_fixture
from nats_contract_plan_test import live
from rollout_census_test import CREATED
import nats_root_plan as module
from controller import Blocked

class Tests(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup);self.source=Path(self.tmp.name)
        self.kube,self.enrollment,self.rows,self.sha=enrollment_fixture()
        self.streams=live();self.tree={'streams':2,'consumers':0,'messages':14,'bytes':200,'total':1,'account_details':[{'id':'A'+'A'*55,'stream_detail':[]}]}
        for name,info in self.streams.items():
            state=dict(info['state'],consumer_count=0,bytes=100)
            self.tree['account_details'][0]['stream_detail'].append({'name':name,'created':CREATED,'config':info['config'],'state':state,'consumer_detail':[]})
        chat=self.streams['chat_events']['config']['subjects']+['space.deletion_scheduled','space.restored']
        social=self.streams['social_events']['config']['subjects']
        common='    stream chat_events '+' '.join(chat)+'\n'
        target=common+'    stream_with_duplicate_window social_events 86400000000000 '+' '.join(social)+'\n    consumer social_events rt_realtime1_friend_removed social.friend_removed _INBOX.voice.realtime1.friend_removed\n'
        self.files={}
        for role,text in [('realtime',target),('analytics-chat',common)]:
            p=self.source/('deploy/templates/nats-'+role+'-bootstrap.yaml');p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text)
            self.files[role]=hashlib.sha256(p.read_bytes()).hexdigest()
        self.contract={'scripts':[{'part':part} for part in ('realtime','analytics-chat','notification','search')]}
    def decode(self,raw):
        text=raw.decode();part='realtime' if 'stream_with_duplicate_window' in text else 'analytics-chat'
        return {'kind':'ConfigMap','metadata':{'name':'voice-nats-'+part+'-bootstrap'},'data':{'bootstrap.sh':text}}
    def run_plan(self,auth=None):
        with patch.object(module,'TARGET_FILES',self.files),patch.object(module,'monitor',return_value=self.tree),\
             patch.object(bootstrap_selection,'ACCOUNT_SHA',self.sha),\
             patch('bootstrap_auth.prove',return_value=auth or copy.deepcopy(self.enrollment['auth_proof'])) as prove:
            result=module.preflight(self.kube,self.source,self.enrollment,self.contract,self.decode)
            self.assertEqual(prove.call_args.args[0],b'renewed-private-creds')
            return result
    def test_root_producer_binds_source_existing_enrollment_server_and_plan(self):
        from nats_contract_plan import digest
        result=self.run_plan();self.assertEqual(len(result['plan']['actions']),3)
        self.assertEqual(result['binding']['plan_sha256'],digest(result['plan']))
        self.assertEqual(result['binding']['enrollment_sha256'],digest(self.enrollment))
        self.assertEqual(set(result['scripts']),{'realtime','analytics-chat'})
    def test_pending_records_can_advance_without_changing_config_intent(self):
        first=self.run_plan();stream=self.tree['account_details'][0]['stream_detail'][0]
        stream['state'].update(messages=8,last_seq=18);self.tree['messages']=15
        second=self.run_plan();self.assertEqual(first['plan'],second['plan']);self.assertEqual(first['binding'],second['binding'])
    def test_fresh_auth_default_or_source_change_refused(self):
        auth=copy.deepcopy(self.enrollment['auth_proof']);auth['normalized_consumer']['max_ack_pending']=999
        with self.assertRaises(Blocked):self.run_plan(auth)
        self.files['realtime']='0'*64
        with self.assertRaises(Blocked):self.run_plan()
if __name__=='__main__':unittest.main()
