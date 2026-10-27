import copy
import unittest
import hashlib
import hmac
import json
import pathlib
import tempfile
from types import SimpleNamespace
from unittest.mock import patch
from auth_user_adoption_restore import qualify
from auth_user_adoption import RecoveryRequired
from auth_user_adoption_backup import validate_restore


class RestoreQualificationTest(unittest.TestCase):
    def setUp(self):
        self.manifest = {'format':1, 'operation':'owned', 'files':{'auth_db.enc':{'sha256':'a'*64,'hmac':'b'*64},
                        'user_db.enc':{'sha256':'c'*64,'hmac':'d'*64}, 'roles.enc':{'sha256':'e'*64,'hmac':'f'*64}},
                         'catalog':{'auth_db':{'owner':'voice','tables':{'accounts':'voice'}},'user_db':{'owner':'voice','tables':{'profiles':'voice'}}}}
        self.report = {'protocol':'voice-auth-user-backup-v1', 'operation':'owned', 'result':'PASS',
                       'files':copy.deepcopy(self.manifest['files']), 'restored_catalog':copy.deepcopy(self.manifest['catalog']),
                       'off_node':True, 'restore_exit':{'auth_db':0,'user_db':0,'roles':0},
                       'postgres_major':16,'verifier_sha256':'1'*64,'tools_image':'sha256:'+'2'*64,
                       'fixture_cleanup':True}

    def test_correct_bound_restore_is_eligible(self):
        self.assertTrue(validate_restore(self.manifest,self.report,'owned'))

    def test_partial_stale_foreign_or_unrestored_report_cannot_authorize_migration(self):
        for fault in ['operation','files','restored_catalog','off_node','result','restore_exit','postgres_major','verifier_sha256','fixture_cleanup']:
            report=copy.deepcopy(self.report)
            report[fault] = False
            with self.subTest(fault=fault), self.assertRaises(RecoveryRequired):
                validate_restore(self.manifest,report,'owned')

    def test_restore_cleanup_never_targets_replacement_name_or_unverified_owner(self):
        for owner in ('foreign', 'Dimas-16'):
            with self.subTest(owner=owner), tempfile.TemporaryDirectory() as folder:
                directory=pathlib.Path(folder); key='a'*64; image='sha256:'+'b'*64
                raw=b'encrypted'; files={}
                for member in ('roles.enc','auth_db.enc','user_db.enc'):
                    (directory/member).write_bytes(raw)
                    files[member]={'sha256':hashlib.sha256(raw).hexdigest(),
                        'hmac':hmac.new(bytes.fromhex(key),raw,hashlib.sha256).hexdigest()}
                (directory/'backup.key').write_text(key)
                (directory/'backup-manifest.json').write_text(json.dumps({
                    'format':1,'operation':'owned','origin_host':'other-host', 'files':files}))
                created='c'*64; foreign='d'*64; calls=[]
                def run(command,**kwargs):
                    calls.append(command)
                    args=command[1:]; stdout=b''; code=0
                    if args[0]=='info': stdout=b'linux'
                    elif args[:2]==['image','inspect']: stdout=json.dumps([{'Id':image}]).encode()
                    elif args[:2]==['run','-d']: stdout=created.encode()+b'\n'
                    elif args[0]=='run': stdout=b'CREATE ROLE voice;'
                    elif args[0]=='inspect':
                        # The mutable name already belongs to someone else.
                        identity=created if args[1]==created else foreign
                        stdout=json.dumps([{'Id':identity,'Config':{'Labels':{'voice.test.owner':owner}},
                            'Mounts':[{'Type':'volume','Name':'private-volume'}]}]).encode()
                    elif args[0]=='exec': code=1
                    return SimpleNamespace(returncode=code,stdout=stdout,stderr=b'')
                with patch('auth_user_adoption_restore.subprocess.run',side_effect=run), \
                     patch('auth_user_adoption_restore.time.sleep'):
                    with self.assertRaises((RuntimeError,AssertionError)):
                        qualify(directory,image,'owned',directory)
                removed=[c[-1] for c in calls if c[1]=='rm']
                self.assertEqual(removed,[] if owner=='foreign' else [created])
                self.assertTrue(all(c[2]==created for c in calls if c[1]=='inspect'))
                if owner=='foreign':
                    recovery=list(directory.glob('restore-recovery-*.json'))
                    self.assertEqual(len(recovery),1)
                    receipt=json.loads(recovery[0].read_text())
                    self.assertEqual(receipt['container_id'],created)
                    self.assertFalse(receipt['cleanup_authorized'])
