import copy
import unittest
import json
from pathlib import Path
from unittest.mock import patch

from controller import Blocked
import rollout_census
from rollout_census_test import fixture, Runtime, ACCOUNT
import operator_reset as reset
import nats_active_contract
import installer


def baseline():
    before=rollout_census.census(Runtime(fixture()),'test',ACCOUNT)
    after=copy.deepcopy(before)
    for row in after['streams']:
        row['created']='2026-10-14T01:00:00Z'
        row['state'].update(messages=0,bytes=0,first_seq=0,last_seq=0)
    for row in after['consumers']:
        row['created']='2026-10-14T01:00:00Z'
        row['delivered']=dict(consumer_seq=0,stream_seq=0)
        row['ack_floor']=dict(consumer_seq=0,stream_seq=0)
        row.update(num_ack_pending=0,num_redelivered=0,num_pending=0)
    return before,after


class ResetBaselineTests(unittest.TestCase):
    def test_new_zero_baseline_preserves_config_but_not_old_record_claim(self):
        before,after=baseline()
        proof=reset.zero_baseline(before,after)
        self.assertEqual(proof['messages'],0)
        self.assertFalse(proof['historical_preservation'])
        self.assertEqual(proof['configuration_sha256'],reset.configuration(before))
        self.assertEqual(proof['census_sha256'],reset.canonical(after))

    def test_changed_config_missing_durable_or_new_record_refuses(self):
        for kind in ('stream-config','consumer-config','missing','messages','bytes','last-sequence',
                     'first-sequence','pending','redelivery','ack','delivered','account','lost','deleted'):
            before,after=baseline()
            if kind=='stream-config':after['streams'][0]['config_sha256']='f'*64
            elif kind=='consumer-config':after['consumers'][0]['config_sha256']='f'*64
            elif kind=='missing':after['consumers'].pop()
            elif kind=='messages':after['streams'][0]['state']['messages']=1
            elif kind=='bytes':after['streams'][0]['state']['bytes']=1
            elif kind=='last-sequence':after['streams'][0]['state']['last_seq']=1
            elif kind=='first-sequence':after['streams'][0]['state']['first_seq']=2
            elif kind=='pending':after['consumers'][0]['num_pending']=1
            elif kind=='redelivery':after['consumers'][0]['num_redelivered']=1
            elif kind=='ack':after['consumers'][0]['ack_floor']['stream_seq']=1
            elif kind=='delivered':after['consumers'][0]['delivered']['stream_seq']=1
            elif kind=='account':after['account']='A'+'C'*55
            elif kind=='lost':after['streams'][0]['state']['lost']={'msgs':[1],'bytes':20}
            elif kind=='deleted':after['streams'][0]['state']['deleted']=[1]
            with self.subTest(kind=kind),self.assertRaises(Blocked):reset.zero_baseline(before,after)

    def test_restore_requires_entire_fresh_census_and_native_inventory(self):
        before,after=baseline();manifest={'files':[{'path':'streams/A/msgs/index.db','sha256':'a'*64}]}
        proof=reset.restored_baseline(before,after,copy.deepcopy(after),manifest,copy.deepcopy(manifest))
        self.assertTrue(proof['restore_verified'])
        for part in ('census','native'):
            other=copy.deepcopy(after);native=copy.deepcopy(manifest)
            if part=='census':other['consumers'][0]['created']='2026-10-14T01:01:00Z'
            else:native['files'][0]['sha256']='b'*64
            with self.subTest(part=part),self.assertRaises(Blocked):reset.restored_baseline(before,after,other,manifest,native)

    def test_expired_index_is_not_exempted(self):
        before,after=baseline();old={'files':[{'path':'streams/user_events/msgs/index.db','sha256':'a'*64}]}
        new=copy.deepcopy(old);new['files'][0]['sha256']='b'*64
        with self.assertRaises(Blocked):reset.restored_baseline(before,after,after,old,new)


class ResetAuthorityTests(unittest.TestCase):
    def test_real_public_15_43_predecessor_contract_is_pinned_without_hash_mock(self):
        from bridge_active_contract_test import Tests
        fixture=Tests();fixture.setUp()
        path=Path(__file__).parent/'fixtures/reset-a83-public-contract.json'
        contract=json.loads(path.read_text())
        self.assertEqual(reset.canonical(contract),reset.CONTRACT_SHA)
        self.assertEqual((len(contract['streams']),len(contract['consumer_pairs'])),(15,43))
        baseline=json.loads((Path(__file__).parents[1]/'nats-known-baseline/deployed-contract.json').read_text())
        predecessor=copy.deepcopy(fixture.active);predecessor['contract']=contract
        proof={'schema':'voice-empty-reset-baseline-v1','configuration_sha256':'a'*64,'census_sha256':'b'*64,
            'streams':15,'consumers':43,'messages':0,'historical_preservation':False,
            'restore_verified':True,'native_files_verified':True}
        new_uid='11111111-1111-1111-1111-111111111111'
        row=reset.reset_active_contract(predecessor,proof,new_uid,fixture.expected['namespace_uid'],fixture.expected['generation'],'c'*64)
        with patch.object(reset,'validate_active_evidence'):
            nats_active_contract.validate_identity(row,dict(fixture.expected,source_claim_uid=new_uid),baseline['consumer_pairs'])
            changed=copy.deepcopy(row);changed['contract']['consumer_pairs'].pop()
            with self.assertRaises(Blocked):nats_active_contract.validate_identity(changed,dict(fixture.expected,source_claim_uid=new_uid),baseline['consumer_pairs'])

    def test_active_hash_must_resolve_to_completed_protected_authority(self):
        storage={'claim':{'metadata':{'uid':'new'}}};proof={'messages':0}
        authority=reset.reset_authority(storage,proof)
        row={'reset_authority_sha256':reset.canonical(authority),'baseline':proof,'pvc_uid':'new'}
        receipt={'new_storage':storage,'baseline':proof}
        def read(path,*args,**kwargs):
            name=Path(path).name
            value=authority if name.endswith('-authority.json') else row if name.endswith('-active-contract-after.json') else {'terminal':True}
            return json.dumps(value).encode()
        with patch('operator_retirement.private_bytes',side_effect=read),patch.object(reset,'verify_terminal',return_value=receipt) as terminal:
            reset.validate_active_evidence(row);terminal.assert_called_once()
            changed=dict(row,reset_authority_sha256='0'*64)
            with self.assertRaises(Blocked):reset.validate_active_evidence(changed)
    def test_reset_contract_binds_new_pvc_and_keeps_old_proof_historical(self):
        from bridge_active_contract_test import Tests
        fixture=Tests();fixture.setUp()
        old=copy.deepcopy(fixture.active);original=copy.deepcopy(old)
        proof={'schema':'voice-empty-reset-baseline-v1','configuration_sha256':'a'*64,'census_sha256':'b'*64,
            'streams':15,'consumers':43,'messages':0,'historical_preservation':False,
            'restore_verified':True,'native_files_verified':True}
        new_uid='11111111-1111-1111-1111-111111111111'
        with patch.object(reset,'CONTRACT_SHA',reset.canonical(old['contract'])),patch.object(reset,'validate_active_evidence') as evidence:
            row=reset.reset_active_contract(old,proof,new_uid,fixture.expected['namespace_uid'],fixture.expected['generation'],'c'*64)
            expected=dict(fixture.expected,source_claim_uid=new_uid)
            nats_active_contract.validate_identity(row,expected,fixture.baseline)
            evidence.assert_called_once_with(row)
            self.assertEqual(old,original)
            self.assertEqual(row['predecessor'],original)
            self.assertFalse(row['deployment_pass']);self.assertFalse(row['rollback_eligible'])
            for field,value in (('pvc_uid',old['pvc_uid']),('deployment_pass',True),('rollback_eligible',True),
                    ('reset_authority_sha256',''),('operation','b'*12)):
                mutated=copy.deepcopy(row);mutated[field]=value
                with self.subTest(field=field),self.assertRaises(Blocked):nats_active_contract.validate_identity(mutated,expected,fixture.baseline)
            for field,value in (('messages',1),('restore_verified',False),('historical_preservation',True),('consumers',42)):
                mutated=copy.deepcopy(row);mutated['baseline'][field]=value
                with self.subTest(field=field),self.assertRaises(Blocked):nats_active_contract.validate_identity(mutated,expected,fixture.baseline)
            with self.assertRaises(Blocked):reset.reset_active_contract(old,proof,old['pvc_uid'],fixture.expected['namespace_uid'],fixture.expected['generation'],'c'*64)

    def test_active_envelope_is_creation_evidence_not_a_forever_empty_live_guard(self):
        # Its API intentionally takes only expected identity/configuration,
        # never a live message count. The next normal rollout captures writes.
        import inspect
        self.assertEqual(list(inspect.signature(reset.validate_active).parameters),['row','expected','baseline_pairs'])

    def test_installer_uses_exact_predecessor_and_unique_preserved_route(self):
        with patch.object(installer,'_upgrade_idle_transition') as call:
            installer.upgrade_idle_expiry_reset_v1('public-candidate')
        self.assertEqual(call.call_args.args[:4],('public-candidate',reset.CODE_SHA,'idle-expiry-reset-v1','expiry-43f904'))

    def test_snapshot_hash_precedes_any_structural_reset_permission(self):
        with self.assertRaises(Blocked):reset.snapshot(b'{"phase":"NATS_CONTRACT_MIGRATION"}')

    def test_authority_does_not_depend_on_later_marker_resource_version(self):
        authority=reset.reset_authority({'claim':{'uid':'new'}},{'messages':0})
        self.assertNotIn('released_marker',authority)
        self.assertNotIn('receipt_sha256',authority)
        self.assertEqual(authority['snapshot_sha256'],reset.CHECKPOINT_SHA)


if __name__=='__main__':unittest.main()
