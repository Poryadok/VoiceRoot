import copy
import unittest
from pathlib import Path
from nats_contract_plan import compile_plan, execute, verify_census, ContractError, DECLARATION

SOURCE=Path(__file__).parents[3]
def live():
    chat=['chat.created','chat.member_changed','chat.dm_peer_deleted','space.tree_changed','space.created','voice.room_created','voice.room_deleted','space.invite_created','space.member_joined','space.member_left','space.updated','space.deleted']
    social=['social.friend_request','social.friend_accepted','social.friend_removed','social.user_blocked','social.contacts_synced']
    def info(name,subjects):
        return {'config':{'name':name,'subjects':subjects,'storage':'file','retention':'limits','max_age':604800000000000,'duplicate_window':120000000000,'max_msgs':-1,'metadata':{'preserve':'yes'}},'state':{'messages':7,'first_seq':11,'last_seq':17,'consumer_count':4}}
    return {'chat_events':info('chat_events',chat),'social_events':info('social_events',social)}
class Tests(unittest.TestCase):
    def plan(self,streams=None,consumer=None): return compile_plan(SOURCE,streams or live(),consumer,copy.deepcopy(DECLARATION))
    def test_exact_three_actions_preserve_other_config(self):
        p=self.plan();self.assertEqual([a['api'] for a in p['actions']],['$JS.API.STREAM.UPDATE.chat_events','$JS.API.STREAM.UPDATE.social_events','$JS.API.CONSUMER.CREATE.social_events.rt_realtime1_friend_removed'])
        for action in p['actions'][:2]:
            self.assertEqual(action['after']['metadata'],{'preserve':'yes'});self.assertEqual(action['after']['max_msgs'],-1)
        self.assertEqual(p['actions'][2]['after']['config']['deliver_policy'],'new')
    def test_exact_desired_retry_is_preserve(self):
        p=self.plan();s=live()
        for action in p['actions'][:2]: s[action['object']]['config']=action['after']
        self.assertEqual(self.plan(s,{'config':copy.deepcopy(DECLARATION)})['actions'],[])
    def test_unrelated_subject_drift_rejected(self):
        s=live();s['chat_events']['config']['subjects'].append('unknown.subject')
        with self.assertRaises(ContractError):self.plan(s)
    def test_intermediate_unapproved_window_rejected(self):
        s=live();s['social_events']['config']['duplicate_window']=300000000000
        with self.assertRaises(ContractError):self.plan(s)
    def test_existing_durable_default_drift_rejected(self):
        c={'config':dict(DECLARATION,max_ack_pending=5)}
        with self.assertRaises(ContractError):self.plan(consumer=c)
    def test_no_existing_record_state_in_mutation_payload(self):
        p=self.plan();self.assertTrue(all('state' not in a['after'] for a in p['actions']))
    def test_dynamic_stream_overlap_refused_before_plan(self):
        for subject in ('space.restored','space.*','space.>','>'):
            s=live();s['dynamic_extra']={'config':{'name':'dynamic_extra','subjects':[subject]}}
            with self.assertRaises(ContractError):self.plan(s)
    def test_nonoverlapping_dynamic_stream_preserved(self):
        s=live();s['dynamic_extra']={'config':{'name':'dynamic_extra','subjects':['space.other','other.>']}}
        self.assertEqual(len(self.plan(s)['actions']),3)
class Actor:
    def __init__(self,p):self.current={a['object']:copy.deepcopy(a['before']) for a in p['actions']};self.calls=[];self.safe=True;self.crash=False
    def assert_closed_isolation(self):
        if not self.safe:raise ContractError('isolation_failed')
    def info_config(self,obj):return copy.deepcopy(self.current[obj])
    def mutate(self,api,payload):
        self.calls.append(api)
        obj=api.split('.UPDATE.')[-1] if '.UPDATE.' in api else 'social_events/rt_realtime1_friend_removed'
        self.current[obj]=copy.deepcopy(payload.get('config',payload))
        if self.crash:self.crash=False;raise RuntimeError('process_interrupted_after_mutation')
class ExecuteTests(Tests):
    def test_isolation_veto_before_intent_or_mutation(self):
        p=self.plan();a=Actor(p);a.safe=False;j=[]
        with self.assertRaises(ContractError):execute(p,a,j.append)
        self.assertEqual((j,a.calls),([],[]))
    def test_intent_persisted_before_each_mutation(self):
        p=self.plan();a=Actor(p);j=[]
        original=a.mutate
        def mutate(api,payload):
            self.assertEqual([e['kind'] for e in j[-2:]],['nats_contract_intent','nats_contract_mutation_issued'])
            self.assertEqual(j[-1]['api'],api);self.assertEqual(j[-1]['target_sha256'],j[-2]['target_sha256'])
            original(api,payload)
        a.mutate=mutate;execute(p,a,j.append)
        self.assertEqual([x['kind'] for x in j],['nats_contract_intent','nats_contract_mutation_issued','nats_contract_applied']*3)
    def test_interrupted_after_update_retries_only_remaining_actions(self):
        p=self.plan();a=Actor(p);a.crash=True;j=[]
        with self.assertRaises(RuntimeError):execute(p,a,j.append)
        execute(p,a,j.append);self.assertEqual(len(a.calls),3)
    def test_unrelated_config_drift_never_adopted(self):
        p=self.plan();a=Actor(p);a.current['chat_events']['max_msgs']=1
        with self.assertRaises(ContractError):execute(p,a,lambda event:None)
        self.assertEqual(a.calls,[])
class CensusTests(Tests):
    def proof(self):
        p=self.plan(); before={'schema':'voice-nats-census-v1','account':'same-account','streams':[], 'consumers':[]}
        for name,info in live().items():
            before['streams'].append({'name':name,'created':'same-created','config_sha256':p['observed'][name]['config_sha256'],'state':copy.deepcopy(info['state'])})
        before['consumers']=[{'stream':'social_events','name':'existing','config_sha256':'old-config','ack_floor':{'consumer_seq':2,'stream_seq':12},'num_ack_pending':1,'num_redelivered':1,'num_pending':3}]
        after=copy.deepcopy(before)
        from nats_contract_plan import digest
        for a in p['actions'][:2]:next(s for s in after['streams'] if s['name']==a['object'])['config_sha256']=digest(a['after'])
        next(s for s in after['streams'] if s['name']=='social_events')['state']['consumer_count']+=1
        last=next(s for s in before['streams'] if s['name']=='social_events')['state']['last_seq']
        after['consumers'].append({'stream':'social_events','name':DECLARATION['name'],'durable':DECLARATION['name'],'config_sha256':p['normalized_consumer_sha256'],'num_ack_pending':0,'num_redelivered':0,'num_pending':0,
            'delivered':{'consumer_seq':0,'stream_seq':last},'ack_floor':{'consumer_seq':0,'stream_seq':0}})
        return p,before,after
    def test_exact_config_delta_preserves_old_records_and_ack(self):
        p,b,a=self.proof();self.assertEqual(verify_census(p,b,a)['messages'],14)
    def test_new_durable_wrong_initial_delivery_or_ack_floor_refused(self):
        for field in ('delivered','ack_floor'):
            for seq in ('consumer_seq','stream_seq'):
                p,b,a=self.proof();last=next(s for s in b['streams'] if s['name']=='social_events')['state']['last_seq']
                row=a['consumers'][-1]
                row['delivered']={'consumer_seq':0,'stream_seq':last};row['ack_floor']={'consumer_seq':0,'stream_seq':0}
                row[field][seq]+=1
                with self.assertRaises(ContractError):verify_census(p,b,a)
    def test_old_record_sequence_or_ack_change_refused(self):
        for field in ('sequence','ack'):
            p,b,a=self.proof()
            if field=='sequence':a['streams'][0]['state']['last_seq']+=1
            else:a['consumers'][0]['num_ack_pending']=0
            with self.assertRaises(ContractError):verify_census(p,b,a)
    def test_missing_dynamic_consumer_or_wrong_account_refused(self):
        for field in ('consumer','account'):
            p,b,a=self.proof()
            if field=='consumer':a['consumers'].pop(0)
            else:a['account']='different-account'
            with self.assertRaises(ContractError):verify_census(p,b,a)
    def test_extra_new_consumer_or_config_drift_refused(self):
        for field in ('consumer','config'):
            p,b,a=self.proof()
            if field=='consumer':a['consumers'].append({'stream':'social_events','name':'extra'})
            else:a['streams'][0]['config_sha256']='foreign'
            with self.assertRaises(ContractError):verify_census(p,b,a)
if __name__=='__main__':unittest.main()
