"""Actual isolated launcher custody checks; no staging/root host login."""
import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest

class BundleCaptureTest(unittest.TestCase):
    def setUp(self):
        self.assertEqual(os.getuid(),0)
        if not Path('/usr/bin/python3').exists():Path('/usr/bin/python3').symlink_to(sys.executable)
        self.root=Path('/var/lib/voice-nats-preservation');self.root.mkdir(parents=True,exist_ok=True,mode=0o700)
        self.source=Path('/home/pmd/voice-nats-rollout-v7/rollout-bundle.tar');self.source.parent.mkdir(parents=True,exist_ok=True)
        self.source.unlink(missing_ok=True)
        contents={'nats-rollout-preservation/root_cli.py':b"print('CAPTURED_ROOT_ENTRY')\n"}
        contents['capture-manifest.json']=json.dumps({n:hashlib.sha256(v).hexdigest() for n,v in contents.items()}).encode()
        output=io.BytesIO()
        with tarfile.open(fileobj=output,mode='w') as archive:
            for name,raw in contents.items():
                member=tarfile.TarInfo(name);member.size=len(raw);archive.addfile(member,io.BytesIO(raw))
        self.raw=output.getvalue();self.source.write_bytes(self.raw);os.chown(self.source,1000,1000);self.source.chmod(0o644)
        temp=tempfile.TemporaryDirectory();self.addCleanup(temp.cleanup);self.launcher=Path(temp.name)/'launch.sh'
        template=Path(__file__).with_name('root-launch.template.sh').read_text()
        self.launcher.write_text(template.replace('__BUNDLE_SHA256__',hashlib.sha256(self.raw).hexdigest()).replace('__BUNDLE_BYTES__',str(len(self.raw))).replace('__BUNDLE_FILES__',repr(sorted(contents))))
    def run_launcher(self):
        return subprocess.run(['/bin/bash',str(self.launcher),'--status','unused'],capture_output=True,text=True,timeout=3)
    def test_exact_captured_bytes_execute_root_private_copy(self):
        result=self.run_launcher();self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(result.stdout.strip(),'CAPTURED_ROOT_ENTRY')
    def test_tampered_source_denied_before_any_entry_execution(self):
        self.source.write_bytes(self.raw[:-1]+b'x');result=self.run_launcher()
        self.assertNotEqual(result.returncode,0);self.assertNotIn('CAPTURED_ROOT_ENTRY',result.stdout)
    def test_group_writable_public_archive_denied(self):
        self.source.chmod(0o664);result=self.run_launcher()
        self.assertNotEqual(result.returncode,0);self.assertNotIn('CAPTURED_ROOT_ENTRY',result.stdout)
    def test_fifo_does_not_block_or_execute(self):
        self.source.unlink();os.mkfifo(self.source);os.chown(self.source,1000,1000)
        result=self.run_launcher();self.assertNotEqual(result.returncode,0);self.assertNotIn('CAPTURED_ROOT_ENTRY',result.stdout)

if __name__=='__main__':unittest.main()
