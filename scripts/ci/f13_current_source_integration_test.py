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
FIXTURE_BASE = 'f7fe4f0b1521633b04fbcdf1990636301f7c0537'
PR1091_SOURCE = '79bfb9ba3d31626f83079eebeb5c4fd20948ca6e'


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
        cls.git('merge', '--no-ff', '-qm', 'Fixture merge PR1091 source', PR1091_SOURCE)
        cls.carrier = cls.git('rev-parse', 'HEAD')
        cls.git('merge', '--no-ff', '-qm', 'Fixture incorporate actual base', FIXTURE_BASE)
        cls.source = cls.git('rev-parse', 'HEAD')
        cls.git('checkout', '-qb', 'fixture-event', FIXTURE_BASE)
        cls.git('merge', '--no-ff', '-qm', 'Fixture merge', cls.source)
        cls.target = cls.git('rev-parse', 'HEAD')

    @classmethod
    def git(cls, *args):
        return subprocess.check_output(['git', '-C', str(cls.repo), *args],
                                       stderr=subprocess.PIPE).decode().strip()

    def run_fixture(self, program):
        env = os.environ.copy()
        env.update(GITHUB_EVENT_NAME='push', GITHUB_REF='refs/heads/master', GITHUB_SHA=self.target,
                   VOICE_CI_PUSH_BEFORE_SHA=FIXTURE_BASE, GITHUB_BASE_REF='')
        result = subprocess.run([sys.executable, '-c', program, FIXTURE_BASE, self.source, self.target, self.carrier],
                                cwd=self.repo, env=env, text=True, capture_output=True, check=False)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_public_consumers_against_real_git(self):
        self.run_fixture('''
import hashlib,json,os,pathlib,subprocess,sys,tempfile,zipfile
sys.path.insert(0,'scripts/ci')
import f13_native_p3_authority as native
import f13_current_source as current
import f13_production_source as production
import f13_role_conditional as role
import f13_f54_history as history
base,source,target,carrier=sys.argv[1:]
b=native.qualify('.',base,source,target,'push','refs/heads/master',target,base,'')
assert b['version']==current.NATIVE_VERSION
assert b['qualification']=='PRESERVED_UNIT_BASE_BRIDGE'
assert b['foundation']['initial_base']==base and b['foundation']['carrier']==carrier
with tempfile.TemporaryDirectory() as directory:
    binding=pathlib.Path(directory)/'native-binding.json'
    binding.write_text(json.dumps(b),encoding='utf-8')
    output=subprocess.check_output([sys.executable,'scripts/ci/f13_release_source.py',
                                    'collection-base','.',str(binding)],text=True).strip()
    forged=dict(b,record_sha256='0'*64)
    binding.write_text(json.dumps(forged),encoding='utf-8')
    denied=subprocess.run([sys.executable,'scripts/ci/f13_release_source.py',
                           'collection-base','.',str(binding)],capture_output=True,text=True,check=False)
    assert denied.returncode != 0 and 'current_native_binding' in denied.stderr,denied.stderr
    binding.write_text(json.dumps(b),encoding='utf-8')
    bad_event=dict(os.environ,VOICE_CI_PUSH_BEFORE_SHA='0'*40)
    denied=subprocess.run([sys.executable,'scripts/ci/f13_release_source.py',
                           'collection-base','.',str(binding)],env=bad_event,capture_output=True,text=True,check=False)
    assert denied.returncode != 0 and 'bridge_master_event' in denied.stderr,denied.stderr
assert output==current.release.COLLECTION_BASE
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
base,source,target,carrier=sys.argv[1:]
def rejects(action, reason):
    try: action()
    except ValueError as exc: assert reason in str(exc),str(exc)
    else: raise AssertionError('accepted '+reason)
rejects(lambda:c.native_qualify('.',base,source,target,'push','refs/heads/master',target,'0'*40,''),
        'bridge_master_event')
rejects(lambda:c.native_qualify('.',base,source,source,'push','refs/heads/master',source,base,''),
        'current_actual_head')
path=pathlib.Path('scripts/ci/f13_source_dag.py'); original=path.read_bytes()
try:
    path.write_bytes(original+b'\\n# fixture byte drift\\n')
    rejects(lambda:c._check_carrier('.',carrier),'current_executed_carrier')
finally: path.write_bytes(original)
''')


if __name__ == '__main__':
    unittest.main()
