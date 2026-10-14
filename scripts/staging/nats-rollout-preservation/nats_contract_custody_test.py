import copy,hashlib,unittest
from unittest.mock import Mock
from nats_contract_custody import advance,ANNOTATION
from controller import Blocked
from nats_contract_plan import DECLARATIONS,SCHEMA,MIGRATION_ID
def fixture():
    old='old fixed script\n';new='reviewed target script\n'
    sha=lambda s:hashlib.sha256(s.encode()).hexdigest()
    old_pair=['analytics_events','analytics_v2_telemetry']
    targets=[{'stream':stream,'name':config['durable_name'],'config_sha256':hashlib.sha256((stream+'/'+config['durable_name']).encode()).hexdigest()} for stream,config in DECLARATIONS]
    state={'operation':'123456abcdef','contract':{'scripts':[],'consumer_pairs':[old_pair]},
        'nats_contract':{'schema':SCHEMA,'migration_id':MIGRATION_ID,'sources':{},'consumer_targets':targets},
        'nats_target_scripts':{},'nats_migration':{'verified':True,'old_record_files_verified':True,'proof':{}}}
    rows={}
    for part in ('realtime','analytics-chat'):
        name='voice-nats-'+part+'-bootstrap'
        rows[name]={'metadata':{'name':name,'uid':part+'-uid','resourceVersion':'1'},'data':{'bootstrap.sh':old}}
        state['contract']['scripts'].append({'part':part,'configMapUID':part+'-uid','configMapResourceVersion':'1','sha256':sha(old),'bytes':len(old)})
        state['nats_target_scripts'][part]={'script':new,'sha256':sha(new)}
    class Kube:
        calls=[];crash=False
        def get(self,kind,name):return copy.deepcopy(rows[name])
        def cas(self,kind,row,changes):
            self.calls.append(row['metadata']['name']);current=rows[row['metadata']['name']]
            if current['metadata']!=row['metadata']:raise AssertionError('external drift')
            current['data']['bootstrap.sh']=changes[0]['value'];current['metadata']['annotations']=changes[1]['value'];current['metadata']['resourceVersion']='2'
            if self.crash:self.crash=False;raise KeyboardInterrupt()
            return copy.deepcopy(current)
    kube=Kube();kube.calls=[];stage=Mock();stage.expected={'generation':'r20260930a4','namespace_uid':'ns','source_claim_uid':'same-pvc'}
    return kube,stage,state,rows
class Tests(unittest.TestCase):
    def test_reviewed_search_script_advances_with_same_proof_and_cas(self):
        k,s,state,rows=fixture();events=[]
        old=copy.deepcopy(rows['voice-nats-realtime-bootstrap'])
        old['metadata'].update(name='voice-nats-search-bootstrap',uid='search-uid')
        rows['voice-nats-search-bootstrap']=old
        contract=copy.deepcopy(state['contract']['scripts'][0]);contract.update(part='search',configMapUID='search-uid')
        state['contract']['scripts'].append(contract)
        state['nats_target_scripts']['search']=copy.deepcopy(state['nats_target_scripts']['realtime'])
        receipt=advance(k,state,s,events.append)
        self.assertIn('voice-nats-search-bootstrap',k.calls)
        selected=next(row for row in receipt['contract']['scripts'] if row['part']=='search')
        self.assertEqual(selected['configMapUID'],'search-uid')
        self.assertEqual(selected['configMapResourceVersion'],'2')
        self.assertEqual(selected['sha256'],state['nats_target_scripts']['search']['sha256'])
        self.assertEqual([e['kind'] for e in events if e['part']=='search'],['nats_contract_cm_intent','nats_contract_cm_applied'])
    def test_unrelated_notification_script_is_not_authorized(self):
        k,s,state,rows=fixture()
        state['nats_target_scripts']={'notification':copy.deepcopy(state['nats_target_scripts']['realtime'])}
        with self.assertRaisesRegex(Blocked,'nats_contract_script_unbound'):advance(k,state,s,lambda e:None)
        self.assertEqual(k.calls,[])
    def test_requires_native_semantic_proof_before_cm_write(self):
        k,s,state,rows=fixture();state['nats_migration']['verified']=False
        with self.assertRaises(Blocked):advance(k,state,s,lambda e:None)
        self.assertEqual(k.calls,[])
    def test_crash_after_cas_adopts_only_owned_exact_target(self):
        k,s,state,rows=fixture();events=[];k.crash=True
        with self.assertRaises(KeyboardInterrupt):advance(k,state,s,events.append)
        self.assertEqual(events[-1]['kind'],'nats_contract_cm_intent')
        receipt=advance(k,state,s,events.append)
        self.assertEqual(len(k.calls),2);self.assertEqual(receipt['pvc_uid'],'same-pvc')
        self.assertEqual(receipt['contract']['consumer_pairs'],sorted([['analytics_events','analytics_v2_telemetry']]+[[stream,config['durable_name']] for stream,config in DECLARATIONS]))
    def test_only_exact_v3_targets_are_added_before_any_configmap_write(self):
        for mutate in (
            lambda plan:plan['consumer_targets'].pop(),
            lambda plan:plan['consumer_targets'][1].update(name='unexpected'),
            lambda plan:plan['consumer_targets'][2].update(config_sha256='x'),
        ):
            k,s,state,rows=fixture();mutate(state['nats_contract'])
            before=copy.deepcopy(rows)
            with self.subTest(plan=state['nats_contract']),self.assertRaises(Blocked):advance(k,state,s,lambda e:None)
            self.assertEqual(k.calls,[]);self.assertEqual(rows,before)
    def test_unowned_same_target_or_new_uid_refused(self):
        for drift in ('owner','uid'):
            k,s,state,rows=fixture();row=rows['voice-nats-realtime-bootstrap']
            row['data']['bootstrap.sh']=state['nats_target_scripts']['realtime']['script'];row['metadata']['resourceVersion']='2'
            row['metadata']['annotations']={ANNOTATION:'unowned'}
            if drift=='uid':row['metadata']['uid']='new'
            with self.assertRaises(Blocked):advance(k,state,s,lambda e:None)
            self.assertEqual(k.calls,[])
if __name__=='__main__':unittest.main()
