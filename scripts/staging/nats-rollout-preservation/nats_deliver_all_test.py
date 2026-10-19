import copy
import unittest

import nats_deliver_all as module
from nats_contract_plan import V6_SCHEMA,V6_MIGRATION_ID,digest,ContractError
from docker_runtime import NATS_IMAGE

def fixture():
    cfg={'name':'voice_events','subjects':['voice.roster_membership_changed','voice.state_changed'],
        'storage':'file','retention':'limits','max_age':604800000000000}
    original={'name':'voice_events','created':'2026-10-18T23:00:00Z','config_sha256':digest(cfg),
        'state':{'messages':4,'first_seq':3,'last_seq':8,'consumer_count':0},'observations':{}}
    p={'schema':V6_SCHEMA,'migration_id':V6_MIGRATION_ID,'sources':{'fixed':'a'*64},
        'consumer_targets':[{'stream':module.STREAM,'name':module.DURABLE,'config_sha256':'b'*64}]}
    census={'schema':'voice-nats-census-v1','account':'A'+'A'*55,'streams':[original],'consumers':[]}
    manifest={'files':[],'dirs':[],'archive_sha256':'c'*64}
    state={'operation':'123456abcdef','nats_contract':p,'bootstrap_enrollment':{'retained':True},
        'cut':{'census':census,'census_sha256':digest(census),'manifest':manifest,
            'manifest_sha256':digest(manifest),'archive_sha256':'c'*64}}
    state['nats_contract_binding']={'plan_sha256':digest(p),'enrollment_sha256':digest(state['bootstrap_enrollment']),
        'live_auth_sha256':'d'*64,'sources':p['sources'],'server_image':NATS_IMAGE}
    response={'type':'io.nats.jetstream.api.v1.stream_info_response','offset':0,'limit':100000,
        'total':1,'config':cfg,'created':original['created'],
        'state':dict(original['state'],subjects={module.SUBJECT:2})}
    restore={'container_id':'e'*64,'server_image':NATS_IMAGE,'started_at':'2026-10-18T23:00:01Z',
        'finished_at':'2026-10-18T23:00:02Z','native_message_sha256':'f'*64}
    return state,response,restore

class CountAndCut(unittest.TestCase):
    def test_count_is_exact_literal_retained_matches_not_total_or_sequence_span(self):
        state,response,restore=fixture()
        cut=module.make_cut(state,response,restore)
        self.assertEqual(cut['matching_messages'],2)
        self.assertEqual(cut['original_stream']['state']['messages'],4)
        self.assertEqual(cut['bindings']['census_sha256'],state['cut']['census_sha256'])
        self.assertEqual(cut['count_response_sha256'],digest(response))
    def test_changed_query_shape_state_identity_and_count_fail_closed(self):
        for field in ('type','offset','limit','total','config','created','state','zero','subject','negative','extra'):
            state,response,restore=fixture()
            if field=='type':response['type']='other'
            if field=='offset':response['offset']=1
            if field=='limit':response['limit']=1
            if field=='total':response['total']=2
            if field=='config':response['config']['subjects'].append('voice.>')
            if field=='created':response['created']='2026-10-18T23:00:03Z'
            if field=='state':response['state']['last_seq']+=1
            if field=='zero':response['total']=0
            if field=='subject':response['state']['subjects']={'voice.*':2}
            if field=='negative':response['state']['subjects'][module.SUBJECT]=-1
            if field=='extra':response['unreviewed']=True
            with self.subTest(field=field),self.assertRaises(ContractError):module.make_cut(state,response,restore)
    def test_successful_zero_subjects_requires_explicit_untruncated_zero_shape(self):
        state,response,restore=fixture();response.update(total=0,limit=0)
        response['state'].pop('subjects')
        self.assertEqual(module.make_cut(state,response,restore)['matching_messages'],0)
        response['limit']=100000
        with self.assertRaises(ContractError):module.make_cut(state,response,restore)
        response['limit']=0
        response.pop('total')
        with self.assertRaises(ContractError):module.make_cut(state,response,restore)
    def test_original_cut_and_enrollment_cannot_be_recaptured_on_retry(self):
        for field in ('manifest','census','archive','plan','enrollment','auth'):
            state,response,restore=fixture();cut=module.make_cut(state,response,restore)
            if field=='manifest':state['cut']['manifest']['files'].append({'path':'foreign'})
            if field=='census':state['cut']['census']['streams'][0]['state']['messages']+=1
            if field=='archive':state['cut']['archive_sha256']='a'*64
            if field=='plan':state['nats_contract']['sources']['foreign']='a'*64
            if field=='enrollment':state['bootstrap_enrollment']['retained']=False
            if field=='auth':state['nats_contract_binding']['live_auth_sha256']='a'*64
            with self.subTest(field=field),self.assertRaises(ContractError):module.validate_cut(cut,state)
    def test_partial_native_roster_directory_is_not_original_absence(self):
        state,response,restore=fixture()
        root='jetstream/'+state['cut']['census']['account']+'/streams/voice_events/obs/'+module.DURABLE
        state['cut']['manifest']['dirs'].append(root)
        state['cut']['manifest_sha256']=digest(state['cut']['manifest'])
        with self.assertRaises(ContractError):module.make_cut(state,response,restore)

class InitialState(unittest.TestCase):
    def rows(self):
        state,response,restore=fixture();cut=module.make_cut(state,response,restore)
        row={'stream':module.STREAM,'name':module.DURABLE,'durable':module.DURABLE,
            'config_sha256':'b'*64,'created':'2026-10-18T23:00:04Z','inactive_threshold':0,
            'delivered':{'consumer_seq':0,'stream_seq':2},'ack_floor':{'consumer_seq':0,'stream_seq':0},
            'num_pending':2,'num_ack_pending':0,'num_redelivered':0,'observations':{'num_waiting':0}}
        native={'schema':'nats-closed-durable-consumer-v1','version':2,
            'delivered':{'consumer':0,'stream':2},'ack_floor':{'consumer':0,'stream':0},'pending':{},'redelivered':{}}
        return cut,row,native
    def test_native_qualified_first_sequence_above_one_uses_zero_ack_floor(self):
        cut,row,native=self.rows();module.untouched_initial(cut,row,native)
        row['ack_floor']['stream_seq']=2
        with self.assertRaises(ContractError):module.untouched_initial(cut,row,native)
    def test_activity_or_forged_pending_never_passes_preservation(self):
        for field in ('pending','delivered','waiting','push','native','ack','active'):
            cut,row,native=self.rows()
            if field=='pending':row['num_pending']=0
            if field=='delivered':row['delivered']['consumer_seq']=1
            if field=='waiting':row['observations']['num_waiting']=1
            if field=='push':row['observations']['push_bound']=True
            if field=='native':native['pending']={'3':{'consumer_sequence':1,'timestamp_ns':1792364400000000000}}
            if field=='ack':native['ack_floor']['stream']=1
            if field=='active':row['observations']['delivered_last_active']='2026-10-18T23:00:05Z'
            with self.subTest(field=field),self.assertRaises((ContractError,ValueError)):module.untouched_initial(cut,row,native)
    def test_pristine_and_advanced_empty_streams_keep_distinct_start(self):
        for f,l,want in ((0,0,1),(6,5,6),(3,8,3)):
            m=0 if f in (0,6) else 4
            self.assertEqual(module.start_sequence({'first_seq':f,'last_seq':l,'messages':m}),want)
        for state in ({'first_seq':0,'last_seq':5,'messages':0},
                      {'first_seq':3,'last_seq':8,'messages':7}):
            with self.assertRaises(ContractError):module.start_sequence(state)

if __name__=='__main__':unittest.main()
