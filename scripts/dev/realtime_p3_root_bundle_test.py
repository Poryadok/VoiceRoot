import hashlib
import io
import json
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest

import realtime_p3_root_bundle as bundle


class BundleTests(unittest.TestCase):
    def test_exact_allowlist_manifest_and_pinned_launcher(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source, output = root / 'source', root / 'output'
            source.mkdir()
            for name in bundle.FILES:
                (source / name).write_bytes(('fixture ' + name).encode())
            digest = bundle.build(source, output)
            raw = (output / 'p3-source.tar').read_bytes()
            self.assertEqual(digest, hashlib.sha256(raw).hexdigest())
            with tarfile.open(fileobj=io.BytesIO(raw), mode='r:') as archive:
                self.assertEqual(set(archive.getnames()), set(bundle.FILES) | {'capture-manifest.json'})
                manifest = json.loads(archive.extractfile('capture-manifest.json').read())
                for name in bundle.FILES:
                    self.assertEqual(manifest[name], hashlib.sha256((source / name).read_bytes()).hexdigest())
            launcher = (output / 'p3-root-launch.sh').read_text()
            self.assertIn(digest, launcher)
            self.assertIn(str(len(raw)), launcher)
            self.assertNotIn('__BUNDLE_', launcher)

    def test_missing_source_never_produces_bundle(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            with self.assertRaises(Exception):
                bundle.build(root, root / 'output')
            self.assertFalse((root / 'output' / 'p3-source.tar').exists())

    def test_git_capture_uses_blob_not_changed_checkout(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary); dev = root / 'scripts' / 'dev'; dev.mkdir(parents=True)
            subprocess.run(['git', 'init', '-q', str(root)], check=True)
            subprocess.run(['git', '-C', str(root), 'config', 'user.name', 'Fixture'], check=True)
            subprocess.run(['git', '-C', str(root), 'config', 'user.email', 'fixture@example.invalid'], check=True)
            for name in bundle.FILES:
                (dev / name).write_bytes(b'canonical\n')
            (dev / 'realtime_p3_root_launch.template.sh').write_text('__BUNDLE_SHA256__ __BUNDLE_BYTES__\n')
            subprocess.run(['git', '-C', str(root), 'add', 'scripts/dev'], check=True)
            subprocess.run(['git', '-C', str(root), 'commit', '-qm', 'fixture'], check=True)
            sha = subprocess.check_output(['git', '-C', str(root), 'rev-parse', 'HEAD']).decode().strip()
            (dev / bundle.FILES[0]).write_bytes(b'canonical\r\n')
            output = root / 'output'; bundle.build_git(root, sha, output)
            with tarfile.open(output / 'p3-source.tar', mode='r:') as archive:
                self.assertEqual(archive.extractfile(bundle.FILES[0]).read(), b'canonical\n')

    def test_launcher_executes_only_exact_bundle_and_rejects_tamper(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source, output, capture = root / 'source', root / 'output', root / 'capture'
            source.mkdir(); capture.mkdir(mode=0o700)
            for name in bundle.FILES:
                (source / name).write_text("print('fixture-executed')\n" if name in ('realtime_p3_missing_issuer.py', 'realtime_p3_root_export.py') else '')
            # Nonempty files are required by the production launcher.
            for name in ('credentials.py', 'reset_credentials.py'):
                (source / name).write_text('# fixture\n')
            bundle.build(source, output)
            archive = output / 'p3-source.tar'
            launcher = (output / 'p3-root-launch.sh').read_text()
            launcher = launcher.replace("pathlib.Path('/home/pmd/voice-realtime-p3/p3-source.tar')", 'pathlib.Path(' + repr(str(archive)) + ')')
            launcher = launcher.replace("pathlib.Path('/home/voice-dev/issuance')", 'pathlib.Path(' + repr(str(capture)) + ')')
            launcher = launcher.replace('(*ROOT.parents[::-1], ROOT)', '(ROOT,)').replace("pwd.getpwnam('pmd').pw_uid", 'os.geteuid()')
            run = lambda: subprocess.run(['bash'], input=launcher.encode(), capture_output=True)
            good = run()
            self.assertEqual(good.returncode, 0, good.stderr.decode())
            self.assertIn(b'fixture-executed', good.stdout)
            data = bytearray(archive.read_bytes()); data[0] ^= 1; archive.write_bytes(data)
            bad = run()
            self.assertNotEqual(bad.returncode, 0)
            self.assertNotIn(b'fixture-executed', bad.stdout)
            self.assertIn(b'P3_SOURCE_CAPTURE=BLOCKED', bad.stderr)


if __name__ == '__main__': unittest.main()
