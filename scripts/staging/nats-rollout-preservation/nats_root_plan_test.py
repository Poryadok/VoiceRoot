import copy,hashlib,json,shutil,tempfile,unittest
from pathlib import Path
from unittest.mock import patch
import bootstrap_selection
from bootstrap_selection_test import owned_fixture as enrollment_fixture
from nats_contract_plan_test import live
from nats_contract_plan import DECLARATIONS,V3_DECLARATIONS,V5_SOURCE_HASHES
from rollout_census_test import CREATED
import nats_root_plan as module
from controller import Blocked

class Tests(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup);self.source=Path(self.tmp.name)
        self.kube,self.enrollment,self.rows,self.installation=enrollment_fixture()
        self.streams=live();self.tree={'streams':3,'consumers':0,'messages':14,'bytes':200,'total':1,'account_details':[{'id':'A'+'A'*55,'stream_detail':[]}]}
        for name,info in self.streams.items():
            state=dict(info['state'],consumer_count=0,bytes=100)
            self.tree['account_details'][0]['stream_detail'].append({'name':name,'created':CREATED,'config':info['config'],'state':state,'consumer_detail':[]})
        chat=self.streams['chat_events']['config']['subjects']+['chat.updated','chat.deleted','space.voice_room_access_invalidated']
        social=self.streams['social_events']['config']['subjects']
        role=self.streams['role_events']['config']['subjects']
        common='    stream chat_events '+' '.join(chat)+'\n'
        role_line='    stream role_events '+' '.join(role+['role.voice_policy_invalidated'])+'\n'
        declarations=''.join('    consumer '+stream+' '+config['name']+' '+config['filter_subject']+' '+config['deliver_subject']+'\n' for stream,config in V3_DECLARATIONS)
        target=common+'    stream_with_duplicate_window social_events 86400000000000 '+' '.join(social)+'\n'+role_line+declarations
        self.files={}
        for role,text in [('realtime',target),('analytics-chat',common+role_line+'    consumer file_events messaging_music_metadata file.processed _INBOX.voice.messaging.messaging_music_metadata new messaging_music_metadata\n')]:
            p=self.source/('deploy/templates/nats-'+role+'-bootstrap.yaml');p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text)
            self.files[role]=hashlib.sha256(p.read_bytes()).hexdigest()
        self.contract={'scripts':[{'part':part} for part in ('realtime','analytics-chat','notification','search')]}
        normalized=[{'stream':stream,'durable':config['durable_name'],'config':copy.deepcopy(config)} for stream,config in DECLARATIONS]
        legacy=normalized[0]['config']
        from nats_contract_plan import digest
        self.legacy_auth=copy.deepcopy(self.enrollment['auth_proof'])
        self.fresh_auth=copy.deepcopy(self.legacy_auth)
        self.fresh_auth['schema']='voice-bootstrap-auth-v4'
        self.fresh_auth['normalized_consumers']=normalized
        self.fresh_auth['normalized_consumers_sha256']=digest(normalized)
    def decode(self,raw):
        text=raw.decode();part='realtime' if 'stream_with_duplicate_window' in text else 'analytics-chat'
        return {'kind':'ConfigMap','metadata':{'name':'voice-nats-'+part+'-bootstrap'},'data':{'bootstrap.sh':text}}
    def run_plan(self,auth=None):
        with patch.object(module,'TARGET_FILES',self.files),patch.object(module,'monitor',return_value=self.tree),patch.object(module,'census',return_value={'fixture_census':True}),\
             patch('bootstrap_auth.prove',return_value=auth or copy.deepcopy(self.fresh_auth)) as prove:
            result=module.preflight(self.kube,self.source,self.enrollment,self.contract,self.decode,installation=self.installation)
            self.assertEqual(prove.call_args.args[0],b'renewed-private-creds')
            return result
    def test_root_producer_binds_source_existing_enrollment_server_and_plan(self):
        from nats_contract_plan import digest
        result=self.run_plan();self.assertEqual(len(result['plan']['actions']),7)
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
    def test_v4_enrollment_is_supported_only_when_fresh_proof_matches_exactly(self):
        self.enrollment['auth_proof']=copy.deepcopy(self.fresh_auth)
        result=self.run_plan()
        from nats_contract_plan import digest
        self.assertEqual(result['binding']['enrollment_sha256'],digest(self.enrollment))
        changed=copy.deepcopy(self.fresh_auth);changed['normalized_consumers'][2]['config']['ack_policy']='all'
        with self.assertRaises(Blocked):self.run_plan(changed)
class CanonicalBootstrapTests(unittest.TestCase):
    def setUp(self):
        # Reuse only the external root/enrollment/broker fixture. Keep production
        # TARGET_FILES and the actual historical contract so a missing pin fails.
        Tests.setUp(self)
        repo=Path(__file__).resolve().parents[3]
        self.contract=json.loads((repo/'scripts/staging/nats-known-baseline/deployed-contract.json').read_bytes())
        for part in ('realtime','analytics-chat','notification','search'):
            relative='deploy/templates/nats-'+part+'-bootstrap.yaml'
            historical=Path(__file__).parent/'fixtures/v5-bootstrap'/Path(relative).name
            raw=historical.read_bytes()
            self.assertEqual(hashlib.sha256(raw).hexdigest(),V5_SOURCE_HASHES[relative])
            (self.source/relative).write_bytes(raw)
        from nats_contract_plan import VOICE_SUBJECTS_10,USER_SUBJECTS_10
        for name,subjects in [('voice_events',VOICE_SUBJECTS_10),('user_events',USER_SUBJECTS_10)]:
            info=copy.deepcopy(self.streams['chat_events'])
            info['config'].update(name=name,subjects=sorted(subjects));self.streams[name]=info
            self.tree['account_details'][0]['stream_detail'].append({'name':name,'created':CREATED,
                'config':info['config'],'state':dict(info['state'],consumer_count=0,bytes=100),'consumer_detail':[]})
    def run_plan(self):
        import yaml
        with patch.object(module,'monitor',return_value=self.tree),patch.object(module,'census',return_value={'fixture_census':True}),\
             patch('bootstrap_auth.prove',return_value=copy.deepcopy(self.fresh_auth)):
            return module.preflight(self.kube,self.source,self.enrollment,self.contract,yaml.safe_load,installation=self.installation)
    def compatible(self,authority):
        import compiler,yaml
        return compiler.compatible_bootstrap(self.source,self.contract,yaml.safe_load,
            {part:row['sha256'] for part,row in authority['scripts'].items()})
    def mutate(self,part):
        path=self.source/('deploy/templates/nats-'+part+'-bootstrap.yaml')
        path.write_bytes(path.read_bytes().replace(b'    set -eu',b'    set -eu\n    echo unreviewed-change'))
    def test_current_unapproved_source_tuple_is_refused_by_historical_root(self):
        repo=Path(__file__).resolve().parents[3]
        relative='deploy/templates/nats-realtime-bootstrap.yaml'
        raw=(repo/relative).read_bytes()
        self.assertNotEqual(hashlib.sha256(raw).hexdigest(),V5_SOURCE_HASHES[relative])
        (self.source/relative).write_bytes(raw)
        with self.assertRaisesRegex(Blocked,'nats_migration_root_preflight_refused'):
            self.run_plan()
    def test_canonical_search_authorized_by_root_then_accepted_by_compiler(self):
        authority=self.run_plan()
        paths=self.compatible(authority)
        self.assertIn('deploy/templates/nats-search-bootstrap.yaml',paths)
        self.assertEqual(set(authority['scripts']),{'realtime','analytics-chat','search','notification'})
        script=authority['scripts']['search']
        self.assertIn('require_stream_subjects chat_events chat.created chat.updated chat.deleted',script['script'])
        legacy=next(row['sha256'] for row in self.contract['scripts'] if row['part']=='search')
        self.assertNotEqual(script['sha256'],legacy)
        self.assertEqual(script['sha256'],hashlib.sha256(script['script'].encode()).hexdigest())
    def test_mutated_search_refused_by_root_before_compiler(self):
        self.mutate('search')
        with self.assertRaisesRegex(Blocked,'nats_migration_root_preflight_refused'):
            self.run_plan()
    def test_canonical_authority_survives_custody_and_next_rollout_readback(self):
        from unittest.mock import Mock
        import nats_contract_custody,nats_active_contract
        from nats_contract_custody_test import fixture
        authority=self.run_plan();self.compatible(authority)
        baseline_pairs=copy.deepcopy(self.contract['consumer_pairs'])
        kube,stage,_,rows=fixture();rows.clear()
        contract=copy.deepcopy(self.contract)
        # Mock only the prior ConfigMap bytes and Kubernetes CAS. Source pins,
        # extracted target scripts, plan and historical compiler check are real.
        for expected in contract['scripts']:
            part=expected['part']
            if part not in authority['scripts']:continue
            old='previous fixture '+part+' script\n'
            expected.update(sha256=hashlib.sha256(old.encode()).hexdigest(),bytes=len(old))
            name='voice-nats-'+part+'-bootstrap'
            rows[name]={'metadata':{'name':name,'uid':expected['configMapUID'],
                'resourceVersion':expected['configMapResourceVersion']},'data':{'bootstrap.sh':old}}
        state={'operation':'123456abcdef','contract':contract,
            'nats_contract':copy.deepcopy(authority['plan']),
            'nats_target_scripts':copy.deepcopy(authority['scripts']),
            'nats_migration':{'verified':True,'old_record_files_verified':True,'proof':{
                'schema':'voice-nats-additive-census-proof-v4','migration_id':authority['plan']['migration_id'],
                'before_sha256':'a'*64,'after_sha256':'b'*64,'messages':14,
                'old_consumers_preserved':len(baseline_pairs),'created_consumers':4}}}
        # These proof flags stand for the isolated native proof, not staging evidence.
        events=[];guard=Mock()
        active=nats_contract_custody.advance(kube,state,stage,events.append,mutation_guard=guard)
        self.assertEqual(set(kube.calls),{'voice-nats-'+part+'-bootstrap' for part in authority['scripts']})
        self.assertEqual(guard.call_count,4)
        self.assertEqual([row['part'] for row in events if row['kind']=='nats_contract_cm_applied'],list(authority['scripts']))
        # Round-trip the same persisted receipt consumed by the next prepare.
        active=json.loads(json.dumps(active));nats_active_contract.validate_identity(active,stage.expected,baseline_pairs)
        for expected in active['contract']['scripts']:
            if expected['part'] not in authority['scripts']:continue
            row=kube.get('configmap','voice-nats-'+expected['part']+'-bootstrap')
            self.assertEqual(expected['configMapUID'],row['metadata']['uid'])
            self.assertEqual(expected['configMapResourceVersion'],row['metadata']['resourceVersion'])
            self.assertEqual(expected['sha256'],hashlib.sha256(row['data']['bootstrap.sh'].encode()).hexdigest())
        self.contract=active['contract']
        # Next prepare observes the committed migration, not the old broker
        # fixture. Only external Kubernetes/monitor IO remains mocked.
        detail=self.tree['account_details'][0]['stream_detail']
        for action in authority['plan']['actions']:
            if action['api'].startswith('$JS.API.STREAM.UPDATE.'):
                next(row for row in detail if row['name']==action['object'])['config']=copy.deepcopy(action['after'])
            else:
                stream,durable=action['object'].split('/')
                normalized=next(row['config'] for row in self.fresh_auth['normalized_consumers'] if row['stream']==stream and row['durable']==durable)
                next(row for row in detail if row['name']==stream)['consumer_detail'].append({'name':durable,'durable':durable,'config':copy.deepcopy(normalized)})
        next_authority=self.run_plan();self.compatible(next_authority)
        self.assertEqual(next_authority['scripts'],authority['scripts'])
        self.assertEqual(next_authority['plan']['actions'],[])
    def test_unreviewed_notification_change_is_refused_by_root(self):
        self.mutate('notification')
        with self.assertRaisesRegex(Blocked,'nats_migration_root_preflight_refused'):
            self.run_plan()
    def test_notification_requires_fourth_root_hash_and_remains_toctou_bound(self):
        authority=self.run_plan()
        self.assertIn('notification',authority['scripts'])
        three=copy.deepcopy(authority);three['scripts'].pop('notification')
        with self.assertRaisesRegex(Blocked,'rollout_bootstrap_contract_unsupported'):self.compatible(three)
        self.mutate('notification')
        with self.assertRaisesRegex(Blocked,'rollout_bootstrap_contract_unsupported'):self.compatible(authority)
    def test_search_change_after_root_authority_refused_by_compiler(self):
        authority=self.run_plan()
        self.mutate('search')
        with self.assertRaisesRegex(Blocked,'rollout_bootstrap_contract_unsupported'):
            self.compatible(authority)

if __name__=='__main__':unittest.main()
