import copy
import unittest
import bridge
import bridge_client
import bridge_root
import provisional
from pathlib import Path
from unittest.mock import patch

class Tests(unittest.TestCase):
    def setUp(self):
        self.env={'GITHUB_TOKEN':'private-token','GITHUB_RUN_ID':'123','GITHUB_RUN_ATTEMPT':'1',
            'GITHUB_SHA':'a'*40,'GITHUB_JOB':'deploy'}
        self.args=['prepare','images-only','voice','a'*40,'123']

    def test_typed_flag_changes_nonce_and_is_bound(self):
        ordinary=bridge_client.request(self.args,self.env)
        provisional=bridge_client.request(self.args+['true'],self.env)
        self.assertNotIn('provisional_staging',ordinary)
        self.assertIs(provisional['provisional_staging'],True)
        self.assertNotEqual(ordinary['nonce'],provisional['nonce'])
        for bad in ('true',1,None):
            row=dict(provisional,provisional_staging=bad)
            with self.assertRaises(bridge.BridgeError):bridge.validate(row)

    def test_provisional_full_mode_and_status_override_rejected(self):
        row=bridge_client.request(self.args+['true'],self.env)
        with self.assertRaises(bridge.BridgeError):bridge.validate(dict(row,mode='full'))
        status=bridge_client.request(['status','1'*12],self.env)
        with self.assertRaises(bridge.BridgeError):bridge.validate(dict(status,provisional_staging=True))

    def test_named_repair_parent_and_acceptance_identity(self):
        row=bridge_client.request(['prepare-repair','1'*12,*self.args[1:],'true'],self.env)
        self.assertEqual(row['operation'],'1'*12)
        self.assertEqual(row['source_sha'],self.env['GITHUB_SHA'])
        other=bridge_client.request(['prepare-repair','2'*12,*self.args[1:],'true'],self.env)
        self.assertNotEqual(row['nonce'],other['nonce'])
        accepted=bridge_client.request(['accept-provisional','1'*12,'123'],self.env)
        self.assertEqual(accepted['action'],'accept-provisional')
        self.assertNotIn('provisional_staging',accepted)

    def test_repair_cannot_mix_dispatch_source(self):
        args=['prepare-repair','1'*12,'images-only','voice','b'*40,'123','true']
        with self.assertRaisesRegex(bridge.BridgeError,'bridge_workflow_source_changed'):
            bridge_client.request(args,self.env)

    def test_real_repair_dispatch_requires_all_parent_services_before_new_capture(self):
        actions=object.__new__(bridge_root.Actions);actions.binding={'fixed':'hash'}
        parent={'operation':'1'*12,'target':{'changed_services':['voice','gateway']}}
        request={'action':'prepare-repair','operation':'1'*12,'changed_services':['web']}
        with patch.object(actions,'state',return_value=(Path('/absent-parent'),parent)),\
             patch.object(provisional,'require_ready'),patch.object(provisional,'verify_runtime') as runtime,\
             patch.object(actions,'_prepare') as prepare:
            with self.assertRaisesRegex(bridge_root.Blocked,'provisional_repair_scope_missing'):
                actions.execute(request)
        runtime.assert_not_called();prepare.assert_not_called()

    def test_real_repair_dispatch_rechecks_runtime_before_fresh_transaction(self):
        actions=object.__new__(bridge_root.Actions);actions.binding={'fixed':'hash'}
        parent={'operation':'1'*12,'target':{'changed_services':['voice']}}
        request={'action':'prepare-repair','operation':'1'*12,'changed_services':['voice','web']}
        order=[]
        with patch.object(actions,'state',return_value=(Path('/absent-parent'),parent)),\
             patch.object(provisional,'require_ready'),patch.object(bridge_root,'Kube'),\
             patch.object(bridge_root,'private_json',return_value={}),\
             patch.object(provisional,'verify_runtime',side_effect=lambda *a,**k:order.append('runtime')) as runtime,\
             patch.object(actions,'_prepare',side_effect=lambda *a,**k:order.append('fresh')) as prepare:
            actions.execute(request)
        self.assertEqual(order,['runtime','fresh'])
        self.assertFalse(runtime.call_args.kwargs['require_healthy'])
        prepare.assert_called_once_with(request,repair=parent)

if __name__=='__main__':unittest.main()
