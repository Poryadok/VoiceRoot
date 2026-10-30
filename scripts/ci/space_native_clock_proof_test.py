"""Fail closed when a clock fixture name or cleanup inventory is uncertain."""
import os
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / 'scripts/ci/space_native_clock_proof.sh'
ID = 'a' * 64


class ClockFixtureCleanupTests(unittest.TestCase):
    def test_account_ban_proofs_use_fresh_ordered_database_and_receipts(self):
        text = SCRIPT.read_text(encoding='utf-8')
        start = text.index("DROP DATABASE space_db'")
        created = text.index("CREATE DATABASE space_ban_db'", start)
        transition45 = text.index("receipt account-ban-transition45", created)
        transition50 = text.index("receipt account-ban-transition50", transition45)
        ordered = [
            ('account-ban-native', 'space_account_ban_native50_required.json', 'TestAccountBanNative50'),
            ('account-ban-ledger', 'space_account_ban_ledger50_required.json', 'TestAccountBanLedgerGuards50'),
            ('account-ban-expiry', 'space_account_ban_expiry_required.json', 'TestAccountBanFixtureClockExpiry'),
        ]
        positions = []
        for label, inventory, case in ordered:
            positions.append(text.index("receipt " + label + " " + inventory + " '^" + case + "$'", created))
            self.assertEqual(json.loads((ROOT / 'scripts/ci' / inventory).read_text(encoding='utf-8')),
                             {'space': [['voice/backend/space/internal/store', case]]})
        self.assertEqual(positions, sorted(positions))
        self.assertLess(transition50, positions[0])
        self.assertGreater(text.index("DROP DATABASE space_ban_db'"), positions[-1])
        self.assertIn('--reject-skips', text)

    def run_case(self, mode):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            tools = root / 'tools'
            tools.mkdir()
            (tools / 'docker').write_text('''#!/usr/bin/env bash
printf '%s\\n' "$*" >> "$SIM_LOG"
if [[ "$1 $2" == 'container ls' ]]; then
  if [[ "$SIM_MODE" == foreign ]]; then printf '%s\\n' "$SIM_ID"; exit 0; fi
  if [[ -f "$SIM_STATE" ]]; then
    [[ "$SIM_MODE" != uncertain ]] || exit 1
    name="$(cat "$SIM_STATE")"
    if [[ "$*" == *"name=^/${name}$"* || "$*" == *"id=$SIM_ID"* ]]; then printf '%s\\n' "$SIM_ID"; fi
  fi
  exit 0
fi
if [[ "$1 $2" == 'container inspect' ]]; then printf '%s\\n' '123-1'; exit 0; fi
if [[ "$1 $2" == 'container rm' ]]; then rm -f "$SIM_STATE"; exit 0; fi
if [[ "$1" == create ]]; then
  shift
  while [[ "$1" != --name ]]; do shift; done
  printf '%s' "$2" > "$SIM_STATE"
  exit 1
fi
exit 2
''', encoding='utf-8')
            for executable in tools.iterdir():
                executable.chmod(0o755)
            state, log = root / 'state', root / 'docker.log'
            env = dict(os.environ, RUNNER_TEMP=root.as_posix(), GITHUB_RUN_ID='123',
                       GITHUB_RUN_ATTEMPT='1', CANDIDATE_SHA='b' * 40,
                       SIM_MODE=mode, SIM_ID=ID, SIM_STATE=state.as_posix(), SIM_LOG=log.as_posix(),
                       PATH=str(tools) + os.pathsep + os.environ['PATH'])
            bash = 'C:/Program Files/Git/bin/bash.exe' if os.name == 'nt' else shutil.which('bash')
            result = subprocess.run([bash, str(SCRIPT)], cwd=ROOT, env=env,
                                    capture_output=True, text=True, timeout=30)
            return result.returncode, state.exists(), log.read_text(encoding='utf-8')

    def test_existing_foreign_name_is_not_removed(self):
        code, _, log = self.run_case('foreign')
        self.assertNotEqual(code, 0)
        self.assertNotIn('container rm', log)

    def test_failed_create_cleans_only_owned_exact_id(self):
        code, exists, log = self.run_case('owned')
        self.assertNotEqual(code, 0)
        self.assertFalse(exists)
        self.assertIn('container rm -f ' + ID, log)

    def test_uncertain_cleanup_retains_resource_and_fails(self):
        code, exists, log = self.run_case('uncertain')
        self.assertNotEqual(code, 0)
        self.assertTrue(exists)
        self.assertNotIn('container rm', log)


if __name__ == '__main__':
    unittest.main()
