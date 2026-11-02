import unittest
from pathlib import Path
from unittest.mock import Mock, patch

import bridge_root
import resource_claim as claim
from resource_claim_test import MemoryJournal, card, context, scoped_target


class BridgeAuthorizationScopeTests(unittest.TestCase):
    def test_foreign_domain_is_rejected_before_custody_or_expired_admission(self):
        c=context();helper={'nats-rollout-preservation/resource_claim.py':'b'*64}
        session=claim.Session(c,'a'*12,claim.digest(helper),lambda:card(c),MemoryJournal())
        session.admit(start=True)
        foreign=claim.InstallationAuthority('owned-isolated-v1','11111111-1111-4111-8111-111111111111',
            'r20260930a4','a'*64,'b'*64,'c'*64)
        other=claim.Session(c,session.operation,claim.digest(helper),lambda:card(c),MemoryJournal(),installation=foreign)
        other.admit(start=True)
        target=scoped_target(c);target['installation_sha256']=foreign.sha256
        other.journal.row['target_sha256']=claim.digest(target)
        actions=object.__new__(bridge_root.Actions);actions.tt_session=session
        state={'operation':session.operation,'schema':'nats-rollout-root-v2',
               'installation_sha256':foreign.sha256,'resource_fence':other.binding(),
               'code_capture':helper,'target':target}
        claim.historical_binding(state,installation=foreign)
        actions.state=Mock(return_value=(Path('/private/operation'),state))
        with patch('github_custody.verify_artifact') as custody, \
             patch('preserved_upload.verify_artifact') as preserved, \
             patch('expired_transport.commit_authority') as commit, \
             patch.object(actions,'_expired_source_admission') as source:
            for action in ('authorize','authorize-preserved-upload'):
                with self.subTest(action=action), self.assertRaisesRegex(RuntimeError,'staging_installation_binding_changed'):
                    actions.execute({'action':action,'operation':session.operation})
            with self.assertRaisesRegex(RuntimeError,'staging_installation_binding_changed'):
                actions._authorize_expired_native({'operation':session.operation})
            custody.assert_not_called();preserved.assert_not_called();commit.assert_not_called();source.assert_not_called()

    def test_foreign_operation_is_rejected_before_custody_or_source_admission(self):
        c=context();session=claim.Session(c,'a'*12,'b'*64,lambda:card(c),MemoryJournal())
        session.admit(start=True)
        actions=object.__new__(bridge_root.Actions);actions.tt_session=session
        foreign={'operation':'c'*12,'target':{'mode':'images-only'}}
        actions.state=Mock(return_value=(Path('/private/foreign'),foreign))
        with patch('github_custody.verify_artifact') as custody, \
             patch.object(actions,'_expired_source_admission') as source:
            for action in ('authorize','authorize-preserved-upload'):
                with self.subTest(action=action), self.assertRaisesRegex(bridge_root.Blocked,'staging_authorization_operation_changed'):
                    actions.execute({'action':action,'operation':'c'*12})
            with self.assertRaisesRegex(bridge_root.Blocked,'staging_authorization_operation_changed'):
                actions._authorize_expired_native({'operation':'c'*12})
            custody.assert_not_called();source.assert_not_called()


if __name__=='__main__':unittest.main()
