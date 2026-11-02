"""Exercise the published issuer proof with config and health consumers.

Kubernetes, package, cut and packet crypto are synthetic; the issuer lifecycle,
its Journal/Fence, and both consumer dispatch paths are real.
"""
import copy
import hashlib
import unittest
from unittest.mock import patch

import realtime_config_transition_authority as authority
import realtime_config_transition_space_health as health
import realtime_config_transition_space_health_test as health_fixture
import realtime_jwks_publisher_phase_test as shared
import resource_fencing
import space_p3_issuer_create as issuer
import space_p3_issuer_create_test as issuer_fixture


class IssuerConsumersTest(unittest.TestCase):
    def cleanup_case(self, case):
        if not case.doCleanups(): raise RuntimeError('fixture cleanup failed')

    def test_completed_issuer_drives_config_and_health_and_rejects_drift(self):
        created=issuer_fixture.PhaseTest('test_create_only_receipt_rechecks_live_exact_objects')
        self.addCleanup(self.cleanup_case,created); created.setUp()
        proof=created.run_finish()
        fixture=health_fixture.HealthTest('test_journaled_post_resume_health')
        self.addCleanup(self.cleanup_case,fixture); fixture.setUp()
        fence=resource_fencing.Fence(created.base.context,lambda:copy.deepcopy(created.base.card),
                                     shared.View(created.base.ledger,fixture.client))
        for component in (fixture.cut,fixture.package,fixture.image):
            component.fence=fence; component.lock=created.base.lock; component.namespace_uid='ns'
        fixture.client.journal=fixture.journal
        expected=b'{"keys":["public"]}'
        fixture.observation.update(jwks_bytes=expected,jwks_sha256=hashlib.sha256(expected).hexdigest())
        with patch.object(issuer.packet,'validate',return_value=(copy.deepcopy(created.objects),{'keys':['public']})), \
             patch.object(issuer.crypto,'der',return_value=b'fixture-der'):
            source,keys=authority._space(fixture.cut,created.phase,'token')
            self.assertEqual(source,proof); self.assertEqual(keys,expected)
            phase=health.SpaceHealthPhase(fixture.journal,fence,fixture.client,created.base.lock,
                                          'ns',fixture.package,fixture.cut,created.phase,fixture.image,fixture.config)
            result=phase.run('token')
            self.assertEqual(health.verify_completed(phase,fixture.cut,'token'),result)
            created.actual['voice-space-principal-signing']['metadata']['uid']='foreign'
            with self.assertRaises(issuer.CreateError): authority._space(fixture.cut,created.phase,'token')


if __name__ == '__main__': unittest.main()
