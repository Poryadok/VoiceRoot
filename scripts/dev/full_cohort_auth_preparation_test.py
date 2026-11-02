"""Bounded Auth tool preparation; Kube, registry and local image inspection are synthetic."""
import copy
import unittest
from types import SimpleNamespace
from unittest.mock import patch

import full_cohort_backup_phase as backup


class AuthPreparationTest(unittest.TestCase):
    def test_preparation_binds_package_node_images_and_rejects_drift(self):
        tool='sha256:'+'a'*64
        package={'nodes':[{'name':'pg-node','uid':'node-uid','architecture':'amd64'}],
                 'platform_digests':{'voice-auth/auth':'sha256:'+'b'*64}}
        state={'replicas':1,'deployment_uid':'auth-uid'}
        class Package:
            def verify_current(self, token):
                self.assert_token=token
                return copy.deepcopy(package)
        class Client:
            def call(self,*args):
                if args[:3]==('get','pod','voice-postgres-0'):
                    return {'metadata':{'name':'voice-postgres-0','namespace':'voice-dev','uid':'pod-uid'},
                            'spec':{'nodeName':'pg-node'}}
                if args[:3]==('get','node','pg-node'):
                    return {'metadata':{'name':'pg-node','uid':'node-uid','resourceVersion':'1'},
                            'status':{'nodeInfo':{'operatingSystem':'linux','architecture':'amd64'},
                                      'conditions':[{'type':'Ready','status':'True'}]}}
                if args[:3]==('get','deployment','voice-auth'):
                    return {'metadata':{'name':'voice-auth','namespace':'voice-dev','uid':state['deployment_uid']},
                            'spec':{'replicas':state['replicas'],'template':{'spec':{'containers':[{'name':'auth'}]}}}}
                raise AssertionError(args)
        class Supervisor:
            def __init__(self):
                self.context={'operation_id':'media-c1-'+'a'*32,'commit':'c'*40,
                              'images':{'voice-auth/auth':'image@sha256:'+'d'*64}}
                self.cut=SimpleNamespace(started=False,drained=False)
                self._execution_actor='actor';self._execution_token='token'
                self.package=Package();self.client=Client();self.guards=0
            def _guard(self):self.guards+=1
        with patch.object(backup.full_cohort_supervisor,'Supervisor',Supervisor), \
             patch.object(backup.auth_live,'flyway_platform',return_value=tool), \
             patch.object(backup.auth_live.eligibility,'candidate_target',side_effect=lambda spec,image:spec), \
             patch.object(backup.auth_live.eligibility,'read_image_config',return_value={'env':{},'config_digest':'digest'}), \
             patch.object(backup.auth_live.eligibility,'read_references',return_value=({},{})), \
             patch.object(backup.auth_live.eligibility,'resolve_environment',return_value={}), \
             patch.object(backup.auth_live.eligibility,'check_environment',return_value={'flyway':'disabled'}), \
             patch.object(backup.auth_live.eligibility,'check_database_route',return_value={'database':'auth_db'}), \
             patch.object(backup.auth_live.eligibility,'_source_tree',return_value='a'*40), \
             patch.object(backup.story_media_backup_restore,'_check_image',side_effect=lambda image:image):
            owner=Supervisor();prepared=backup.AuthToolPreparation(owner,'token')
            result=prepared.verify_current('token')
            self.assertEqual(result['tool']['node_uid'],'node-uid')
            self.assertEqual(result['pod_uid'],'pod-uid')
            self.assertGreater(owner.guards,0)
            state['replicas']=0
            self.assertEqual(prepared.verify_current('token'),result)
            state['deployment_uid']='foreign'
            with self.assertRaises(backup.BackupError):prepared.verify_current('token')
            state['deployment_uid']='auth-uid'
            package['nodes'][0]['uid']='foreign-node'
            with self.assertRaises(backup.auth_live.PhaseError):prepared.verify_current('token')
            package['nodes'][0]['uid']='node-uid'
            owner._execution_token='other'
            with self.assertRaises(backup.BackupError):prepared.verify_current('token')

    def test_preparation_refuses_after_drain(self):
        class Supervisor:
            context={'operation_id':'media-c1-'+'a'*32}
            cut=SimpleNamespace(started=True,drained=True)
            _execution_actor='actor';_execution_token='token'
        with patch.object(backup.full_cohort_supervisor,'Supervisor',Supervisor):
            with self.assertRaises(backup.BackupError):backup.AuthToolPreparation(Supervisor(),'token')

    def test_prepared_c1_requires_auth_database_before_journal(self):
        phase=object.__new__(backup.BackupPhase)
        phase.supervisor=SimpleNamespace(context={'operation_id':'media-c1-'+'a'*32})
        phase.auth_preparation=object()
        with self.assertRaisesRegex(backup.BackupError,'prepared C1 Auth database unavailable'):
            phase._auth_intent({'databases':['postgres']},'token')
        phase.auth_preparation=None
        self.assertIsNone(phase._auth_intent({'databases':['postgres']},'token'))


if __name__=='__main__':unittest.main()
