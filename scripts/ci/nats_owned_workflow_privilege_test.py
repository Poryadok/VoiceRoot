"""Exercise workflow commands as an unprivileged runner against a root-only tree.

Run as root on a disposable Linux host, via sudo from a passwordless runner.
No Docker daemon, production paths, or credentials are used.
"""
import os
import grp
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT=Path(__file__).resolve().parents[2]
WORKFLOW=ROOT/'.github/workflows/nats-owned-enrollment-proof.yml'


class Tests(unittest.TestCase):
    def test_actual_group_provision_is_idempotent_for_unprivileged_runner(self):
        self.assertEqual(os.geteuid(),0)
        runner=os.environ.get('SUDO_USER','voice-runner')
        owned=WORKFLOW.read_text(encoding='utf8').split('  owned-enrollment:',1)[1]
        lines=[line.strip() for line in owned.splitlines()]
        commands=[line for line in lines if line=='sudo -n groupadd --force --system pmd']
        self.assertEqual(len(commands),1)
        test_line=next(i for i,line in enumerate(lines) if 'test_public_prepare_to_qualified_continuation_with_external_boundaries' in line)
        self.assertLess(lines.index(commands[0]),test_line)
        identities=[]
        for _ in range(2):
            result=subprocess.run(['runuser','-u',runner,'--','bash','-eu','-c',commands[0]],
                                  stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=30)
            self.assertEqual(result.returncode,0,result.stderr.decode('utf8',errors='replace')[-1000:])
            row=grp.getgrnam('pmd');identities.append((row.gr_gid,tuple(row.gr_mem)))
        self.assertEqual(identities[0],identities[1])

    def test_actual_workflow_commands_cross_root_boundary_and_check_cleanup(self):
        self.assertEqual(os.geteuid(),0)
        runner=os.environ.get('SUDO_USER','voice-runner')
        owned=WORKFLOW.read_text(encoding='utf8').split('  owned-enrollment:',1)[1]
        lines=owned.splitlines()
        commands=[line.strip() for line in lines if line.strip().startswith("sudo -n sh -eu -c 'cd ")]
        self.assertEqual(len(commands),2)
        checks=[line.strip() for line in lines if line.strip().startswith('if sudo -n test -d "$work"')]
        self.assertEqual(len(checks),1)
        with tempfile.TemporaryDirectory(prefix='voice-owned-workflow-test-',dir='/root') as tmp:
            work=Path(tmp);source=work/'source'
            for relative in ('scripts/ci','scripts/staging/nats-known-baseline',
                             'scripts/staging/nats-rollout-preservation','deploy/nats'):
                shutil.copytree(ROOT/relative,source/relative)
            for path in [source,*source.rglob('*')]:
                if not path.is_symlink():path.chmod(path.stat().st_mode & ~0o222)
            def run(command):
                return subprocess.run(['runuser','-u',runner,'--','env','work='+str(work),
                                       'bash','-eu','-c',command],stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=60)
            for command in commands:
                result=run(command)
                self.assertEqual(result.returncode,0, result.stderr.decode('utf8',errors='replace')[-2000:])
            result=run(checks[0]+' :; else exit 1; fi')
            self.assertEqual(result.returncode,0, result.stderr.decode('utf8',errors='replace')[-1000:])

if __name__=='__main__':unittest.main()
