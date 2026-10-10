"""The retired mail-only dispatch must reject before any cluster access."""
from pathlib import Path
import re
import subprocess
import unittest


class MailWorkflowTests(unittest.TestCase):
    def test_actual_mail_only_job_refuses_without_credentials_or_mutation_steps(self):
        workflow=(Path(__file__).resolve().parents[3]/'.github/workflows/staging-deploy.yml').read_text()
        job=re.search(r'(?ms)^  mail-only:\n(.*?)(?=^  [a-z][a-z0-9-]*:\n|\Z)',workflow).group(1)
        self.assertIn('inputs.mail_only == true',job)
        self.assertNotIn('inputs.validate_app_secret_only',job)
        self.assertNotIn('inputs.nats_probe_only',job)
        self.assertIn('runs-on: ubuntu-latest',job)
        self.assertEqual(job.count('      - '),1)
        for forbidden in ('environment:', 'secrets.', 'uses:', 'env:', 'kubectl', 'mail-only-resend.sh', 'download_staging_source'):
            self.assertNotIn(forbidden,job)
        script=job.split('        run: |\n',1)[1]
        script='\n'.join(line[10:] for line in script.splitlines())
        result=subprocess.run(['bash','-c',script],capture_output=True,text=True,timeout=5)
        self.assertEqual(result.returncode,1)
        self.assertIn('mail_only is disabled',result.stderr)
        self.assertIn('deploy_mode=full or app-only',result.stderr)
        self.assertIn('mail_only=false',result.stderr)


if __name__=='__main__':unittest.main()
