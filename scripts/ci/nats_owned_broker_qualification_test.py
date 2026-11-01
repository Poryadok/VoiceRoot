"""Retention-order test only: synthetic generator and failed runtime."""
import json
from pathlib import Path
import tempfile
import types
import unittest
from unittest.mock import patch
import nats_owned_broker_qualification as harness
import nats_owned_runtime_cleanup as cleanup


class Tests(unittest.TestCase):
    def test_runtime_failure_retains_private_files_until_external_reconcile(self):
        with tempfile.TemporaryDirectory(dir='/root') as temp:
            root=Path(temp);cid='a'*64;op='actor'+'b'*12;present=[cid];holder={}
            def generate(args,**kwargs):
                private=Path(args[1]);holder['private']=private
                genesis={'realm_id':'734fe817-dc75-4732-89ac-6aecb83f0a32','generation':'r20260930a4',
                         'account':'synthetic-account','users':{'bootstrap':'synthetic-bootstrap'}}
                (private/'genesis.json').write_text(json.dumps(genesis))
                for name in ('bootstrap.creds','operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public'):
                    (private/name).write_bytes(b'synthetic')
                return types.SimpleNamespace(returncode=0)
            with patch.object(harness.subprocess,'run',side_effect=generate),patch.object(harness,'enroll_owned',side_effect=harness.Blocked('runtime cleanup failed')):
                with self.assertRaises(harness.Blocked):harness._qualify_in_fixture(Path('/unused'),Path('/unused'),root)
            private=holder['private'];self.assertTrue((private/'bootstrap.creds').is_file())
            row={'Id':cid,'Name':'/voice-known-'+op+'-broker','Config':{'Labels':{cleanup.LABEL:op}},
                 'Mounts':[{'Type':'bind','Source':str(private/'bootstrap.creds')}]}
            def docker(*args):
                if args[0]=='ps':return '\n'.join(present)
                if args[0]=='inspect':return json.dumps([row])
                if args[0]=='rm':
                    self.assertTrue((private/'bootstrap.creds').is_file())
                    present.remove(cid);return cid
                raise AssertionError(args)
            with patch.object(cleanup,'docker',side_effect=docker):cleanup.reconcile(root)
            self.assertTrue((private/'bootstrap.creds').is_file())
            self.assertEqual(present,[])

if __name__=='__main__':unittest.main()
