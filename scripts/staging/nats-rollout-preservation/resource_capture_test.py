"""Exercise the actual standalone renderer's protected capture reader."""
import ast
import hashlib
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


class CaptureTests(unittest.TestCase):
    def program(self,root):
        source=(Path(__file__).resolve().parents[1]/'render-and-apply.sh').read_text()
        body=source.split("python3 -I -S - <<'PY'\n",1)[1].split('\nPY\n',1)[0]
        tree=ast.parse(body)
        roots=[n for n in ast.walk(tree) if isinstance(n,ast.Constant) and n.value=='/var/lib/voice-nats-preservation']
        self.assertEqual(len(roots),1);roots[0].value=str(root)
        return compile(tree,'<actual-standalone-capture-reader>','exec')

    def fixture(self,root):
        base=root/('rollout-'+'a'*12);code=base/'code';code.mkdir(parents=True,mode=0o750)
        entry=code/'nats-rollout-preservation/runner.py';entry.parent.mkdir(mode=0o750)
        entry.write_text('# Capture-only fixture. Real executor boundary has separate tests.\n');entry.chmod(0o440)
        helper=entry.with_name('resource_claim.py');helper.write_text('# Capture-only fixture.\n');helper.chmod(0o440)
        manifest={str(p.relative_to(code)):hashlib.sha256(p.read_bytes()).hexdigest() for p in (entry,helper)}
        path=code/'capture-manifest.json';path.write_text(json.dumps(manifest));path.chmod(0o440)
        return base,entry,path,manifest

    def test_uses_root_captured_runner_instead_of_target_checkout(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder);base,entry,_,_=self.fixture(root)
            with patch.dict(os.environ,VOICE_NATS_PRESERVATION_RECEIPT=str(base/'apply-authorization.json')),\
                patch('runpy.run_path') as run,patch('sys.argv',[]):
                exec(self.program(root),{})
            run.assert_called_once_with(str(entry),run_name='__main__')

    def test_old_changed_and_unsafe_members_never_execute_runner(self):
        for fault in ('old','hash','symlink','duplicate','group-write'):
            with self.subTest(fault=fault),tempfile.TemporaryDirectory() as folder:
                root=Path(folder);base,entry,path,manifest=self.fixture(root)
                if fault=='old':del manifest['nats-rollout-preservation/resource_claim.py'];path.write_text(json.dumps(manifest))
                elif fault=='hash':entry.write_text('CHANGED=True')
                elif fault=='symlink':entry.unlink();entry.symlink_to('/etc/hostname')
                elif fault=='duplicate':path.write_text('{"x":"a","x":"b"}')
                else:entry.chmod(0o460)
                with patch.dict(os.environ,VOICE_NATS_PRESERVATION_RECEIPT=str(base/'apply-authorization.json')),patch('runpy.run_path') as run:
                    with self.assertRaisesRegex(SystemExit,'STAGING_RESOURCE_CAPTURE_REFUSED'):exec(self.program(root),{})
                run.assert_not_called()


if __name__=='__main__':unittest.main()
