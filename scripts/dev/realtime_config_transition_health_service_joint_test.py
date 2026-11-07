"""Real Service authority consumed by Space health, synthetic runtime inputs."""
import copy
import unittest
from unittest.mock import patch

import realtime_config_transition_joint_test as mixed_fixture
import realtime_config_transition_space_health as health
import realtime_config_transition_space_health_test as health_fixture
import realtime_jwks_publisher_phase_test as shared
import resource_fencing
import writer_cut


class HealthServiceJointTest(unittest.TestCase):
    def cleanup_case(self,case):
        if not case.doCleanups(): raise RuntimeError('fixture cleanup failed')

    def test_health_consumes_real_service_journal_and_rejects_drift(self):
        mixed=mixed_fixture.MixedTest('test_ordered_real_producers_with_shared_cut')
        self.addCleanup(self.cleanup_case,mixed); mixed.setUp()
        actual_cut_type=writer_cut.WriterCut
        fixture=health_fixture.HealthTest('test_journaled_post_resume_health')
        self.addCleanup(self.cleanup_case,fixture); fixture.setUp()
        health_client=copy.copy(mixed.base.client); health_client.journal=fixture.journal
        fence=resource_fencing.Fence(mixed.base.context,lambda:copy.deepcopy(mixed.base.card),
                                     shared.View(mixed.base.ledger,health_client))
        fixture.package.fence=fence; fixture.package.lock=mixed.base.lock
        fixture.image.fence=fence; fixture.image.lock=mixed.base.lock
        fixture.plan.cut=mixed.base.cut
        fixture.observation['identity']['service']={'uid':'ss','resource_version':'2'}
        original_call=fixture.client.call
        def read(verb,kind,name,*args):
            if kind=='service': return copy.deepcopy(mixed.services['voice-space'])
            return original_call(verb,kind,name,*args)
        health_client.call=read
        resumed={'controllers':[{'kind':'Deployment','name':'voice-space','uid':'dep','generation':4,'spec':{},'replicas':1}]}
        with patch.object(writer_cut,'WriterCut',actual_cut_type), \
             patch.object(health.frontend_runtime_executor,'KubectlTransport',type(mixed.base.client)), \
             patch.object(mixed.base.cut,'current_resumed_proof',return_value=resumed):
            # Health fixture installs a stub; restore the actual verifier explicitly.
            fixture.patches[-2].stop()
            phase=health.SpaceHealthPhase(fixture.journal,fence,health_client,mixed.base.lock,'ns',fixture.package,
                                          mixed.base.cut,fixture.plan,fixture.image,mixed.config)
            health.space_service._source(mixed.base.cut,mixed.config)
            health.space_service._check(mixed.base.cut,mixed.config)
            phase._sources('token'); phase._service(fixture.observation)
            result=phase.run('token')
            self.assertEqual(health.verify_completed(phase,mixed.base.cut,'token'),result)
            mixed.services['voice-space']['metadata']['resourceVersion']='foreign'
            with self.assertRaises(health.HealthError): health.verify_completed(phase,mixed.base.cut,'token')


if __name__=='__main__': unittest.main()
