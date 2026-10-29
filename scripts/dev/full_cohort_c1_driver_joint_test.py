"""Actual C1 driver authority checks with synthetic external TT/Kubernetes."""
import copy
import unittest
from unittest.mock import patch

import full_cohort_backup_phase_joint_test as fixture
import full_cohort_c1_driver as driver
import full_cohort_supervisor as supervisor


def checked_nested(case,created):
    ok=case.doCleanups()
    for base in created:
        try:base.doCleanups()
        except BaseException:ok=False
    if not ok:raise RuntimeError('nested fixture cleanup failed')


def checked_base(base):
    cleanup=base.doCleanups
    def checked():
        if not cleanup():raise RuntimeError('base fixture cleanup failed')
    base.doCleanups=checked
    return base


class DriverJointTest(unittest.TestCase):
    def test_changed_tt_generation_refuses_before_package_effect(self):
        """Exercise the real driver, supervisor, journals and fence without a live claim."""
        class StopFixture(Exception):pass
        case=fixture.RealPreflightTest('test_pre_supervisor_hook_uses_the_real_root_facade')
        created=[]; self.addCleanup(checked_nested,case,created)
        original=fixture.writer_cut_test.WriterCutTest
        def make(*args):
            base=checked_base(original(*args)); created.append(base); return base
        observed=[]
        def before(work):
            fixture.fixture_begin_image_only(work)
            owner=supervisor.Supervisor(work.journal,work.fence,work.client,work.base.lock,work.namespace_uid)
            owner._guard(); initial=copy.deepcopy(work.base.ledger.row); work.base.card['generation']+=1
            nodes=[{'name':'node','uid':'uid','hostname':'node','architecture':'amd64'}]
            with self.assertRaises(driver.DriverError):driver.C1Driver(owner).prepare('token','actor',1,1,nodes)
            self.assertIsNone(owner.package)
            self.assertFalse((work.operation/'package-journal').exists())
            self.assertNotIn('full_cohort_package',work.base.ledger.row)
            self.assertEqual(work.base.ledger.row,initial); observed.append(True)
            raise StopFixture()
        with patch.object(fixture.writer_cut_test,'WriterCutTest',side_effect=make):
            with self.assertRaises(StopFixture):
                with case.phases(before_supervisor=before):pass
        self.assertEqual(observed,[True])

    def test_nested_cleanup_failure_is_reported_and_remaining_cleanup_runs(self):
        outer=unittest.TestCase(); base=checked_base(unittest.TestCase()); events=[]
        base.addCleanup(lambda:events.append('later'))
        def fail():raise RuntimeError('synthetic cleanup failure')
        base.addCleanup(fail); outer.addCleanup(base.doCleanups)
        with self.assertRaises(RuntimeError):checked_nested(outer,[base])
        self.assertEqual(events,['later'])


if __name__=='__main__':unittest.main()
