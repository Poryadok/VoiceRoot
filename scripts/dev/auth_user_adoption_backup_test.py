import copy
import unittest
from auth_user_adoption import RecoveryRequired
from auth_user_adoption_backup import validate_restore


class RestoreQualificationTest(unittest.TestCase):
    def setUp(self):
        self.manifest = {'format':1, 'operation':'owned', 'files':{'auth_db.enc':{'sha256':'a'*64,'hmac':'b'*64},
                        'user_db.enc':{'sha256':'c'*64,'hmac':'d'*64}, 'roles.enc':{'sha256':'e'*64,'hmac':'f'*64}},
                         'catalog':{'auth_db':{'owner':'voice','tables':{'accounts':'voice'}},'user_db':{'owner':'voice','tables':{'profiles':'voice'}}}}
        self.report = {'protocol':'voice-auth-user-backup-v1', 'operation':'owned', 'result':'PASS',
                       'files':copy.deepcopy(self.manifest['files']), 'restored_catalog':copy.deepcopy(self.manifest['catalog']),
                       'off_node':True, 'restore_exit':{'auth_db':0,'user_db':0,'roles':0},
                       'postgres_major':16,'verifier_sha256':'1'*64,'tools_image':'sha256:'+'2'*64,
                       'fixture_cleanup':True}

    def test_correct_bound_restore_is_eligible(self):
        self.assertTrue(validate_restore(self.manifest,self.report,'owned'))

    def test_partial_stale_foreign_or_unrestored_report_cannot_authorize_migration(self):
        for fault in ['operation','files','restored_catalog','off_node','result','restore_exit','postgres_major','verifier_sha256','fixture_cleanup']:
            report=copy.deepcopy(self.report)
            report[fault] = False
            with self.subTest(fault=fault), self.assertRaises(RecoveryRequired):
                validate_restore(self.manifest,report,'owned')
