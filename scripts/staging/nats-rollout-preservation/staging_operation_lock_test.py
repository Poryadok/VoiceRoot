import tempfile
import unittest
import json
from types import SimpleNamespace
from pathlib import Path
from unittest.mock import patch

import fcntl
import root_main
import resource_claim
import resource_claim_test
import guard


class OperationLockProofTest(unittest.TestCase):
    def test_proof_tracks_actual_lock_and_scope(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(root_main, 'ROOT', Path(directory)):
            with root_main.operation_lock() as proof:
                self.assertIs(type(proof), root_main.OperationLockProof)
                proof.check()
                with self.assertRaisesRegex(root_main.Blocked, 'operation_already_running'):
                    with root_main.operation_lock():
                        self.fail('concurrent lock acquired')
                fcntl.flock(proof._fd, fcntl.LOCK_UN)
                with self.assertRaisesRegex(root_main.Blocked, 'operation_lock_proof_invalid'):
                    proof.check()
            with self.assertRaisesRegex(root_main.Blocked, 'operation_lock_proof_invalid'):
                proof.check()

    def test_replaced_lock_path_invalidates_proof(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(root_main, 'ROOT', Path(directory)):
            with root_main.operation_lock() as proof:
                path = Path(directory) / 'known-baseline.lock'
                path.rename(path.with_name('old.lock'))
                path.touch(mode=0o600)
                with self.assertRaisesRegex(root_main.Blocked, 'operation_lock_proof_invalid'):
                    proof.check()

    def test_real_root_scope_binds_live_proof_and_journal(self):
        context = resource_claim_test.context()
        helper = {'fixture': 'b' * 64}
        request = {'action': 'authorize', 'operation': 'a' * 12, 'nonce': 'e' * 64,
                   'resource_context': context}
        with tempfile.TemporaryDirectory(dir=Path(__file__).resolve().parents[3]) as directory:
            root = Path(directory)
            (root / 'installed').mkdir(mode=0o700)
            journal = root / 'installed' / resource_claim.JOURNAL
            journal.write_text(json.dumps({'schema': 'voice-staging-resource-fence-v1',
                'context': context, 'operation': 'a' * 12, 'helper_sha256': resource_claim.digest(helper),
                'status': 'active', 'phase': 'apply-ready', 'target_sha256': 'd' * 64}), encoding='utf8')
            journal.chmod(0o600)
            with patch.object(root_main, 'ROOT', root), patch.object(guard, 'ROOT', root), \
                 patch.object(resource_claim, 'require_helper'), \
                 patch.object(resource_claim.Check, '__call__', return_value=resource_claim_test.card(context)):
                with root_main.operation_lock() as proof:
                    actions = SimpleNamespace(binding=helper, _operation_lock=proof, tt_session=None)
                    with resource_claim.root_scope(actions, request) as session:
                        session.admit_request(request)
                        self.assertIs(type(session.journal), resource_claim.RootJournal)
                        self.assertIs(session.operation_lock, proof)
                        self.assertEqual(session.check()['phase'], 'apply-ready')
                        proof.check()
                with self.assertRaisesRegex(root_main.Blocked, 'operation_lock_proof_invalid'):
                    session.operation_lock.check()


if __name__ == '__main__':
    unittest.main()
