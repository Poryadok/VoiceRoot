import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import time
import unittest


class CaptureTests(unittest.TestCase):
    def setUp(self):
        # Runs only inside the disposable Linux test container, never a host
        # root login or a staging filesystem.
        self.assertEqual(os.getuid(),0)
        if not Path('/usr/bin/python3.13').exists():
            Path('/usr/bin/python3.13').symlink_to(sys.executable)
        Path('/var/lib/voice-nats-preservation').mkdir(mode=0o750,parents=True,exist_ok=True)
        self.source=Path('/home/pmd/voice-known-baseline/known-baseline-bundle.tar')
        self.source.parent.mkdir(parents=True,exist_ok=True)
        if self.source.exists() or self.source.is_symlink():self.source.unlink()
        self.marker=Path('/tmp/known-nats-capture-executed')
        self.marker.unlink(missing_ok=True)
        content={name:b'fixed-test-input' for name in ('controller.py','commands.py','docker_runtime.py','stage_runtime.py','scenario.py','deployed-contract.json','kernel')}
        content['root_main.py']=b'from pathlib import Path\nPath("/tmp/known-nats-capture-executed").touch()\n'
        content['capture-manifest.json']=json.dumps({'files':{k:hashlib.sha256(v).hexdigest() for k,v in content.items()}}).encode()
        raw=io.BytesIO()
        with tarfile.open(fileobj=raw,mode='w') as archive:
            for name,data in content.items():
                member=tarfile.TarInfo(name);member.size=len(data);archive.addfile(member,io.BytesIO(data))
        self.raw=raw.getvalue();self.source.write_bytes(self.raw)
        os.chown(self.source,1000,1000);self.source.chmod(0o644)
        template=Path('/work/root-launch.template.sh').read_text()
        self.launcher=Path('/tmp/known-nats-launch.sh')
        self.launcher.write_text(template.replace('__BUNDLE_SHA256__',hashlib.sha256(self.raw).hexdigest()).replace('__BUNDLE_BYTES__',str(len(self.raw))))

    def run_launcher(self,args=None):
        return subprocess.run(['/bin/bash',str(self.launcher),*(args or ['--prepare'])],stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=3)

    def test_exact_fence_continuation_reaches_private_code(self):
        result=self.run_launcher(['--continue-fence','/var/lib/voice-nats-preservation/known-baseline-0049430b0dbb'])
        self.assertEqual(result.returncode,0,result.stderr.decode())
        self.assertTrue(self.marker.exists())

    def test_other_fence_continuation_operation_never_executes(self):
        result=self.run_launcher(['--continue-fence','/var/lib/voice-nats-preservation/known-baseline-44debe2248bc'])
        self.assertNotEqual(result.returncode,0)
        self.assertFalse(self.marker.exists())

    def test_exact_existing_baseline_recovery_reaches_private_code(self):
        result=self.run_launcher(['--continue-staging-baseline','/var/lib/voice-nats-preservation/known-baseline-0049430b0dbb'])
        self.assertEqual(result.returncode,0,result.stderr.decode())
        self.assertTrue(self.marker.exists())

    def test_other_baseline_recovery_operation_never_executes(self):
        result=self.run_launcher(['--continue-staging-baseline','/var/lib/voice-nats-preservation/known-baseline-44debe2248bc'])
        self.assertNotEqual(result.returncode,0)
        self.assertFalse(self.marker.exists())

    def test_exact_captured_bytes_execute_private_copy(self):
        result=self.run_launcher()
        self.assertEqual(result.returncode,0,result.stderr.decode())
        self.assertTrue(self.marker.exists())

    def test_changed_public_bytes_never_execute(self):
        self.source.write_bytes(self.raw[:-1]+b'x')
        result=self.run_launcher()
        self.assertNotEqual(result.returncode,0)
        self.assertFalse(self.marker.exists())

    def test_source_symlink_never_followed(self):
        other=Path('/tmp/known-nats-untrusted-target');other.write_bytes(self.raw);os.chown(other,1000,1000)
        self.source.unlink();self.source.symlink_to(other)
        self.assertNotEqual(self.run_launcher().returncode,0)
        self.assertFalse(self.marker.exists())

    def test_source_fifo_never_blocks_or_executes(self):
        self.source.unlink();os.mkfifo(self.source);os.chown(self.source,1000,1000)
        started=time.monotonic()
        self.assertNotEqual(self.run_launcher().returncode,0)
        self.assertLess(time.monotonic()-started,2)
        self.assertFalse(self.marker.exists())


if __name__=='__main__':unittest.main()
