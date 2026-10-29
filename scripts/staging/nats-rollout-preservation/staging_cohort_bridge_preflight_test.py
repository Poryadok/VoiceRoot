import copy
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import actor_root
import bridge_root
import commands
import nats_contract_plan
import nats_root_plan
import resource_claim
import resource_claim_test


class CohortBridgePreflightTest(unittest.TestCase):
    def test_actual_session_live_preflight_and_wrong_commit_refusal(self):
        context=resource_claim_test.context();card=resource_claim_test.card(context)
        session=resource_claim.Session(context,'a'*12,'b'*64,lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal())
        session.admit(start=True)
        actions=object.__new__(bridge_root.Actions);actions.tt_session=session;actions.code=Path('/fixed-code');actions.binding={}
        stage=SimpleNamespace(operation=session.operation,kube=object(),expected={},snapshots={'voice-user':{},bridge_root.HUB:{}})
        previous={'schema':'fixture'}
        enrollment={'schema':'voice-nats-bootstrap-enrollment-v4','previous_enrollment':previous,
                    'previous_enrollment_sha256':nats_contract_plan.digest(previous)}
        def read(path):
            if path.name=='active-contract.json':raise FileNotFoundError()
            return enrollment if path.name=='bootstrap-enrollment-v4.json' else previous
        request={'mode':'full'};approved={'source_sha':context['commit']};workspace=bridge_root.INSTALLED/'sources'/session.operation
        with patch.object(commands,'authority_guard',session.check),patch.object(bridge_root,'private_json',side_effect=read), \
             patch.object(actor_root,'preflight',return_value={'actor':'qualified'}) as actor, \
             patch.object(nats_root_plan,'preflight',return_value={'nats':'qualified'}) as nats:
            result=bridge_root.capture_cohort_authorities(actions,request,approved,workspace,stage,{'consumer_pairs':[]})
            self.assertEqual(result['services'],['user']);self.assertEqual(result['actor'],{'actor':'qualified'})
            self.assertEqual(result['nats'],{'nats':'qualified'})
            with self.assertRaisesRegex(bridge_root.Blocked,'cohort_preflight_authority_invalid'):
                bridge_root.capture_cohort_authorities(actions,request,{'source_sha':'f'*40},workspace,stage,{'consumer_pairs':[]})
            self.assertEqual(actor.call_count,1);self.assertEqual(nats.call_count,1)


if __name__=='__main__':unittest.main()
