#!/usr/bin/env python3
import contextlib
import importlib.util
import io
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch
import uuid

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('fixture', Path(__file__).with_name('compose_mm_accept_fixture.py'))
fixture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fixture)
GAME = 'd43ae80b-bfe8-4abd-aa85-d5b3972b3ccf'


class Contracts(unittest.TestCase):
    def test_refuses_non_disposable_project_before_docker(self):
        for project in ['', 'voice', 'voice-dev', 'voice-compose-e2e-x-1']:
            with self.subTest(project=project), patch.dict(os.environ, COMPOSE_PROJECT_NAME=project), patch.object(fixture.subprocess, 'run') as run:
                with self.assertRaises(fixture.FixtureError):
                    fixture.apply('create', GAME)
                run.assert_not_called()

    def test_rejects_unsafe_identity_and_unknown_action_before_docker(self):
        for action, identity in [('create', "';secret"), ('create', GAME.upper()), ('erase', GAME)]:
            with self.subTest(action=action), patch.dict(os.environ, COMPOSE_PROJECT_NAME='voice-compose-e2e-123-1'), patch.object(fixture.subprocess, 'run') as run:
                with self.assertRaises(fixture.FixtureError):
                    fixture.apply(action, identity)
                run.assert_not_called()

    def test_sql_uses_private_stdin_and_never_prints_database_failure(self):
        for result in [subprocess.CompletedProcess([], 1, '', 'private-token'), subprocess.CompletedProcess([], 0, 'private-id', '')]:
            output = io.StringIO()
            with patch.dict(os.environ, COMPOSE_PROJECT_NAME='voice-compose-e2e-123-1'), patch.object(fixture.subprocess, 'run', return_value=result) as run, contextlib.redirect_stderr(output):
                self.assertEqual(fixture.main(['create', GAME]), 1)
                self.assertEqual(output.getvalue(), 'compose_mm_fixture failed=database\n')
                self.assertNotIn(GAME, ' '.join(run.call_args.args[0]))
                self.assertEqual(run.call_args.kwargs['timeout'], 20)
                self.assertTrue(run.call_args.kwargs['capture_output'])

    def test_transport_error_does_not_emit_exception_payload(self):
        output = io.StringIO()
        with patch.dict(os.environ, COMPOSE_PROJECT_NAME='voice-compose-e2e-123-1'), patch.object(fixture.subprocess, 'run', side_effect=OSError('private-token')), contextlib.redirect_stderr(output):
            self.assertEqual(fixture.main(['create', GAME]), 1)
        self.assertEqual(output.getvalue(), 'compose_mm_fixture failed=transport\n')

    def test_exit_trap_archives_on_failure_and_preserves_original_status(self):
        source = (ROOT / 'scripts/ci/compose-e2e-smoke.sh').read_text()
        definitions = source[source.index('cleanup_mm_accept_fixture() {'):source.index('trap cleanup_fcm_diagnostic EXIT')]
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            (root / 'scripts/ci').mkdir(parents=True)
            (root / 'scripts/ci/compose_mm_accept_fixture.py').write_text(
                "import os,sys\nfrom pathlib import Path\nPath(os.environ['ARCHIVE_LOG']).write_text(sys.argv[1])\nsys.exit(int(os.environ['ARCHIVE_STATUS']))\n")
            for original, cleanup, expected in [(0, 0, 0), (17, 0, 17), (0, 1, 1), (17, 1, 17)]:
                with self.subTest(original=original, cleanup=cleanup):
                    log = root / 'archive.log'
                    env = dict(os.environ, ARCHIVE_LOG=str(log), ARCHIVE_STATUS=str(cleanup))
                    script = f"set -euo pipefail\nROOT='{root}'\nTRACE_FILE=''\nmm_accept_game_id='{GAME}'\n{definitions}\ntrap cleanup_fcm_diagnostic EXIT\nexit {original}\n"
                    result = subprocess.run(['bash', '-c', script], env=env, capture_output=True, text=True, timeout=10)
                    self.assertEqual(result.returncode, expected)
                    self.assertEqual(log.read_text(), 'archive')
                    self.assertNotIn(GAME, result.stdout + result.stderr)

    def test_smoke_always_passes_owned_game_and_keeps_acceptance_assertions(self):
        smoke = (ROOT / 'scripts/ci/compose-e2e-smoke.sh').read_text()
        self.assertLess(smoke.index('mm_accept_game_id="$(python3'), smoke.index('compose_mm_accept_fixture.py" create'))
        self.assertLess(smoke.index('compose_mm_accept_fixture.py" create'), smoke.index('flutter test --concurrency=1'))
        self.assertIn('--dart-define=VOICE_COMPOSE_MM_ACCEPT_GAME_ID="${mm_accept_game_id}"', smoke)
        dart = (ROOT / 'src/frontend/test/matchmaking_e2e_live_test.dart').read_text()
        self.assertIn("String.fromEnvironment('VOICE_COMPOSE_MM_ACCEPT_GAME_ID')", dart)
        self.assertIn('expect(resp.statusCode, 200,', dart)
        self.assertIn("expect(match['status'], 'active')", dart)
        self.assertIn('expect(value, 5.0)', dart)


@unittest.skipUnless(os.environ.get('MM_ACCEPT_FIXTURE_TEST_DSN'), 'set private PostgreSQL DSN for SQL integration')
class Postgres(unittest.TestCase):
    def setUp(self):
        import psycopg
        self.conn = psycopg.connect(os.environ['MM_ACCEPT_FIXTURE_TEST_DSN'], autocommit=True)
        self.schema = 'mm_fixture_' + uuid.uuid4().hex
        self.conn.execute('CREATE SCHEMA ' + self.schema)
        self.conn.execute('SET search_path TO ' + self.schema)
        for name in ['000001_init.up.sql', '000012_mm_duo_live_seed.up.sql', '000013_mm_duo_live_lfp_party.up.sql']:
            self.conn.execute((ROOT / 'src/backend/migrations/matchmaking_db' / name).read_text())

    def tearDown(self):
        self.conn.execute('DROP SCHEMA ' + self.schema + ' CASCADE')
        self.conn.close()

    def test_clones_exact_config_and_archives_only_its_fixture(self):
        self.conn.execute(fixture.fixture_sql('create', GAME))
        self.assertTrue(self.conn.execute("SELECT a.config=b.config AND a.status='active' FROM games a,games b WHERE a.id=%s AND b.name='MM Duo Live'", (GAME,)).fetchone()[0])
        self.conn.execute(fixture.fixture_sql('archive', GAME))
        self.conn.execute(fixture.fixture_sql('archive', GAME))
        self.assertEqual(self.conn.execute('SELECT status FROM games WHERE id=%s', (GAME,)).fetchone()[0], 'archived')
        self.assertEqual(self.conn.execute("SELECT status FROM games WHERE name='MM Duo Live'").fetchone()[0], 'active')

    def test_missing_canonical_does_not_create_fixture(self):
        self.conn.execute("DELETE FROM games WHERE name='MM Duo Live'")
        with self.assertRaises(Exception):
            self.conn.execute(fixture.fixture_sql('create', GAME))
        self.assertEqual(self.conn.execute('SELECT count(*) FROM games WHERE id=%s', (GAME,)).fetchone()[0], 0)

    def test_cleanup_after_uncertain_creation_cannot_archive_foreign_row(self):
        self.conn.execute(fixture.fixture_sql('archive', GAME))
        self.conn.execute("INSERT INTO games(id,name,config,status) VALUES(%s,'foreign','{}','active')", (GAME,))
        with self.assertRaises(Exception):
            self.conn.execute(fixture.fixture_sql('archive', GAME))
        self.assertEqual(self.conn.execute('SELECT status FROM games WHERE id=%s', (GAME,)).fetchone()[0], 'active')


if __name__ == '__main__':
    unittest.main()
