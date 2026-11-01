"""Exercise the actual seeded-job cleanup as a non-root sudo-capable user."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import textwrap
import unittest


ROOT=Path(__file__).resolve().parents[2]


@unittest.skipUnless(hasattr(os,'geteuid') and os.geteuid()!=0 and shutil.which('sudo'),
                     'requires non-root user with passwordless sudo')
class SeededWorkflowCleanupTests(unittest.TestCase):
    def test_actual_owned_job_runs_tests_without_user_traversal(self):
        workflow=(ROOT/'.github/workflows/nats-owned-enrollment-proof.yml').read_text()
        job=workflow.split('  owned-enrollment:',1)[1]
        start=job.index('          sudo -n --preserve-env=RUNNER_ENVIRONMENT sh ',job.index('migrate_image=$('))
        end=job.index('\n          test "$(git rev-parse HEAD)"',start)
        command=textwrap.dedent(job[start:end])
        sudo=lambda *args:subprocess.run(['sudo','-n',*args],check=False,capture_output=True,text=True)
        made=sudo('mktemp','-d','/root/voice-owned-unittest-test.XXXXXXXX')
        self.assertEqual(made.returncode,0);path=made.stdout.strip()
        directory=path+'/source/scripts/staging/nats-rollout-preservation'
        module=directory+'/staging_cohort_preservation_restore_test.py'
        try:
            self.assertEqual(sudo('mkdir','-p',directory).returncode,0)
            expected={'RUNNER_ENVIRONMENT':'github-hosted','VOICE_COHORT_DOCKER':'1',
                'VOICE_QUALIFIED_BINARIES':path+'/qualified','VOICE_COHORT_RETAINED_ROOT':path+'/runtime',
                'TMPDIR':path+'/runtime','PYTHONDONTWRITEBYTECODE':'1',
                'PYTHONPATH':directory+':'+path+'/source/scripts/staging/nats-known-baseline:'+path+'/source/scripts/dev'}
            fixture='import os,unittest\nassert __file__ == '+repr(module)+'\n'
            fixture+='assert all(os.environ.get(k)==v for k,v in '+repr(expected)+'.items())\n'
            fixture+='class RestoreTransportTests(unittest.TestCase):\n'
            for name in ('test_public_prepare_to_qualified_continuation_with_external_boundaries',
                         'test_public_qualified_continuation_reaches_c1_final_prepare',
                         'test_public_c1_continuation_issues_typed_final_admission'):
                fixture+=' def '+name+'(self): pass\n'
            result=subprocess.run(['sudo','-n','tee',module],input=fixture,capture_output=True,text=True)
            self.assertEqual(result.returncode,0)
            script='work='+shlex_quote(path)+'\nexport RUNNER_ENVIRONMENT=github-hosted\n'+command+'\n'
            with tempfile.TemporaryDirectory() as shadow:
                (Path(shadow)/'staging_cohort_preservation_restore_test.py').write_text('raise RuntimeError("shadow imported")\n')
                result=subprocess.run(['bash','-c',script],capture_output=True,text=True,cwd=shadow)
            self.assertEqual(result.returncode,0,result.stderr)
            self.assertIn('Ran 3 tests',result.stderr)
        finally:
            self.assertEqual(sudo('rm','-f','--',module).returncode,0)
            for item in (directory,path+'/source/scripts/staging',path+'/source/scripts',path+'/source',path):
                self.assertEqual(sudo('rmdir','--',item).returncode,0)

    def test_actual_owned_job_reads_root_owned_image_catalog(self):
        workflow=(ROOT/'.github/workflows/nats-owned-enrollment-proof.yml').read_text()
        job=workflow.split('  owned-enrollment:',1)[1]
        lines=[line.strip() for line in job.splitlines()
               if line.strip().startswith(('pg_image=$(','migrate_image=$('))]
        self.assertEqual(len(lines),2)
        sudo=lambda *args:subprocess.run(['sudo','-n',*args],check=False,capture_output=True,text=True)
        made=sudo('mktemp','-d','/root/voice-owned-import-test.XXXXXXXX')
        self.assertEqual(made.returncode,0);path=made.stdout.strip()
        dev=path+'/source/scripts/dev';module=dev+'/auth_user_adoption_runtime.py'
        try:
            self.assertEqual(sudo('mkdir','-p',dev).returncode,0)
            contents='assert __file__ == '+repr(module)+'\nQUALIFIED_TOOLS={"postgres":"pg-fixture","migrate":"migrate-fixture"}\n'
            result=subprocess.run(['sudo','-n','tee',module],input=contents,capture_output=True,text=True)
            self.assertEqual(result.returncode,0)
            script='set -e\nwork='+shlex_quote(path)+'\n'+'\n'.join(lines)+'\nprintf "%s\\n%s\\n" "$pg_image" "$migrate_image"\n'
            with tempfile.TemporaryDirectory() as shadow:
                (Path(shadow)/'auth_user_adoption_runtime.py').write_text('raise RuntimeError("shadow imported")\n')
                result=subprocess.run(['bash','-c',script],capture_output=True,text=True,cwd=shadow)
            self.assertEqual(result.returncode,0,result.stderr)
            self.assertEqual(result.stdout.splitlines(),['pg-fixture','migrate-fixture'])
        finally:
            self.assertEqual(sudo('rm','-f','--',module).returncode,0)
            for directory in (dev,path+'/source/scripts',path+'/source',path):
                self.assertEqual(sudo('rmdir','--',directory).returncode,0)

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
                    if sudo('test','-e',path).returncode==0:
                        self.assertEqual(sudo('rmdir','--',path).returncode,0)


def shlex_quote(value):
    import shlex
    return shlex.quote(value)


if __name__=='__main__':unittest.main()
