"""Fixed reminder-stream transition; source and server boundaries are fixtures."""
import copy
from pathlib import Path
import unittest

import nats_root_plan_test as fixtures
import nats_contract_plan as plan
import nats_root_plan as root
from controller import Blocked


class ReminderContractTests(unittest.TestCase):
    def setUp(self):
        fixture=fixtures.CanonicalBootstrapTests(methodName='runTest');fixture.setUp();self.addCleanup(fixture.doCleanups)
        self.source=fixture.source;self.streams=fixture.streams;self.fresh_auth=fixture.fresh_auth;self.enrollment=fixture.enrollment
        source=Path(__file__).resolve().parents[3]/'deploy/templates/nats-realtime-bootstrap.yaml'
        (self.source/'deploy/templates/nats-realtime-bootstrap.yaml').write_bytes(source.read_bytes())

    def test_fixed_new_stream_is_created_before_its_consumer(self):
        normalized={(stream,config['durable_name']):copy.deepcopy(config)
                    for stream,config in plan.V7_DECLARATIONS}
        stream={'name':'notification_reminder_hints','subjects':['notification.subscription_grace_reminder'],
                'storage':'file','retention':'limits','max_age':604800000000000,'duplicate_window':120000000000}
        record=plan.compile_plan(self.source,self.streams,{},normalized,
            version=plan.V7_SCHEMA,normalized_reminder_stream=stream)
        actions=record['actions']
        create=next(i for i,a in enumerate(actions) if a['api']=='$JS.API.STREAM.CREATE.notification_reminder_hints')
        consumer=next(i for i,a in enumerate(actions) if a['api']=='$JS.API.CONSUMER.CREATE.notification_reminder_hints.rt_realtime1_subscription_reminder')
        self.assertLess(create,consumer)
        self.assertIsNone(actions[create]['before'])
        self.assertEqual(actions[create]['after'],stream)

    def test_scratch_authority_binds_exact_reminder_config_and_enrollment(self):
        auth=copy.deepcopy(self.fresh_auth);auth['schema']='voice-bootstrap-auth-v6'
        stream,config=plan.V7_DECLARATIONS[-1]
        auth['normalized_consumers'].append({'stream':stream,'durable':config['durable_name'],'config':copy.deepcopy(config)})
        auth['normalized_consumers_sha256']=plan.digest(auth['normalized_consumers'])
        auth['normalized_reminder_stream']={'name':stream,'subjects':[plan.REMINDER_SUBJECT],'storage':'file',
            'retention':'limits','max_age':604800000000000,'duplicate_window':120000000000}
        auth['normalized_reminder_stream_sha256']=plan.digest(auth['normalized_reminder_stream'])
        self.assertEqual(len(root._validated_normalized_consumers(auth,self.enrollment['auth_proof'],version=plan.V7_SCHEMA)),5)
        auth['normalized_reminder_stream']['subjects']=['foreign']
        with self.assertRaises(Blocked):root._validated_normalized_consumers(auth,self.enrollment['auth_proof'],version=plan.V7_SCHEMA)

    def test_reminder_subject_overlap_fails_before_any_intent(self):
        normalized={(stream,config['durable_name']):copy.deepcopy(config)
                    for stream,config in plan.V7_DECLARATIONS}
        stream={'name':'notification_reminder_hints','subjects':['notification.subscription_grace_reminder'],
                'storage':'file','retention':'limits','max_age':604800000000000,'duplicate_window':120000000000}
        existing=copy.deepcopy(self.streams)
        existing['other']={'config':{'name':'other','subjects':['notification.>']}}
        with self.assertRaises(plan.ContractError):
            plan.compile_plan(self.source,existing,{},normalized,version=plan.V7_SCHEMA,normalized_reminder_stream=stream)

    def test_census_preserves_old_streams_and_new_stream_is_empty(self):
        normalized={(s,c['durable_name']):copy.deepcopy(c) for s,c in plan.V7_DECLARATIONS}
        stream={'name':plan.REMINDER_STREAM,'subjects':[plan.REMINDER_SUBJECT],'storage':'file',
                'retention':'limits','max_age':604800000000000,'duplicate_window':120000000000}
        record=plan.compile_plan(self.source,self.streams,{},normalized,version=plan.V7_SCHEMA,normalized_reminder_stream=stream)
        before={'schema':'voice-nats-census-v1','account':'fixture','streams':[],'consumers':[]}
        for name,info in self.streams.items():
            before['streams'].append({'name':name,'created':'old','config_sha256':record['observed'][name]['config_sha256'],
                                      'state':copy.deepcopy(info['state'])})
        after=copy.deepcopy(before)
        for action in record['actions']:
            if action['api'].startswith('$JS.API.STREAM.UPDATE.'):
                next(item for item in after['streams'] if item['name']==action['object'])['config_sha256']=plan.digest(action['after'])
        after['streams'].append({'name':plan.REMINDER_STREAM,'created':'new','config_sha256':plan.digest(stream),
            'state':{'messages':0,'bytes':0,'first_seq':0,'last_seq':0,'consumer_count':1}})
        for name,config in plan.V7_DECLARATIONS:
            durable=config['durable_name'];old=next((item for item in before['streams'] if item['name']==name),None)
            last=old['state']['last_seq'] if old else 0
            if old:next(item for item in after['streams'] if item['name']==name)['state']['consumer_count']+=1
            after['consumers'].append({'stream':name,'name':durable,'durable':durable,
                'config_sha256':plan.digest(normalized[(name,durable)]),'num_ack_pending':0,'num_redelivered':0,'num_pending':0,
                'delivered':{'consumer_seq':0,'stream_seq':last},'ack_floor':{'consumer_seq':0,'stream_seq':0}})
        self.assertEqual(plan.verify_census(record,before,after)['schema'],'voice-nats-additive-census-proof-v6')
        for kind in ('old-state','new-message','new-config','extra-stream'):
            altered=copy.deepcopy(after)
            if kind=='old-state':altered['streams'][0]['state']['last_seq']+=1
            if kind=='new-message':altered['streams'][-1]['state']['messages']=1
            if kind=='new-config':altered['streams'][-1]['config_sha256']='foreign'
            if kind=='extra-stream':altered['streams'].append({'name':'foreign'})
            with self.subTest(kind=kind),self.assertRaises(plan.ContractError):plan.verify_census(record,before,altered)

    def test_lost_create_reply_is_not_adopted_or_replayed(self):
        normalized={(s,c['durable_name']):copy.deepcopy(c) for s,c in plan.V7_DECLARATIONS}
        stream={'name':plan.REMINDER_STREAM,'subjects':[plan.REMINDER_SUBJECT],'storage':'file',
                'retention':'limits','max_age':604800000000000,'duplicate_window':120000000000}
        record=plan.compile_plan(self.source,self.streams,{},normalized,version=plan.V7_SCHEMA,normalized_reminder_stream=stream)
        action=next(a for a in record['actions'] if a['api']=='$JS.API.STREAM.CREATE.'+plan.REMINDER_STREAM)
        record['actions']=[action]
        class Actor:
            current=None;calls=0
            def assert_closed_isolation(self):pass
            def info_config(self,obj):return copy.deepcopy(self.current)
            def mutate(self,api,body):
                self.calls+=1;self.current=copy.deepcopy(body);raise RuntimeError('create_reply_lost')
        actor=Actor();events=[]
        with self.assertRaisesRegex(RuntimeError,'create_reply_lost'):plan.execute(record,actor,events.append)
        with self.assertRaises(plan.ContractError):plan.execute(record,actor,events.append)
        self.assertEqual(actor.calls,1)
        self.assertEqual([event['kind'] for event in events],['nats_contract_intent','nats_contract_mutation_issued'])
