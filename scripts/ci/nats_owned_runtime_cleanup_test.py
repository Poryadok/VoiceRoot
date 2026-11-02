"""Reconciler tests use synthetic Docker observations; no daemon is touched."""
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
import nats_owned_runtime_cleanup as cleanup


class Tests(unittest.TestCase):
    def test_removes_only_exact_container_bound_to_private_fixture(self):
        with tempfile.TemporaryDirectory(dir='/root') as temp:
            root=Path(temp);cid='a'*64;operation='actor'+'b'*12;present=[cid];calls=[]
            row={'Id':cid,'Name':'/voice-known-'+operation+'-broker','Config':{'Labels':{cleanup.LABEL:operation}},
                 'Mounts':[{'Type':'bind','Source':str(root/'scratch'/'server.conf')}]}
            def docker(*args):
                calls.append(args)
                if args[0]=='ps':return '\n'.join(present)
                if args[0]=='inspect':return json.dumps([row])
                if args[0]=='rm':present.remove(cid);return cid
                raise AssertionError(args)
            with patch.object(cleanup,'docker',side_effect=docker):cleanup.reconcile(root)
            self.assertIn(('rm','-f',cid),calls)
            self.assertEqual(present,[])

    def test_foreign_mount_or_daemon_failure_never_removes(self):
        with tempfile.TemporaryDirectory(dir='/root') as temp:
            root=Path(temp);cid='a'*64;operation='auth'+'b'*12;calls=[]
            row={'Id':cid,'Name':'/voice-known-'+operation+'-proof','Config':{'Labels':{cleanup.LABEL:operation}},
                 'Mounts':[{'Type':'bind','Source':'/etc/foreign'}]}
            def docker(*args):
                calls.append(args)
                if args[0]=='ps':return cid
                if args[0]=='inspect':return json.dumps([row])
                raise AssertionError(args)
            with patch.object(cleanup,'docker',side_effect=docker),self.assertRaisesRegex(ValueError,'foreign_nats_fixture_mount'):
                cleanup.reconcile(root)
            self.assertFalse(any(call[0]=='rm' for call in calls))
            with patch.object(cleanup,'docker',side_effect=RuntimeError('daemon unavailable')),self.assertRaises(RuntimeError):
                cleanup.reconcile(root)

if __name__=='__main__':unittest.main()
