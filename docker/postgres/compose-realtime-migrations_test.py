"""Exercise Compose's real shell bootstrap against a stateful command boundary."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
TOOLS = r'''#!/usr/bin/env python3
import json, os, pathlib, sys
p = pathlib.Path(os.environ['TEST_DB_STATE'])
s = json.loads(p.read_text())
args = sys.argv[1:]
tool = pathlib.Path(sys.argv[0]).name
if tool == 'bot-source.py':
    # Adjacent native owners are command seams, not custody simulations.
    assert args == ['/native-bot-evidence/source.uuid', '--', 'migrate',
                    '-path', os.environ['MIGRATIONS_DIR'] + '/bot_db',
                    '-database', 'postgres://voice:voice@postgres:5432/bot_db?sslmode=disable', 'up']
    s['bootstrap_calls'].append('bot')
    if os.environ.get('TEST_BOT_SOURCE_FAILURE') == '1':
        p.write_text(json.dumps(s))
        sys.exit(2)
elif tool == 'native-migrate':
    assert args == ['--mode=provision', '--migrations-dir=' + os.environ['MIGRATIONS_DIR'] + '/space_db',
                    '--migrate=/usr/local/bin/native-space-migrate-driver']
    s['bootstrap_calls'].append('space')
elif tool == 'psql':
    sql = ' '.join(args)
    if 'SELECT 1 FROM pg_database' in sql:
        name = sql.split("datname = '")[1].split("'")[0]
        print('1' if name in s['databases'] else '')
    elif 'CREATE DATABASE ' in sql:
        name = sql.split('CREATE DATABASE ')[1].split(';')[0]
        s['databases'].append(name)
    elif "to_regclass('public.schema_migrations')" in sql:
        print('t' if s['tracked'] else 'f')
    elif 'SELECT dirty' in sql:
        print('t' if s['dirty'] else 'f')
    elif 'SELECT version' in sql:
        print('1')
    elif 'COUNT(*) FROM information_schema.tables' in sql:
        print(s['tables'])
elif tool == 'migrate':
    s['migration_calls'].append(args)
    if 'force' in args:
        s['dirty'] = False
        s['tracked'] = True
    elif args[-1] == 'up':
        s['tracked'] = True
        s['tables'] = 6
        print('no change' if len(s['migration_calls']) > 1 else '1/u admission')
p.write_text(json.dumps(s))
'''


class RealtimeBootstrapTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name)
        self.state = self.base / 'state.json'
        self.initial = dict(databases=[], tracked=False, dirty=False, tables=0,
                            migration_calls=[], bootstrap_calls=[])
        self.state.write_text(json.dumps(self.initial))
        (self.base / 'migrations/realtime_db').mkdir(parents=True)
        (self.base / 'migrations/realtime_db/000001_admission.up.sql').write_text('-- boundary fixture')
        (self.base / 'schema').mkdir()
        for tool in ('psql', 'migrate', 'bot-source.py', 'native-migrate'):
            path = self.base / tool
            path.write_text(TOOLS)
            path.chmod(0o755)
        self.env = dict(os.environ, TEST_DB_STATE=str(self.state),
                        PATH=str(self.base) + os.pathsep + os.environ['PATH'],
                        MIGRATIONS_DIR=str(self.base / 'migrations'),
                        SCHEMA_DIR=str(self.base / 'schema'),
                        PGHOST='postgres', PGUSER='voice', PGPASSWORD='voice',
                        SPACE_NATIVE_PRIOR_EVIDENCE='', SPACE_DB_PASSWORD='disposable-test',
                        GAME_INTEGRATION_DB_PASSWORD='disposable-test')

    def run_migration(self, bootstrap=False):
        # Relocate image-installed commands only; retain the real Realtime
        # migration body and all of its dirty/untracked safety decisions.
        text = (ROOT / 'docker/postgres/compose-migrate-dbs.sh').read_text()
        script = self.base / 'migrate.sh'
        script.write_text(text.replace('/usr/local/bin/provision-bot-membership-source.py',
                                       str(self.base / 'bot-source.py')))
        if bootstrap:
            # Relocate only the image-installed executable path for this unit seam.
            text = (ROOT / 'docker/postgres/ensure-compose-schema.sh').read_text()
            script_copy = self.base / 'ensure.sh'
            script_copy.write_text(text.replace('/usr/local/bin/compose-migrate-dbs.sh', str(script)))
            script = script_copy
        result = subprocess.run(['sh', str(script)], env=self.env, capture_output=True, text=True)
        return result, json.loads(self.state.read_text())

    def test_existing_volume_creates_and_migrates_realtime(self):
        result, state = self.run_migration(bootstrap=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('realtime_db', state['databases'])
        self.assertTrue(state['tracked'], 'Realtime migration must run before startup')
        self.assertEqual(state['tables'], 6)
        self.assertEqual(len(state['migration_calls']), 1)
        self.assertEqual(state['bootstrap_calls'], ['bot', 'space'])
        again, current = self.run_migration(bootstrap=True)
        self.assertEqual(again.returncode, 0, again.stderr)
        self.assertEqual(current['databases'].count('realtime_db'), 1)
        self.assertTrue(all(call[-1] == 'up' for call in current['migration_calls']))
        self.assertEqual(current['bootstrap_calls'], ['bot', 'space', 'bot', 'space'])

    def test_fresh_realtime_is_migrated(self):
        result, state = self.run_migration()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(state['tracked'], 'Fresh realtime_db must receive its canonical migration')
        self.assertEqual(state['tables'], 6)
        self.assertEqual(state['bootstrap_calls'], ['bot', 'space'])

    def test_bot_source_failure_stops_before_realtime_migration(self):
        self.env['TEST_BOT_SOURCE_FAILURE'] = '1'
        result, state = self.run_migration()
        self.assertEqual(result.returncode, 2)
        self.assertEqual(state['bootstrap_calls'], ['bot'])
        self.assertEqual(state['migration_calls'], [])
        self.assertFalse(state['tracked'])

    def test_dirty_realtime_is_rejected_without_force(self):
        self.initial.update(tracked=True, dirty=True, tables=6)
        self.state.write_text(json.dumps(self.initial))
        result, state = self.run_migration()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('realtime_db', result.stderr)
        self.assertTrue(state['dirty'])
        self.assertEqual(state['migration_calls'], [])

    def test_untracked_nonempty_realtime_is_rejected_without_baseline(self):
        self.initial.update(tables=1)
        self.state.write_text(json.dumps(self.initial))
        result, state = self.run_migration()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('realtime_db', result.stderr)
        self.assertFalse(state['tracked'])
        self.assertEqual(state['migration_calls'], [])


if __name__ == '__main__':
    unittest.main()
