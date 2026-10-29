import copy
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import commands
import renderer_root
import renderer_transition
import resource_claim
import resource_claim_test
import staging_cohort_preservation as cohort


class PreservedRendererTest(unittest.TestCase):
    def test_target_uses_protected_original_and_rejects_paused_template_drift(self):
        context=resource_claim_test.context();card=resource_claim_test.card(context);journal=resource_claim_test.MemoryJournal()
        session=resource_claim.Session(context,'a'*12,'b'*64,lambda:copy.deepcopy(card),journal)
        session.admit(start=True);journal.row.update(phase='cohort-preservation',target_sha256='c'*64,cohort_preservation_sha256='c'*64)
        name=renderer_root.HUB
        original={'metadata':{'uid':'uid','resourceVersion':'1'},'spec':{'replicas':1,'template':{'spec':{}}}}
        paused=copy.deepcopy(original);paused['metadata']['resourceVersion']='2';paused['spec']['replicas']=0
        stage=SimpleNamespace(expected={'source_claim':'claim','source_claim_uid':'pvc','source_pv_uid':'pv','generation':'g'},
            snapshots={name:paused},original_snapshots={name:original},old_images={})
        authority={'descriptor':{'source_sha':'commit','hub':{'uid':'uid','resourceVersion':'1'}},
                   'output':{'sha256':'a'*64,'bytes':1,'verified':True}}
        target={'tag':'commit','images':{},'template_hashes':{}}
        with patch.object(commands,'authority_guard',session.check),patch.object(cohort,'capture_final_stage',return_value=stage), \
             patch.object(renderer_transition,'validate') as validate,patch.object(renderer_root,'bind_target',return_value=target):
            renderer_root.validate_target(target,authority,stage)
            self.assertEqual(validate.call_args.args[1],original)
            altered=copy.deepcopy(stage);altered.snapshots[name]['spec']['template']={'changed':True}
            with self.assertRaisesRegex(renderer_root.Blocked,'renderer_target_capture_changed'):
                renderer_root.validate_target(target,authority,altered)


if __name__=='__main__':unittest.main()
