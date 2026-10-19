"""Execute the real A1 EXIT trap with deterministic local command doubles."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

SOURCE = Path(__file__).resolve().parent
TESTS = ['A1TwoAccountsFoundation', 'A1DailyMessagingREST', 'A1GroupReadIsolation',
         'A1ChannelReadIsolation', 'A1BlockDMDenyBothDirections',
         'A1SessionEpochRealtime', 'ThreadsDMReply', 'SpaceCreatorProtectedPublic']


class CleanupTest(unittest.TestCase):
    def run_case(self, case):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            ci = root/'scripts/ci'
            ci.mkdir(parents=True)
            (root/'src/backend/gateway').mkdir(parents=True)
            for name in ['compose-a1-multi-account-proof.sh', 'space-native-ci-bootstrap.py']:
                shutil.copyfile(SOURCE/name, ci/name)
            (ci/'e2e-manifest.sh').write_text('\n'.join('echo TestCompose'+t+'_live' for t in TESTS))
            commands = root/'bin'
            commands.mkdir()
            marker = root/'volume-data'
            network = root/'network'
            if case == 'existing-volume': marker.write_text('foreign data')
            if case == 'existing-network': network.write_text('foreign network')
            mock = '''import json,os,pathlib,sys
r=pathlib.Path(os.environ['CASE_ROOT']); a=sys.argv[1:]; case=os.environ['CASE']
with (r/'commands').open('a') as log: log.write(json.dumps(a)+'\\n')
project=os.environ.get('COMPOSE_PROJECT_NAME','')
volume=project+'_postgres_data'
if a[:1]==['compose']:
 if 'down' in a:
  for p in [r/'volume-data',r/'network']: p.unlink(missing_ok=True)
 elif 'config' in a and '--format' in a:
  print(json.dumps({'name':project,'services':{'postgres':{'volumes':[{'type':'volume','source':'pg'}]},'compose-db-init':{'environment':{'PGUSER':'voice','PGPASSWORD':'private'}}},'volumes':{'pg':{'name':volume}}}))
 elif 'ps' in a and '-q' in a: print('owned-container')
elif a[:1]==['inspect']: print('healthy')
elif a[:1]==['info']: print('engine')
elif a[:2]==['volume','ls']:
 if (r/'volume-data').exists(): print(volume)
elif a[:2]==['network','ls']:
 if (r/'network').exists(): print('network-id')
elif a[:2]==['volume','create']:
 (r/'volume-data').write_text('owned data')
 (r/'labels').write_text(json.dumps(dict(a[i+1].split('=',1) for i,x in enumerate(a) if x=='--label')))
 print(volume)
elif a[:2]==['volume','inspect']:
 if case=='partial-prepare': sys.exit(9)
 print(json.dumps([{'Labels':json.loads((r/'labels').read_text()),'CreatedAt':'fixed'}]))
'''
            def executable(name, body):
                p = commands/name
                p.write_text('#!'+sys.executable+'\n'+body)
                p.chmod(0o755)
            executable('docker', mock)
            executable('python3', '''import os,pathlib,sys
a=sys.argv[1:]
if a[0].endswith('generate-phase0-fixture.py'):
 pathlib.Path(a[1]).mkdir(); sys.exit(0)
if a[0].endswith('restore-phase0-fixture-owner.py'): sys.exit(0)
if a[0].endswith('space-native-ci-bootstrap.py') and a[1]=='install':
 sys.exit(7 if os.environ['CASE']=='install-failure' else 0)
os.execv(sys.executable,[sys.executable]+a)
''')
            executable('go', "import os,sys; sys.exit(8 if os.environ['CASE']=='test-failure' else 0)")
            executable('curl', 'pass')
            env = {k:v for k,v in os.environ.items() if not k.startswith('COMPOSE_') and k != 'SPACE_DATABASE_URL'}
            env.update(PATH=str(commands)+':'+env['PATH'], CASE=case, CASE_ROOT=str(root),
                       TMPDIR=str(root), VOICE_A1_MULTI_ACCOUNT_CLEANUP='true')
            result = subprocess.run(['bash',str(ci/'compose-a1-multi-account-proof.sh')],
                                    env=env, text=True, capture_output=True)
            calls = [json.loads(s) for s in (root/'commands').read_text().splitlines()]
            destructive = [a for a in calls if 'down' in a or a[:2] in [['volume','rm'],['network','rm']]]
            if case == 'existing-volume':
                self.assertTrue(marker.exists(), result.stderr)
                self.assertEqual(marker.read_text(), 'foreign data')
            if case == 'existing-network':
                self.assertTrue(network.exists(), result.stderr)
                self.assertEqual(network.read_text(), 'foreign network')
            state = list(root.glob('voice-a1-multi-account-proof.*'))
            return result, destructive, marker.exists(), network.exists(), bool(state)

    def test_rejected_foreign_resources_are_preserved(self):
        for case in ['existing-volume', 'existing-network']:
            with self.subTest(case=case):
                result, calls, volume, network, _ = self.run_case(case)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(calls, [], result.stderr)
                self.assertTrue(volume if case=='existing-volume' else network)

    def test_owned_success_and_failures_cleanup(self):
        for case, status in [('success',0), ('install-failure',7), ('test-failure',8)]:
            with self.subTest(case=case):
                result, calls, volume, _, state = self.run_case(case)
                self.assertEqual(result.returncode, status, result.stderr)
                self.assertEqual(len(calls), 1)
                self.assertIn('-v', calls[0])
                self.assertFalse(volume)
                self.assertFalse(state)

    def test_partial_prepare_retains_owned_evidence_without_project_delete(self):
        result, calls, volume, _, state = self.run_case('partial-prepare')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(calls, [], result.stderr)
        self.assertTrue(volume)
        self.assertTrue(state)


if __name__ == '__main__':
    unittest.main()
