"""Writer failures remain specific after Docker cleanup, without raw probe output."""
import unittest
from unittest.mock import patch
import commands, deployment_report, story_fixture
from controller import Blocked

POSITIVE={'status':'PASS','create_grpc_code':'OK','response_identity_type_verified':True,
    'sql_fields_verified':True,'story_count':1,'default_ttl_24h_verified':True,
    'archive_plus30d_verified':True,'missing_profile_code':'Unauthenticated',
    'negative_count_unchanged':True,'unrelated_row_count':0,'row_retained':True}
NEGATIVE={'status':'FAIL','phase':'wrong_expected_row_count'}

class WriterDiagnosticsTests(unittest.TestCase):
    def test_specific_writer_guard_is_public(self):
        self.assertEqual(deployment_report.error_code(Blocked('story_compatibility_fixture_create_story_unavailable')),
            'story_compatibility_fixture_create_story_unavailable')
    def test_observed_unavailable_and_action_survive_cleanup(self):
        with patch.object(commands,'_current_action','docker.run'):
            with self.assertRaises(Blocked) as caught:
                story_fixture.validate_writer({'status':'FAIL','phase':'create_story','grpc_code':'Unavailable'},17,NEGATIVE,17)
            failure=story_fixture.fixture_failure(caught.exception,'writer')
        with patch.object(commands,'_current_action','docker.other'):
            self.assertEqual(deployment_report.error_code(failure),'story_compatibility_fixture_create_story_unavailable')
            self.assertEqual(failure.diagnostic['action'],'docker.run')
            self.assertNotIn('positive',failure.diagnostic)
    def test_negative_control_is_mandatory(self):
        story_fixture.validate_writer(POSITIVE,0,NEGATIVE,17)
        for negative,exit_code in ((NEGATIVE,0),({'status':'PASS'},0),({'status':'FAIL','phase':'other'},17)):
            with self.subTest(negative=negative,exit=exit_code),self.assertRaises(Blocked) as caught:
                story_fixture.validate_writer(POSITIVE,0,negative,exit_code)
            self.assertEqual(caught.exception.args,('story_compatibility_fixture_negative_control_failed',))
    def test_parse_failure_freezes_action_without_private_exception(self):
        with patch.object(commands,'_current_action','docker.run'):
            failure=story_fixture.fixture_failure(ValueError('private body'),'writer',invalid=True)
        with patch.object(commands,'_current_action','docker.other'):
            self.assertEqual(deployment_report.error_code(failure),'story_compatibility_fixture_writer_invalid')
            self.assertEqual(failure.diagnostic['action'],'docker.run')
            self.assertNotIn('private',str(failure))
    def test_untrusted_probe_fields_do_not_enter_public_label(self):
        with self.assertRaises(Blocked) as caught:
            story_fixture.validate_writer({'status':'FAIL','phase':'secret material','grpc_code':'private'},17,NEGATIVE,17)
        self.assertEqual(caught.exception.args,('story_compatibility_fixture_positive_failed',))

if __name__=='__main__':unittest.main()
