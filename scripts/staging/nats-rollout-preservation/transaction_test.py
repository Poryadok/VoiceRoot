import copy
import datetime as dt
from pathlib import Path
import unittest
from unittest.mock import Mock, patch
import transaction


class TransactionFailureTest(unittest.TestCase):
    def test_successful_release_rollback_package_uses_actual_old_images_and_fresh_cut(self):
        old={'metadata':{'name':'voice-gateway','namespace':'voice-staging','uid':'deployment'},
             'kind':'Deployment','apiVersion':'apps/v1','spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'gateway','image':'mutable-old-tag'}]}}}}
        pin='example.invalid/gateway@sha256:'+'a'*64
        previous={'status':'PASS','operation':'123456abcdef','contract':{},
            'nonnats':{'checks':[],'actions':[]},'target':{'images':{}},
            'input_target':{'registry':'example.invalid','tag':'b'*40,'mode':'images-only','changed_services':['gateway'],
                'source_hashes':{'deploy/nats/acl-intent.yaml':'c'*64},'images':{'voice-gateway/gateway':'new'},'template_hashes':{'voice-gateway':'d'*64}},
            'context':{'original_snapshots':{'voice-gateway':old},'old_images':{'voice-gateway/gateway':pin}}}
        package=transaction.rollback_package(previous)
        self.assertEqual(package['target']['images'],{'voice-gateway/gateway':pin})
        self.assertEqual(package['target']['tag'],'b'*40) # trusted source identity; images remain explicit
        self.assertEqual(package['migrations'],[])
        self.assertNotIn('cut',package)
        self.assertEqual(previous['context']['original_snapshots']['voice-gateway']['spec']['template']['spec']['containers'][0]['image'],'mutable-old-tag')
    def test_guarded_image_rollback_proves_store_before_only_verified_restart(self):
        receipt={'operation':'123456abcdef','target':{'mode':'images-only'}}
        state={'phase':'PAUSED_APPLY','status':'BLOCKED','operation':'123456abcdef','target':receipt['target'],
               'authorization':receipt,'events':[],'provenance':{},'cut':{},'context':{}}
        stage=Mock();stage.final_path=Path('/selected-store');order=[]
        for method in ('adopt_rollback_target','restore_original_templates','verified','restart'):
            getattr(stage,method).side_effect=lambda *a,m=method:order.append(m)
        with patch.object(transaction,'reconstruct',return_value=stage),patch.object(transaction,'context',return_value={}),\
             patch.object(transaction,'revalidate_inputs'),patch.object(transaction,'DockerRuntime'),patch.object(transaction,'save'),\
             patch.object(transaction,'verify_post_apply',side_effect=lambda *a:order.append('proof') or {'verified':True}):
            result=transaction.rollback(None,Path('/private'),state,receipt,'101',{})
        self.assertEqual(order,['adopt_rollback_target','restore_original_templates','proof','verified','restart'])
        self.assertEqual(result['status'],'ROLLED_BACK')
    def test_rollback_after_restart_or_with_database_changes_has_zero_mutations(self):
        for phase,mode in (('RESTART','images-only'),('PAUSED_APPLY','full')):
            receipt={'operation':'123456abcdef','target':{'mode':mode}}
            state={'phase':phase,'status':'BLOCKED','operation':'123456abcdef','target':receipt['target'],'authorization':receipt}
            with patch.object(transaction,'reconstruct') as reconstruct:
                with self.assertRaisesRegex(transaction.Blocked,'rollback_fresh_transaction_required'):
                    transaction.rollback(None,Path('/private'),state,receipt,'101',{})
            reconstruct.assert_not_called()
    def test_process_interruption_retains_current_restart_mutation_context(self):
        receipt={'operation':'123456abcdef','target':{},'expires_at':(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=1)).isoformat()}
        state={'phase':'PAUSED_APPLY','status':'WAITING','operation':'123456abcdef','target':{},'authorization':receipt,
               'events':[],'provenance':{},'cut':{},'context':{}}
        stage=Mock();stage.final_path=Path('/selected-store');stage.current={'replicas':0,'override':False}
        written=[]
        def reconstruct(kube,state,journal):
            stage.save=journal
            return stage
        def interrupted_restart():
            stage.current={'replicas':1,'override':True}
            stage.save({'kind':'user_cycle_override','owner':state['operation']})
            raise KeyboardInterrupt('process killed after owned mutation')
        stage.restart.side_effect=interrupted_restart
        with patch.object(transaction,'reconstruct',side_effect=reconstruct),\
             patch.object(transaction,'context',side_effect=lambda s:copy.deepcopy(s.current)),\
             patch.object(transaction,'revalidate_inputs'),patch.object(transaction,'DockerRuntime'),\
             patch.object(transaction,'verify_post_apply',return_value={'verified':True}),\
             patch.object(transaction,'save',side_effect=lambda p,s:written.append(copy.deepcopy(s))):
            with self.assertRaises(KeyboardInterrupt):transaction.finish(None,Path('/private'),state,receipt,'101',{})
        self.assertEqual(written[-1]['context'],{'replicas':1,'override':True})
        self.assertEqual(written[-1]['events'][-1]['kind'],'user_cycle_override')
        self.assertEqual(written[-1]['phase'],'RESTART')
        stage.refence.assert_not_called() # abrupt death has no exception recovery
    def test_post_apply_proof_failure_records_blocked_and_never_restarts(self):
        stage=Mock();stage.final_path=Path('/selected-store')
        stage.refence.return_value={'verified':True}
        receipt={'operation':'123456abcdef','target':{},'expires_at':(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=1)).isoformat()}
        state={'phase':'PAUSED_APPLY','status':'WAITING','operation':'123456abcdef','target':{},'authorization':receipt,
               'events':[],'provenance':{},'cut':{},'context':{}}
        written=[]
        with patch.object(transaction,'reconstruct',return_value=stage),\
             patch.object(transaction,'context',return_value={}),\
             patch.object(transaction,'revalidate_inputs'),\
             patch.object(transaction,'DockerRuntime'),\
             patch.object(transaction,'save',side_effect=lambda p,s:written.append(copy.deepcopy(s))),\
             patch.object(transaction,'verify_post_apply',side_effect=transaction.Blocked('rollout_native_store_changed')):
            with self.assertRaises(transaction.Blocked):
                transaction.finish(None,Path('/private'),state,receipt,'101',{})
        stage.restart.assert_not_called();stage.verified.assert_not_called()
        self.assertEqual(written[-1]['status'],'BLOCKED')
        self.assertEqual(written[-1]['fence_status'],'VERIFIED')

if __name__=='__main__':unittest.main()
