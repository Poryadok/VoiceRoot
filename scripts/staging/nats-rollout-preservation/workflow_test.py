"""Exercise the actual workflow bootstrap, not a mocked module guard."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import unittest

class WorkflowCaptureTest(unittest.TestCase):
    def setUp(self):
        self.root=Path('/var/lib/voice-nats-preservation');self.root.mkdir(exist_ok=True,mode=0o700)
        self.base=self.root/'rollout-123456abcdef'
        if self.base.exists():shutil.rmtree(self.base)
        self.addCleanup(lambda:shutil.rmtree(self.base))
        self.code=self.base/'code';self.code.mkdir(parents=True,mode=0o750)
        entry=self.code/'nats-rollout-preservation/workflow_entry.py';entry.parent.mkdir(mode=0o750)
        entry.write_text("print('CAPTURED_ENTRY_EXECUTED')\n")
        self.dependency=self.code/'dependency.py';self.dependency.write_text('SAFE=True\n')
        hashes={str(p.relative_to(self.code)):hashlib.sha256(p.read_bytes()).hexdigest() for p in (entry,self.dependency)}
        (self.code/'capture-manifest.json').write_text(json.dumps(hashes))
        text=(Path(__file__).resolve().parents[3]/'.github/workflows/staging-deploy.yml').read_text()
        python=text.split("          python3 -I -S - <<'PY'\n",1)[1].split('          PY\n',1)[0]
        self.python='\n'.join(line[10:] for line in python.splitlines())
    def run_bootstrap(self):
        return subprocess.run([sys.executable,'-I','-S','-'],input=self.python,text=True,capture_output=True,
            timeout=10,env={'PATH':'/usr/bin:/bin','VOICE_NATS_ROLLOUT_OPERATION':'123456abcdef','ROLLOUT_ACTION':'--preflight'})
    def test_valid_captured_entry_runs_outside_target_checkout(self):
        result=self.run_bootstrap();self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(result.stdout.strip(),'CAPTURED_ENTRY_EXECUTED')
    def test_dependency_tamper_denies_before_entry_execution(self):
        self.dependency.write_text('CHANGED=True\n');result=self.run_bootstrap()
        self.assertNotEqual(result.returncode,0);self.assertNotIn('CAPTURED_ENTRY_EXECUTED',result.stdout)
    def test_group_writable_code_denies_before_entry_execution(self):
        self.dependency.chmod(0o664);result=self.run_bootstrap()
        self.assertNotEqual(result.returncode,0);self.assertNotIn('CAPTURED_ENTRY_EXECUTED',result.stdout)
    def test_symlink_dependency_denies_before_entry_execution(self):
        self.dependency.unlink();self.dependency.symlink_to('/etc/hostname');result=self.run_bootstrap()
        self.assertNotEqual(result.returncode,0);self.assertNotIn('CAPTURED_ENTRY_EXECUTED',result.stdout)

    def test_actual_prepare_bootstrap_selects_only_trusted_frontend_only_mode(self):
        import tempfile
        installed=self.root/'installed'
        self.assertFalse(installed.exists())
        installed.mkdir(mode=0o750);self.addCleanup(lambda:shutil.rmtree(installed))
        code=installed/'code';entry=code/'nats-rollout-preservation/bridge_client.py'
        entry.parent.mkdir(parents=True,mode=0o750)
        entry.write_text('import json,sys\nprint(json.dumps(sys.argv[1:]))\n')
        (code/'capture-manifest.json').write_text(json.dumps({'nats-rollout-preservation/bridge_client.py':hashlib.sha256(entry.read_bytes()).hexdigest()}))
        for changed,full,wanted in [('web,admin','false','images-only'),('gateway','false','app-only'),('web','true','app-only'),('','false','app-only')]:
            with self.subTest(changed=changed,full=full):
                result=subprocess.run([sys.executable,'-I','-S','-'],input=self.python,text=True,capture_output=True,timeout=10,
                    env={'PATH':'/usr/bin:/bin','ROLLOUT_ACTION':'--bridge-prepare','DEPLOY_MODE':'app-only','CHANGED_SERVICES':changed,
                        'ROLLOUT_NEEDS_FULL_ROLLOUT':full,'VOICE_IMAGE_TAG':'a'*40,'GITHUB_RUN_ID':'123'})
                self.assertEqual(result.returncode,0,result.stderr)
                self.assertEqual(json.loads(result.stdout),['prepare',wanted,changed,'a'*40,'123'])

if __name__=='__main__':unittest.main()
