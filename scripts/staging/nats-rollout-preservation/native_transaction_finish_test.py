import datetime as dt
import unittest
from unittest.mock import patch

import provisional
import transaction


class Stage:
    def __init__(self):
        self.native_api = object()
        self.verified_count = 0
        self.restart_count = 0
        self.final_claim = {'metadata': {'uid': 'claim-uid'}}
        self.marker = {'metadata': {'uid': 'marker-uid'}}

    def adopt_applied_target(self, receipt, claim_rv):
        return None

    def verify_final_storage(self):
        return None

    def verified(self):
        self.verified_count += 1

    def restart(self):
        self.restart_count += 1
        return {'restarted': True}

    def refence(self):
        return {'verified': True}


class NativeFinishTests(unittest.TestCase):
    def test_native_finish_uses_typed_preservation_runtime_and_never_docker(self):
        stage=Stage()
        api=stage.native_api
        target={'manifest_sha256':'b'*64}
        receipt={'operation':'0123456789ab','target':target,
            'expires_at':(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=1)).isoformat()}
        state={'operation':'0123456789ab','target':target,'authorization':receipt,
            'phase':'PAUSED_APPLY','status':'WAITING','provenance':{},'events':[],
            'cut':{'manifest':{},'census':{},'census_sha256':'c'*64},'contract':{}}
        runtime=object()
        preserved={'native_files_verified':True,'census_verified':True}
        config={'phase_image':'registry.example/phase@sha256:'+'a'*64}
        with patch.object(provisional,'receipt_binding'), \
             patch.object(transaction,'checkpoint_journal',return_value=lambda event:None), \
             patch.object(transaction,'reconstruct',return_value=stage) as reconstruct, \
             patch.object(transaction,'revalidate_inputs'), \
             patch.object(transaction,'native_preservation_runtime',return_value=runtime) as make_runtime, \
             patch.object(transaction,'verify_post_apply',return_value=preserved) as verify, \
             patch.object(transaction,'attach_closed_preservation'), \
             patch.object(transaction,'context',return_value={'native':True}), \
             patch.object(transaction,'save'), \
             patch.object(transaction,'DockerRuntime',side_effect=AssertionError('native finish used Docker')):
            result=transaction.finish(None,'/unused',state,receipt,'7',{},native_api=api,
                native_config=config)
        reconstruct.assert_called_once()
        self.assertIs(reconstruct.call_args.kwargs['native_api'],api)
        make_runtime.assert_called_once_with('/unused',state,stage,api,config,state['cut'],'finish')
        verify.assert_called_once_with(runtime,None,state['cut'],stage.verify_final_storage)
        self.assertEqual(result['preservation'],preserved)
        self.assertEqual(result['status'],'PASS')
        self.assertEqual(result['fence_status'],'RELEASED')
        self.assertEqual((stage.verified_count,stage.restart_count),(1,1))

