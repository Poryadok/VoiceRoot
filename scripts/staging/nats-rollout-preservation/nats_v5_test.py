"""V5 regression: real source authority, immutable historical contract."""
import copy, hashlib, unittest
from nats_root_plan_test import CanonicalBootstrapTests
from nats_contract_plan_test import live
import nats_contract_plan as plan
from controller import Blocked

VOICE_10='voice.call_incoming voice.call_accepted voice.call_declined voice.call_missed voice.call_ended voice.state_changed voice.screen_share_started voice.screen_share_stopped voice.call_started voice.member_joined'.split()
USER_10='user.account_deleted user.account_restored user.guest_converted user.profile_created user.profile_updated user.profile_switched user.verified user.presence_changed user.game_detected user.settings_changed'.split()

class V5(CanonicalBootstrapTests):
    def setUp(self):
        super().setUp()
        # The literal old sets are checked against reviewed template target minus
        # only the six accepted additions below, never arbitrary caller subsets.
        import re
        text=(self.source/'deploy/templates/nats-realtime-bootstrap.yaml').read_text()
        self.voice=re.search(r'^    stream voice_events (.+)$',text,re.M)[1].split()
        self.user=re.search(r'^    stream user_events (.+)$',text,re.M)[1].split()
        self.additions={'voice_events':['voice.participant_joined','voice.participant_left'],
            'user_events':['user.registered','user.logged_in','user.logged_out','user.2fa_enabled']}
        self.assertEqual(set(self.streams['voice_events']['config']['subjects']),set(VOICE_10))
        self.assertEqual(set(self.streams['user_events']['config']['subjects']),set(USER_10))
    def test_v5_exact_independent_combinations_and_complete_source_binding(self):
        from nats_contract_plan import compile_plan, DECLARATIONS
        baseline=copy.deepcopy(self.streams)
        normalized={(s,c['durable_name']):copy.deepcopy(c) for s,c in DECLARATIONS}
        existing={key:{'config':copy.deepcopy(value)} for key,value in normalized.items()}
        for voice,user,count in [(False,False,2),(True,False,1),(False,True,1),(True,True,0)]:
            streams=copy.deepcopy(baseline)
            for name,target,complete in [('voice_events',self.voice,voice),('user_events',self.user,user)]:
                if complete:streams[name]['config']['subjects']=target
            p=compile_plan(self.source,streams,existing,normalized,version='voice-nats-additive-contract-v5')
            updates=[a for a in p['actions'] if a['object'] in self.additions]
            self.assertEqual(len(updates),count)
            self.assertEqual(len(p['sources']),4)
            for action in updates:
                before=copy.deepcopy(action['before']);before['subjects']=action['after']['subjects']
                self.assertEqual(before,action['after'])
                self.assertEqual(action['after']['subjects'],action['before']['subjects']+self.additions[action['object']])
    def test_v5_canonical_root_selects_v5_not_repin_v4(self):
        authority=self.run_plan();self.compatible(authority)
        self.assertEqual(authority['plan']['schema'],'voice-nats-additive-contract-v5')
        self.assertEqual(len(authority['plan']['sources']),4)
        self.assertEqual([a['object'] for a in authority['plan']['actions'] if a['object'] in self.additions],['voice_events','user_events'])
    def test_v5_partial_extra_duplicate_and_overlap_failclosed(self):
        normalized={(s,c['durable_name']):copy.deepcopy(c) for s,c in plan.DECLARATIONS}
        for stream in self.additions:
            for mutate in (lambda s:s.append(self.additions[stream][0]),lambda s:s.append('unknown.event'),lambda s:s.append(s[0])):
                streams=copy.deepcopy(self.streams);mutate(streams[stream]['config']['subjects'])
                with self.subTest(stream=stream),self.assertRaises(plan.ContractError):
                    plan.compile_plan(self.source,streams,{},normalized,version='voice-nats-additive-contract-v5')
        for subject in ('voice.*','voice.>','user.*','user.registered'):
            streams=copy.deepcopy(self.streams);streams['competing']={'config':{'name':'competing','subjects':[subject]}}
            with self.subTest(subject=subject),self.assertRaises(plan.ContractError):
                plan.compile_plan(self.source,streams,{},normalized,version='voice-nats-additive-contract-v5')
    def test_notification_missing_old_or_mutated_is_not_a_four_source_authority(self):
        path=self.source/'deploy/templates/nats-notification-bootstrap.yaml';original=path.read_bytes()
        for raw in (None,original.replace(b' voice.participant_joined voice.participant_left',b''),original+b'\n# unreviewed\n'):
            if raw is None:path.unlink()
            else:path.write_bytes(raw)
            with self.subTest(missing=raw is None),self.assertRaises(Blocked):self.run_plan()
        path.write_bytes(original)
    def _custody(self):
        from unittest.mock import Mock
        from nats_contract_custody_test import fixture
        authority=self.run_plan();kube,stage,_,rows=fixture();rows.clear()
        contract=copy.deepcopy(self.contract)
        for expected in contract['scripts']:
            part=expected['part'];old='old '+part+'\n'
            expected.update(sha256=hashlib.sha256(old.encode()).hexdigest(),bytes=len(old))
            rows['voice-nats-'+part+'-bootstrap']={'metadata':{'name':'voice-nats-'+part+'-bootstrap',
                'uid':expected['configMapUID'],'resourceVersion':expected['configMapResourceVersion']},'data':{'bootstrap.sh':old}}
        state={'operation':'123456abcdef','contract':contract,'nats_contract':authority['plan'],
            'nats_target_scripts':authority['scripts'],'nats_migration':{'verified':True,'old_record_files_verified':True,'proof':{
                'schema':'voice-nats-additive-census-proof-v4','migration_id':plan.MIGRATION_ID,
                'before_sha256':'a'*64,'after_sha256':'b'*64,'messages':14,'old_consumers_preserved':1,'created_consumers':4}}}
        return kube,stage,state,rows
    def test_custody_rejects_version_source_hash_and_proof_injection_before_cas(self):
        from nats_contract_custody import advance
        for fault in ('oldversion','source','missingnotification','hash','proof','proofschema'):
            kube,stage,state,rows=self._custody()
            if fault=='oldversion':state['nats_contract'].update(schema=plan.V4_SCHEMA,migration_id=plan.V4_MIGRATION_ID)
            if fault=='source':state['nats_contract']['sources'].pop('deploy/templates/nats-notification-bootstrap.yaml')
            if fault=='missingnotification':state['nats_target_scripts'].pop('notification')
            if fault=='hash':state['nats_target_scripts']['notification']['sha256']='0'*64
            if fault=='proof':state['nats_migration']['verified']=False
            if fault=='proofschema':state['nats_migration']['proof']['schema']='voice-nats-additive-census-proof-v3'
            with self.subTest(fault=fault),self.assertRaises(Blocked):advance(kube,state,stage,lambda event:None)
            self.assertEqual(kube.calls,[])
    def test_fourth_cas_retry_requires_exact_owned_annotation_uid_and_rv(self):
        from nats_contract_custody import advance,ANNOTATION
        for fault in ('none','annotation','uid','rv'):
            kube,stage,state,rows=self._custody();original=kube.cas
            def crash(kind,row,changes):
                if row['metadata']['name']=='voice-nats-notification-bootstrap':kube.crash=True
                return original(kind,row,changes)
            kube.cas=crash
            with self.assertRaises(KeyboardInterrupt):advance(kube,state,stage,lambda event:None)
            self.assertEqual(len(kube.calls),4)
            row=rows['voice-nats-notification-bootstrap'];kube.cas=original
            if fault=='annotation':row['metadata']['annotations'][ANNOTATION]='foreign-operation'
            if fault=='uid':row['metadata']['uid']='foreign-uid'
            if fault=='rv':row['metadata']['resourceVersion']='3';row['metadata']['annotations'][ANNOTATION]='foreign-operation'
            if fault=='none':
                receipt=advance(kube,state,stage,lambda event:None)
                self.assertEqual(receipt['sources'],plan.V5_SOURCE_HASHES)
                self.assertEqual(len(kube.calls),4,'owned exact target recovery must not repeat CAS')
            else:
                with self.subTest(fault=fault),self.assertRaises(Blocked):advance(kube,state,stage,lambda event:None)
    def test_v5_active_readback_refuses_partial_or_unbound_script_rows(self):
        from nats_contract_custody import advance
        from nats_active_contract import validate_identity
        kube,stage,state,rows=self._custody();baseline=copy.deepcopy(state['contract']['consumer_pairs'])
        active=advance(kube,state,stage,lambda event:None)
        validate_identity(active,stage.expected,baseline)
        for fault in ('missing','hash','uid','rv','bytes','three_sources'):
            changed=copy.deepcopy(active)
            if fault=='missing':changed['contract']['scripts'].pop()
            if fault=='hash':changed['contract']['scripts'][0]['sha256']='0'*64
            if fault=='uid':changed['contract']['scripts'][0]['configMapUID']=''
            if fault=='rv':changed['contract']['scripts'][0]['configMapResourceVersion']=''
            if fault=='bytes':changed['contract']['scripts'][0]['bytes']+=1
            if fault=='three_sources':changed['sources'].pop('deploy/templates/nats-notification-bootstrap.yaml')
            with self.subTest(fault=fault),self.assertRaises(Blocked):validate_identity(changed,stage.expected,baseline)

class VersionGuards(unittest.TestCase):
    def test_literal_v4_receipt_keeps_two_sources_and_v3_proof(self):
        from nats_active_contract import validate_identity
        from nats_contract_versions_test import OLD
        rows=OLD+[('file_events','messaging_music_metadata','file.processed','_INBOX.voice.messaging.messaging_music_metadata')]
        census={'schema':'voice-nats-census-v1','account':'retained','streams':[], 'consumers':[]}
        saved={'schema':'voice-nats-additive-contract-v4','migration_id':'chat-space-social-role-media-music-consumers-v4',
            'actions':[],'observed':{},'consumer_targets':[{'stream':s,'name':d,'config_sha256':'a'*64} for s,d,_,_ in rows]}
        proof=plan.verify_census(saved,census,census)
        self.assertEqual(proof['schema'],'voice-nats-additive-census-proof-v3')
        expected={'generation':'old','namespace_uid':'oldnamespace','source_claim_uid':'oldpvc'}
        receipt={'schema':'voice-nats-active-contract-v1','migration_id':saved['migration_id'],
            'generation':'old','namespace_uid':'oldnamespace','pvc_uid':'oldpvc','plan_sha256':plan.digest(saved),
            'contract':{'consumer_pairs':[[s,d] for s,d,_,_ in rows]},'proof':proof,
            'sources':{'deploy/templates/nats-realtime-bootstrap.yaml':'b'*64,'deploy/templates/nats-analytics-chat-bootstrap.yaml':'c'*64}}
        validate_identity(receipt,expected,[])
        for change in ('proof','source','migration'):
            mixed=copy.deepcopy(receipt)
            if change=='proof':mixed['proof']['schema']='voice-nats-additive-census-proof-v4'
            if change=='source':mixed['sources']['deploy/templates/nats-search-bootstrap.yaml']='d'*64
            if change=='migration':mixed['migration_id']=plan.MIGRATION_ID
            with self.subTest(change=change),self.assertRaises(Blocked):validate_identity(mixed,expected,[])
    def test_old_plans_cannot_smuggle_new_stream_updates_into_execution_or_census(self):
        from nats_contract_plan_test import Actor
        from nats_contract_actor import Actor as NativeActor
        for schema,identity,declarations in [(plan.V3_SCHEMA,plan.V3_MIGRATION_ID,plan.V3_DECLARATIONS),
                                             (plan.V4_SCHEMA,plan.V4_MIGRATION_ID,plan.V4_DECLARATIONS)]:
            for stream in ('voice_events','user_events'):
                p={'schema':schema,'migration_id':identity,'consumer_targets':[{'stream':s,'name':c['durable_name'],'config_sha256':'a'*64} for s,c in declarations],
                    'observed':{},'actions':[{'api':'$JS.API.STREAM.UPDATE.'+stream,'object':stream,'before':{},'after':{}}]}
                events=[];actor=Actor(p)
                with self.assertRaises(plan.ContractError):plan.execute(p,actor,events.append)
                self.assertEqual((events,actor.calls),([],[]))
                census={'schema':'voice-nats-census-v1','account':'same','streams':[],'consumers':[]}
                with self.assertRaises(plan.ContractError):plan.verify_census(p,census,census)
                native=NativeActor(None,'owned','id','/owned',lambda:None,plan=p)
                with self.assertRaises(plan.ContractError):native.mutate('$JS.API.STREAM.UPDATE.'+stream,{'name':stream})
    def test_version_pairs_and_v5_proof_sources_are_not_interchangeable(self):
        for schema,identity in [(plan.SCHEMA,plan.V4_MIGRATION_ID),(plan.V4_SCHEMA,plan.MIGRATION_ID)]:
            with self.assertRaises(plan.ContractError):plan.declarations_for({'schema':schema,'migration_id':identity})

if __name__=='__main__':unittest.main()
