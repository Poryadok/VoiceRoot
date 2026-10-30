import unittest
from unittest.mock import Mock, patch
import realtime_migration_verify as verify


class SchemaTest(unittest.TestCase):
    def test_fixed_login_and_guarded_readonly_queries(self):
        calls = []
        def guard(): calls.append('guard')
        snapshot = Mock(return_value={'pod_uid':'p'})
        result = Mock(returncode=0, stdout=b'1|f|13|realtime\n')
        with patch.object(verify.subprocess,'run',return_value=result) as process:
            receipt = verify.check(b'a'*32,13,{'pod_uid':'p'},guard,snapshot)
        self.assertEqual(receipt,{'database_oid':13,'version':1,'dirty':False,'scope':'realtime-login-schema-readiness'})
        self.assertGreaterEqual(len(calls),4)
        args = process.call_args
        self.assertNotIn('a'*32,str(args.args[0]))
        self.assertIn(b'a'*32,args.kwargs['input'])

    def test_wrong_oid_or_failed_login_refuses(self):
        for output, code in ((b'1|f|14|realtime\n',0),(b'',1),(b'1|t|13|realtime\n',0)):
            with patch.object(verify.subprocess,'run',return_value=Mock(returncode=code,stdout=output)):
                with self.assertRaises(verify.RecoveryRequired):
                    verify.check(b'a'*32,13,{'pod_uid':'p'},lambda:None,lambda:{'pod_uid':'p'})

    def test_changed_engine_refuses(self):
        with self.assertRaises(verify.RecoveryRequired):
            verify.check(b'a'*32,13,{'pod_uid':'p'},lambda:None,lambda:{'pod_uid':'changed'})


if __name__ == '__main__': unittest.main()
