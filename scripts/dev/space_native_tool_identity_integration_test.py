"""Explicit local-image fixture; never publishes or connects to a database."""
import hashlib
import os
from pathlib import Path
import subprocess
import unittest
from unittest import mock

import publish_candidate
import space_native_tool_identity as tool


@unittest.skipUnless(os.environ.get('VOICE_NATIVE_TOOL_FIXTURE_IMAGE') and os.environ.get('VOICE_NATIVE_TOOL_FIXTURE_SHA'),
                     'requires separately built immutable local native-tool fixture')
class LocalNativeImageTest(unittest.TestCase):
    def test_actual_binary_sql_compiled_gate_and_owned_cleanup(self):
        root=Path(__file__).resolve().parents[2]; image=os.environ['VOICE_NATIVE_TOOL_FIXTURE_IMAGE']
        source=os.environ['VOICE_NATIVE_TOOL_FIXTURE_SHA']; files={}
        for path in (root/'src/backend').rglob('*'):
            if path.is_file() and not path.is_symlink(): files[path.relative_to(root).as_posix()]=hashlib.sha256(path.read_bytes()).hexdigest()
        inventory=tool.source_inventory(root,{'source_sha':source,'files':files})
        actual=publish_candidate.command; created=[]
        config=actual(['docker','image','inspect',image,'--format','{{.Id}}']).decode().strip()
        def owned(args,**kwargs):
            if args[:2]==['docker','pull']: return b''
            result=actual(args,**kwargs)
            if args[:2]==['docker','create']: created.append(result.decode().strip())
            return result
        with mock.patch.object(publish_candidate,'command',side_effect=owned), \
             mock.patch.object(publish_candidate,'source_snapshot',return_value={'source_sha':source,'files':files}):
            proof=tool._extract_image(image,config,inventory,root)
            self.assertEqual(proof['compiled_gate_probe'],'native51-source-gates-reached-v1')
            self.assertEqual(proof['sql_inventory_sha256'],inventory['sql_inventory_sha256'])
            self.assertIn('src/backend/space/cmd/native-migrate',proof['closure']['packages'])
            with self.assertRaises(tool.ToolIdentityError): tool._run_compiled_probe(image,'0'*64)
            uri='postgres://fixture:fixture@127.0.0.1:1/postgres?sslmode=disable&connect_timeout=2'
            cid=owned(['docker','create','--network','none','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges',
                       '--pids-limit','64','--memory','128m','--cpus','1','--env','SPACE_NATIVE_ADMIN_URL='+uri,
                       '--env','SPACE_NATIVE_REQUIRED_SOURCE_SHA256='+inventory['build_source_sha256'],image,
                       '--mode=provision','--migrations-dir=/tmp']).decode().strip()
            try:
                result=subprocess.run(['docker','start','-a',cid],capture_output=True,timeout=30)
                self.assertEqual(result.returncode,1); self.assertEqual(result.stdout,b'')
                self.assertEqual(result.stderr,b'incomplete SQL inventory\n')
            finally: owned(['docker','rm','--force',cid])
        self.assertGreaterEqual(len(created),5)
        for cid in created:
            with self.assertRaises(publish_candidate.Refused): actual(['docker','inspect',cid])


if __name__=='__main__': unittest.main()
