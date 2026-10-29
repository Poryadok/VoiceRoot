"""Opt-in isolated signed NATS authorization and stream-default proof."""
import os
import unittest
from unittest.mock import patch
import bootstrap_auth
import nats_contract_plan as plan
import staging_cohort_preservation_auth_test as fixtures


@unittest.skipUnless(os.environ.get('VOICE_COHORT_AUTH_INPUTS'),'isolated generated auth fixture required')
class ReminderAuthIntegrationTests(unittest.TestCase):
    def test_reminder_actual_signed_bootstrap_auth(self):
        prove=bootstrap_auth.prove
        seen=[]
        def fixed(*args,**kwargs):
            result=prove(*args,**kwargs,version='voice-bootstrap-auth-v6')
            seen.append(result)
            return result
        with patch.object(bootstrap_auth,'prove',side_effect=fixed):
            fixtures.ActualAuthTests('test_actual_actor_and_bootstrap_scratch_with_root_session').test_actual_actor_and_bootstrap_scratch_with_root_session()
        self.assertEqual(len(seen),1)
        self.assertEqual(seen[0]['normalized_reminder_stream']['name'],plan.REMINDER_STREAM)
        self.assertEqual(seen[0]['normalized_reminder_stream']['subjects'],[plan.REMINDER_SUBJECT])
        self.assertEqual(seen[0]['normalized_reminder_stream_sha256'],plan.digest(seen[0]['normalized_reminder_stream']))
