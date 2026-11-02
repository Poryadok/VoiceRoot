"""Synthetic structural records never substitute for protected P2 authority."""
import copy
import types
import unittest
import resource_claim
import story_witness as witness
import story_fixture
from controller import Blocked

class PreservedWitnessTests(unittest.TestCase):
    def proof(self):
        image='ghcr.io/poryadok/voiceroot/story@sha256:'+'a'*64
        inputs={name:'b'*64 for name in ('context_sha256','helper_sha256','plan_sha256','source_capture_sha256',
            'profile_sha256','migration_plan_sha256','migration_files_sha256','archive_sha256','binding_sha256',
            'dump_receipt_sha256','dump_plaintext_sha256')}
        inputs.update(schema='voice-story-preserved-target8-input-v1',operation='c'*12,restore_attempt='d'*32,
            phase='local',postgres='e'*64,runtime_user='actual_runtime',images={'story0':image},caller_sha256=story_fixture.PRESERVED_CALLER_SHA256)
        new='10000000-0000-4000-8000-000000000001'
        positive={'status':'PASS','contract':'restored-data8-v1','create_grpc_code':'OK',
            'response_identity_type_verified':True,'sql_fields_verified':True,'baseline_story_count':2,'story_count':3,
            'new_story_id':new,'default_ttl_24h_verified':True,'archive_plus30d_verified':True,
            'missing_profile_code':'Unauthenticated','negative_count_unchanged':True,'unrelated_row_count':5,'row_retained':True}
        result={'image':image,'child_sha256':'a'*64,'config_sha256':'f'*64,'binary_sha256':'1'*64,
            'contract':'restored-data8-v1','positive':positive,'positive_exit':0,
            'negative':{'status':'FAIL','phase':'wrong_expected_row_count'},'negative_exit':17,
            'invalid_input':{'status':'FAIL','phase':'baseline_stories_input'},'invalid_exit':17,
            'before_sha256':'2'*64,'after_sha256':'3'*64,'allowed_new_story_id':new}
        row={name:'4'*64 for name in ('baseline_sha256','catalog_sha256','authority_sha256','baseline_rows_sha256','target_rows_sha256')}
        row.update(schema='voice-story-preserved-target8-proof-v1',input=inputs,target_sha256='2'*64,
            table_count=24,created_gate_owner_incarnation=new,writer_results=[result],final_sha256='3'*64)
        return row

    def test_structural_proof_requires_full_target8_writer_negative_and_lineage(self):
        row=self.proof();self.assertEqual(witness._preserved_proof(row),row)
        for fault in ('old_probe','cleanup_copy','missing_negative','missing_input','foreign_chain','count','identity','bool','empty','extra'):
            changed=copy.deepcopy(row);writer=changed['writer_results'][0]
            if fault=='old_probe':changed['input']['caller_sha256']='0'*64
            elif fault=='cleanup_copy':changed['schema']='voice-story-current4-automatic-witness-v1'
            elif fault=='missing_negative':writer['negative_exit']=0
            elif fault=='missing_input':del changed['input']['dump_plaintext_sha256']
            elif fault=='foreign_chain':writer['before_sha256']='0'*64
            elif fault=='count':changed['table_count']=7
            elif fault=='identity':writer['allowed_new_story_id']='00000000-0000-4000-8000-000000000000'
            elif fault=='bool':writer['positive']['sql_fields_verified']=1
            elif fault=='empty':changed['writer_results']=[]
            else:writer['positive']['trusted_restore']=True
            with self.subTest(fault=fault),self.assertRaises(Blocked):witness._preserved_proof(changed)

    def test_operation_profile_backup_keys_do_not_alias_current4_or_other_capture(self):
        row={'operation':'a'*12,'profile':{'schema':'voice-story-baseline4-target8-composition-v1'},'archive_sha256':'b'*64}
        key=witness._preserved_key(row)
        for field,value in (('operation','c'*12),('profile',{'schema':'voice-story-current4-automatic-witness-v1'}),('archive_sha256','d'*64)):
            changed=copy.deepcopy(row);changed[field]=value
            self.assertNotEqual(witness._preserved_key(changed),key)
        selected={'schema':'voice-story-preserved-target8-selector-v1','revision':2,
            'entries':{key:{'witness_sha256':'e'*64,'enabled':True,'revision':1}}}
        self.assertEqual(witness._preserved_authority(witness._preserved_selector(selected),key)['registry_revision'],1)
        selected['entries'][key]['enabled']=False
        with self.assertRaises(Blocked):witness._preserved_authority(witness._preserved_selector(selected),key)
        with self.assertRaises(Blocked):witness.selector(selected)

    def test_forged_session_or_structural_record_cannot_load_or_promote_authority(self):
        for fake in (None,types.SimpleNamespace(operation='a'*12),object.__new__(resource_claim.Session)):
            with self.subTest(fake=type(fake).__name__),self.assertRaises(Blocked):witness.lookup_preserved_target(fake)
            with self.subTest(final=type(fake).__name__),self.assertRaises(Blocked):witness.load_preserved_target(fake,{'target':{}},self.proof())

if __name__=='__main__':unittest.main()
