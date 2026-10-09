"""Revision predicate controls; physical installer custody is separate evidence."""
import copy
import json
import unittest
from unittest.mock import patch
import paused_recovery as recovery
import v8_upgrade as upgrade
from controller import Blocked


class RevisionTests(unittest.TestCase):
    def test_failed_execution_is_exact_and_never_completed_or_replayed(self):
        encode=lambda value:json.dumps(value,sort_keys=True).encode()
        request={'action':'resume-cold-backup','nonce':upgrade.FAILED_NONCE,
                 'operation':recovery.OPERATION,'run_id':upgrade.FAILED_RUN}
        journal={'request':request,'request_sha256':recovery.digest(request),
                 'phase':'STARTED','prepare_error':'guard_rejected'}
        response={'status':'BLOCKED','error':'exception_Blocked'}
        upgrade.failed_execution(encode(journal),encode(response))
        for field,value in (('action','prepare'),('nonce','f'*64),
                            ('operation','other'),('run_id',upgrade.FAILED_RUN+1)):
            changed=copy.deepcopy(journal);changed['request'][field]=value
            changed['request_sha256']=recovery.digest(changed['request'])
            with self.subTest(request=field),self.assertRaises(Blocked):
                upgrade.failed_execution(encode(changed),encode(response))
        for field,value in (('phase','COMPLETE'),('prepare_error','other'),('request_sha256','f'*64)):
            changed=copy.deepcopy(journal);changed[field]=value
            with self.subTest(journal=field),self.assertRaises(Blocked):
                upgrade.failed_execution(encode(changed),encode(response))
        for field,value in (('status','PASS'),('error','other')):
            changed=copy.deepcopy(response);changed[field]=value
            with self.subTest(response=field),self.assertRaises(Blocked):
                upgrade.failed_execution(encode(journal),encode(changed))
        self.assertEqual(journal['phase'],'STARTED')

    def test_fixed_chain_preserves_original_adoption_and_rejects_state_drift(self):
        previous={'fixture.py':'a'*64};replacement={'fixture.py':'b'*64}
        adoption={'schema':'voice-paused-cold-backup-adoption-v1','operation':recovery.OPERATION,
            'original_code_sha256':recovery.V6_BINDING,'replacement_code_sha256':recovery.digest(previous)}
        state={'operation':recovery.OPERATION,'phase':'COLD_BACKUP','status':'BLOCKED',
            'fence_status':'VERIFIED','error':'rollout_selected_store_custody_invalid',
            'code_capture':previous,'repair_adoption':recovery.digest(adoption),
            'target':{'tag':recovery.SOURCE,'mode':'images-only','changed_services':['story']}}
        encode=lambda value:json.dumps(value,sort_keys=True).encode()
        # Only the synthetic code inventory replaces the exact public release
        # pin; record/state semantics run unmodified. Physical cases use V7 tar.
        with patch.object(upgrade,'V7_BINDING',recovery.digest(previous)):
            result=upgrade.revision_record(encode(state),encode(adoption),previous,replacement)
            self.assertEqual(result['replacement_code_sha256'],recovery.digest(replacement))
            for field,value in (('operation','other'),('phase','AWAITING_OFF_NODE'),
                ('status','WAITING'),('fence_status','UNKNOWN'),('error','other'),
                ('code_capture',replacement),('repair_adoption','f'*64),('cut',{}),
                ('source_authority',{}),('execution_authority',{}),('paused_recovery_authority',{})):
                changed=copy.deepcopy(state);changed[field]=value
                with self.subTest(field=field),self.assertRaises(Blocked):
                    upgrade.revision_record(encode(changed),encode(adoption),previous,replacement)
            for field,value in (('schema','other'),('operation','other'),
                ('original_code_sha256','f'*64),('replacement_code_sha256','f'*64)):
                changed=copy.deepcopy(adoption);changed[field]=value
                changed_state=copy.deepcopy(state);changed_state['repair_adoption']=recovery.digest(changed)
                with self.subTest(adoption=field),self.assertRaises(Blocked):
                    upgrade.revision_record(encode(changed_state),encode(changed),previous,replacement)
            with self.assertRaises(Blocked):upgrade.revision_record(encode(state),encode(adoption),previous,previous)
            self.assertEqual(state['repair_adoption'],recovery.digest(adoption))


if __name__=='__main__':unittest.main()
