"""Exercise the isolated historical Space v34 proof producer without Docker."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / 'scripts/ci/space_large34_proof.sh'
SOURCE_GUARD = ROOT / 'scripts/ci/space_large34_source.py'
MATRIX = ROOT / 'scripts/ci/go_integration_matrix.py'
TEST_NAME = 'TestAccountBanFixtureLargeCohort34'
PACKAGE = 'voice/backend/space/internal/store'
MANIFEST_NAME = 'space-large34-proof.json'


def matrix_module():
    sys.path.insert(0, str(MATRIX.parent))
    spec = importlib.util.spec_from_file_location('space_large34_go_matrix', MATRIX)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class Large34ProofTests(unittest.TestCase):
    def run_proof(self, mode='success'):
        self.assertTrue(SCRIPT.is_file(), 'the source-bound v34 producer must exist')
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            tools = root / 'tools'
            tools.mkdir()
            state = root / 'database-oid'
            log = root / 'commands.log'
            seed = root / 'seed.sql'
            inventory = root / 'inventory.json'
            inventory.write_text(json.dumps({'space': [[PACKAGE, TEST_NAME]]}), encoding='utf-8')
            matrix_script = MATRIX.as_posix()
            real_python = sys.executable.replace('\\', '/')

            stubs = {
                'psql': r'''#!/usr/bin/env bash
set -euo pipefail
sql="$*"
if [[ "$sql" == *'COALESCE'* ]]; then
  if [[ -f "$MOCK_STATE" ]]; then
    printf 'inspect-present\n' >> "$MOCK_LOG"
  elif [[ -f "$MOCK_DROPPED" ]]; then
    printf 'inspect-absent manifest=%s\n' "$([[ -f "$MOCK_FINAL_RECEIPT" ]] && printf yes || printf no)" >> "$MOCK_LOG"
  else
    printf 'inspect-absent\n' >> "$MOCK_LOG"
  fi
  if [[ "$MOCK_MODE" == cleanup_inspect_fail && -f "$MOCK_TEST_DONE" ]]; then exit 71; fi
  if [[ "$MOCK_MODE" == oid_drift && -f "$MOCK_TEST_DONE" ]]; then printf '999\n'; exit 0; fi
  if [[ "$MOCK_MODE" == post_drop_inspect_fail && -f "$MOCK_DROPPED" ]]; then exit 72; fi
  if [[ -f "$MOCK_STATE" ]]; then cat "$MOCK_STATE"; else printf '0\n'; fi
elif [[ "$sql" == *'pg_control_system'* ]]; then
  if [[ "$*" == *'private-test-secret'* ]]; then
    printf 'identity-test\n' >> "$MOCK_LOG"
    case "$MOCK_MODE" in
      test_cluster_mismatch) printf '99999|127.0.0.1|5432\n' ;;
      test_server_mismatch) printf '12345|127.0.0.2|5432\n' ;;
      *) printf '12345|127.0.0.1|5432\n' ;;
    esac
  else
    printf 'identity-admin\n' >> "$MOCK_LOG"
    printf '12345|127.0.0.1|5432\n'
  fi
elif [[ "$sql" == *'current_database'* ]]; then
  printf 'identity-database\n' >> "$MOCK_LOG"
  case "$MOCK_MODE" in
    url_database_mismatch|test_database_mismatch) printf 'space_other|123\n' ;;
    test_oid_mismatch) printf 'space_large34|999\n' ;;
    *) printf 'space_large34|123\n' ;;
  esac
elif [[ "$sql" == *'schema_migrations'* ]]; then
  printf 'version-34-check\n' >> "$MOCK_LOG"
  if [[ "$MOCK_MODE" == wrong_version ]]; then printf '35\n'
  elif [[ "$MOCK_MODE" == dirty_version ]]; then printf 'invalid\n'
  else printf '34\n'; fi
elif [[ "$sql" == *'CREATE DATABASE'* ]]; then
  printf '123\n' > "$MOCK_STATE"
  printf 'create\n' >> "$MOCK_LOG"
  if [[ "$MOCK_MODE" == partial_create ]]; then exit 73; fi
elif [[ "$sql" == *'DROP DATABASE'* ]]; then
  printf 'drop\n' >> "$MOCK_LOG"
  if [[ "$MOCK_MODE" == drop_fail ]]; then exit 74; fi
  rm -f "$MOCK_STATE"
  touch "$MOCK_DROPPED"
else
  cat > "$MOCK_SEED"
  printf 'seed\n' >> "$MOCK_LOG"
fi
''',
                'go': r'''#!/usr/bin/env bash
set -euo pipefail
printf 'go %s\n' "$*" >> "$MOCK_LOG"
touch "$MOCK_TEST_DONE"
case "$MOCK_MODE" in
  skip) printf '%s\n' '{"Action":"start","Package":"voice/backend/space/internal/store"}' '{"Action":"run","Package":"voice/backend/space/internal/store","Test":"TestAccountBanFixtureLargeCohort34"}' '{"Action":"skip","Package":"voice/backend/space/internal/store","Test":"TestAccountBanFixtureLargeCohort34"}' '{"Action":"pass","Package":"voice/backend/space/internal/store"}' ;;
  fail) printf '%s\n' '{"Action":"start","Package":"voice/backend/space/internal/store"}' '{"Action":"run","Package":"voice/backend/space/internal/store","Test":"TestAccountBanFixtureLargeCohort34"}' '{"Action":"fail","Package":"voice/backend/space/internal/store","Test":"TestAccountBanFixtureLargeCohort34"}' '{"Action":"fail","Package":"voice/backend/space/internal/store"}'; exit 1 ;;
  missing) printf '%s\n' '{"Action":"start","Package":"voice/backend/space/internal/store"}' '{"Action":"pass","Package":"voice/backend/space/internal/store"}' ;;
  malformed) printf '%s\n' '{not-json}' ;;
  truncated) printf '%s' '{"Action":"start","Package":"voice/backend/space/internal/store"}' ;;
  *) printf '%s\n' '{"Action":"start","Package":"voice/backend/space/internal/store"}' '{"Action":"run","Package":"voice/backend/space/internal/store","Test":"TestAccountBanFixtureLargeCohort34"}' '{"Action":"pass","Package":"voice/backend/space/internal/store","Test":"TestAccountBanFixtureLargeCohort34"}' '{"Action":"pass","Package":"voice/backend/space/internal/store"}' ;;
esac
''',
                'python3': r'''#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == *space_large34_source.py ]]; then
  count=$(cat "$MOCK_GUARD_COUNT" 2>/dev/null || printf '0')
  count=$((count + 1)); printf '%s' "$count" > "$MOCK_GUARD_COUNT"
  printf 'guard-%s manifest=%s\n' "$count" "$([[ -f "$MOCK_FINAL_RECEIPT" ]] && printf yes || printf no)" >> "$MOCK_LOG"
  if [[ "$MOCK_MODE" == source_before_fail && "$count" == 1 ]]; then exit 81; fi
  if [[ "$MOCK_MODE" == source_after_fail && "$count" == 2 ]]; then exit 82; fi
  exit 0
fi
if [[ "${1:-}" == *go_integration_matrix.py ]]; then
  shift
  operation="${1:-}"
  args=()
  output_file=""
  while (($#)); do
    if [[ "$1" == --inventory ]]; then args+=(--inventory "$MOCK_INVENTORY"); shift 2
    elif [[ "$1" == --output ]]; then output_file="$2"; args+=("$1" "$2"); shift 2
    else args+=("$1"); shift; fi
  done
  reject_skips=no
  if [[ " ${args[*]} " == *' --reject-skips '* ]]; then reject_skips=yes; fi
  printf 'matrix-%s reject-skips=%s manifest=%s\n' "$operation" "$reject_skips" \
    "$( [[ -f "$MOCK_FINAL_RECEIPT" ]] && printf yes || printf no )" >> "$MOCK_LOG"
  set +e
  "$REAL_PYTHON" "$REAL_MATRIX" "${args[@]}"
  status=$?
  set -e
  if [[ "$status" == 0 && "$operation" == produce && -n "$MOCK_RECEIPT_BAD_FIELD" ]]; then
    "$REAL_PYTHON" -c 'import json,sys; from pathlib import Path; p=Path(sys.argv[1]); v=json.loads(p.read_text(encoding="utf-8")); v["identity"][sys.argv[2]]="0" if sys.argv[2]=="candidate" else "999"; p.write_text(json.dumps(v), encoding="utf-8")' "$output_file" "$MOCK_RECEIPT_BAD_FIELD"
  fi
  exit "$status"
fi
if [[ "${1:-}" == *space_large34* ]]; then
  printf 'proof-helper %s\n' "$*" >> "$MOCK_LOG"
  exec "$REAL_PYTHON" "$@"
fi
printf 'unexpected-python %s\n' "$*" >> "$MOCK_LOG"
exit 83
''',
            }
            for name, body in stubs.items():
                path = tools / name
                path.write_text(body, encoding='utf-8', newline='\n')
                path.chmod(0o755)
            driver = root / 'native-migrate-driver'
            driver.write_text(
                '#!/usr/bin/env bash\nprintf "migrate goto-34\\n" >> "$MOCK_LOG"\n'
                'if [[ "$MOCK_MODE" == migrate_fail ]]; then exit 84; fi\n',
                encoding='utf-8', newline='\n')
            driver.chmod(0o755)
            guard_count = root / 'guard-count'
            final_receipt = root / MANIFEST_NAME
            recovery = root / 'space-large34-cleanup.txt'
            run_id = '123456789'
            attempt = '2'
            candidate = subprocess.check_output(
                ['git', '-C', str(ROOT), 'rev-parse', 'HEAD'], text=True).strip()
            test_database = 'space_other' if mode == 'url_database_mismatch' else 'space_large34'
            test_url = 'postgresql://voice:private-test-secret@pg:5432/' + test_database
            admin_url = 'postgresql://voice:private-admin-secret@pg:5432/postgres'
            bad_field = {'receipt_candidate_mismatch': 'candidate',
                         'receipt_run_mismatch': 'run',
                         'receipt_attempt_mismatch': 'attempt'}.get(mode, '')
            env = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ['PATH'],
                       RUNNER_TEMP=root.as_posix(), GITHUB_RUN_ID=run_id,
                       GITHUB_RUN_ATTEMPT=attempt, CANDIDATE_SHA=candidate,
                       SPACE_NATIVE_ADMIN_URL=admin_url,
                       SPACE_BAN_TEST_DATABASE_URL=test_url,
                       MOCK_MODE=mode, MOCK_LOG=log.as_posix(), MOCK_STATE=state.as_posix(),
                       MOCK_SEED=seed.as_posix(), MOCK_DROPPED=(root / 'dropped').as_posix(),
                       MOCK_TEST_DONE=(root / 'test-done').as_posix(),
                       MOCK_GUARD_COUNT=guard_count.as_posix(),
                       MOCK_FINAL_RECEIPT=final_receipt.as_posix(),
                       MOCK_RECOVERY=recovery.as_posix(),
                       MOCK_RECEIPT_BAD_FIELD=bad_field,
                       MOCK_INVENTORY=inventory.as_posix(),
                       REAL_PYTHON=real_python, REAL_MATRIX=matrix_script)
            if mode == 'preexisting':
                state.write_text('999\n', encoding='ascii')
            bash = 'C:/Program Files/Git/bin/bash.exe' if os.name == 'nt' else shutil.which('bash')
            result = subprocess.run([bash, str(SCRIPT)], cwd=ROOT / 'src/backend/space',
                                    env=env, capture_output=True, text=True, timeout=30)
            event_files = list(root.glob('*events.jsonl'))
            receipt_files = [path for path in root.glob('*receipt.json')
                             if path.name != MANIFEST_NAME]
            return {
                'result': result,
                'exists': state.exists(),
                'oid': state.read_text(encoding='ascii').strip() if state.exists() else None,
                'log': log.read_text(encoding='utf-8') if log.exists() else '',
                'seed': seed.read_text(encoding='utf-8') if seed.exists() else '',
                'manifest': final_receipt.read_bytes() if final_receipt.exists() else None,
                'recovery': recovery.read_text(encoding='utf-8') if recovery.exists() else None,
                'events': event_files[0].read_bytes() if event_files else None,
                'receipt': receipt_files[0].read_bytes() if receipt_files else None,
                'candidate': candidate,
                'dropped': (root / 'dropped').exists(),
            }

    def assert_no_credentials(self, result):
        output = result['result'].stdout + result['result'].stderr
        self.assertNotIn('private-test-secret', output)
        self.assertNotIn('private-admin-secret', output)
        self.assertNotIn('private-test-secret', result['log'])
        self.assertNotIn('private-admin-secret', result['log'])
        if result['recovery']:
            self.assertNotIn('private-test-secret', result['recovery'])
            self.assertNotIn('private-admin-secret', result['recovery'])
        if result['events']:
            self.assertNotIn(b'private-test-secret', result['events'])
            self.assertNotIn(b'private-admin-secret', result['events'])
        if result['receipt']:
            self.assertNotIn(b'private-test-secret', result['receipt'])
            self.assertNotIn(b'private-admin-secret', result['receipt'])
        if result['manifest']:
            self.assertNotIn(b'private-test-secret', result['manifest'])
            self.assertNotIn(b'private-admin-secret', result['manifest'])

    def test_success_proves_only_the_historical_301_row_fixture_and_cleans_before_manifest(self):
        run = self.run_proof()
        self.assertEqual(run['result'].returncode, 0, run['result'].stderr)
        self.assertFalse(run['exists'])
        self.assertTrue(run['manifest'])
        manifest = json.loads(run['manifest'])
        self.assertEqual(set(manifest), {'schema', 'candidate', 'run', 'attempt', 'test',
                                         'event_sha256', 'receipt_sha256', 'database',
                                         'owned_oid', 'cleanup'})
        self.assertEqual(manifest['schema'], 'voice-space-large34-proof-v1')
        self.assertEqual(manifest['candidate'], run['candidate'])
        self.assertEqual(manifest['run'], '123456789')
        self.assertEqual(manifest['attempt'], '2')
        self.assertEqual(manifest['test'], TEST_NAME)
        self.assertRegex(manifest['candidate'], r'^[0-9a-f]{40}$')
        self.assertRegex(manifest['event_sha256'], r'^[0-9a-f]{64}$')
        self.assertRegex(manifest['receipt_sha256'], r'^[0-9a-f]{64}$')
        self.assertEqual(manifest['event_sha256'], hashlib.sha256(run['events']).hexdigest())
        self.assertEqual(manifest['receipt_sha256'], hashlib.sha256(run['receipt']).hexdigest())
        self.assertEqual((manifest['database'], manifest['owned_oid'], manifest['cleanup']),
                         ('space_large34', 123, 'removed'))
        log = run['log']
        self.assertEqual(log.count('guard-'), 2, log)
        for before, after in (('guard-1', 'create'), ('create', 'migrate'),
                              ('migrate', 'version-34-check'), ('version-34-check', 'seed'),
                               ('seed', 'go test '),
                               ('go test ', 'matrix-produce '),
                               ('drop', 'inspect-absent manifest=no'),
                               ('inspect-absent manifest=no', 'guard-2')):
            self.assertLess(log.index(before), log.index(after), log)
        self.assertIn('-tags=spacebanintegration', log)
        self.assertIn('-run ^TestAccountBanFixtureLargeCohort34$', log)
        self.assertIn('matrix-produce reject-skips=yes', log)
        self.assertIn('identity-database', log)
        self.assertIn('inspect-absent manifest=no', log)
        self.assertIn('guard-2 manifest=no', log)
        self.assertIn('matrix-verify reject-skips=no manifest=no', log)
        self.assertLess(log.index('identity-admin'), log.index('migrate'))
        self.assertLess(log.index('identity-test'), log.index('migrate'))
        self.assertLess(log.index('identity-database'), log.index('migrate'))
        self.assertLess(log.rindex('identity-admin'), log.index('drop'))
        self.assertLess(log.rindex('identity-test'), log.index('drop'))
        self.assertLess(log.rindex('identity-database'), log.index('drop'))
        self.assertTrue(run['dropped'])
        self.assertIn('database=space_large34', run['recovery'])
        self.assertIn('state=removed', run['recovery'])
        script = SCRIPT.read_text(encoding='utf-8')
        for fragment in ('pg_control_system', 'system_identifier', 'inet_server_addr',
                         'inet_server_port', 'current_database', 'pg_database'):
            self.assertIn(fragment, script)
        self.assert_no_credentials(run)

    def test_seed_is_the_existing_base_space13_setup_not_a_301_row_backfill(self):
        run = self.run_proof()
        self.assertEqual(run['result'].returncode, 0, run['result'].stderr)
        current = SCRIPT.read_text(encoding='utf-8')
        existing = (ROOT / 'scripts/ci/space_over100_proof.sh').read_text(encoding='utf-8')

        def seed_block(source):
            match = re.search(r"<<'SQL'\n(.*?)\nSQL", source, re.S)
            self.assertIsNotNone(match, 'fixture baseline must stay explicit SQL')
            return match.group(1).strip()

        self.assertEqual(seed_block(current), seed_block(existing))
        self.assertEqual(run['seed'].strip(), seed_block(existing))
        self.assertNotRegex(run['seed'], r'(?i)(generate_series|301\s*\))')

    def test_skip_failure_malformed_or_truncated_go_stream_never_gets_final_manifest(self):
        for mode in ('skip', 'fail', 'missing', 'malformed', 'truncated'):
            with self.subTest(mode=mode):
                run = self.run_proof(mode)
                self.assertNotEqual(run['result'].returncode, 0, run['result'].stdout)
                self.assertFalse(run['exists'], run['log'])
                self.assertIsNone(run['manifest'])
                self.assertTrue(run['dropped'])
                self.assertIn('drop', run['log'])
                self.assert_no_credentials(run)

    def test_preexisting_partial_or_changed_database_identity_is_never_dropped(self):
        for mode, expected_oid in (('preexisting', '999'), ('partial_create', '123'),
                                   ('oid_drift', '123')):
            with self.subTest(mode=mode):
                run = self.run_proof(mode)
                self.assertNotEqual(run['result'].returncode, 0, run['result'].stdout)
                self.assertTrue(run['exists'])
                self.assertEqual(run['oid'], expected_oid)
                self.assertNotIn('drop\n', run['log'])
                self.assertIsNone(run['manifest'])
                self.assert_no_credentials(run)

    def test_cleanup_uncertainty_keeps_no_final_success_manifest(self):
        for mode in ('cleanup_inspect_fail', 'drop_fail', 'post_drop_inspect_fail', 'source_after_fail'):
            with self.subTest(mode=mode):
                run = self.run_proof(mode)
                self.assertNotEqual(run['result'].returncode, 0, run['result'].stdout)
                self.assertIsNone(run['manifest'])
                self.assert_no_credentials(run)
        expected = {'cleanup_inspect_fail': 'inspection_failed', 'drop_fail': 'drop_failed',
                    'post_drop_inspect_fail': 'post_drop_inspection_failed'}
        for mode, reason in expected.items():
            run = self.run_proof(mode)
            self.assertIsNotNone(run['recovery'])
            self.assertIn('state=uncertain', run['recovery'])
            self.assertIn('reason=' + reason, run['recovery'])
            if mode != 'post_drop_inspect_fail':
                self.assertTrue(run['exists'], run['log'])
            if mode == 'cleanup_inspect_fail':
                self.assertNotIn('drop\n', run['log'])
        run = self.run_proof('source_after_fail')
        self.assertFalse(run['exists'], run['log'])
        self.assertTrue(run['dropped'])
        run = self.run_proof('post_drop_inspect_fail')
        self.assertFalse(run['exists'], run['log'])
        self.assertTrue(run['dropped'])

    def test_migration_or_wrong_schema_version_fails_before_seed_and_final_manifest(self):
        for mode in ('migrate_fail', 'wrong_version', 'dirty_version'):
            with self.subTest(mode=mode):
                run = self.run_proof(mode)
                self.assertNotEqual(run['result'].returncode, 0, run['result'].stdout)
                self.assertNotIn('seed\n', run['log'])
                self.assertIsNone(run['manifest'])
                self.assertTrue(run['dropped'])

    def test_uncertain_database_identity_writes_recovery_without_dropping(self):
        for mode in ('partial_create', 'oid_drift'):
            with self.subTest(mode=mode):
                run = self.run_proof(mode)
                self.assertTrue(run['exists'])
                self.assertNotIn('drop\n', run['log'])
                self.assertIsNone(run['manifest'])
                self.assertIn('state=uncertain', run['recovery'])
                self.assertIn('reason=identity_unverified', run['recovery'])
                self.assert_no_credentials(run)

    def test_admin_and_test_urls_cannot_select_a_different_server_database_or_oid(self):
        cases = ('url_database_mismatch', 'test_cluster_mismatch', 'test_server_mismatch',
                 'test_database_mismatch', 'test_oid_mismatch')
        for mode in cases:
            with self.subTest(mode=mode):
                run = self.run_proof(mode)
                self.assertNotEqual(run['result'].returncode, 0, run['result'].stdout)
                self.assertNotIn('migrate ', run['log'])
                self.assertNotIn('seed\n', run['log'])
                self.assertNotIn('go test ', run['log'])
                self.assertIsNone(run['manifest'])
                if mode == 'url_database_mismatch':
                    self.assertNotIn('create\n', run['log'])
                    self.assertFalse(run['exists'], run['log'])
                else:
                    self.assertTrue(run['dropped'], run['log'])
                    self.assertFalse(run['exists'], run['log'])
                self.assert_no_credentials(run)

    def test_matrix_receipt_identity_must_match_candidate_run_and_attempt(self):
        for mode in ('receipt_candidate_mismatch', 'receipt_run_mismatch', 'receipt_attempt_mismatch'):
            with self.subTest(mode=mode):
                run = self.run_proof(mode)
                self.assertNotEqual(run['result'].returncode, 0, run['result'].stdout)
                self.assertIn('matrix-verify reject-skips=no manifest=no', run['log'])
                self.assertIsNone(run['manifest'])
                self.assertTrue(run['dropped'], run['log'])
                self.assertFalse(run['exists'], run['log'])
                self.assert_no_credentials(run)

    def test_pre_source_guard_failure_precedes_every_mutation(self):
        run = self.run_proof('source_before_fail')
        self.assertNotEqual(run['result'].returncode, 0)
        self.assertFalse(run['exists'])
        self.assertNotIn('create\n', run['log'])
        self.assertIsNone(run['manifest'])

    def test_source_checkout_verifier_rejects_wrong_head_dirty_tree_and_untracked_go(self):
        matrix = matrix_module()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            paths = [root / 'scripts/ci/space_large34_required.json',
                     root / 'scripts/ci/space_large34_proof.sh',
                     root / 'scripts/ci/space_large34_source.py']
            for path in paths:
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text('fixture\n', encoding='utf-8')
            subprocess.run(['git', '-C', str(root), 'init', '-q'], check=True)
            subprocess.run(['git', '-C', str(root), 'config', 'user.email', 'tests@example.invalid'], check=True)
            subprocess.run(['git', '-C', str(root), 'config', 'user.name', 'Integration Test'], check=True)
            subprocess.run(['git', '-C', str(root), 'add', '--', *[p.relative_to(root).as_posix() for p in paths]], check=True)
            subprocess.run(['git', '-C', str(root), 'commit', '-qm', 'fixture'], check=True)
            first = subprocess.check_output(['git', '-C', str(root), 'rev-parse', 'HEAD'], text=True).strip()
            script_path = root / 'scripts/ci/go_integration_matrix.py'
            old_cwd = Path.cwd()
            try:
                os.chdir(root)
                with mock.patch.object(matrix, '__file__', str(script_path)):
                    matrix.verify_source_checkout(root, first, [p.relative_to(root).as_posix() for p in paths])
                    with self.assertRaisesRegex(ValueError, 'does not match candidate'):
                        matrix.verify_source_checkout(root, '0' * 40, [p.relative_to(root).as_posix() for p in paths])
                    paths[1].write_text('dirty\n', encoding='utf-8')
                    with self.assertRaisesRegex(ValueError, 'modified'):
                        matrix.verify_source_checkout(root, first, [p.relative_to(root).as_posix() for p in paths])
                    paths[1].write_text('fixture\n', encoding='utf-8')
                    untracked = root / 'src/backend/space/untracked.go'
                    untracked.parent.mkdir(parents=True)
                    untracked.write_text('package space\n', encoding='utf-8')
                    with self.assertRaisesRegex(ValueError, 'untracked Go source'):
                        matrix.verify_source_checkout(root, first, [p.relative_to(root).as_posix() for p in paths])
            finally:
                os.chdir(old_cwd)

    def test_source_wrapper_binds_candidate_and_all_proof_sources(self):
        self.assertTrue(SOURCE_GUARD.is_file(), 'source guard wrapper must be committed with its producer')
        source = SOURCE_GUARD.read_text(encoding='utf-8')
        self.assertIn('verify_source_checkout', source)
        self.assertIn('--candidate', source)
        for path in ('space_large34_required.json', 'space_large34_proof.sh', 'space_large34_source.py'):
            with self.subTest(path=path):
                self.assertIn(path, source)

    def test_ci_selects_exact_fixture_and_always_uploads_bound_artifacts(self):
        import yaml
        inventory_path = ROOT / 'scripts/ci/space_large34_required.json'
        inventory = json.loads(inventory_path.read_text(encoding='utf-8'))
        self.assertEqual(inventory, {'space': [[PACKAGE, TEST_NAME]]})

        exceptions = json.loads((ROOT / 'scripts/ci/go_integration_exceptions.json').read_text(encoding='utf-8'))
        entry = next(row for row in exceptions if row.get('service') == 'space' and row.get('test') == TEST_NAME)
        self.assertEqual((entry.get('status'), entry.get('reason')),
                         ('dedicated', 'space-native-installer-integration'))

        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8'))
        jobs = workflow['jobs']
        job = jobs['space-native-installer-integration']
        step = next(row for row in job['steps'] if 'space_large34_proof.sh' in row.get('run', ''))
        self.assertEqual(step.get('name'), 'Run Space historical 301-member fixture proof')
        self.assertIn('bash ../../../scripts/ci/space_large34_proof.sh', step['run'])
        self.assertIn('space_large34', step['env']['SPACE_BAN_TEST_DATABASE_URL'])
        self.assertEqual(step['env']['CANDIDATE_SHA'], '${{ github.sha }}')
        uploads = [row for row in job['steps']
                   if 'space-large34-proof.json' in row.get('with', {}).get('path', '')]
        self.assertEqual(len(uploads), 1)
        upload = uploads[0]
        self.assertTrue(upload.get('if', '').startswith('always()'))
        for artifact in ('space-large34-events.jsonl', 'space-large34-receipt.json',
                         'space-large34-proof.json', 'space-large34-cleanup.txt'):
            self.assertIn(artifact, upload['with']['path'])
        self.assertIn('space-native-installer-integration', jobs['ci-gate']['needs'])


if __name__ == '__main__':
    unittest.main()
