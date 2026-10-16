"""Historical admission fixtures are literal, never derived from CURRENT rows."""
import copy
import unittest
import nats_contract_plan as plan
import nats_root_plan as root
from controller import Blocked

V3_SCHEMA='voice-nats-additive-contract-v3'
V3_ID='chat-space-social-role-media-consumers-v3'
V4_SCHEMA='voice-nats-additive-contract-v4'
V4_ID='chat-space-social-role-media-music-consumers-v4'
OLD=[
    ('social_events','rt_realtime1_friend_removed','social.friend_removed','_INBOX.voice.realtime1.friend_removed'),
    ('chat_events','voice_space_media_chat','space.voice_room_access_invalidated','_INBOX.voice.voice.space_media_chat'),
    ('role_events','voice_space_media_role','role.voice_policy_invalidated','_INBOX.voice.voice.space_media_role'),
]

def auth(rows):
    normalized=[{'stream':s,'durable':d,'config':{'name':d,'durable_name':d,
        'filter_subject':f,'deliver_subject':i,'deliver_policy':'new','ack_policy':'explicit',
        'max_ack_pending':1000}} for s,d,f,i in rows]
    if len(rows)==4:normalized[-1]['config']['deliver_group']='messaging_music_metadata'
    first=normalized[0]['config']
    return {'verified':True,'server_image':'pinned-image','account_sha256':'a'*64,
        'reply_prefix':'_INBOX.voice.bootstrap.reply','normalized_consumer':first,
        'normalized_consumer_sha256':plan.digest(first),'normalized_consumers':normalized,
        'normalized_consumers_sha256':plan.digest(normalized)}

class Versions(unittest.TestCase):
    def test_literal_v3_noop_census_and_active_receipt_keep_old_proof_schema(self):
        from nats_active_contract import validate_identity
        saved={'schema':V3_SCHEMA,'migration_id':V3_ID,'actions':[],
            'observed':{},'consumer_targets':[{'stream':s,'name':d,'config_sha256':'a'*64} for s,d,_,_ in OLD]}
        census={'schema':'voice-nats-census-v1','account':'retained-account',
            'streams':[{'name':s,'config_sha256':'b'*64,'state':{'messages':7,'last_seq':17,'consumer_count':1}} for s,_,_,_ in OLD],
            'consumers':[{'stream':s,'name':d,'config_sha256':'a'*64,'delivered':{'consumer_seq':5,'stream_seq':14},
                'ack_floor':{'consumer_seq':3,'stream_seq':12},'num_ack_pending':2} for s,d,_,_ in OLD]}
        proof=plan.verify_census(saved,census,census)
        self.assertEqual(proof,{'schema':'voice-nats-additive-census-proof-v2','migration_id':V3_ID,
            'before_sha256':plan.digest(census),'after_sha256':plan.digest(census),'messages':21,
            'old_consumers_preserved':3,'created_consumers':0})
        expected={'generation':'retained-generation','namespace_uid':'retained-namespace','source_claim_uid':'retained-pvc'}
        receipt={'schema':'voice-nats-active-contract-v1','migration_id':V3_ID,
            'generation':expected['generation'],'namespace_uid':expected['namespace_uid'],'pvc_uid':expected['source_claim_uid'],
            'contract':{'consumer_pairs':[[s,d] for s,d,_,_ in OLD]},'plan_sha256':plan.digest(saved),
            'sources':{'deploy/templates/nats-realtime-bootstrap.yaml':'c'*64,
                'deploy/templates/nats-analytics-chat-bootstrap.yaml':'d'*64},'proof':proof}
        validate_identity(receipt,expected,[])
        mixed=copy.deepcopy(saved);mixed['consumer_targets'].append({'stream':'file_events','name':'messaging_music_metadata','config_sha256':'e'*64})
        with self.assertRaises(plan.ContractError):plan.verify_census(mixed,census,census)
        mixed=copy.deepcopy(receipt);mixed['proof']['schema']='voice-nats-additive-census-proof-v3'
        with self.assertRaises(Blocked):validate_identity(mixed,expected,[])

    def test_saved_v3_declarations_remain_exact(self):
        rows=plan.declarations_for({'schema':V3_SCHEMA,'migration_id':V3_ID})
        self.assertEqual([(s,c['durable_name'],c['filter_subject'],c['deliver_subject']) for s,c in rows],OLD)
        self.assertEqual(len(plan.declarations_for({'schema':V4_SCHEMA,'migration_id':V4_ID})),4)
        for schema,identity in ((V3_SCHEMA,V4_ID),(V4_SCHEMA,V3_ID),('unknown',V4_ID)):
            with self.subTest(schema=schema,identity=identity),self.assertRaises(plan.ContractError):
                plan.declarations_for({'schema':schema,'migration_id':identity})

    def test_v4_auth_extends_but_never_repins_v3_enrollment(self):
        enrolled=auth(OLD);saved=copy.deepcopy(enrolled)
        fresh=auth(OLD+[('file_events','messaging_music_metadata','file.processed',
            '_INBOX.voice.messaging.messaging_music_metadata')])
        fresh['schema']='voice-bootstrap-auth-v4'
        self.assertEqual(len(root._validated_normalized_consumers(fresh,enrolled)),4)
        self.assertEqual(enrolled,saved)
        for field in ('account_sha256','server_image','reply_prefix'):
            changed=copy.deepcopy(fresh);changed[field]='different'
            with self.subTest(field=field),self.assertRaises(Blocked):
                root._validated_normalized_consumers(changed,enrolled)
        changed=copy.deepcopy(fresh)
        changed['normalized_consumers'][1]['config']['max_ack_pending']=999
        changed['normalized_consumers_sha256']=plan.digest(changed['normalized_consumers'])
        with self.assertRaises(Blocked):root._validated_normalized_consumers(changed,enrolled)
        for mutate in (lambda v:v.pop('schema'),lambda v:v.update(schema='unknown')):
            changed=copy.deepcopy(fresh);mutate(changed)
            with self.assertRaises(Blocked):root._validated_normalized_consumers(changed,enrolled)

if __name__=='__main__':unittest.main()
