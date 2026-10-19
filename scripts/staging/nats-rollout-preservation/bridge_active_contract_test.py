import copy
import json
import unittest
from pathlib import Path

from nats_active_contract import LEGACY_NATS_MIGRATION_ID, validate_identity
from controller import Blocked
from nats_contract_plan import V4_DECLARATIONS as DECLARATIONS, V4_MIGRATION_ID as MIGRATION_ID, verify_census
from nats_contract_plan_test import CensusTests
from nats_contract_custody_test import fixture as custody_fixture
from nats_contract_custody import advance


class Tests(unittest.TestCase):
    def setUp(self):
        self.expected={'generation':'r20261012a1','namespace_uid':'namespace-uid','source_claim_uid':'pvc-uid'}
        self.baseline=[['baseline_events','baseline_consumer']]
        self.sources={
            'deploy/templates/nats-realtime-bootstrap.yaml':'a'*64,
            'deploy/templates/nats-analytics-chat-bootstrap.yaml':'b'*64,
        }
        self.contract={'consumer_pairs':self.baseline+[['social_events','rt_realtime1_friend_removed']]}
        legacy_fixture=json.loads((Path(__file__).parent/'fixtures'/'legacy-9ae-census-proof.json').read_text(encoding='utf8'))
        self.legacy_fixture=legacy_fixture
        self.active={'schema':'voice-nats-active-contract-v1','migration_id':LEGACY_NATS_MIGRATION_ID,
            'generation':self.expected['generation'],'namespace_uid':self.expected['namespace_uid'],
            'pvc_uid':self.expected['source_claim_uid'],'contract':copy.deepcopy(self.contract),
            'plan_sha256':'c'*64,'sources':copy.deepcopy(self.sources),'proof':copy.deepcopy(legacy_fixture['proof'])}

    @staticmethod
    def _proof(migration_id):
        return {'schema':'voice-nats-additive-census-proof-v3','migration_id':migration_id,
            'before_sha256':'d'*64,'after_sha256':'e'*64,'messages':17,
            'old_consumers_preserved':42,'created_consumers':1,'ephemeral_recovery_observations':[]}

    def _v3(self):
        active=copy.deepcopy(self.active)
        active['migration_id']=MIGRATION_ID
        active['contract']['consumer_pairs']=self.baseline+[
            [stream,declaration['durable_name']] for stream,declaration in DECLARATIONS]
        active['proof']=self._proof(MIGRATION_ID)
        active['plan_sha256']='f'*64
        return active

    def test_exact_legacy_and_current_receipts_bind_declared_pairs(self):
        validate_identity(self.active,self.expected,self.baseline)
        validate_identity(self._v3(),self.expected,self.baseline)

    def test_legacy_receipt_fixture_is_generated_from_the_pinned_producer_shape(self):
        provenance=self.legacy_fixture['provenance'];proof=self.active['proof']
        self.assertEqual(provenance['producer_commit'],'9ae2e0c259e5e039109f7331aaf62b3607e549dd')
        self.assertEqual(provenance['plan_blob'],'36b4b63d784751cee9a28f97a6f45e54df8d9a2a')
        self.assertEqual(provenance['migration_blob'],'e2c1e889f2905a4399858a8369acca785032c0e8')
        self.assertEqual(provenance['recovery_blob'],'1eb943885937f7a656d34cdc0a9f72308a516bae')
        self.assertEqual(proof['schema'],'voice-nats-additive-census-proof-v1')
        self.assertEqual(proof['migration_id'],LEGACY_NATS_MIGRATION_ID)
        self.assertEqual(len(proof['ephemeral_recovery_observations']),1)
        validate_identity(self.active,self.expected,self.baseline)
        no_op=copy.deepcopy(self.active);no_op['proof'].pop('ephemeral_recovery_observations')
        validate_identity(no_op,self.expected,self.baseline)

    def test_current_producer_proof_survives_custody_and_active_receipt_validation(self):
        census=CensusTests();census.setUp();plan,before,after=census.proof()
        proof=verify_census(plan,before,after)
        self.assertEqual(proof['schema'],'voice-nats-additive-census-proof-v3')
        proof['ephemeral_recovery_observations']=[]
        kube,stage,state,_=custody_fixture()
        baseline=copy.deepcopy(state['contract']['consumer_pairs'])
        state['nats_contract']=plan
        state['nats_migration']['proof']=proof
        receipt=advance(kube,state,stage,lambda event:None)
        self.assertEqual(receipt['proof'],proof)
        expected={'generation':stage.expected['generation'],'namespace_uid':stage.expected['namespace_uid'],
            'source_claim_uid':stage.expected['source_claim_uid']}
        validate_identity(receipt,expected,baseline)

    def test_census_proof_schema_is_bound_to_the_receipt_migration(self):
        for active,wrong_schema in ((self.active,'voice-nats-additive-census-proof-v2'),
            (self._v3(),'voice-nats-additive-census-proof-v1')):
            active=copy.deepcopy(active);active['proof']['schema']=wrong_schema
            with self.subTest(migration_id=active['migration_id'],schema=wrong_schema),self.assertRaises(Blocked):
                validate_identity(active,self.expected,self.baseline)

    def test_empty_incomplete_duplicate_and_cross_version_pairs_are_rejected(self):
        mutations=[]
        empty=copy.deepcopy(self.active);empty['contract']['consumer_pairs']=[];mutations.append(empty)
        old_with_v3=copy.deepcopy(self.active)
        old_with_v3['contract']['consumer_pairs']=self._v3()['contract']['consumer_pairs']
        mutations.append(old_with_v3)
        v3_missing= self._v3();v3_missing['contract']['consumer_pairs'].pop();mutations.append(v3_missing)
        v3_with_legacy= self._v3();v3_with_legacy['contract']['consumer_pairs']=[self.baseline[0],['social_events','rt_realtime1_friend_removed']];mutations.append(v3_with_legacy)
        duplicate=copy.deepcopy(self.active);duplicate['contract']['consumer_pairs'].append(copy.deepcopy(self.baseline[0]));mutations.append(duplicate)
        for active in mutations:
            with self.subTest(pairs=active['contract']['consumer_pairs']),self.assertRaises(Blocked):
                validate_identity(active,self.expected,self.baseline)

    def test_plan_sources_and_proof_are_version_bound_and_strict(self):
        mutations=[]
        bad=copy.deepcopy(self.active);bad['plan_sha256']='not-a-hash';mutations.append(bad)
        bad=copy.deepcopy(self.active);bad['sources']['unknown.yaml']='a'*64;mutations.append(bad)
        bad=copy.deepcopy(self.active);bad['sources']['deploy/templates/nats-realtime-bootstrap.yaml']='x';mutations.append(bad)
        bad=copy.deepcopy(self.active);bad['proof']['migration_id']=MIGRATION_ID;mutations.append(bad)
        bad=copy.deepcopy(self.active);bad['proof']['unreviewed']=True;mutations.append(bad)
        bad=copy.deepcopy(self._v3());bad['proof']['ephemeral_recovery_observations']={};mutations.append(bad)
        bad=copy.deepcopy(self.active);bad['proof']['ephemeral_recovery_observations'][0]['unreviewed']=True;mutations.append(bad)
        bad=copy.deepcopy(self.active);bad['proof']['ephemeral_recovery_observations'][0]['recovery']['extra']='unreviewed';mutations.append(bad)
        for active in mutations:
            with self.subTest(active=active),self.assertRaises(Blocked):
                validate_identity(active,self.expected,self.baseline)

    def test_identity_and_baseline_shape_remain_fail_closed(self):
        for field,value in (('schema','voice-nats-active-contract-v2'),('migration_id','unreviewed-v4'),
            ('generation','other-generation'),('namespace_uid','other-namespace'),('pvc_uid','other-pvc')):
            active=copy.deepcopy(self.active);active[field]=value
            with self.subTest(field=field),self.assertRaises(Blocked):
                validate_identity(active,self.expected,self.baseline)
        with self.assertRaises(Blocked):validate_identity(self.active,self.expected,[])


if __name__=='__main__':
    unittest.main()
