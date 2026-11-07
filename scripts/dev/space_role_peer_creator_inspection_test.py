"""Actual typed Creator and preflight/read path; Kubernetes and package are synthetic."""
import copy
import unittest
from unittest import mock

import dev_peer_issuer_metadata as metadata
import full_cohort_package_phase as package
import reset_credentials as reset
import space_creator_runtime_profile_delegated_test as fixture
import space_role_peer_create as role


class CreatorInspectionTest(unittest.TestCase):
    def test_creator_preflight_reads_existing_role_without_consuming_role_slot(self):
        case=fixture.PhaseTest('test_predrain_existing_observation_has_no_effect_or_drain_dependency'); case.profile='creator'
        def cleanup():
            if not case.doCleanups(): raise RuntimeError('nested cleanup failed')
        self.addCleanup(cleanup); case.setUp()
        case.package.journal.state['steps'][package.STEP]={'status':'done','receipt':{'scope':'synthetic'}}
        signer=reset.ResetMaterial(case.root); signer.signer('voice-role-principal-signing',active=True)
        obj=copy.deepcopy(signer.items[0]); name=obj['metadata']['name']; obj['metadata'].update(uid='role-uid',resourceVersion='7')
        case.actual[name]=obj; case.identities[name]={'state':'present','uid':'role-uid','resourceVersion':'7','keys':sorted(metadata.MEMBERS[name])}
        case.initial['creator_secrets']=copy.deepcopy(case.identities); case.preflight.record['creator_secrets']=copy.deepcopy(case.identities)
        case.preflight.record['receipt_sha256']=role.writer_cut.digest(case.initial)
        case.preflight.journal.state['steps'][case.preflight.step]=copy.deepcopy(case.preflight.record)|{'receipt':copy.deepcopy(case.initial)}
        case.base.ledger.row[case.preflight.ledger]=copy.deepcopy(case.preflight.record); before=case.base.ledger.read()
        with mock.patch.object(role,'_get',side_effect=lambda item:copy.deepcopy(case.actual.get(item))), \
             mock.patch.object(case.base.cut,'verify_drained',side_effect=AssertionError('inspection must not drain')), \
             mock.patch.object(role,'_create') as write:
            result=role.inspect_role_from_creator(case.phase,'token')
            self.assertEqual(result['profile'],'role'); self.assertEqual(set(result['present']),{name})
            self.assertEqual(result['missing'],['voice-role-media-client-tls']); write.assert_not_called()
            self.assertEqual(case.base.ledger.read(),before)
            case.identities[name]['resourceVersion']='8'
            with self.assertRaises(role.CreateError):role.inspect_role_from_creator(case.phase,'token')


if __name__=='__main__': unittest.main()
