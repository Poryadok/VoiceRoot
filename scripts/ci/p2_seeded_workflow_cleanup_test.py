"""Exercise the actual seeded-job cleanup as a non-root sudo-capable user."""
import os
from pathlib import Path
import shutil
import subprocess
import textwrap
import unittest


ROOT=Path(__file__).resolve().parents[2]


@unittest.skipUnless(hasattr(os,'geteuid') and os.geteuid()!=0 and shutil.which('sudo'),
                     'requires non-root user with passwordless sudo')
class SeededWorkflowCleanupTests(unittest.TestCase):
    def test_actual_cleanup_success_retention_and_validation_failure(self):
        workflow=(ROOT/'.github/workflows/nats-owned-enrollment-proof.yml').read_text()
        job=workflow.split('  seeded-enrollment:',1)[1].split('  owned-enrollment:',1)[0]
        body=job.split('          cleanup() {',1)[1].split('          trap cleanup EXIT',1)[0]
        cleanup='cleanup() {\n'+textwrap.dedent(body)
        sudo=lambda *args:subprocess.run(['sudo','-n',*args],check=False,capture_output=True,text=True)
        for mode in ('success','failure','invalid'):
            with self.subTest(mode=mode):
                made=sudo('mktemp','-d','/root/voice-seeded-test.XXXXXXXX')
                self.assertEqual(made.returncode,0);path=made.stdout.strip()
                try:
                    if mode=='invalid':self.assertEqual(sudo('rmdir','--',path).returncode,0)
                    script='work='+shlex_quote(path)+'\n'+cleanup+'\ntrap cleanup EXIT\nexit '+('17' if mode=='failure' else '0')+'\n'
                    result=subprocess.run(['bash','-c',script],capture_output=True,text=True)
                    self.assertEqual(result.returncode,17 if mode=='failure' else 1 if mode=='invalid' else 0,result.stderr)
                    present=sudo('test','-d',path).returncode==0
                    self.assertEqual(present,mode=='failure')
                finally:
                    sudo('rmdir','--',path)


def shlex_quote(value):
    import shlex
    return shlex.quote(value)


if __name__=='__main__':unittest.main()
