"""A3 authority must be new, exact and unable to widen historical versions."""
import copy
import shutil
import tempfile
from pathlib import Path
import unittest

import nats_contract_plan as plan
from nats_contract_plan_test import live

V6='voice-nats-additive-contract-v6'
ROSTER=('voice_events',{'name':'matchmaking_voice_roster_v1',
    'durable_name':'matchmaking_voice_roster_v1',
    'filter_subject':'voice.roster_membership_changed',
    'deliver_policy':'all','ack_policy':'explicit'})
SOURCE=Path(__file__).parent/'testdata/a3-roster-source'

class Planner(unittest.TestCase):
    def inputs(self,voice=12):
        streams=live()
        for name,subjects in [('voice_events',sorted(plan.VOICE_SUBJECTS_10)),
                              ('user_events',sorted(plan.USER_SUBJECTS_14))]:
            info=copy.deepcopy(streams['chat_events'])
            info['config'].update(name=name,subjects=subjects)
            streams[name]=info
        if voice>=12:streams['voice_events']['config']['subjects']+=list(plan.VOICE_ADDITIONS)
        if voice==13:streams['voice_events']['config']['subjects'].append(ROSTER[1]['filter_subject'])
        declarations=plan.V4_DECLARATIONS+(ROSTER,)
        normalized={(s,c['durable_name']):copy.deepcopy(c) for s,c in declarations}
        current={k:{'config':copy.deepcopy(v)} for k,v in normalized.items() if k[1]!=ROSTER[1]['name']}
        return streams,current,normalized
    def compile(self,voice=12):
        return plan.compile_plan(SOURCE,*self.inputs(voice),version=V6)
    def test_roster_has_separate_version_and_exact_pull_all_declaration(self):
        result=self.compile()
        self.assertEqual(result['schema'],V6)
        self.assertEqual(len(result['sources']),4)
        action=next(a for a in result['actions'] if a['object']=='voice_events/matchmaking_voice_roster_v1')
        self.assertEqual(action['after']['config'],ROSTER[1])
        voice=next(a for a in result['actions'] if a['object']=='voice_events')
        self.assertEqual(voice['after']['subjects'],voice['before']['subjects']+['voice.roster_membership_changed'])
        self.assertEqual(len(result['consumer_targets']),5)
    def test_complete_reviewed_voice_predecessors_only(self):
        for size in (10,12,13):
            with self.subTest(size=size):self.compile(size)
        for extra in ('voice.*','voice.>','voice.unreviewed','voice.participant_joined'):
            streams,current,normalized=self.inputs()
            streams['voice_events']['config']['subjects'].append(extra)
            with self.subTest(extra=extra),self.assertRaises(plan.ContractError):
                plan.compile_plan(SOURCE,streams,current,normalized,version=V6)
    def test_v5_cannot_select_roster_tuple_or_consumer(self):
        streams,current,normalized=self.inputs()
        with self.assertRaises(plan.ContractError):
            plan.compile_plan(SOURCE,streams,current,normalized,version=plan.SCHEMA)
    def test_noop_after_exact_successor_preserves_existing_consumer(self):
        streams,current,normalized=self.inputs()
        result=plan.compile_plan(SOURCE,streams,current,normalized,version=V6)
        for action in result['actions']:
            if '.STREAM.UPDATE.' in action['api']:streams[action['object']]['config']=action['after']
            else:
                s,d=action['object'].split('/')
                current[(s,d)]={'config':copy.deepcopy(normalized[(s,d)])}
        self.assertEqual(plan.compile_plan(SOURCE,streams,current,normalized,version=V6)['actions'],[])
    def test_each_of_four_sources_is_exact_not_independently_repinable(self):
        for part in ('realtime','analytics-chat','search','notification'):
            with self.subTest(part=part),tempfile.TemporaryDirectory() as td:
                source=Path(td)/'source';shutil.copytree(SOURCE,source)
                p=source/('deploy/templates/nats-'+part+'-bootstrap.yaml')
                p.write_bytes(p.read_bytes()+b'\n# unrelated change\n')
                with self.assertRaises(plan.ContractError):plan.compile_plan(source,*self.inputs(),version=V6)
    def test_server_normalization_cannot_turn_pull_into_push_or_multifilter(self):
        for field,value in [('deliver_subject','_INBOX.other'),('deliver_group','other'),
                            ('filter_subjects',['voice.other'])]:
            streams,current,normalized=self.inputs()
            normalized[('voice_events',ROSTER[1]['name'])][field]=value
            with self.subTest(field=field),self.assertRaises(plan.ContractError):
                plan.compile_plan(SOURCE,streams,current,normalized,version=V6)

class ActorBoundaries(unittest.TestCase):
    def actor(self,runtime,version=V6):
        from nats_contract_actor import Actor
        return Actor(runtime,'owned-broker','owned-id',Path('/selected-pvc'),lambda:None,
            plan={'schema':version,'migration_id':plan.V6_MIGRATION_ID if version==V6 else plan.MIGRATION_ID})
    def test_exact_v6_roster_create_reaches_actor_without_widening_v5(self):
        from nats_contract_actor_test import Runtime
        api='$JS.API.CONSUMER.CREATE.voice_events.matchmaking_voice_roster_v1'
        payload={'stream_name':'voice_events','action':'create','config':copy.deepcopy(ROSTER[1])}
        actor=self.actor(Runtime());calls=[];actor._request=lambda *args,**kw:calls.append(args)
        actor.mutate(api,payload)
        self.assertEqual(calls,[(api,payload)])
        older=self.actor(Runtime(),plan.SCHEMA)
        older._request=lambda *args,**kw:self.fail('v5 must not issue v6 API')
        with self.assertRaises(plan.ContractError):older.mutate(api,payload)
    def test_v6_missing_info_accepts_only_bound_consumer_not_stream(self):
        from nats_contract_actor_test import RequestRuntime
        actor=self.actor(RequestRuntime({'error':{'err_code':10014}}))
        self.assertIsNone(actor.info_config('voice_events/matchmaking_voice_roster_v1'))
        with self.assertRaises(plan.ContractError):actor.info_config('voice_events')

class RootAuthority(unittest.TestCase):
    def setUp(self):
        from nats_root_plan_test import CanonicalBootstrapTests
        CanonicalBootstrapTests.setUp(self)
        for part in ('realtime','analytics-chat','search','notification'):
            path='deploy/templates/nats-'+part+'-bootstrap.yaml'
            shutil.copyfile(SOURCE/path,self.source/path)
        self.enrollment['auth_proof']=copy.deepcopy(self.fresh_auth)
        self.fresh_auth['schema']='voice-bootstrap-auth-v5'
        self.fresh_auth['normalized_consumers'].append({'stream':ROSTER[0],
            'durable':ROSTER[1]['durable_name'],'config':copy.deepcopy(ROSTER[1])})
        self.fresh_auth['normalized_consumers_sha256']=plan.digest(self.fresh_auth['normalized_consumers'])
    def run_plan(self,auth=None):
        import yaml,nats_root_plan
        from unittest.mock import patch
        with patch.object(nats_root_plan,'monitor',return_value=self.tree),\
             patch.object(nats_root_plan,'census',return_value={'fixture_census':True}),\
             patch('bootstrap_auth.prove',return_value=copy.deepcopy(auth or self.fresh_auth)) as prove:
            result=nats_root_plan.preflight(self.kube,self.source,self.enrollment,self.contract,yaml.safe_load,installation=self.installation)
            self.assertEqual(prove.call_args.kwargs['version'],'voice-bootstrap-auth-v5')
            return result
    def test_exact_four_target_sources_select_new_authority_and_keep_enrollment(self):
        old=copy.deepcopy(self.enrollment)
        authority=self.run_plan()
        self.assertEqual(authority['plan']['schema'],V6)
        self.assertEqual(authority['plan']['sources'],plan.V6_SOURCE_HASHES)
        self.assertEqual(set(authority['scripts']),{'realtime','analytics-chat','search','notification'})
        self.assertEqual(authority['binding']['live_auth_sha256'],plan.digest(self.fresh_auth))
        self.assertEqual(self.enrollment,old)
    def test_fifth_live_row_cannot_rewrite_old_enrolled_defaults(self):
        from controller import Blocked
        changed=copy.deepcopy(self.fresh_auth)
        changed['normalized_consumers'][1]['config']['max_ack_pending']=123
        changed['normalized_consumers_sha256']=plan.digest(changed['normalized_consumers'])
        with self.assertRaises(Blocked):self.run_plan(changed)
    def test_v4_or_partial_live_proof_cannot_authorize_v6(self):
        from controller import Blocked
        for fault in ('schema','missing','reordered','duplicate','extra'):
            auth=copy.deepcopy(self.fresh_auth)
            if fault=='schema':auth['schema']='voice-bootstrap-auth-v4'
            if fault=='missing':auth['normalized_consumers'].pop()
            if fault=='reordered':auth['normalized_consumers'].reverse()
            if fault=='duplicate':auth['normalized_consumers'][-1]=auth['normalized_consumers'][0]
            if fault=='extra':auth['unreviewed']=True
            auth['normalized_consumers_sha256']=plan.digest(auth['normalized_consumers'])
            with self.subTest(fault=fault),self.assertRaises(Blocked):self.run_plan(auth)
    def test_root_census_four_cas_and_serialized_active_receipt_use_one_v6_identity(self):
        import hashlib,json,nats_deliver_all
        from nats_contract_custody import advance
        from nats_contract_custody_test import fixture as custody_fixture
        from nats_active_contract import validate_identity
        from controller import Blocked
        authority=self.run_plan();p=authority['plan']
        for part,row in authority['scripts'].items():
            self.assertEqual(row['sha256'],plan.V6_SCRIPT_HASHES[part])
            self.assertEqual(len(row['script'].encode()),plan.V6_SCRIPT_BYTES[part])
        before={'schema':'voice-nats-census-v1','account':'A'+'A'*55,'streams':[],'consumers':[]}
        for name,row in self.streams.items():
            before['streams'].append({'name':name,'created':'2026-10-18T23:00:00Z',
                'config_sha256':plan.digest(row['config']),
                'state':dict(row['state'],consumer_count=0),'observations':{}})
        after=copy.deepcopy(before)
        targets={(r['stream'],r['name']):r['config_sha256'] for r in p['consumer_targets']}
        for action in p['actions']:
            if '.STREAM.UPDATE.' in action['api']:
                next(r for r in after['streams'] if r['name']==action['object'])['config_sha256']=plan.digest(action['after'])
            else:
                stream,durable=action['object'].split('/')
                original=next(r for r in before['streams'] if r['name']==stream)
                next(r for r in after['streams'] if r['name']==stream)['state']['consumer_count']+=1
                roster=durable==ROSTER[1]['name']
                after['consumers'].append({'stream':stream,'name':durable,'durable':durable,
                    'config_sha256':targets[(stream,durable)],'created':'2026-10-18T23:00:04Z','inactive_threshold':0,
                    'delivered':{'consumer_seq':0,'stream_seq':original['state']['first_seq']-1 if roster else original['state']['last_seq']},
                    'ack_floor':{'consumer_seq':0,'stream_seq':0},'num_pending':2 if roster else 0,
                    'num_ack_pending':0,'num_redelivered':0,'observations':{'num_waiting':0}})
        original=next(r for r in before['streams'] if r['name']=='voice_events')
        manifest={'files':[],'dirs':[],'archive_sha256':'c'*64}
        state={'operation':'123456abcdef','nats_contract':p,'nats_contract_binding':authority['binding'],
            'bootstrap_enrollment':self.enrollment,'cut':{'manifest':manifest,'manifest_sha256':plan.digest(manifest),
                'census':before,'census_sha256':plan.digest(before)}}
        response={'type':'io.nats.jetstream.api.v1.stream_info_response','offset':0,'limit':100000,'total':1,
            'config':copy.deepcopy(self.streams['voice_events']['config']),'created':original['created'],
            'state':dict(original['state'],subjects={ROSTER[1]['filter_subject']:2})}
        from docker_runtime import NATS_IMAGE
        attempt={'container_id':'e'*64,'server_image':NATS_IMAGE,'started_at':'2026-10-18T23:00:03Z','finished_at':'2026-10-18T23:00:05Z'}
        cut=nats_deliver_all.make_cut(state,response,{**attempt,'native_message_sha256':'f'*64})
        row=next(r for r in after['consumers'] if r['name']==ROSTER[1]['name'])
        action=next(a for a in p['actions'] if a['object']=='voice_events/'+ROSTER[1]['name'])
        root='jetstream/'+before['account']+'/streams/voice_events/obs/'+ROSTER[1]['name']
        evidence={'stream':'voice_events','name':ROSTER[1]['name'],'cut_sha256':plan.digest(cut),
            'plan_sha256':plan.digest(p),'operation':state['operation'],'action_sha256':plan.digest(action),
            'target_config_sha256':row['config_sha256'],'attempt':attempt,'issued_event_sha256':'f'*64,
            'native_created':row['created'],'native_members':{root+'/'+n:'a'*64 for n in ('meta.inf','meta.sum','o.dat')},
            'native_state':{'schema':'nats-closed-durable-consumer-v1','version':2,
                'delivered':{'consumer':0,'stream':original['state']['first_seq']-1},
                'ack_floor':{'consumer':0,'stream':0},'pending':{},'redelivered':{}},'restored_row_sha256':plan.digest(row)}
        proof=plan.verify_census(p,before,after,deliver_all={'cut':cut,'created':[evidence],'authority_plan':p})
        kube,stage,unused,rows=custody_fixture();rows.clear();contract=copy.deepcopy(self.contract)
        baseline=copy.deepcopy(contract['consumer_pairs'])
        for expected in contract['scripts']:
            part=expected['part'];old='old fixture '+part+'\n'
            expected.update(sha256=hashlib.sha256(old.encode()).hexdigest(),bytes=len(old))
            rows['voice-nats-'+part+'-bootstrap']={'metadata':{'name':'voice-nats-'+part+'-bootstrap',
                'uid':expected['configMapUID'],'resourceVersion':expected['configMapResourceVersion']},'data':{'bootstrap.sh':old}}
        state.update(contract=contract,nats_target_scripts=authority['scripts'],nats_deliver_all_cut=cut,
            nats_migration={'verified':True,'old_record_files_verified':True,'proof':proof})
        active=advance(kube,state,stage,lambda e:None)
        self.assertEqual(len(kube.calls),4)
        active=json.loads(json.dumps(active));validate_identity(active,stage.expected,baseline)
        for fault in ('proofversion','missingnotification','script','operation','native'):
            changed=copy.deepcopy(active)
            if fault=='proofversion':changed['proof']['schema']='voice-nats-additive-census-proof-v4'
            if fault=='missingnotification':changed['sources'].pop('deploy/templates/nats-notification-bootstrap.yaml')
            if fault=='script':changed['contract']['scripts'][0]['bytes']+=1
            if fault=='operation':changed['operation']='foreign123'
            if fault=='native':changed['proof']['created_consumer_evidence'][0]['native_state']['ack_floor']['stream']=1
            with self.subTest(fault=fault),self.assertRaises(Blocked):validate_identity(changed,stage.expected,baseline)

class CensusFailClosed(unittest.TestCase):
    def test_v6_cannot_reuse_new_policy_zero_pending_predicate_without_original_cut(self):
        from nats_contract_plan_test import CensusTests
        p,b,a=CensusTests().proof()
        p.update(schema=V6,migration_id=plan.V6_MIGRATION_ID)
        stream={'name':'voice_events','created':'same-created','config_sha256':'b'*64,
            'state':{'messages':3,'first_seq':3,'last_seq':5,'consumer_count':0}}
        b['streams'].append(copy.deepcopy(stream));a['streams'].append(copy.deepcopy(stream))
        a['streams'][-1]['state']['consumer_count']=1
        target={'stream':ROSTER[0],'name':ROSTER[1]['name'],'config_sha256':plan.digest(ROSTER[1])}
        p['consumer_targets'].append(target)
        p['actions'].append({'api':'$JS.API.CONSUMER.CREATE.voice_events.matchmaking_voice_roster_v1',
            'object':'voice_events/matchmaking_voice_roster_v1','before':None,
            'after':{'stream_name':ROSTER[0],'action':'create','config':ROSTER[1]},
            'normalized_config_sha256':target['config_sha256']})
        # This is the OLD deliver-new predicate. It must never prove all-policy.
        a['consumers'].append({**target,'durable':ROSTER[1]['name'],
            'num_ack_pending':0,'num_redelivered':0,'num_pending':0,
            'delivered':{'consumer_seq':0,'stream_seq':5},
            'ack_floor':{'consumer_seq':0,'stream_seq':0}})
        with self.assertRaises(plan.ContractError):plan.verify_census(p,b,a)

def all_census_fixture():
    from nats_contract_plan_test import CensusTests
    from nats_deliver_all_test import fixture as all_fixture
    import nats_deliver_all as all_policy
    p,b,a=CensusTests().proof();state,response,restore=all_fixture()
    p.update(schema=V6,migration_id=plan.V6_MIGRATION_ID)
    b['account']=a['account']='A'+'A'*55
    stream=copy.deepcopy(state['cut']['census']['streams'][0])
    b['streams'].append(copy.deepcopy(stream));a['streams'].append(copy.deepcopy(stream))
    a['streams'][-1]['state']['consumer_count']=1
    target={'stream':ROSTER[0],'name':ROSTER[1]['name'],'config_sha256':plan.digest(ROSTER[1])}
    p['consumer_targets'].append(target)
    action={'api':'$JS.API.CONSUMER.CREATE.voice_events.matchmaking_voice_roster_v1',
        'object':'voice_events/matchmaking_voice_roster_v1','before':None,
        'after':{'stream_name':ROSTER[0],'action':'create','config':ROSTER[1]},
        'normalized_config_sha256':target['config_sha256']}
    p['actions'].append(action)
    state['nats_contract']=p
    state['nats_contract_binding'].update(plan_sha256=plan.digest(p),sources=p['sources'])
    state['cut'].update(census=b,census_sha256=plan.digest(b))
    cut=all_policy.make_cut(state,response,restore)
    row={**target,'durable':ROSTER[1]['name'],'created':'2026-10-18T23:00:04Z','inactive_threshold':0,
        'num_ack_pending':0,'num_redelivered':0,'num_pending':2,
        'delivered':{'consumer_seq':0,'stream_seq':2},'ack_floor':{'consumer_seq':0,'stream_seq':0},
        'observations':{'num_waiting':0}}
    a['consumers'].append(row)
    root='jetstream/'+b['account']+'/streams/voice_events/obs/'+ROSTER[1]['name']
    evidence={'stream':ROSTER[0],'name':ROSTER[1]['name'],'cut_sha256':plan.digest(cut),
        'plan_sha256':plan.digest(p),'operation':state['operation'],'action_sha256':plan.digest(action),
        'target_config_sha256':target['config_sha256'],
        'attempt':{'container_id':'e'*64,'server_image':restore['server_image'],
            'started_at':'2026-10-18T23:00:03Z','finished_at':'2026-10-18T23:00:05Z'},
        'issued_event_sha256':'f'*64,'native_created':row['created'],
        'native_members':{root+'/'+n:'a'*64 for n in ('meta.inf','meta.sum','o.dat')},
        'native_state':{'schema':'nats-closed-durable-consumer-v1','version':2,
            'delivered':{'consumer':0,'stream':2},'ack_floor':{'consumer':0,'stream':0},'pending':{},'redelivered':{}},
        'restored_row_sha256':plan.digest(row)}
    return p,b,a,{'cut':cut,'created':[evidence],'authority_plan':p}

class CensusAllPolicy(unittest.TestCase):
    def test_cut_bound_two_matching_messages_pass_with_holes_and_zero_native_ack(self):
        p,b,a,evidence=all_census_fixture()
        proof=plan.verify_census(p,b,a,deliver_all=evidence)
        self.assertEqual(proof['schema'],'voice-nats-additive-census-proof-v5')
        self.assertEqual(proof['created_consumers'],5)
        self.assertEqual(proof['deliver_all_cut_sha256'],plan.digest(evidence['cut']))
    def test_full_row_activity_is_checked_before_semantic_removes_observations(self):
        p,b,a,evidence=all_census_fixture()
        a['consumers'][-1]['observations']['num_waiting']=1
        evidence['created'][0]['restored_row_sha256']=plan.digest(a['consumers'][-1])
        with self.assertRaises(plan.ContractError):plan.verify_census(p,b,a,deliver_all=evidence)
    def test_stale_cut_and_foreign_created_attempt_are_rejected(self):
        for fault in ('count','plan','cut','attempt','native','hash'):
            p,b,a,evidence=all_census_fixture()
            if fault=='count':evidence['cut']['matching_messages']=3
            if fault=='plan':evidence['cut']['bindings']['plan_sha256']='a'*64
            if fault=='cut':evidence['cut']['bindings']['census_sha256']='a'*64
            if fault=='attempt':evidence['created'][0]['attempt']['finished_at']='2026-10-18T23:00:03Z'
            if fault=='native':evidence['created'][0]['native_state']['ack_floor']['stream']=2
            if fault=='hash':evidence['created'][0]['action_sha256']='0'*64
            with self.subTest(fault=fault),self.assertRaises(plan.ContractError):plan.verify_census(p,b,a,deliver_all=evidence)

if __name__=='__main__':unittest.main()
