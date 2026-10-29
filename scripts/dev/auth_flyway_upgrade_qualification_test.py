import copy
import pathlib
import hashlib
import hmac
import json
import multiprocessing
import os
import subprocess
import tempfile
import unittest
from unittest.mock import patch
from types import SimpleNamespace

from auth_flyway_upgrade_qualification import snapshot_protected, source_catalog, validate_admission, validate_backup_inputs, validate_history, validate_markers
from auth_flyway_upgrade_qualification_docker import OwnedDocker


class QualificationContract(unittest.TestCase):
    def test_resource_removal_is_journaled_before_effect(self):
        class Journal:
            def __init__(self): self.state={}; self.snapshots=[]
            def save(self): self.snapshots.append(copy.deepcopy(self.state))
            def guard(self): pass
        journal=Journal(); cid='a'*64
        def run(command, **kwargs):
            args=command[1:]
            if args[:2] == ['info','--format']: out=b'linux\n'
            elif args[0] == 'inspect':
                out=json.dumps([{'Id':cid,'Image':'sha256:'+'b'*64,'Config':{'Labels':{'voice.test.owner':'auth-flyway-qualification'}}}]).encode()
            elif args[0] == 'rm':
                self.assertEqual(journal.snapshots[-1]['flyway_clone']['resources'][0]['state'],'removing'); out=b''
            else: out=b''
            return SimpleNamespace(returncode=0,stdout=out)
        with patch('auth_flyway_upgrade_qualification_docker.subprocess.run',side_effect=run):
            owned=OwnedDocker(journal)
            owned.records.append({'id':cid,'image_id':'sha256:'+'b'*64,'state':'owned','volumes':[]})
            owned.save(); owned.cleanup()
        self.assertEqual(journal.state['flyway_clone']['resources'][0]['state'],'removed')

    def test_protected_input_is_bounded_finite_and_detached(self):
        value = {'nested': ['original']}
        saved = snapshot_protected(value)
        value['nested'][0] = 'changed'
        self.assertEqual(saved, {'nested': ['original']})
        for invalid in ({'v': 'x' * (2 * 1024 * 1024)}, {'v': float('nan')}, {'v': object()}):
            with self.assertRaises(ValueError):
                snapshot_protected(invalid)

    def test_encrypted_members_and_receipt_bind_exact_admission(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            key = '11' * 32
            (root / 'backup.key').write_text(key)
            os.chmod(root / 'backup.key', 0o600)
            files = {}
            for name in ('auth_db.enc', 'user_db.enc', 'roles.enc'):
                data = name.encode()
                (root / name).write_bytes(data)
                os.chmod(root / name, 0o600)
                files[name] = {'sha256': hashlib.sha256(data).hexdigest(),
                               'hmac': hmac.new(bytes.fromhex(key), data, hashlib.sha256).hexdigest()}
            admission = {'source_sha': 'a' * 40, 'target': 32, 'flyway_enabled': False,
                         'selector_mode': 'absent', 'persistence': 'jdbc', 'plan_sha256': 'b' * 64,
                         'operator_sha256': 'c' * 64, 'sources_sha256': 'd' * 64}
            binding = {k: admission[k] for k in ('source_sha', 'operator_sha256', 'sources_sha256')}
            manifest = {'format': 1, 'operation': 'op', 'binding': binding, 'files': files, 'catalog': {}}
            report = {'protocol': 'voice-auth-user-backup-v1', 'operation': 'op', 'result': 'PASS',
                      'files': files, 'binding': binding, 'restored_catalog': {}, 'off_node': True,
                      'restore_exit': {'auth_db': 0, 'user_db': 0, 'roles': 0}, 'postgres_major': 16,
                      'verifier_sha256': 'e' * 64, 'tools_image': 'sha256:' + 'f' * 64,
                      'fixture_cleanup': True, 'scope': 'actual-existing-dev-backup'}
            returned_key, members = validate_backup_inputs(admission, manifest, report, 'op', root)
            self.assertEqual((returned_key, set(members)), (key, set(files)))
            (root / 'auth_db.enc').write_bytes(b'tampered')
            with self.assertRaises(ValueError):
                validate_backup_inputs(admission, manifest, report, 'op', root)
            bad = dict(report, scope='disposable-fixture-only')
            with self.assertRaises(ValueError):
                validate_backup_inputs(admission, manifest, bad, 'op', root)
            (root / 'backup.key').unlink()
            os.mkfifo(root / 'backup.key', 0o600)
            def check_fifo():
                try:
                    validate_backup_inputs(admission, manifest, report, 'op', root)
                except Exception:
                    return
                os._exit(2)
            process = multiprocessing.get_context('fork').Process(target=check_fifo)
            process.start(); process.join(3)
            if process.is_alive():
                process.terminate(); process.join()
                self.fail('FIFO read blocked before private-file check')
            self.assertEqual(process.exitcode, 0)

    def test_catalog_resolves_git_tree_not_mutable_files(self):
        source = pathlib.Path(__file__).resolve().parents[2] / 'src/backend/auth/src/main/resources/db/migration'
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            target = root / 'src/backend/auth/src/main/resources/db/migration'
            target.mkdir(parents=True)
            for path in source.glob('*.sql'):
                (target / path.name).write_bytes(path.read_bytes())
            def git(*args):
                return subprocess.run(['git', '-C', str(root), *args], check=True, capture_output=True).stdout.decode().strip()
            git('init', '-q')
            git('config', 'user.email', 'fixture@example.invalid')
            git('config', 'user.name', 'fixture')
            git('add', '.')
            git('commit', '-qm', 'fixture')
            commit = git('rev-parse', 'HEAD')
            _, catalog = source_catalog(root, commit)
            (target / 'V28__role_identity_owner.sql').write_text('tampered')
            _, again = source_catalog(root, commit)
            self.assertEqual(catalog, again)
            git('add', '.')
            git('commit', '-qm', 'change transition')
            with self.assertRaises(ValueError):
                source_catalog(root, git('rev-parse', 'HEAD'))

    def test_exact_protected_admission(self):
        source = 'a' * 40
        admission = {'source_sha': source, 'target': 32, 'flyway_enabled': False,
                     'selector_mode': 'absent', 'persistence': 'jdbc', 'plan_sha256': 'b' * 64,
                     'operator_sha256': 'c' * 64, 'sources_sha256': 'd' * 64}
        validate_admission(admission, source)
        for key, value in [('source_sha', 'c' * 40), ('target', 31),
                           ('flyway_enabled', True), ('selector_mode', 'active'), ('persistence', 'memory'),
                           ('plan_sha256', '')]:
            changed = dict(admission, **{key: value})
            with self.subTest(key=key), self.assertRaises(ValueError):
                validate_admission(changed, source)

    def test_history_is_exact_prefix_and_selected_append(self):
        before = [{'version': '26', 'script': 'V26__authority_source_revision.sql',
                   'checksum': 123, 'success': True, 'type': 'SQL'}]
        after = before + [{'version': str(n), 'script': f'V{n}__x.sql',
                           'checksum': n, 'success': True, 'type': 'SQL'} for n in (28, 31, 32)]
        validate_history(before, after)
        for broken in (after[:-1], [dict(before[0], checksum=124)] + after[1:],
                       before + [dict(after[1], success=False)] + after[2:]):
            with self.assertRaises(ValueError):
                validate_history(before, broken)

    def test_marker_changes_must_match_v28_and_revision(self):
        before = {'accounts': [{'id': '1', 'status': 'active', 'deleted_at': '2020', 'security_revision': 2},
                               {'id': '2', 'status': 'active', 'deleted_at': None, 'security_revision': 4}],
                  'auth_authority_revision': [{'singleton': True, 'revision': 7}],
                  'other': [{'id': 'x', 'value': 3}]}
        after = copy.deepcopy(before)
        after['accounts'][0]['status'] = 'deleted'
        after['accounts'][0]['security_revision'] = 3
        after['auth_authority_revision'][0]['revision'] = 8
        validate_markers(before, after)
        for table in ('accounts', 'other', 'auth_authority_revision'):
            broken = copy.deepcopy(after)
            broken[table].clear()
            with self.subTest(table=table), self.assertRaises(ValueError):
                validate_markers(before, broken)


if __name__ == '__main__':
    unittest.main()
