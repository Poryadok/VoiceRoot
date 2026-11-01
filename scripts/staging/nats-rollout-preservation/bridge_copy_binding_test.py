import copy
import unittest
from pathlib import Path
from unittest.mock import Mock, patch

import bridge_root
import root_cli
import resource_claim as claim
from resource_claim_test import MemoryJournal, card, context


class BridgeCopyBindingTests(unittest.TestCase):
    def test_scoped_parent_and_copy_are_bound_before_decryption(self):
        installation = claim.production_installation()
        c = context(); helper = {'nats-rollout-preservation/resource_claim.py': 'b'*64}
        session = claim.Session(c, 'a'*12, claim.digest(helper), lambda: card(c), MemoryJournal())
        session.admit(start=True)
        target = {'registry':'registry', 'tag':c['commit'], 'mode':'full', 'changed_services':[],
            'source_hashes':{'source':'1'*64}, 'template_hashes':{'voice-user':'2'*64},
            'input_template_hashes':{'voice-user':'3'*64}, 'images':c['images'],
            'manifest_sha256':'4'*64, 'migration_sha256':'5'*64, 'nonnats_sha256':'6'*64,
            'installation_sha256':installation.sha256}
        session.journal.row.update(target_sha256=claim.digest(target))
        state = {'schema':'nats-rollout-root-v2', 'installation_sha256':installation.sha256,
            'operation':'a'*12, 'resource_fence':session.binding(), 'code_capture':helper, 'target':target}
        cut = {'manifest':{'archive_sha256':'a'*64}, 'manifest_sha256':'b'*64, 'census_sha256':'c'*64}
        expected = bridge_root._copy_checkpoint(state, cut, installation)
        self.assertEqual(expected['schema'], 'nats-rollout-copy-v2')
        self.assertEqual(expected['installation_sha256'], installation.sha256)
        changed = copy.deepcopy(state); changed['target']['manifest_sha256'] = 'e'*64
        with self.assertRaises(RuntimeError): bridge_root._copy_checkpoint(changed, cut, installation)
        changed = copy.deepcopy(expected); changed['installation_sha256'] = 'f'*64
        with self.assertRaisesRegex(bridge_root.Blocked, 'rollout_copy_checkpoint_changed'):
            bridge_root._validate_copy_checkpoint(changed, state, cut, installation)
        actions=object.__new__(bridge_root.Actions);actions._installation=installation
        state['cut']=cut;state['cipher_binding']={}
        with patch.object(bridge_root, 'private_json', return_value=changed), \
             patch('encrypted_cut.decrypt_verify') as decrypt:
            with self.assertRaisesRegex(bridge_root.Blocked, 'rollout_copy_checkpoint_changed'):
                actions._verify_decrypted_restore(Path('/private/operation'), state, Mock())
            decrypt.assert_not_called()
        state['phase']='FENCED';state['fence_status']='VERIFIED'
        state['cut']['census']={'streams': [], 'consumers': []}
        self.assertEqual(root_cli._copy_record(state)['installation_sha256'],installation.sha256)
        foreign=copy.deepcopy(state);foreign['installation_sha256']='f'*64
        with self.assertRaises(RuntimeError):root_cli._copy_record(foreign)
        legacy={'schema':'nats-rollout-root-v1','operation':'a'*12,'target':{},'cut':state['cut'],
                'phase':'FENCED','fence_status':'VERIFIED'}
        self.assertEqual(root_cli._copy_record(legacy)['schema'],'nats-rollout-copy-v1')
        legacy['installation_sha256']=installation.sha256
        with self.assertRaisesRegex(bridge_root.Blocked,'rollout_copy_checkpoint_changed'):
            root_cli._copy_record(legacy)


if __name__ == '__main__':
    unittest.main()
