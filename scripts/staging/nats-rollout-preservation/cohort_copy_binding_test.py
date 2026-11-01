import copy
import unittest
from unittest.mock import patch

import resource_claim
import staging_cohort_preservation as cohort


class CohortCopyBindingTests(unittest.TestCase):
    def test_offnode_reader_accepts_bound_legacy_and_scoped_production_records(self):
        installation = resource_claim.production_installation()
        operation = '123456abcdef'
        cut = {'manifest': {'archive_sha256': 'a' * 64},
               'manifest_sha256': 'b' * 64, 'census_sha256': 'c' * 64}
        plan = {'plan': 'fixture'}
        archive = {'manifest': {'fixture': True}}
        binding = {'operation': operation, 'challenge': 'd'*32, 'run_id': 1,
                   'head_sha': 'f'*40, 'cipher_sha256': '1'*64, 'cipher_bytes': 10}
        custody = {'schema': 'voice-nats-custody-v1', 'destination': 'github:Poryadok/VoiceRoot',
                   **binding, 'artifact_id': 1, 'verified': True}
        resources = {'resources': {'one': {'status': 'REMOVED'}}}
        restores = {}
        for phase, nonce in (('local', '2'*32), ('offnode', '3'*32)):
            restores[phase] = {'nonce': nonce, 'phase': phase, 'status': 'COMPLETE',
                'plan_sha256': cohort.digest(plan), 'resources_sha256': cohort.digest(resources),
                'proof': {'cleanup': 'verified', 'manifest_sha256': cohort.digest(archive['manifest'])}}
        restored = {'schema': 'voice-nats-decrypted-restore-v1', 'cipher_sha256': binding['cipher_sha256'],
            'cipher_bytes': 10, 'decrypted_members_verified': True,
            'remote_equivalence': 'FULL_CIPHERTEXT_SHA256_AND_SIZE', 'members': {'copy-checkpoint.json': '4'*64},
            'cohort_readback': {'schema': 'voice-staging-cohort-offnode-restore-v1', 'operation': operation,
                'restore_sha256': cohort.digest(restores['offnode']), 'manifest_sha256': cohort.digest(archive['manifest']),
                'cipher_members_verified': True, 'cleanup': 'verified'}}
        for scoped in (False, True):
            copy_row = cohort._copy_checkpoint(operation, cut, installation, scoped)
            rows = {'cipher-binding.json': binding, 'offnode-custody.json': custody,
                'decrypted-restore.json': restored, 'archive-record.json': archive,
                'local-restore.json': restores['local'], 'offnode-restore.json': restores['offnode'],
                'native-cut.json': cut, 'copy-checkpoint.json': copy_row, 'resources.json': resources}
            def read(path):
                if path.name == 'receipt.json':
                    return restores['local' if path.parent.name == '2'*32 else 'offnode']
                return rows[path.name]
            def identity(path):
                if path.name == 'copy-checkpoint.json': return {'sha256': '4'*64}
                return {'sha256': binding['cipher_sha256'], 'bytes': binding['cipher_bytes']}
            with self.subTest(scoped=scoped), patch.object(cohort, '_read', side_effect=read), \
                 patch.object(cohort, '_file_identity', side_effect=identity):
                self.assertEqual(cohort._offnode_records_data(operation, {'commit': 'f'*40}, plan,
                    installation=installation, scoped=scoped), custody)
                copy_row['schema'] = 'nats-rollout-copy-v1' if scoped else 'nats-rollout-copy-v2'
                with self.assertRaisesRegex(cohort.Blocked, 'cohort_copy_checkpoint_changed'):
                    cohort._offnode_records_data(operation, {'commit': 'f'*40}, plan,
                        installation=installation, scoped=scoped)

    def test_offnode_rejects_authenticated_member_that_differs_from_local_copy(self):
        installation = resource_claim.production_installation()
        cut = {'manifest': {'archive_sha256': 'a' * 64},
               'manifest_sha256': 'b' * 64, 'census_sha256': 'c' * 64}
        copy_row = cohort._copy_checkpoint('123456abcdef', cut, installation, True)
        rows = {'cipher-binding.json': {}, 'offnode-custody.json': {},
                'decrypted-restore.json': {'members': {'copy-checkpoint.json': 'd' * 64}},
                'archive-record.json': {}, 'offnode-restore.json': {}, 'local-restore.json': {},
                'copy-checkpoint.json': copy_row, 'native-cut.json': cut}
        with patch.object(cohort, '_read', side_effect=lambda path: rows[path.name]), \
             patch.object(cohort, '_file_identity', return_value={'sha256': 'e' * 64}):
            with self.assertRaisesRegex(cohort.Blocked, 'cohort_copy_checkpoint_changed'):
                cohort._offnode_records_data('123456abcdef', {}, {}, installation=installation, scoped=True)

    def test_historical_copy_is_checked_before_either_profile_is_accepted(self):
        installation = resource_claim.production_installation()
        operation = '123456abcdef'
        plan = {'profile': 'test'}
        binding = {'schema': 'voice-staging-resource-fence-v2', 'context': {'id': 'context'}}
        rollout = {'status': 'PASS', 'phase': 'POST_RESTART', 'fence_status': 'RELEASED', 'operation': operation}
        for profile in ('ordinary', 'target8'):
            state = {'plan': plan, 'profile': profile}
            with self.subTest(profile=profile), \
                 patch.object(cohort, '_read', side_effect=[state, {}, rollout]), \
                 patch.object(resource_claim, 'historical_binding', return_value=binding), \
                 patch.object(cohort, '_offnode_records_data', side_effect=cohort.Blocked('copy_refused')) as check:
                with self.assertRaisesRegex(cohort.Blocked, 'copy_refused'):
                    cohort.validate_historical_capture(operation, installation=installation)
                check.assert_called_once_with(operation, binding['context'], plan,
                    installation=installation, scoped=True)

    def test_exact_checkpoint_schema_domain_and_cut(self):
        installation = resource_claim.production_installation()
        cut = {'manifest': {'archive_sha256': 'a' * 64},
               'manifest_sha256': 'b' * 64, 'census_sha256': 'c' * 64}
        for scoped in (False, True):
            expected = cohort._copy_checkpoint('123456abcdef', cut, installation, scoped)
            cohort._validate_copy_checkpoint(expected, '123456abcdef', cut, installation, scoped)
            for mutation in ('schema', 'operation', 'archive_sha256', 'installation_sha256', 'extra'):
                changed = copy.deepcopy(expected)
                if mutation == 'extra': changed['extra'] = True
                else: changed[mutation] = 'f' * 64
                with self.subTest(scoped=scoped, mutation=mutation), self.assertRaisesRegex(cohort.Blocked, 'cohort_copy_checkpoint_changed'):
                    cohort._validate_copy_checkpoint(changed, '123456abcdef', cut, installation, scoped)
            mixed = cohort._copy_checkpoint('123456abcdef', cut, installation, not scoped)
            with self.assertRaisesRegex(cohort.Blocked, 'cohort_copy_checkpoint_changed'):
                cohort._validate_copy_checkpoint(mixed, '123456abcdef', cut, installation, scoped)
        owned = resource_claim.InstallationAuthority('owned-isolated-v1',
            '11111111-1111-4111-8111-111111111111', 'r20260930a4', 'a'*64, 'b'*64, 'c'*64)
        scoped = cohort._copy_checkpoint('123456abcdef', cut, owned, True)
        cohort._validate_copy_checkpoint(scoped, '123456abcdef', cut, owned, True)
        with self.assertRaisesRegex(cohort.Blocked, 'cohort_copy_checkpoint_changed'):
            cohort._validate_copy_checkpoint(scoped, '123456abcdef', cut, installation, True)
        with self.assertRaises(RuntimeError):
            cohort._copy_checkpoint('123456abcdef', cut, owned, False)


if __name__ == '__main__':
    unittest.main()
