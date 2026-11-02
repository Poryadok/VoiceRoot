"""Disposable real-Git carrier/event fixture; never evidence of hosted CI."""
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import f13_current_source as current

ROOT = Path(__file__).resolve().parents[2]


class CurrentSourceRealGitTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory(prefix='f13-current-source-fixture-')
        cls.addClassCleanup(cls.temp.cleanup)
        cls.repo = Path(cls.temp.name) / 'repo'
        subprocess.run(['git', 'clone', '--shared', '--no-checkout', str(ROOT), str(cls.repo)],
                       check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        cls.git('config', 'user.name', 'Fixture')
        cls.git('config', 'user.email', 'fixture@example.invalid')
        cls.git('config', 'core.autocrlf', 'false')
        cls.git('checkout', '-qb', 'fixture-carrier', current.RECORD_COMMIT)
        for name in current.CARRIER_PATHS:
            destination = cls.repo / name
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, destination)
        cls.git('add', '--', *sorted(current.CARRIER_PATHS))
        cls.git('commit', '-qm', 'Fixture carrier')
        cls.carrier = cls.git('rev-parse', 'HEAD')
        cls.git('checkout', '-qb', 'fixture-event', current.SOURCE)
        cls.git('merge', '--no-ff', '-qm', 'Fixture merge', cls.carrier)
        cls.target = cls.git('rev-parse', 'HEAD')

    @classmethod
    def git(cls, *args):
        return subprocess.check_output(['git', '-C', str(cls.repo), *args],
                                       stderr=subprocess.PIPE).decode().strip()

    def run_fixture(self, program):
        env = os.environ.copy()
        env.update(GITHUB_EVENT_NAME='push', GITHUB_REF='refs/heads/master', GITHUB_SHA=self.target,
                   VOICE_CI_PUSH_BEFORE_SHA=current.SOURCE, GITHUB_BASE_REF='')
        result = subprocess.run([sys.executable, '-c', program, current.SOURCE, self.carrier, self.target],
                                cwd=self.repo, env=env, text=True, capture_output=True, check=False)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_public_consumers_against_real_git(self):
        self.run_fixture('''
import hashlib,json,sys,zipfile
sys.path.insert(0,'scripts/ci')
import f13_native_p3_authority as native
import f13_current_source as current
import f13_production_source as production
import f13_role_conditional as role
import f13_f54_history as history
base,source,target=sys.argv[1:]
b=native.qualify('.',base,source,target,'push','refs/heads/master',target,base,'')
assert b['version']==current.NATIVE_VERSION
v,r=native.verified_record('.',b)
assert v is current and r['source']==current.SOURCE
p=production.qualify('.')
assert p['type']==current.PRODUCTION_VERSION and p['record']==b['record']
old=json.loads(role.git('.', 'show', role.PINNED_RECORD_COMMIT+':'+role.RECORD_PATH))
m=old['incoming_metadata_at_failure']
prior='\\t'.join(old['actual_prior_row'])
incoming='\\t'.join([role.ROLE_PATH,'true',m['owner'],m['parent'],m['source_base'],m['source_head'],m['content_base'],role.PINNED_BASE])
receipt=current.role_qualify('.',target,prior,incoming)
assert receipt['retain_runtime_and_content_taint'] is True and receipt['runtime_pass'] is False
old_production=production.load_record('.')
selected=[x for x in old_production['archive']['selected']
          if x['logical_path']=='/original/baseline/f13-native-p3-binding.json']
assert len(selected)==1
with zipfile.ZipFile(production.ARCHIVE_PATH) as archive:
    raw=archive.read('objects/'+selected[0]['sha256'])
assert hashlib.sha256(raw).hexdigest()==selected[0]['sha256'] and len(raw)==selected[0]['bytes']
historical=json.loads(raw)
result=history.current_author_J_native_applicability('.',native,historical,b)
assert result['runtime_pass'] is False and result['admission'] is False
''')

    def test_rejects_changed_event_and_executed_carrier(self):
        self.run_fixture('''
import pathlib,sys
sys.path.insert(0,'scripts/ci')
import f13_current_source as c
base,source,target=sys.argv[1:]
def rejects(action, reason):
    try: action()
    except ValueError as exc: assert reason in str(exc),str(exc)
    else: raise AssertionError('accepted '+reason)
rejects(lambda:c.native_qualify('.',base,source,target,'push','refs/heads/master',target,'0'*40,''),
        'release_master_event')
rejects(lambda:c.native_qualify('.',base,source,source,'push','refs/heads/master',source,base,''),
        'current_actual_head')
path=pathlib.Path('scripts/ci/f13_source_dag.py'); original=path.read_bytes()
try:
    path.write_bytes(original+b'\\n# fixture byte drift\\n')
    rejects(lambda:c._check_carrier('.',source),'current_executed_carrier')
finally: path.write_bytes(original)
''')


if __name__ == '__main__':
    unittest.main()
