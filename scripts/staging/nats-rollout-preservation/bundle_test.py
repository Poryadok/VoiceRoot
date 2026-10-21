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

class CapturedImportClosureTest(unittest.TestCase):
    def test_actual_archive_imports_both_policy_consumers_without_checkout_fallback(self):
        import bundle
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            # build() checks ELF custody shape; these inert bytes are NEVER run.
            elf = root / 'synthetic-not-executable.elf'
            elf.write_bytes(b'\x7fELFsynthetic-offline-packaging-fixture')
            output = root / 'output'
            archive_sha = bundle.build(elf, output, elf, elf)
            raw = (output / 'rollout-bundle.tar').read_bytes()
            self.assertEqual(hashlib.sha256(raw).hexdigest(), archive_sha)
            captured = root / 'captured'
            captured.mkdir()
            with tarfile.open(fileobj=io.BytesIO(raw)) as archive:
                contents = {m.name: archive.extractfile(m).read() for m in archive.getmembers()}
            manifest = json.loads(contents.pop('capture-manifest.json'))
            self.assertEqual(manifest, {name: hashlib.sha256(data).hexdigest() for name, data in contents.items()})
            for name, data in contents.items():
                path = captured / name
                self.assertTrue(path.resolve().is_relative_to(captured.resolve()))
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(data)
            # -I/-S ignore checkout/PYTHONPATH/site-packages. Actual importer
            # origins must be the extracted archive, not the test process.
            script = '''import pathlib, sys
root = pathlib.Path(sys.argv[1]).resolve()
sys.path[:0] = [str(root / 'nats-rollout-preservation'), str(root / 'nats-known-baseline')]
import compiler, source_plan, release_policy_wiring
for module in (compiler, source_plan, release_policy_wiring):
    assert pathlib.Path(module.__file__).resolve().is_relative_to(root)
assert compiler.release_policy_wiring is source_plan.release_policy_wiring
print('CAPTURED_IMPORT_CLOSURE_OK')
'''
            result = subprocess.run([sys.executable, '-I', '-S', '-c', script, str(captured)],
                                    cwd=captured, capture_output=True, text=True, timeout=15)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout.strip(), 'CAPTURED_IMPORT_CLOSURE_OK')
            helper = 'nats-rollout-preservation/release_policy_wiring.py'
            self.assertEqual(contents[helper], Path(__file__).with_name('release_policy_wiring.py').read_bytes())

class BundleCaptureTest(unittest.TestCase):
    def setUp(self):
        self.assertEqual(os.getuid(),0)
        if not Path('/usr/bin/python3').exists():Path('/usr/bin/python3').symlink_to(sys.executable)
        self.root=Path('/var/lib/voice-nats-preservation');self.root.mkdir(parents=True,exist_ok=True,mode=0o700)
        self.source=Path('/home/pmd/voice-nats-rollout-full-repair/rollout-bundle.tar');self.source.parent.mkdir(parents=True,exist_ok=True)
        self.source.unlink(missing_ok=True)
        contents={'nats-rollout-preservation/operator_retirement.py':b"import sys\nprint('RETIRE_ARGS=' + ','.join(sys.argv[1:]))\n",
            'nats-rollout-preservation/root_cli.py':b"print('CAPTURED_ROOT_ENTRY')\n",
            'nats-rollout-preservation/installer.py':b"import sys\nprint('INSTALLER_ARGS=' + ','.join(sys.argv[1:]))\n"}
        contents['capture-manifest.json']=json.dumps({n:hashlib.sha256(v).hexdigest() for n,v in contents.items()}).encode()
        output=io.BytesIO()
        with tarfile.open(fileobj=output,mode='w') as archive:
            for name,raw in contents.items():
                member=tarfile.TarInfo(name);member.size=len(raw);archive.addfile(member,io.BytesIO(raw))
        self.raw=output.getvalue();self.source.write_bytes(self.raw);self.source.chmod(0o644)
        temp=tempfile.TemporaryDirectory();self.addCleanup(temp.cleanup);self.launcher=Path(temp.name)/'launch.sh'
        template=Path(__file__).with_name('root-launch.template.sh').read_text()
        self.launcher.write_text(template.replace('__BUNDLE_SHA256__',hashlib.sha256(self.raw).hexdigest()).replace('__BUNDLE_BYTES__',str(len(self.raw))).replace('__BUNDLE_FILES__',repr(sorted(contents))))
    def run_launcher(self,*arguments):
        # Transfer only after the root-owned test setup/mutations are complete.
        # The real launcher always observes the canonical unprivileged owner.
        os.chown(self.source,1000,1000)
        return subprocess.run(['/bin/bash',str(self.launcher),*(arguments or ('--status','unused'))],capture_output=True,text=True,timeout=3)
    def test_expanded_native_bundle_above32MiB_is_captured_with_member_bounds(self):
        with tarfile.open(fileobj=io.BytesIO(self.raw)) as archive:
            contents={m.name:archive.extractfile(m).read() for m in archive.getmembers() if m.name!='capture-manifest.json'}
        contents['nats-known-baseline/kernel']=b'\x7fELF'+b'k'*(20<<20)
        contents['nats-rollout-preservation/story-current4-probe']=b'\x7fELF'+b'p'*(15<<20)
        contents['capture-manifest.json']=json.dumps({n:hashlib.sha256(v).hexdigest() for n,v in contents.items()}).encode()
        output=io.BytesIO()
        with tarfile.open(fileobj=output,mode='w') as archive:
            for name,raw in contents.items():
                member=tarfile.TarInfo(name);member.size=len(raw);archive.addfile(member,io.BytesIO(raw))
        self.raw=output.getvalue();self.assertGreater(len(self.raw),32<<20);self.assertLess(len(self.raw),64<<20)
        self.source.write_bytes(self.raw)
        template=Path(__file__).with_name('root-launch.template.sh').read_text()
        self.launcher.write_text(template.replace('__BUNDLE_SHA256__',hashlib.sha256(self.raw).hexdigest()).replace('__BUNDLE_BYTES__',str(len(self.raw))).replace('__BUNDLE_FILES__',repr(sorted(contents))))
        result=self.run_launcher();self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(result.stdout.strip(),'CAPTURED_ROOT_ENTRY')
    def test_exact_idle_upgrade_argument_reaches_captured_installer(self):
        result=self.run_launcher('--install','--upgrade-idle')
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(result.stdout.strip(),'INSTALLER_ARGS=--upgrade-idle')
    def test_exact_ceccb_idle_upgrade_argument_reaches_captured_installer(self):
        result=self.run_launcher('--install','--upgrade-idle-ceccb-v1')
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(result.stdout.strip(),'INSTALLER_ARGS=--upgrade-idle-ceccb-v1')
    def test_exact_source_capture_idle_upgrade_argument_reaches_captured_installer(self):
        result=self.run_launcher('--install','--upgrade-idle-source-capture-v1')
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(result.stdout.strip(),'INSTALLER_ARGS=--upgrade-idle-source-capture-v1')
    def test_exact_search_bootstrap_idle_upgrade_argument_reaches_captured_installer(self):
        result=self.run_launcher('--install','--upgrade-idle-search-bootstrap-v1')
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(result.stdout.strip(),'INSTALLER_ARGS=--upgrade-idle-search-bootstrap-v1')
    def test_exact_expiry_reset_idle_upgrade_argument_reaches_captured_installer(self):
        result=self.run_launcher('--install','--upgrade-idle-expiry-reset-v1')
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(result.stdout.strip(),'INSTALLER_ARGS=--upgrade-idle-expiry-reset-v1')
    def test_exact_media_idle_upgrade_argument_reaches_captured_installer(self):
        result=self.run_launcher('--install','--upgrade-idle-media-v1')
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(result.stdout.strip(),'INSTALLER_ARGS=--upgrade-idle-media-v1')
    def test_exact_retirement_arguments_reach_root_only_captured_entry(self):
        result=self.run_launcher('--retire','/root/reviewed-plan.json','a'*64)
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(result.stdout.strip(),'RETIRE_ARGS=/root/reviewed-plan.json,'+'a'*64)
    def test_exact_continuation_arguments_reach_root_only_captured_entry(self):
        result=self.run_launcher('--continue-retirement','/root/reviewed-continuation.json','b'*64)
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(result.stdout.strip(),'RETIRE_ARGS=--continue,/root/reviewed-continuation.json,'+'b'*64)
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
