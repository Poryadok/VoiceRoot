"""Exercise CI fixture sequencing and owned-database cleanup on failure."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / 'scripts/ci/space_over100_proof.sh'


class Over100ProofTests(unittest.TestCase):
    def run_proof(self, mode):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            tools = root / 'tools'
            tools.mkdir()
            stubs = {
                'psql': r'''#!/usr/bin/env bash
printf 'psql %s\n' "$*" >> "$MOCK_LOG"
if [[ "$*" == *'COALESCE'* ]]; then
  if [[ "$MOCK_MODE" == oid_failure && -f "$MOCK_STATE" && ! -f "$MOCK_ONCE" ]]; then touch "$MOCK_ONCE"; exit 1; fi
  if [[ "$MOCK_MODE" == cleanup_inspect && -f "$MOCK_STATE" ]] && grep -q space_over100_consumer_required.json "$MOCK_LOG"; then exit 1; fi
  if [[ -f "$MOCK_STATE" ]]; then printf '123\n'; else printf '0\n'; fi
elif [[ "$*" == *'CREATE DATABASE'* ]]; then touch "$MOCK_STATE"
elif [[ "$*" == *'DROP DATABASE'* ]]; then
  if [[ "$MOCK_MODE" == cleanup_drop ]]; then exit 1; fi
  rm "$MOCK_STATE"
fi
''',
                'go': r'''#!/usr/bin/env bash
printf 'go %s\n' "$*" >> "$MOCK_LOG"
exit 0
''',
                'python3': r'''#!/usr/bin/env bash
printf 'receipt %s\n' "$*" >> "$MOCK_LOG"
if [[ "$MOCK_MODE" == fail && "$*" == *space_over100_consumer_required.json* ]]; then exit 1; fi
exit 0
''',
            }
            for name, body in stubs.items():
                path = tools / name
                path.write_text(body, encoding='utf-8', newline='\n')
                path.chmod(0o755)
            driver = root / 'native-migrate-driver'
            driver.write_text('#!/usr/bin/env bash\nprintf "migrate %s\\n" "$*" >> "$MOCK_LOG"\n', encoding='utf-8', newline='\n')
            driver.chmod(0o755)
            state, log = root / 'db-state', root / 'commands'
            env = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ['PATH'],
                       RUNNER_TEMP=root.as_posix(), GITHUB_RUN_ID='1', GITHUB_RUN_ATTEMPT='1',
                       CANDIDATE_SHA='a' * 40, SPACE_NATIVE_ADMIN_URL='test-admin',
                       SPACE_BAN_TEST_DATABASE_URL='test-database', MOCK_MODE=mode,
                       MOCK_LOG=log.as_posix(), MOCK_STATE=state.as_posix(), MOCK_ONCE=(root / 'once').as_posix())
            bash = 'C:/Program Files/Git/bin/bash.exe' if os.name == 'nt' else shutil.which('bash')
            result = subprocess.run([bash, str(SCRIPT)], cwd=ROOT / 'src/backend/space',
                                    env=env, capture_output=True, text=True, timeout=30)
            recovery = (root / 'space-over100-cleanup.txt').read_text(encoding='utf-8')
            return result.returncode, state.exists(), log.read_text(encoding='utf-8'), recovery

    def test_consumer_failure_is_not_masked_and_owned_database_is_removed(self):
        code, exists, log, recovery = self.run_proof('fail')
        self.assertNotEqual(code, 0)
        self.assertFalse(exists)
        self.assertIn('space_over100_consumer_required.json', log)
        self.assertIn('DROP DATABASE space_over100', log)
        self.assertIn('state=removed', recovery)
        self.assertEqual(log.count('go test '), 4)

    def test_success_requires_all_four_receipts_and_cleanup(self):
        code, exists, log, recovery = self.run_proof('success')
        self.assertEqual(code, 0, log)
        self.assertFalse(exists)
        self.assertEqual(log.count('receipt '), 4)
        self.assertEqual(log.count('--reject-skips'), 4)
        self.assertIn('DROP DATABASE space_over100', log)
        self.assertIn('state=removed', recovery)

    def test_lost_oid_preserves_uncertain_database_for_recovery(self):
        code, exists, log, recovery = self.run_proof('oid_failure')
        self.assertNotEqual(code, 0)
        self.assertTrue(exists)
        self.assertNotIn('DROP DATABASE', log)
        self.assertIn('state=uncertain', recovery)
        self.assertIn('reason=identity_unverified', recovery)

    def test_successful_body_does_not_mask_cleanup_failure(self):
        for mode, reason in (('cleanup_inspect', 'inspection_failed'), ('cleanup_drop', 'drop_failed')):
            with self.subTest(mode=mode):
                code, exists, log, recovery = self.run_proof(mode)
                self.assertNotEqual(code, 0)
                self.assertTrue(exists)
                self.assertEqual(log.count('receipt '), 4)
                self.assertIn('state=uncertain', recovery)
                self.assertIn('reason=' + reason, recovery)


if __name__ == '__main__':
    unittest.main()
