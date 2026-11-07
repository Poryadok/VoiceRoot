"""Route guard continuity; external fence and Kubernetes observations are synthetic."""
import copy
from types import SimpleNamespace
import unittest

import full_cohort_backup_phase as backup


class RouteGuardTest(unittest.TestCase):
    def test_lost_route_or_local_marker_denied(self):
        route={'version':'full-cohort-backup-route-v1','binding_sha256':'a'*64}
        row={'full_cohort_backup_route':copy.deepcopy(route)};revision=('uid','rv')
        ledger=SimpleNamespace(read=lambda:(copy.deepcopy(row),revision));context={'operation_id':'op'}
        fence=SimpleNamespace(context=context,ledger=ledger);lock={'fixture':True}
        def dep():return SimpleNamespace(fence=fence,lock=lock,namespace_uid='ns')
        package,inventory,catalog,census,cut,physical,metadata=[dep() for _ in range(7)]
        cut.verify_drained=lambda:None
        supervisor=SimpleNamespace(package=package,inventory=inventory,catalog=catalog,census=census,
            fence=fence,lock=lock,namespace_uid='ns',intent={'cohort_route_mode':'full-cohort-v1'},_guard=lambda:None)
        phase=object.__new__(backup.BackupPhase)
        phase.supervisor=supervisor;phase.cut=cut;phase.physical=physical;phase.metadata=metadata
        phase._dependencies=(supervisor,cut,physical,metadata,package,inventory,catalog,census)
        phase.journal=SimpleNamespace(state={'steps':{'full-cohort-backup-route':{'status':'done','route':copy.deepcopy(route)}}})
        phase._bound_journal=phase.journal;phase.cohort=phase._bound_cohort=None;phase._route=route
        phase.auth_preparation=phase._auth_preparation=None
        self.assertEqual(phase._guard(),(row,revision))
        phase.auth_preparation=object()
        with self.assertRaises(backup.BackupError):phase._guard()
        phase.auth_preparation=None
        del row['full_cohort_backup_route']
        with self.assertRaises(backup.BackupError):phase._guard()
        row['full_cohort_backup_route']=copy.deepcopy(route)
        phase.journal.state['steps'].clear()
        with self.assertRaises(backup.BackupError):phase._guard()


if __name__=='__main__':unittest.main()
