"""Actual transaction boundary: no pre-mutation write, truthful post-mutation block."""
import copy
import datetime as dt
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import Mock,patch
import transaction


class Clock(dt.datetime):
    current=None
    @classmethod
    def now(cls,tz=None):return cls.current


class DeadlineTests(unittest.TestCase):
    def run_transaction(self,*,dedicated=True,expire_before=False,expire_after=False,expire_prepared=False,expire_receipt=False):
        Clock.current=Clock(2026,10,9,8,0,tzinfo=dt.timezone.utc)
        deadline=Clock.current+dt.timedelta(minutes=5)
        if expire_before:deadline=Clock.current
        expiry=Clock.current+dt.timedelta(hours=3)
        state={'operation':'123456abcdef','phase':'AWAITING_OFF_NODE','status':'WAITING',
            'fence_status':'VERIFIED','context':{},'provenance':{},'migrations':[],
            'migration_secret_metadata':{},'custody':{'verified':True},'nats_contract_expires_at':expiry.isoformat(),
            'target':{'mode':'images-only','migration_sha256':transaction.canonical([])},
            'cut':{'manifest':{'archive_sha256':'a'*64},'manifest_sha256':'b'*64,'census_sha256':'c'*64}}
        original=copy.deepcopy(state);saved=[]
        stage=Mock();stage.final_path=Path('/same');stage.expected={'namespace_uid':'namespace'}
        stage.marker={'metadata':{'uid':'marker','resourceVersion':'7'},'data':{'generation':'g','previousGeneration':'p','dataPVC':'same'}}
        stage.final_claim={'metadata':{'name':'same','uid':'pvc'},'spec':{'volumeName':'pv'}}
        stage.final_pv={'metadata':{'uid':'pv'},'spec':{'local':{}}};stage.refence.return_value={'verified':True}
        prepared=[False]
        def prepare_marker():
            prepared[0]=True
            stage.marker['data']['phase']='rollout-prepared'
            stage.marker['metadata']['resourceVersion']='8'
            if expire_prepared:Clock.current=deadline
        stage.prepared.side_effect=prepare_marker
        class Manifest(dict):
            def __getitem__(self,key):
                if expire_receipt and prepared[0]:Clock.current=deadline
                return super().__getitem__(key)
        state['cut']['manifest']=Manifest(state['cut']['manifest'])
        def mutate(*args):
            if expire_after:Clock.current=deadline
            return ['recorded-job']
        with patch.object(transaction.dt,'datetime',Clock),patch.object(transaction,'reconstruct',return_value=stage),\
             patch.object(transaction,'context',side_effect=lambda st:{'marker':copy.deepcopy(st.marker)}),\
             patch.object(transaction,'save',side_effect=lambda path,row:saved.append((Path(path).name,copy.deepcopy(row)))),\
             patch.object(transaction,'file_sha',return_value='b'*64),patch.object(transaction,'verify_archive',return_value=True),\
             patch.object(transaction,'revalidate_inputs'),patch.object(transaction.space_safeguards,'before_migrate'),\
             patch.object(transaction.space_safeguards,'after_migrate',return_value={'verified':True}),\
             patch.object(transaction.migrations,'execute',side_effect=mutate) as mutate_call:
            kwargs={'authorization_deadline':deadline} if dedicated else {}
            if expire_before or expire_after or expire_prepared or expire_receipt:
                with self.assertRaisesRegex(transaction.Blocked,'preserved_upload_authorization_expired'):
                    transaction.authorize(None,Path('/private'),state,'a'*64,'b'*64,{},**kwargs)
                if expire_before:
                    self.assertEqual(state,original);self.assertEqual(saved,[]);mutate_call.assert_not_called();stage.refence.assert_not_called()
                else:
                    self.assertEqual(state['status'],'BLOCKED');self.assertEqual(state['fence_status'],'VERIFIED')
                    self.assertEqual(state['migration_jobs'],['recorded-job']);stage.refence.assert_called_once()
                    self.assertTrue(saved);self.assertFalse(any(name=='apply-authorization.json' for name,_ in saved))
                if expire_prepared or expire_receipt:
                    stage.prepared.assert_called_once()
                    self.assertEqual(state['context']['marker']['data']['phase'],'rollout-prepared')
                    self.assertEqual(state['context']['marker']['metadata']['resourceVersion'],'8')
                else:stage.prepared.assert_not_called()
            else:
                receipt=transaction.authorize(None,Path('/private'),state,'a'*64,'b'*64,{},**kwargs)
                self.assertEqual(state['nats_contract_expires_at'],original['nats_contract_expires_at'])
                self.assertEqual(dt.datetime.fromisoformat(receipt['expires_at']),deadline if dedicated else Clock.current+dt.timedelta(hours=4))
                self.assertEqual(state['phase'],'PAUSED_APPLY');stage.prepared.assert_called_once()

    def test_expiry_during_prepared_cas_records_current_truth_and_no_receipt(self):self.run_transaction(expire_prepared=True)
    def test_expiry_during_final_receipt_construction_records_truth_and_no_receipt(self):self.run_transaction(expire_receipt=True)

    def test_dedicated_receipt_cannot_extend_fresh_window_or_original_proof(self):self.run_transaction()
    def test_ordinary_default_keeps_existing_four_hour_receipt_contract(self):self.run_transaction(dedicated=False)
    def test_expired_before_mutation_preserves_original_checkpoint(self):self.run_transaction(expire_before=True)
    def test_expiry_after_mutation_preserves_truthful_progress_and_refences(self):self.run_transaction(expire_after=True)


if __name__=='__main__':unittest.main()
