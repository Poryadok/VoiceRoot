import copy
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock,patch

import actor_auth
import bootstrap_auth
import commands
import guard
import resource_claim
import resource_claim_test
import staging_cohort_preservation as cohort


class AuthRoutingTest(unittest.TestCase):
    def test_uncertain_cohort_create_retains_mounted_credentials(self):
        installation=resource_claim.InstallationAuthority('owned-isolated-v1','734fe817-dc75-4732-89ac-6aecb83f0a32',
            'r20260930a4',bootstrap_auth.hash_bytes(b'fixture'),'b'*64,'c'*64)
        context=resource_claim_test.context();card=resource_claim_test.card(context);journal=resource_claim_test.MemoryJournal()
        session=resource_claim.Session(context,'a'*12,'b'*64,lambda:copy.deepcopy(card),journal,installation=installation)
        session.admit(start=True);journal.row.update(phase='cohort-preservation',target_sha256='c'*64,cohort_preservation_sha256='c'*64)
        for module,kind in ((actor_auth,'actor'),(bootstrap_auth,'bootstrap')):
            with self.subTest(kind=kind),tempfile.TemporaryDirectory(dir='/root') as folder:
                root=Path(folder);binary=root/'binary';binary.write_bytes(b'fixture')
                runtime=Mock();runtime.owned={};runtime.create.side_effect=RuntimeError('lost create reply')
                runtime.start_broker.side_effect=RuntimeError('lost create reply')
                runtime.finish_auth.side_effect=RuntimeError('pending cohort create')
                with patch.object(guard,'ROOT',root),patch.object(commands,'authority_guard',session.check), \
                     patch.object(cohort,'auth_runtime',return_value=runtime,create=True), \
                     patch.object(module,'secret_bytes',return_value=b'fixture'),patch.object(module.os,'chown'):
                    with self.assertRaisesRegex(RuntimeError,'pending cohort create'):
                        if kind=='actor':module.prove(b'fixture',{},binary,lambda:None)
                        else:module.prove(b'fixture',{},lambda:None,installation=installation)
                dirs=[path for path in root.iterdir() if path.is_dir()];self.assertEqual(len(dirs),1)
                name='actor.creds' if kind=='actor' else 'bootstrap.creds'
                self.assertEqual((dirs[0]/'inputs'/name).read_bytes(),b'fixture')
                runtime.finish_auth.assert_called_once_with()

    def test_fixed_runtime_is_finalized_when_scratch_creation_fails(self):
        context=resource_claim_test.context();card=resource_claim_test.card(context);journal=resource_claim_test.MemoryJournal()
        installation=resource_claim.InstallationAuthority('owned-isolated-v1','734fe817-dc75-4732-89ac-6aecb83f0a32',
            'r20260930a4',bootstrap_auth.hash_bytes(b'fixture'),'b'*64,'c'*64)
        session=resource_claim.Session(context,'a'*12,'b'*64,lambda:copy.deepcopy(card),journal,installation=installation)
        session.admit(start=True);journal.row.update(phase='cohort-preservation',target_sha256='c'*64,cohort_preservation_sha256='c'*64)
        for module,kind in ((actor_auth,'actor'),(bootstrap_auth,'bootstrap')):
            with self.subTest(kind=kind),tempfile.TemporaryDirectory(dir='/root') as folder:
                root=Path(folder);binary=root/'binary';binary.write_bytes(b'fixture')
                runtime=Mock();runtime.owned={};runtime.create.side_effect=RuntimeError('controlled create failure')
                runtime.start_broker.side_effect=RuntimeError('controlled create failure')
                with patch.object(guard,'ROOT',root),patch.object(commands,'authority_guard',session.check), \
                     patch.object(cohort,'auth_runtime',return_value=runtime,create=True) as factory, \
                     patch.object(module,'secret_bytes',return_value=b'fixture'),patch.object(module.os,'chown'):
                    with self.assertRaisesRegex(RuntimeError,'controlled create failure'):
                        if kind=='actor':module.prove(b'fixture',{},binary,lambda:None)
                        else:module.prove(b'fixture',{},lambda:None,installation=installation)
                self.assertEqual(factory.call_args.args[0],session);self.assertEqual(factory.call_args.args[2],kind)
                runtime.finish_auth.assert_called_once_with()


if __name__=='__main__':unittest.main()
